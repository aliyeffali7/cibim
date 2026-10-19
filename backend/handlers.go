package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- Login (hackathon only) ----

func (app *App) demoUsersHandler(w http.ResponseWriter, r *http.Request) {
	app.state.mu.Lock()
	users := app.loadDemoUsers()
	app.state.mu.Unlock()
	if len(users) == 0 {
		writeError(w, http.StatusBadGateway, "BANK_UNAVAILABLE", "Could not reach the bank gateway for demo users")
		return
	}
	out := []map[string]string{}
	for _, du := range users {
		out = append(out, map[string]string{"psuId": du.PSUID, "name": du.Name, "persona": du.Persona})
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) demoLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PSUID string `json:"psuId"`
	}
	if !decode(w, r, &req) {
		return
	}
	app.state.mu.Lock()
	defer app.state.mu.Unlock()
	name, ok := app.demoUserName(req.PSUID)
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown psuId; see GET /api/v1/auth/demo-users")
		return
	}
	u := app.state.user(req.PSUID, name)
	app.persist()
	writeJSON(w, http.StatusOK, map[string]any{"token": "demo." + u.PSUID, "user": meView(u)})
}

// ---- Banks and connections ----

func (app *App) listBanks(w http.ResponseWriter, r *http.Request, u *UserState) {
	codes := []string{"ABB", "XALQ", "PASHA", "LEO"}
	if banks, err := app.cbar.Banks(); err == nil && len(banks) > 0 {
		codes = codes[:0]
		for _, b := range banks {
			codes = append(codes, b.BankCode)
			staticBanks[b.BankCode] = b.Name
		}
	}
	out := []map[string]any{}
	for _, code := range codes {
		c := u.connectionForBank(code)
		b := bankView(code)
		out = append(out, map[string]any{"code": b.Code, "name": b.Name, "logoUrl": b.LogoURL, "connected": c != nil && c.Status == "active"})
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) redirectFor(connID, override string) string {
	tmpl := app.redirectURL
	if override != "" {
		tmpl = override
	}
	return strings.ReplaceAll(tmpl, "{id}", connID)
}

// createConnection starts linking a bank: it asks CBAR for a consent and returns
// the bank's approval page. Body: { bankCode, redirectUri? }. redirectUri lets
// Expo Go or a web build override where the bank sends the user back ("{id}" is replaced).
func (app *App) createConnection(w http.ResponseWriter, r *http.Request, u *UserState) {
	var req struct {
		BankCode    string `json:"bankCode"`
		RedirectURI string `json:"redirectUri"`
	}
	if !decode(w, r, &req) {
		return
	}
	c := u.connectionForBank(req.BankCode)
	if c != nil && (c.Status == "active" || c.Status == "expired") {
		writeError(w, http.StatusConflict, "VALIDATION_ERROR", "This bank is already linked; use POST /connections/"+c.ID+"/renew to renew it")
		return
	}
	if c == nil {
		c = &Connection{ID: u.nextID("con"), BankCode: req.BankCode}
	}
	cs, err := app.cbar.CreateConsent(u.PSUID, req.BankCode, app.redirectFor(c.ID, req.RedirectURI))
	if err != nil {
		writeCBARError(w, err)
		return
	}
	c.ConsentID, c.Status, c.PendingConsentID = cs.ConsentID, "pending", ""
	if u.connection(c.ID) == nil {
		u.Connections = append(u.Connections, c)
	}
	app.persist()
	writeJSON(w, http.StatusCreated, map[string]string{"connectionId": c.ID, "status": "pending", "authUrl": cs.Links.SCARedirect.Href})
}

func writeCBARError(w http.ResponseWriter, err error) {
	if ce, ok := err.(*CBARError); ok && ce.Status == http.StatusBadRequest {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", ce.Text)
		return
	}
	writeError(w, http.StatusBadGateway, "BANK_UNAVAILABLE", err.Error())
}

func (app *App) listConnections(w http.ResponseWriter, r *http.Request, u *UserState) {
	changed := false
	for _, c := range u.Connections {
		if c.Status == "pending" {
			changed = app.refreshConnection(c) || changed
		}
	}
	if changed {
		_ = app.syncUser(u)
	}
	out := []connectionJSON{}
	for _, c := range u.Connections {
		out = append(out, app.connectionView(u, c))
	}
	writeJSON(w, http.StatusOK, out)
}

// getConnection is what the app polls after the bank approval page. The first
// time it sees the consent approved, it syncs that bank's data before answering.
func (app *App) getConnection(w http.ResponseWriter, r *http.Request, u *UserState) {
	c := u.connection(r.PathValue("id"))
	if c == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No connection "+r.PathValue("id"))
		return
	}
	if app.refreshConnection(c) {
		_ = app.syncUser(u)
	}
	app.persist()
	writeJSON(w, http.StatusOK, app.connectionView(u, c))
}

