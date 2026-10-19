package main

import "time"

// JSON shapes from the App API Contract's "Shared objects".

type moneyJSON struct {
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

func azn(m int64) moneyJSON { return moneyJSON{m, "AZN"} }

type bankJSON struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
}

func bankView(code string) bankJSON { return bankJSON{Code: code, Name: bankName(code)} }

type accountJSON struct {
	ID           string    `json:"id"`
	Bank         bankJSON  `json:"bank"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	MaskedPAN    *string   `json:"maskedPan"`
	Balance      moneyJSON `json:"balance"`
	BalanceAzn   moneyJSON `json:"balanceAzn"`
	ConnectionID string    `json:"connectionId"`
}

func accountView(acc *StoredAccount, fx map[string]string) accountJSON {
	j := accountJSON{
		ID: acc.ID, Bank: bankView(acc.BankCode), Name: acc.Name, Type: acc.Type,
		Balance:      moneyJSON{acc.BalanceMinor, acc.Currency},
		BalanceAzn:   azn(toAZN(acc.BalanceMinor, acc.Currency, fx)),
		ConnectionID: acc.ConnectionID,
	}
	if acc.MaskedPAN != "" {
		j.MaskedPAN = &acc.MaskedPAN
	}
	return j
}

type merchantJSON struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
}

type txJSON struct {
	ID                 string       `json:"id"`
	AccountID          string       `json:"accountId"`
	Bank               bankJSON     `json:"bank"`
	Date               string       `json:"date"`
	Status             string       `json:"status"`
	Amount             moneyJSON    `json:"amount"`
	AmountAzn          moneyJSON    `json:"amountAzn"`
	Merchant           merchantJSON `json:"merchant"`
	RawDescription     string       `json:"rawDescription"`
	Category           Category     `json:"category"`
	IsInternalTransfer bool         `json:"isInternalTransfer"`
	RecurringID        *string      `json:"recurringId"`
	LoanID             *string      `json:"loanId"`
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (a *Analysis) txView(t *txView) txJSON {
	return txJSON{
		ID: t.ID, AccountID: t.AccountID, Bank: bankView(t.BankCode),
		Date:   t.date.Format(dateLayout), // pending entries carry today's date
		Status: t.Status, Amount: moneyJSON{t.AmountMinor, t.Currency}, AmountAzn: azn(t.amountAzn),
		Merchant: merchantJSON{Name: t.merchant}, RawDescription: t.Remittance,
		Category: categoryByKey(t.category, a.lang), IsInternalTransfer: t.internal,
		RecurringID: optional(t.recurringID), LoanID: optional(t.loanID),
	}
}

type connectionJSON struct {
	ID            string     `json:"id"`
	Bank          bankJSON   `json:"bank"`
	Status        string     `json:"status"`
	ValidUntil    *string    `json:"validUntil"`
	DaysLeft      *int       `json:"daysLeft"`
	LastSyncedAt  *time.Time `json:"lastSyncedAt"`
	AccountsCount int        `json:"accountsCount"`
}

func (app *App) connectionView(u *UserState, c *Connection) connectionJSON {
	j := connectionJSON{ID: c.ID, Bank: bankView(c.BankCode), Status: c.Status, ValidUntil: optional(c.ValidUntil)}
	if dl := daysLeft(c.ValidUntil, app.now()); dl >= 0 {
		j.DaysLeft = &dl
		if c.Status == "active" && dl <= 7 {
			j.Status = "expiring"
		}
	}
	if !c.LastSyncedAt.IsZero() {
		t := c.LastSyncedAt.In(baku)
		j.LastSyncedAt = &t
	}
	for _, a := range u.Accounts {
		if a.ConnectionID == c.ID {
			j.AccountsCount++
		}
	}
	return j
}

type recurringJSON struct {
	ID            string       `json:"id"`
	Type          string       `json:"type"`
	Name          string       `json:"name"`
	Merchant      merchantJSON `json:"merchant"`
	Category      Category     `json:"category"`
	Cadence       string       `json:"cadence"`
	TypicalAmount moneyJSON    `json:"typicalAmount"`
	LastDate      *string      `json:"lastDate"`
	NextDate      string       `json:"nextDate"`
	Status        string       `json:"status"`
	Source        string       `json:"source"`
	Confirmed     bool         `json:"confirmed"`
}

func (a *Analysis) recurringView(r *recurringItem) recurringJSON {
	return recurringJSON{
		ID: r.ID, Type: r.Type, Name: r.Name, Merchant: merchantJSON{Name: r.Merchant},
		Category: categoryByKey(r.Category, a.lang), Cadence: r.Cadence, TypicalAmount: azn(r.Typical),
		LastDate: optional(r.LastDate), NextDate: r.NextDate.Format(dateLayout), Status: r.Status,
		Source: r.Source, Confirmed: r.Confirmed,
	}
}

type alertJSON struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Severity     string            `json:"severity"`
	CreatedAt    time.Time         `json:"createdAt"`
	Read         bool              `json:"read"`
	Params       map[string]any    `json:"params"`
	FallbackText map[string]string `json:"fallbackText"`
	Action       map[string]any    `json:"action"`
}

func alertView(al *Alert, lang string) alertJSON {
	return alertJSON{
		ID: al.ID, Type: al.Type, Severity: al.Severity, CreatedAt: al.CreatedAt.In(baku), Read: al.Read,
		Params: al.Params, FallbackText: fallbackText(al, lang),
		Action: map[string]any{"screen": al.Screen, "targetId": optional(al.TargetID)},
	}
}

type budgetJSON struct {
	ID       string    `json:"id"`
	Category *Category `json:"category"`
	Period   string    `json:"period"`
	Limit    moneyJSON `json:"limit"`
	Spent    moneyJSON `json:"spent"`
	Pct      float64   `json:"pct"`
	Status   string    `json:"status"`
	Basis    string    `json:"basis"`
}

func (a *Analysis) budgetView(b budgetView) budgetJSON {
	j := budgetJSON{ID: b.ID, Period: b.Period, Limit: azn(b.Limit), Spent: azn(b.Spent), Pct: b.pct(), Status: b.status(), Basis: b.Basis}
	if b.Category != "" {
		c := categoryByKey(b.Category, a.lang)
		j.Category = &c
	}
	return j
}

type loanJSON struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Lender          string     `json:"lender"`
	Source          string     `json:"source"`
	MonthlyPayment  moneyJSON  `json:"monthlyPayment"`
	NextPaymentDate *string    `json:"nextPaymentDate"`
	Remaining       *moneyJSON `json:"remaining"`
	PayoffDate      *string    `json:"payoffDate"`
	PaymentsSeen    int        `json:"paymentsSeen"`
	PayingAccountID *string    `json:"payingAccountId"`
}

func loanView(l *loanItem) loanJSON {
	j := loanJSON{ID: l.ID, Name: l.Name, Lender: l.Lender, Source: l.Source, MonthlyPayment: azn(l.MonthlyPayment),
		NextPaymentDate: optional(l.NextPaymentDate), PayoffDate: l.payoffDate(), PaymentsSeen: l.PaymentsSeen,
		PayingAccountID: optional(l.PayingAccountID)}
	if l.Remaining != nil {
		m := azn(*l.Remaining)
		j.Remaining = &m
	}
	return j
}

// dataUpdatedAt is the last successful sync, or null before the first one.
func dataUpdatedAt(u *UserState) *time.Time {
	if u.DataUpdatedAt.IsZero() {
		return nil
	}
	t := u.DataUpdatedAt.In(baku)
	return &t
}
