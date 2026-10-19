package main

import (
	"crypto/sha1"
	"encoding/hex"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Baku is UTC+4 all year.
var baku = time.FixedZone("AZT", 4*60*60)

const dateLayout = "2006-01-02"

func dateOf(t time.Time) time.Time {
	t = t.In(baku)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, baku)
}

func parseDate(s string) (time.Time, error) { return time.ParseInLocation(dateLayout, s, baku) }

func daysBetween(a, b time.Time) int { return int(math.Round(b.Sub(a).Hours() / 24)) }

func shortHash(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:4])
}

// parseMinor turns "-42.50" into -4250.
func parseMinor(s string) int64 {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	whole, frac, _ := strings.Cut(s, ".")
	for len(frac) < 2 {
		frac += "0"
	}
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(frac[:2], 10, 64)
	v := w*100 + f
	if neg {
		v = -v
	}
	return v
}

// toAZN converts using CBAR's rate, held as a string like "1.7000".
func toAZN(minor int64, currency string, fx map[string]string) int64 {
	if currency == "AZN" || currency == "" {
		return minor
	}
	rate, err := strconv.ParseFloat(fx[currency], 64)
	if err != nil || rate == 0 {
		return minor
	}
	return int64(math.Round(float64(minor) * rate))
}

// txView is a stored transaction enriched by the engine.
type txView struct {
	*StoredTx
	date        time.Time // pending entries count as today
	merchant    string
	category    string
	internal    bool
	amountAzn   int64
	recurringID string
	loanID      string
}

func (t *txView) isExpense() bool {
	return t.AmountMinor < 0 && !t.internal && categoryByKey(t.category, "az").Kind == "expense"
}

func (t *txView) isIncome() bool {
	return t.AmountMinor > 0 && !t.internal && categoryByKey(t.category, "az").Kind == "income"
}

// discretionary is day-to-day spending: not a bill, subscription or instalment
// that safe-to-spend already set aside.
func (t *txView) discretionary() bool { return t.isExpense() && t.recurringID == "" }

type recurringItem struct {
	ID        string
	Type      string // income | expense
	Name      string
	Merchant  string
	Category  string
	Cadence   string // weekly | monthly | yearly
	Typical   int64  // signed, in AZN
	LastDate  string
	NextDate  time.Time
	Status    string // active | late | changed
	Source    string // detected | manual
	Confirmed bool
	AccountID string // account that pays it, from the latest hit
	FirstDate string
	Hits      int
}

func (r *recurringItem) step(d time.Time) time.Time {
	switch r.Cadence {
	case "weekly":
		return d.AddDate(0, 0, 7)
	case "yearly":
		return d.AddDate(1, 0, 0)
	}
	return d.AddDate(0, 1, 0)
}

// occurrences lists due dates in [from, to).
func (r *recurringItem) occurrences(from, to time.Time) []time.Time {
	var out []time.Time
	for d := r.NextDate; d.Before(to); d = r.step(d) {
		if !d.Before(from) {
			out = append(out, d)
		}
		if len(out) > 60 {
			break
		}
	}
	return out
}

type loanItem struct {
	ID              string
	Name            string
	Lender          string
	Source          string
	MonthlyPayment  int64
	NextPaymentDate string
	Remaining       *int64
	PaymentsSeen    int
	PayingAccountID string
}

// Analysis is everything derived from one user's data at one moment.
type Analysis struct {
	today     time.Time
	lang      string
	fx        map[string]string
	txs       []*txView // newest first
	txByID    map[string]*txView
	recurring []*recurringItem
	loans     []*loanItem
	firstDate time.Time // earliest booked transaction
}

var cardRef = regexp.MustCompile(`\*(\d{4})`)