func (app *App) renewConnection(w http.ResponseWriter, r *http.Request, u *UserState) {
	c := u.connection(r.PathValue("id"))
	if c == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No connection "+r.PathValue("id"))
		return
	}
	var req struct {
		RedirectURI string `json:"redirectUri"`
	}
	if r.ContentLength > 0 && !decode(w, r, &req) {
		return
	}
	cs, err := app.cbar.CreateConsent(u.PSUID, c.BankCode, app.redirectFor(c.ID, req.RedirectURI))
	if err != nil {
		writeCBARError(w, err)
		return
	}
	if c.Status == "active" {
		c.PendingConsentID = cs.ConsentID // keep reading with the old consent until the new one is approved
	} else {
		c.ConsentID, c.Status = cs.ConsentID, "pending"
	}
	app.persist()
	writeJSON(w, http.StatusOK, map[string]string{"authUrl": cs.Links.SCARedirect.Href})
}

func (app *App) deleteConnection(w http.ResponseWriter, r *http.Request, u *UserState) {
	c := u.connection(r.PathValue("id"))
	if c == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No connection "+r.PathValue("id"))
		return
	}
	if c.ConsentID != "" {
		_ = app.cbar.DeleteConsent(c.ConsentID) // best effort: we drop our side either way
	}
	removeConnectionData(u, c.ID)
	kept := u.Connections[:0]
	for _, x := range u.Connections {
		if x.ID != c.ID {
			kept = append(kept, x)
		}
	}
	u.Connections = kept
	app.persist()
	w.WriteHeader(http.StatusNoContent)
}

// syncHandler runs the sync before answering, so the app can refetch right away.
func (app *App) syncHandler(w http.ResponseWriter, r *http.Request, u *UserState) {
	started := app.now().In(baku)
	if err := app.syncUser(u); err != nil && len(u.Accounts) == 0 {
		writeError(w, http.StatusBadGateway, "BANK_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"syncStartedAt": started, "dataUpdatedAt": dataUpdatedAt(u)})
}

// demoConnectAll links every bank the demo user has, approving consents through
// the mock. A fallback for the stage if the approval page flow misbehaves.
func (app *App) demoConnectAll(w http.ResponseWriter, r *http.Request, u *UserState) {
	users, err := app.cbar.DemoUsers()
	if err != nil {
		writeCBARError(w, err)
		return
	}
	var banks []string
	for _, du := range users {
		if du.PSUID == u.PSUID {
			banks = du.Banks
		}
	}
	for _, code := range banks {
		c := u.connectionForBank(code)
		if c != nil && c.Status == "active" {
			continue
		}
		if c == nil {
			c = &Connection{ID: u.nextID("con"), BankCode: code}
			u.Connections = append(u.Connections, c)
		}
		cs, err := app.cbar.CreateConsent(u.PSUID, code, app.redirectFor(c.ID, ""))
		if err != nil {
			writeCBARError(w, err)
			return
		}
		if err := app.cbar.ApproveConsent(cs.ConsentID); err != nil {
			writeCBARError(w, err)
			return
		}
		c.ConsentID, c.PendingConsentID, c.Status = cs.ConsentID, "", "pending"
	}
	_ = app.syncUser(u)
	out := []connectionJSON{}
	for _, c := range u.Connections {
		out = append(out, app.connectionView(u, c))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- Home ----

func (app *App) home(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	if len(u.Accounts) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"onboarding": true, "dataUpdatedAt": nil, "totalBalance": nil, "accounts": []any{}, "spending": nil,
			"safeToSpend": nil, "streak": nil, "alerts": []any{}, "upcoming": []any{}, "recentTransactions": []any{},
		})
		return
	}
	a := app.analysis(r, u)
	l := lang(r, u)

	var total int64
	accounts := []accountJSON{}
	for _, acc := range u.Accounts {
		v := accountView(acc, a.fx)
		total += v.BalanceAzn.AmountMinor
		accounts = append(accounts, v)
	}

	weekStart, _ := periodRange("week", a.today)
	monthStart, _ := periodRange("month", a.today)
	s2s := a.safeToSpend(u)
	st := a.streak(u, s2s.Today)

	var nextIncome any
	if s2s.NextIncome != nil {
		name := s2s.NextIncome.Name
		if s2s.NextIncome.Category == "salary" {
			name = categoryByKey("salary", l).Name
		}
		nextIncome = map[string]any{"name": name, "date": s2s.NextIncomeDate.Format(dateLayout), "amount": azn(s2s.NextIncome.Typical)}
	}

	alerts := []alertJSON{}
	for i := len(u.Alerts) - 1; i >= 0 && len(alerts) < 3; i-- {
		if !u.Alerts[i].Read {
			alerts = append(alerts, alertView(u.Alerts[i], l))
		}
	}

	upcoming := []map[string]any{}
	for _, rec := range a.recurring {
		if len(upcoming) == 3 {
			break
		}
		if !rec.NextDate.Before(a.today) {
			upcoming = append(upcoming, map[string]any{"recurringId": rec.ID, "name": rec.Name, "type": rec.Type,
				"date": rec.NextDate.Format(dateLayout), "amount": azn(rec.Typical)})
		}
	}

	recent := []txJSON{}
	for _, t := range a.txs {
		if len(recent) == 10 {
			break
		}
		recent = append(recent, a.txView(t))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"onboarding":    false,
		"dataUpdatedAt": dataUpdatedAt(u),
		"totalBalance":  azn(total),
		"accounts":      accounts,
		"spending": map[string]any{
			"today":     azn(-a.spending(a.today, a.today, nil)),
			"thisWeek":  azn(-a.spending(weekStart, a.today, nil)),
			"thisMonth": azn(-a.spending(monthStart, a.today, nil)),
		},
		"safeToSpend":        map[string]any{"today": azn(s2s.Today), "nextIncome": nextIncome},
		"streak":             map[string]any{"current": st.Current, "best": st.Best, "today": st.Today},
		"alerts":             alerts,
		"upcoming":           upcoming,
		"recentTransactions": recent,
	})
}

// ---- Accounts ----

func (app *App) listAccounts(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	out := []accountJSON{}
	for _, acc := range u.Accounts {
		out = append(out, accountView(acc, app.state.FX))
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) getAccount(w http.ResponseWriter, r *http.Request, u *UserState) {
	acc := u.account(r.PathValue("id"))
	if acc == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No account "+r.PathValue("id"))
		return
	}
	writeJSON(w, http.StatusOK, struct {
		accountJSON
		DataUpdatedAt *time.Time `json:"dataUpdatedAt"`
	}{accountView(acc, app.state.FX), dataUpdatedAt(u)})
}

// ---- Transactions ----

func (app *App) listTransactions(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	q := r.URL.Query()
	var from, to time.Time
	if v := q.Get("from"); v != "" {
		from, _ = parseDate(v)
	}
	if v := q.Get("to"); v != "" {
		to, _ = parseDate(v)
	}
	categoryKey, _ := categoryKeyFromID(q.Get("categoryId"))
	search := strings.ToLower(q.Get("search"))
	includeTransfers := q.Get("includeTransfers") != "false"

	var items []txJSON
	for _, t := range a.txs {
		switch {
		case q.Get("accountId") != "" && t.AccountID != q.Get("accountId"),
			categoryKey != "" && t.category != categoryKey,
			q.Get("direction") == "in" && t.AmountMinor < 0,
			q.Get("direction") == "out" && t.AmountMinor > 0,
			!from.IsZero() && t.date.Before(from),
			!to.IsZero() && t.date.After(to),
			!includeTransfers && t.internal,
			search != "" && !strings.Contains(strings.ToLower(t.merchant+" "+t.Remittance), search):
			continue
		}
		items = append(items, a.txView(t))
	}
	page, next := paginate(items, q.Get("limit"), q.Get("cursor"))
	writeJSON(w, http.StatusOK, map[string]any{"items": page, "nextCursor": next, "dataUpdatedAt": dataUpdatedAt(u)})
}