func analyze(u *UserState, fx map[string]string, now time.Time, lang string) *Analysis {
	today := dateOf(now)
	a := &Analysis{today: today, lang: lang, fx: fx, txByID: map[string]*txView{}, firstDate: today}

	for _, st := range u.Txs {
		v := &txView{StoredTx: st, date: today}
		if st.Date != "" {
			if d, err := parseDate(st.Date); err == nil {
				v.date = d
				if d.Before(a.firstDate) {
					a.firstDate = d
				}
			}
		}
		v.merchant, v.category = categorize(st.Counterparty, st.Remittance, st.MCC, st.AmountMinor)
		v.amountAzn = toAZN(st.AmountMinor, st.Currency, fx)
		a.txs = append(a.txs, v)
		a.txByID[st.ID] = v
	}
	sort.SliceStable(a.txs, func(i, j int) bool {
		if a.txs[i].Status != a.txs[j].Status {
			return a.txs[i].Status == "pending" // pending first: it is the newest
		}
		if !a.txs[i].date.Equal(a.txs[j].date) {
			return a.txs[i].date.After(a.txs[j].date)
		}
		return a.txs[i].ID > a.txs[j].ID
	})

	markInternalTransfers(u, a.txs)

	// The user's own choices win over the rules.
	for _, v := range a.txs {
		if key, ok := u.MerchantCategory[v.merchant]; ok {
			v.category = key
		}
		if key, ok := u.TxCategory[v.ID]; ok {
			v.category = key
		}
		if v.category == "internal_transfer" {
			v.internal = true
		} else if _, overridden := u.TxCategory[v.ID]; overridden {
			v.internal = false
		}
	}

	a.detectRecurring(u)
	a.buildLoans(u)
	return a
}

// markInternalTransfers flags money moved between the user's own accounts, so it
// never counts as spending or income. Two ways: both legs are visible (same amount,
// opposite sign, different accounts, within 2 days), or the outgoing leg names one
// of the user's own cards ("TO CARD *7731").
func markInternalTransfers(u *UserState, txs []*txView) {
	ownCards := map[string]bool{}
	for _, acc := range u.Accounts {
		if n := len(acc.MaskedPAN); n >= 4 {
			ownCards[acc.MaskedPAN[n-4:]] = true
		}
	}
	isTransfer := func(t *txView) bool {
		up := strings.ToUpper(t.Remittance)
		return strings.Contains(up, "C2C") || strings.Contains(up, "TRANSFER")
	}
	used := map[*txView]bool{}
	for _, out := range txs {
		if out.AmountMinor >= 0 || out.Status != "booked" || !isTransfer(out) {
			continue
		}
		for _, in := range txs {
			if used[in] || in.AmountMinor != -out.AmountMinor || in.Currency != out.Currency ||
				in.AccountID == out.AccountID || in.Status != "booked" || !isTransfer(in) {
				continue
			}
			if d := daysBetween(out.date, in.date); d >= -2 && d <= 2 {
				out.internal, in.internal = true, true
				out.category, in.category = "internal_transfer", "internal_transfer"
				used[in], used[out] = true, true
				break
			}
		}
		if !out.internal {
			if m := cardRef.FindStringSubmatch(out.Remittance); m != nil && ownCards[m[1]] && strings.Contains(strings.ToUpper(out.Remittance), "TO CARD") {
				out.internal = true
				out.category = "internal_transfer"
			}
		}
	}
}