// paginate uses the offset as the cursor.
func paginate[T any](items []T, limitStr, cursor string) ([]T, *string) {
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 200 {
		limit = 50
	}
	start, _ := strconv.Atoi(cursor)
	if start < 0 || start > len(items) {
		start = len(items)
	}
	end := min(start+limit, len(items))
	page := items[start:end]
	if page == nil {
		page = []T{}
	}
	if end < len(items) {
		next := strconv.Itoa(end)
		return page, &next
	}
	return page, nil
}

func (app *App) patchTransaction(w http.ResponseWriter, r *http.Request, u *UserState) {
	var req struct {
		CategoryID      string `json:"categoryId"`
		ApplyToMerchant bool   `json:"applyToMerchant"`
	}
	if !decode(w, r, &req) {
		return
	}
	key, ok := categoryKeyFromID(req.CategoryID)
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown categoryId; see GET /categories")
		return
	}
	a := app.analysis(r, u)
	t := a.txByID[r.PathValue("id")]
	if t == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No transaction "+r.PathValue("id"))
		return
	}
	if req.ApplyToMerchant {
		u.MerchantCategory[t.merchant] = key
		// Clear per-transaction choices for this merchant so the new rule applies everywhere.
		for _, other := range a.txs {
			if other.merchant == t.merchant {
				delete(u.TxCategory, other.ID)
			}
		}
	} else {
		u.TxCategory[t.ID] = key
	}
	app.generateAlerts(u)
	app.persist()
	a = app.analysis(r, u)
	writeJSON(w, http.StatusOK, a.txView(a.txByID[t.ID]))
}

func (app *App) listCategories(w http.ResponseWriter, r *http.Request, u *UserState) {
	writeJSON(w, http.StatusOK, allCategories(lang(r, u)))
}

// ---- Summary ----

func (app *App) summary(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	period := r.URL.Query().Get("period")
	if period != "day" && period != "week" {
		period = "month"
	}
	date := a.today
	if v := r.URL.Query().Get("date"); v != "" {
		if d, err := parseDate(v); err == nil {
			date = d
		}
	}
	from, to := periodRange(period, date)
	prevFrom, prevTo := periodRange(period, from.AddDate(0, 0, -1))

	income, expense := a.income(from, to), a.spending(from, to, nil)

	type catAgg struct {
		key       string
		cur, prev int64
	}
	cats := map[string]*catAgg{}
	banks := map[string]int64{}
	type merchAgg struct {
		amount int64
		count  int
	}
	merchants := map[string]*merchAgg{}
	daily := map[string][2]int64{}
	for _, t := range a.txs {
		inCur := !t.date.Before(from) && !t.date.After(to)
		inPrev := !t.date.Before(prevFrom) && !t.date.After(prevTo)
		if t.isExpense() && (inCur || inPrev) {
			c := cats[t.category]
			if c == nil {
				c = &catAgg{key: t.category}
				cats[t.category] = c
			}
			if inCur {
				c.cur += -t.amountAzn
				banks[t.BankCode] += -t.amountAzn
				m := merchants[t.merchant]
				if m == nil {
					m = &merchAgg{}
					merchants[t.merchant] = m
				}
				m.amount += -t.amountAzn
				m.count++
			} else {
				c.prev += -t.amountAzn
			}
		}
		if inCur {
			d := daily[t.date.Format(dateLayout)]
			if t.isIncome() {
				d[0] += t.amountAzn
			} else if t.isExpense() {
				d[1] += t.amountAzn
			}
			daily[t.date.Format(dateLayout)] = d
		}
	}

	byCategory := []map[string]any{}
	catList := make([]*catAgg, 0, len(cats))
	for _, c := range cats {
		if c.cur > 0 {
			catList = append(catList, c)
		}
	}
	sort.Slice(catList, func(i, j int) bool { return catList[i].cur > catList[j].cur })
	for _, c := range catList {
		var change any
		if c.prev > 0 {
			change = round1(float64(c.cur-c.prev) / float64(c.prev) * 100)
		}
		byCategory = append(byCategory, map[string]any{
			"category": categoryByKey(c.key, a.lang), "amount": azn(-c.cur),
			"sharePct": round1(float64(c.cur) / float64(max(expense, 1)) * 100), "changeVsPreviousPct": change,
		})
	}

	byBank := []map[string]any{}
	for code, amt := range banks {
		byBank = append(byBank, map[string]any{"bank": bankView(code), "expense": azn(-amt)})
	}
	sort.Slice(byBank, func(i, j int) bool {
		return byBank[i]["expense"].(moneyJSON).AmountMinor < byBank[j]["expense"].(moneyJSON).AmountMinor
	})

	days := []map[string]any{}
	for d := from; !d.After(to) && !d.After(a.today); d = d.AddDate(0, 0, 1) {
		v := daily[d.Format(dateLayout)]
		days = append(days, map[string]any{"date": d.Format(dateLayout), "income": azn(v[0]), "expense": azn(v[1])})
	}

	names := make([]string, 0, len(merchants))
	for n := range merchants {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return merchants[names[i]].amount > merchants[names[j]].amount })
	if len(names) > 5 {
		names = names[:5]
	}
	top := []map[string]any{}
	for _, n := range names {
		top = append(top, map[string]any{"merchant": merchantJSON{Name: n}, "amount": azn(-merchants[n].amount), "count": merchants[n].count})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"period": period, "from": from.Format(dateLayout), "to": to.Format(dateLayout),
		"income": azn(income), "expense": azn(-expense), "net": azn(income - expense),
		"byCategory": byCategory, "byBank": byBank, "daily": days, "topMerchants": top,
		"dataUpdatedAt": dataUpdatedAt(u),
	})
}

func round1(f float64) float64 { return float64(int64(f*10+sign(f)*0.5)) / 10 }

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

// ---- Recurring ----