func median(xs []int64) int64 {
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// detectRecurring applies the rule from the strategy doc: same counterparty, amount
// within ±10%, interval 7/30 days ±3, at least 3 hits (2 for monthly in the MVP).
// Utility bills are exempt from the amount check, since they swing with the season;
// a jump then shows up as status "changed".
func (a *Analysis) detectRecurring(u *UserState) {
	groups := map[string][]*txView{}
	for _, t := range a.txs {
		if t.Status != "booked" || t.internal {
			continue
		}
		dir := "expense"
		if t.AmountMinor > 0 {
			dir = "income"
		}
		key := dir + "|" + t.merchant
		groups[key] = append(groups[key], t)
	}

	for key, hits := range groups {
		if len(hits) < 2 {
			continue
		}
		sort.Slice(hits, func(i, j int) bool { return hits[i].date.Before(hits[j].date) })

		cadence := ""
		for _, c := range []struct {
			name     string
			lo, hi   int
			minCount int
		}{{"monthly", 27, 34, 2}, {"weekly", 6, 8, 3}} {
			ok := len(hits) >= c.minCount
			for i := 1; ok && i < len(hits); i++ {
				d := daysBetween(hits[i-1].date, hits[i].date)
				ok = d >= c.lo && d <= c.hi
			}
			if ok {
				cadence = c.name
				break
			}
		}
		if cadence == "" {
			continue
		}

		amounts := make([]int64, len(hits))
		for i, h := range hits {
			amounts[i] = h.amountAzn
		}
		last := hits[len(hits)-1]
		typical := median(amounts)
		if len(hits) >= 3 {
			typical = median(amounts[:len(amounts)-1])
		}
		variable := last.category == "utilities"
		if !variable {
			med := median(amounts)
			for _, x := range amounts {
				if abs64(x-med)*10 > abs64(med) {
					cadence = ""
					break
				}
			}
			if cadence == "" {
				continue
			}
		}

		dir, _, _ := strings.Cut(key, "|")
		item := &recurringItem{
			ID: "rec_" + shortHash(key), Type: dir, Name: last.merchant, Merchant: last.merchant,
			Category: last.category, Cadence: cadence, Typical: typical, LastDate: last.Date,
			Source: "detected", AccountID: last.AccountID, FirstDate: hits[0].Date, Hits: len(hits),
		}
		item.NextDate = item.step(last.date)
		item.Status = "active"
		if abs64(typical) > 0 && abs64(last.amountAzn-typical)*10 > abs64(typical) {
			item.Status = "changed"
		}
		for _, h := range hits {
			h.recurringID = item.ID
		}
		a.recurring = append(a.recurring, item)
	}

	// Manual items the user added (e.g. rent paid in cash).
	for _, m := range u.ManualRecurring {
		next, err := parseDate(m.NextDate)
		if err != nil {
			continue
		}
		a.recurring = append(a.recurring, &recurringItem{
			ID: m.ID, Type: m.Type, Name: m.Name, Merchant: m.Name, Category: m.CategoryKey,
			Cadence: m.Cadence, Typical: m.AmountMinor, NextDate: next, Status: "active",
			Source: "manual", Confirmed: m.Confirmed,
		})
	}

	// Apply the user's edits, roll stale dates forward, flag late items.
	kept := a.recurring[:0]
	for _, r := range a.recurring {
		if o := u.RecurringOverrides[r.ID]; o != nil {
			if o.Deleted {
				for _, t := range a.txs {
					if t.recurringID == r.ID {
						t.recurringID = ""
					}
				}
				continue
			}
			if o.Name != nil {
				r.Name = *o.Name
			}
			if o.Confirmed != nil {
				r.Confirmed = *o.Confirmed
			}
			if o.TypicalAmount != nil {
				r.Typical = *o.TypicalAmount
			}
			if o.NextDate != nil {
				if d, err := parseDate(*o.NextDate); err == nil {
					r.NextDate = d
				}
			}
		}
		if r.Source == "detected" && r.NextDate.Before(a.today.AddDate(0, 0, -3)) {
			r.Status = "late" // expected a few days ago and still not here
		}
		if r.Source == "manual" {
			for r.NextDate.Before(a.today) {
				r.NextDate = r.step(r.NextDate)
			}
		}
		kept = append(kept, r)
	}
	a.recurring = kept
	sort.Slice(a.recurring, func(i, j int) bool { return a.recurring[i].NextDate.Before(a.recurring[j].NextDate) })
}

func (a *Analysis) recurringByID(id string) *recurringItem {
	for _, r := range a.recurring {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// buildLoans turns recurring instalment payments into loans, then merges what the
// user entered. Remaining and payoff stay unknown until the user fills them in.
func (a *Analysis) buildLoans(u *UserState) {
	for _, r := range a.recurring {
		if r.Category != "loans" || r.Type != "expense" || r.Source != "detected" {
			continue
		}
		id := "loan_" + strings.TrimPrefix(r.ID, "rec_")
		info := u.Loans[id]
		if info != nil && info.Deleted {
			continue
		}
		l := &loanItem{
			ID: id, Name: r.Name, Lender: r.Merchant, Source: "detected", MonthlyPayment: -r.Typical,
			NextPaymentDate: r.NextDate.Format(dateLayout), PaymentsSeen: r.Hits, PayingAccountID: r.AccountID,
		}
		if info != nil {
			mergeLoan(l, info)
		}
		for _, t := range a.txs {
			if t.recurringID == r.ID {
				t.loanID = id
			}
		}
		a.loans = append(a.loans, l)
	}
	for _, info := range u.Loans {
		if info.Manual && !info.Deleted {
			l := &loanItem{ID: info.ID, Source: "manual"}
			mergeLoan(l, info)
			a.loans = append(a.loans, l)
		}
	}
	sort.Slice(a.loans, func(i, j int) bool { return a.loans[i].ID < a.loans[j].ID })
}

func mergeLoan(l *loanItem, info *LoanInfo) {
	if info.Name != "" {
		l.Name = info.Name
	}
	if info.Lender != "" {
		l.Lender = info.Lender
	}
	if info.MonthlyPayment != nil {
		l.MonthlyPayment = *info.MonthlyPayment
	}
	if info.NextPaymentDate != "" {
		l.NextPaymentDate = info.NextPaymentDate
	}
	if info.Remaining != nil {
		l.Remaining = info.Remaining
	}
}

// payoffDate estimates when a loan ends: remaining ÷ monthly payment, from the next payment.
func (l *loanItem) payoffDate() *string {
	if l.Remaining == nil || l.MonthlyPayment <= 0 {
		return nil
	}
	next, err := parseDate(l.NextPaymentDate)
	if err != nil {
		return nil
	}
	months := int((*l.Remaining + l.MonthlyPayment - 1) / l.MonthlyPayment)
	d := next.AddDate(0, max(months-1, 0), 0).Format(dateLayout)
	return &d
}

// spending sums expenses (as a positive AZN amount) over [from, to].
func (a *Analysis) spending(from, to time.Time, keep func(*txView) bool) int64 {
	var sum int64
	for _, t := range a.txs {
		if t.isExpense() && !t.date.Before(from) && !t.date.After(to) && (keep == nil || keep(t)) {
			sum += -t.amountAzn
		}
	}
	return sum
}

func (a *Analysis) income(from, to time.Time) int64 {
	var sum int64
	for _, t := range a.txs {
		if t.isIncome() && !t.date.Before(from) && !t.date.After(to) {
			sum += t.amountAzn
		}
	}
	return sum
}

// periodRange returns the day, ISO week (Mon–Sun) or month containing d.
func periodRange(period string, d time.Time) (time.Time, time.Time) {
	switch period {
	case "day":
		return d, d
	case "week":
		offset := (int(d.Weekday()) + 6) % 7
		start := d.AddDate(0, 0, -offset)
		return start, start.AddDate(0, 0, 6)
	default:
		start := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, baku)
		return start, start.AddDate(0, 1, -1)
	}
}

type safeToSpend struct {
	Today          int64
	Remaining      int64
	NextIncome     *recurringItem
	NextIncomeDate time.Time
	DaysLeft       int
	Available      int64
	UpcomingBills  int64
	Buffer         int64
	SpentToday     int64
}

// safeToSpend: card balances at the start of today, minus bills due before the next
// income, minus a 10% buffer, spread over the days left. It stays fixed for the day,
// and today's day-to-day spending is measured against it.
func (a *Analysis) safeToSpend(u *UserState) safeToSpend {
	var s safeToSpend
	cards := map[string]bool{}
	for _, acc := range u.Accounts {
		if acc.Type == "CARD" {
			cards[acc.ID] = true
			s.Available += toAZN(acc.BalanceMinor, acc.Currency, a.fx)
		}
	}
	// Add back everything that left the cards today, to get the start-of-day balance.
	for _, t := range a.txs {
		if cards[t.AccountID] && t.date.Equal(a.today) {
			s.Available -= t.amountAzn
		}
	}
	s.SpentToday = a.spending(a.today, a.today, (*txView).discretionary)

	// Next income: the biggest expected one within ~5 weeks (the salary, not a side gig).
	for _, r := range a.recurring {
		if r.Type != "income" || r.NextDate.Before(a.today) || r.NextDate.After(a.today.AddDate(0, 0, 35)) {
			continue
		}
		if s.NextIncome == nil || r.Typical > s.NextIncome.Typical {
			s.NextIncome = r
		}
	}
	if s.NextIncome != nil {
		s.NextIncomeDate = s.NextIncome.NextDate
	} else {
		_, end := periodRange("month", a.today)
		s.NextIncomeDate = end.AddDate(0, 0, 1)
	}
	s.DaysLeft = max(1, daysBetween(a.today, s.NextIncomeDate))

	for _, r := range a.recurring {
		if r.Type == "expense" {
			s.UpcomingBills += -r.Typical * int64(len(r.occurrences(a.today, s.NextIncomeDate)))
		}
	}
	s.Buffer = max(s.Available, 0) / 10
	s.Remaining = max(s.Available-s.UpcomingBills-s.Buffer, 0)
	s.Today = s.Remaining / int64(s.DaysLeft)
	return s
}

type streakDay struct {
	Date   time.Time
	Status string // under | over | no_data
}

type streak struct {
	Current, Best int
	Today         string // on_track | over | no_data
	Days          []streakDay
}

// streak counts days whose day-to-day spending stayed within today's safe-to-spend.
func (a *Analysis) streak(u *UserState, limit int64) streak {
	var st streak
	if len(u.Accounts) == 0 {
		st.Today = "no_data"
		return st
	}
	run := 0
	for i := 29; i >= 0; i-- {
		d := a.today.AddDate(0, 0, -i)
		day := streakDay{Date: d, Status: "under"}
		if d.Before(a.firstDate) {
			day.Status = "no_data"
		} else if a.spending(d, d, (*txView).discretionary) > limit {
			day.Status = "over"
		}
		st.Days = append(st.Days, day)
		switch day.Status {
		case "under":
			run++
			st.Best = max(st.Best, run)
		case "over":
			run = 0
		}
	}
	st.Current = run
	st.Today = "on_track"
	if st.Days[len(st.Days)-1].Status == "over" {
		st.Today = "over"
	}
	return st
}

// budgetable categories are the ones a baseline budget makes sense for: day-to-day
// spending, not rent, instalments, cash withdrawals or fees.
func budgetable(category string) bool {
	switch category {
	case "housing", "loans", "cash", "fees", "transfers_out":
		return false
	}
	return true
}

type budgetView struct {
	ID       string
	Category string // empty = total
	Period   string
	Limit    int64
	Spent    int64
	Basis    string // user | baseline
}

func (b budgetView) pct() float64 {
	if b.Limit <= 0 {
		return 0
	}
	return math.Round(float64(b.Spent)/float64(b.Limit)*1000) / 10
}

func (b budgetView) status() string {
	switch p := b.pct(); {
	case p >= 100:
		return "over"
	case p >= 80:
		return "warning"
	}
	return "ok"
}

// baseline is the average spending of the last up to 3 full periods we have data for.
func (a *Analysis) baseline(category, period string) int64 {
	start, _ := periodRange(period, a.today)
	var sum int64
	n := 0
	for i := 0; i < 3; i++ {
		var ps, pe time.Time
		if period == "week" {
			ps, pe = start.AddDate(0, 0, -7*(i+1)), start.AddDate(0, 0, -7*i-1)
		} else {
			ps = start.AddDate(0, -(i + 1), 0)
			pe = ps.AddDate(0, 1, -1)
		}
		if ps.Before(a.firstDate) {
			break
		}
		sum += a.spending(ps, pe, func(t *txView) bool { return category == "" || t.category == category })
		n++
	}
	if n == 0 {
		// Not one full period yet: scale the last 30 days.
		last30 := a.spending(a.today.AddDate(0, 0, -30), a.today.AddDate(0, 0, -1), func(t *txView) bool { return category == "" || t.category == category })
		if period == "week" {
			return last30 * 7 / 30
		}
		return last30
	}
	return sum / int64(n)
}

// budgets returns the user's budgets plus baselines for total spending and the
// top 3 categories of the current month that have no budget of their own.
func (a *Analysis) budgets(u *UserState) []budgetView {
	var out []budgetView
	has := map[string]bool{}
	for _, b := range u.Budgets {
		start, end := periodRange(b.Period, a.today)
		out = append(out, budgetView{ID: b.ID, Category: b.CategoryKey, Period: b.Period, Limit: b.LimitMinor, Basis: "user",
			Spent: a.spending(start, end, func(t *txView) bool { return b.CategoryKey == "" || t.category == b.CategoryKey })})
		has[b.CategoryKey+"|"+b.Period] = true
	}
	if len(u.Accounts) == 0 {
		return out
	}

	start, end := periodRange("month", a.today)
	byCat := map[string]int64{}
	for _, t := range a.txs {
		if t.isExpense() && !t.date.Before(start) && !t.date.After(end) && budgetable(t.category) {
			byCat[t.category] += -t.amountAzn
		}
	}
	cats := make([]string, 0, len(byCat))
	for c := range byCat {
		cats = append(cats, c)
	}
	sort.Slice(cats, func(i, j int) bool { return byCat[cats[i]] > byCat[cats[j]] })
	if len(cats) > 3 {
		cats = cats[:3]
	}
	for _, c := range append([]string{""}, cats...) {
		if has[c+"|month"] {
			continue
		}
		limit := a.baseline(c, "month")
		if limit <= 0 {
			continue
		}
		id := "bud_base_total"
		if c != "" {
			id = "bud_base_" + c
		}
		out = append(out, budgetView{ID: id, Category: c, Period: "month", Limit: limit, Basis: "baseline",
			Spent: a.spending(start, end, func(t *txView) bool { return c == "" || t.category == c })})
	}
	return out
}