func (app *App) listRecurring(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	typ := r.URL.Query().Get("type")
	out := []recurringJSON{}
	for _, rec := range a.recurring {
		if typ == "" || rec.Type == typ {
			out = append(out, a.recurringView(rec))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) recurringCalendar(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 120 {
		days = 30
	}
	end := a.today.AddDate(0, 0, days)
	byDay := map[string][]map[string]any{}
	net := map[string]int64{}
	var income, expense int64
	for _, rec := range a.recurring {
		for _, d := range rec.occurrences(a.today, end) {
			k := d.Format(dateLayout)
			byDay[k] = append(byDay[k], map[string]any{"recurringId": rec.ID, "name": rec.Name, "type": rec.Type, "amount": azn(rec.Typical)})
			net[k] += rec.Typical
			if rec.Typical > 0 {
				income += rec.Typical
			} else {
				expense += rec.Typical
			}
		}
	}
	keys := make([]string, 0, len(byDay))
	for k := range byDay {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []map[string]any{}
	for _, k := range keys {
		out = append(out, map[string]any{"date": k, "items": byDay[k], "net": azn(net[k])})
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": out, "totals": map[string]any{"income": azn(income), "expense": azn(expense), "net": azn(income + expense)}})
}

type recurringInput struct {
	Type          string     `json:"type"`
	Name          *string    `json:"name"`
	Amount        *moneyJSON `json:"amount"`
	TypicalAmount *moneyJSON `json:"typicalAmount"`
	Cadence       string     `json:"cadence"`
	NextDate      *string    `json:"nextDate"`
	CategoryID    string     `json:"categoryId"`
	Confirmed     *bool      `json:"confirmed"`
}

func (app *App) createRecurring(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in recurringInput
	if !decode(w, r, &in) {
		return
	}
	if (in.Type != "income" && in.Type != "expense") || in.Name == nil || in.Amount == nil || in.NextDate == nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type (income|expense), name, amount and nextDate are required")
		return
	}
	if _, err := parseDate(*in.NextDate); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "nextDate must be YYYY-MM-DD")
		return
	}
	cadence := in.Cadence
	if cadence != "weekly" && cadence != "yearly" {
		cadence = "monthly"
	}
	amount := abs64(in.Amount.AmountMinor)
	if in.Type == "expense" {
		amount = -amount
	}
	key, ok := categoryKeyFromID(in.CategoryID)
	if !ok {
		key = "other_expense"
		if in.Type == "income" {
			key = "other_income"
		}
	}
	m := &ManualRecurring{ID: u.nextID("rec"), Type: in.Type, Name: *in.Name, AmountMinor: amount, Cadence: cadence,
		NextDate: *in.NextDate, CategoryKey: key, Confirmed: true}
	u.ManualRecurring = append(u.ManualRecurring, m)
	app.persist()
	a := app.analysis(r, u)
	writeJSON(w, http.StatusCreated, a.recurringView(a.recurringByID(m.ID)))
}

func (app *App) patchRecurring(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in recurringInput
	if !decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	a := app.analysis(r, u)
	rec := a.recurringByID(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No recurring item "+id)
		return
	}
	if in.NextDate != nil {
		if _, err := parseDate(*in.NextDate); err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "nextDate must be YYYY-MM-DD")
			return
		}
	}
	signed := func(m *moneyJSON) int64 {
		v := abs64(m.AmountMinor)
		if rec.Type == "expense" {
			return -v
		}
		return v
	}
	if rec.Source == "manual" {
		for _, m := range u.ManualRecurring {
			if m.ID == id {
				if in.Name != nil {
					m.Name = *in.Name
				}
				if in.Confirmed != nil {
					m.Confirmed = *in.Confirmed
				}
				if in.TypicalAmount != nil {
					m.AmountMinor = signed(in.TypicalAmount)
				}
				if in.NextDate != nil {
					m.NextDate = *in.NextDate
				}
			}
		}
	} else {
		o := u.RecurringOverrides[id]
		if o == nil {
			o = &RecurringOverride{}
			u.RecurringOverrides[id] = o
		}
		if in.Name != nil {
			o.Name = in.Name
		}
		if in.Confirmed != nil {
			o.Confirmed = in.Confirmed
		}
		if in.TypicalAmount != nil {
			v := signed(in.TypicalAmount)
			o.TypicalAmount = &v
		}
		if in.NextDate != nil {
			o.NextDate = in.NextDate
		}
	}
	app.persist()
	a = app.analysis(r, u)
	writeJSON(w, http.StatusOK, a.recurringView(a.recurringByID(id)))
}

func (app *App) deleteRecurring(w http.ResponseWriter, r *http.Request, u *UserState) {
	id := r.PathValue("id")
	a := app.analysis(r, u)
	rec := a.recurringByID(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No recurring item "+id)
		return
	}
	if rec.Source == "manual" {
		kept := u.ManualRecurring[:0]
		for _, m := range u.ManualRecurring {
			if m.ID != id {
				kept = append(kept, m)
			}
		}
		u.ManualRecurring = kept
	} else {
		u.RecurringOverrides[id] = &RecurringOverride{Deleted: true}
	}
	app.persist()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Loans ----

func (app *App) listLoans(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	out := []loanJSON{}
	for _, l := range a.loans {
		out = append(out, loanView(l))
	}
	writeJSON(w, http.StatusOK, out)
}

type loanInput struct {
	Name            *string    `json:"name"`
	Lender          *string    `json:"lender"`
	MonthlyPayment  *moneyJSON `json:"monthlyPayment"`
	Remaining       *moneyJSON `json:"remaining"`
	NextPaymentDate *string    `json:"nextPaymentDate"`
}

func (in loanInput) applyTo(info *LoanInfo) {
	if in.Name != nil {
		info.Name = *in.Name
	}
	if in.Lender != nil {
		info.Lender = *in.Lender
	}
	if in.MonthlyPayment != nil {
		v := abs64(in.MonthlyPayment.AmountMinor)
		info.MonthlyPayment = &v
	}
	if in.Remaining != nil {
		v := abs64(in.Remaining.AmountMinor)
		info.Remaining = &v
	}
	if in.NextPaymentDate != nil {
		info.NextPaymentDate = *in.NextPaymentDate
	}
}

func (app *App) createLoan(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in loanInput
	if !decode(w, r, &in) {
		return
	}
	if in.Name == nil || in.MonthlyPayment == nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name and monthlyPayment are required")
		return
	}
	info := &LoanInfo{ID: u.nextID("loan"), Manual: true}
	in.applyTo(info)
	u.Loans[info.ID] = info
	app.persist()
	app.writeLoan(w, r, u, info.ID, http.StatusCreated)
}

func (app *App) patchLoan(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in loanInput
	if !decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	if !app.loanExists(r, u, id) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No loan "+id)
		return
	}
	info := u.Loans[id]
	if info == nil {
		info = &LoanInfo{ID: id}
		u.Loans[id] = info
	}
	in.applyTo(info)
	app.persist()
	app.writeLoan(w, r, u, id, http.StatusOK)
}

func (app *App) deleteLoan(w http.ResponseWriter, r *http.Request, u *UserState) {
	id := r.PathValue("id")
	if !app.loanExists(r, u, id) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No loan "+id)
		return
	}
	info := u.Loans[id]
	if info == nil {
		info = &LoanInfo{ID: id}
		u.Loans[id] = info
	}
	info.Deleted = true
	app.persist()
	w.WriteHeader(http.StatusNoContent)
}

func (app *App) loanExists(r *http.Request, u *UserState, id string) bool {
	for _, l := range app.analysis(r, u).loans {
		if l.ID == id {
			return true
		}
	}
	return false
}

func (app *App) writeLoan(w http.ResponseWriter, r *http.Request, u *UserState, id string, status int) {
	for _, l := range app.analysis(r, u).loans {
		if l.ID == id {
			writeJSON(w, status, loanView(l))
			return
		}
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "No loan "+id)
}

// ---- Budgets ----

func (app *App) listBudgets(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	out := []budgetJSON{}
	for _, b := range a.budgets(u) {
		out = append(out, a.budgetView(b))
	}
	writeJSON(w, http.StatusOK, out)
}

type budgetInput struct {
	CategoryID *string    `json:"categoryId"`
	Period     string     `json:"period"`
	Limit      *moneyJSON `json:"limit"`
}

func (app *App) createBudget(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in budgetInput
	if !decode(w, r, &in) {
		return
	}
	if in.Limit == nil || in.Limit.AmountMinor == 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit is required")
		return
	}
	key := ""
	if in.CategoryID != nil && *in.CategoryID != "" {
		k, ok := categoryKeyFromID(*in.CategoryID)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown categoryId")
			return
		}
		key = k
	}
	period := in.Period
	if period != "week" {
		period = "month"
	}
	b := &Budget{ID: u.nextID("bud"), CategoryKey: key, Period: period, LimitMinor: abs64(in.Limit.AmountMinor)}
	u.Budgets = append(u.Budgets, b)
	app.generateAlerts(u)
	app.persist()
	app.writeBudget(w, r, u, b.ID, http.StatusCreated)
}

// patchBudget edits a budget. Editing a baseline ("bud_base_…") saves it as the user's own.
func (app *App) patchBudget(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in budgetInput
	if !decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	var b *Budget
	for _, x := range u.Budgets {
		if x.ID == id {
			b = x
		}
	}
	if b == nil {
		if base, ok := strings.CutPrefix(id, "bud_base_"); ok {
			key := base
			if key == "total" {
				key = ""
			}
			b = &Budget{ID: u.nextID("bud"), CategoryKey: key, Period: "month"}
			u.Budgets = append(u.Budgets, b)
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No budget "+id)
			return
		}
	}
	if in.Limit != nil {
		b.LimitMinor = abs64(in.Limit.AmountMinor)
	}
	if in.Period == "week" || in.Period == "month" {
		b.Period = in.Period
	}
	app.generateAlerts(u)
	app.persist()
	app.writeBudget(w, r, u, b.ID, http.StatusOK)
}

func (app *App) deleteBudget(w http.ResponseWriter, r *http.Request, u *UserState) {
	id := r.PathValue("id")
	kept := u.Budgets[:0]
	found := false
	for _, b := range u.Budgets {
		if b.ID == id {
			found = true
		} else {
			kept = append(kept, b)
		}
	}
	u.Budgets = kept
	if !found {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No budget "+id)
		return
	}
	app.persist()
	w.WriteHeader(http.StatusNoContent)
}

func (app *App) writeBudget(w http.ResponseWriter, r *http.Request, u *UserState, id string, status int) {
	a := app.analysis(r, u)
	for _, b := range a.budgets(u) {
		if b.ID == id {
			writeJSON(w, status, a.budgetView(b))
			return
		}
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "No budget "+id)
}

// ---- Safe-to-spend and streak ----

func (app *App) safeToSpendHandler(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	s := a.safeToSpend(u)
	writeJSON(w, http.StatusOK, map[string]any{
		"today": azn(s.Today), "remainingUntilIncome": azn(s.Remaining), "spentToday": azn(s.SpentToday),
		"nextIncomeDate": s.NextIncomeDate.Format(dateLayout), "daysLeft": s.DaysLeft,
		"breakdown": map[string]any{"available": azn(s.Available), "upcomingBills": azn(s.UpcomingBills), "buffer": azn(s.Buffer)},
	})
}

func (app *App) streakHandler(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	a := app.analysis(r, u)
	st := a.streak(u, a.safeToSpend(u).Today)
	days := []map[string]string{}
	for _, d := range st.Days {
		days = append(days, map[string]string{"date": d.Date.Format(dateLayout), "status": d.Status})
	}
	writeJSON(w, http.StatusOK, map[string]any{"current": st.Current, "best": st.Best, "today": st.Today, "days": days})
}

// ---- Alerts ----

func (app *App) listAlerts(w http.ResponseWriter, r *http.Request, u *UserState) {
	app.ensureFresh(u)
	l := lang(r, u)
	unread := r.URL.Query().Get("unread") == "true"
	var items []alertJSON
	for i := len(u.Alerts) - 1; i >= 0; i-- {
		if !unread || !u.Alerts[i].Read {
			items = append(items, alertView(u.Alerts[i], l))
		}
	}
	page, next := paginate(items, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	writeJSON(w, http.StatusOK, map[string]any{"items": page, "nextCursor": next})
}

func (app *App) readAlert(w http.ResponseWriter, r *http.Request, u *UserState) {
	for _, al := range u.Alerts {
		if al.ID == r.PathValue("id") {
			al.Read = true
			app.persist()
			writeJSON(w, http.StatusOK, alertView(al, lang(r, u)))
			return
		}
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "No alert "+r.PathValue("id"))
}

func (app *App) readAllAlerts(w http.ResponseWriter, r *http.Request, u *UserState) {
	for _, al := range u.Alerts {
		al.Read = true
	}
	app.persist()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Profile ----

func meView(u *UserState) map[string]any {
	return map[string]any{"id": u.PSUID, "name": u.Name, "language": u.Language, "profile": u.Profile}
}

func (app *App) getMe(w http.ResponseWriter, r *http.Request, u *UserState) {
	writeJSON(w, http.StatusOK, meView(u))
}

func (app *App) patchMe(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in struct {
		Name     *string  `json:"name"`
		Language *string  `json:"language"`
		Profile  *Profile `json:"profile"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		u.Name = *in.Name
	}
	if in.Language != nil && (*in.Language == "az" || *in.Language == "en") {
		u.Language = *in.Language
	}
	if in.Profile != nil {
		u.Profile = *in.Profile
	}
	app.generateAlerts(u)
	app.persist()
	writeJSON(w, http.StatusOK, meView(u))
}
