package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQL persistence. The app works on the in-memory State; after each change
// every user's rows are rewritten in one transaction, so the database always holds
// a consistent copy and the backend restarts from it.

const schema = `
CREATE TABLE IF NOT EXISTS users (
	psu_id              text PRIMARY KEY,
	name                text NOT NULL,
	language            text NOT NULL DEFAULT 'az',
	householder         boolean NOT NULL DEFAULT false,
	school_age_children boolean NOT NULL DEFAULT false,
	data_updated_at     timestamptz,
	seq                 integer NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS connections (
	psu_id             text NOT NULL REFERENCES users ON DELETE CASCADE,
	id                 text NOT NULL,
	bank_code          text NOT NULL,
	consent_id         text NOT NULL DEFAULT '',
	pending_consent_id text NOT NULL DEFAULT '',
	status             text NOT NULL,
	valid_until        date,
	last_synced_at     timestamptz,
	PRIMARY KEY (psu_id, id)
);
CREATE TABLE IF NOT EXISTS accounts (
	psu_id        text NOT NULL REFERENCES users ON DELETE CASCADE,
	id            text NOT NULL,
	connection_id text NOT NULL,
	bank_code     text NOT NULL,
	name          text NOT NULL,
	type          text NOT NULL,
	masked_pan    text NOT NULL DEFAULT '',
	currency      text NOT NULL,
	iban          text NOT NULL,
	balance_minor bigint NOT NULL,
	PRIMARY KEY (psu_id, id)
);
CREATE TABLE IF NOT EXISTS transactions (
	psu_id       text NOT NULL REFERENCES users ON DELETE CASCADE,
	id           text NOT NULL,
	account_id   text NOT NULL,
	bank_code    text NOT NULL,
	booking_date date,
	status       text NOT NULL,
	amount_minor bigint NOT NULL,
	currency     text NOT NULL,
	counterparty text NOT NULL,
	remittance   text NOT NULL,
	mcc          text NOT NULL DEFAULT '',
	PRIMARY KEY (psu_id, id)
);
CREATE INDEX IF NOT EXISTS transactions_account ON transactions (psu_id, account_id, booking_date);
CREATE TABLE IF NOT EXISTS category_overrides (
	psu_id       text NOT NULL REFERENCES users ON DELETE CASCADE,
	scope        text NOT NULL CHECK (scope IN ('transaction', 'merchant')),
	target       text NOT NULL,
	category_key text NOT NULL,
	PRIMARY KEY (psu_id, scope, target)
);
CREATE TABLE IF NOT EXISTS recurring_overrides (
	psu_id         text NOT NULL REFERENCES users ON DELETE CASCADE,
	recurring_id   text NOT NULL,
	name           text,
	confirmed      boolean,
	typical_amount bigint,
	next_date      date,
	deleted        boolean NOT NULL DEFAULT false,
	PRIMARY KEY (psu_id, recurring_id)
);
CREATE TABLE IF NOT EXISTS manual_recurring (
	psu_id       text NOT NULL REFERENCES users ON DELETE CASCADE,
	id           text NOT NULL,
	type         text NOT NULL,
	name         text NOT NULL,
	amount_minor bigint NOT NULL,
	cadence      text NOT NULL,
	next_date    date NOT NULL,
	category_key text NOT NULL,
	confirmed    boolean NOT NULL,
	PRIMARY KEY (psu_id, id)
);
CREATE TABLE IF NOT EXISTS loans (
	psu_id            text NOT NULL REFERENCES users ON DELETE CASCADE,
	id                text NOT NULL,
	manual            boolean NOT NULL,
	deleted           boolean NOT NULL DEFAULT false,
	name              text NOT NULL DEFAULT '',
	lender            text NOT NULL DEFAULT '',
	monthly_payment   bigint,
	remaining         bigint,
	next_payment_date date,
	PRIMARY KEY (psu_id, id)
);
CREATE TABLE IF NOT EXISTS budgets (
	psu_id       text NOT NULL REFERENCES users ON DELETE CASCADE,
	id           text NOT NULL,
	category_key text NOT NULL DEFAULT '',
	period       text NOT NULL,
	limit_minor  bigint NOT NULL,
	PRIMARY KEY (psu_id, id)
);
-- v0.2: amount spent before our data starts, entered by the user.
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS already_spent_minor bigint NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS purchase_plans (
	psu_id       text NOT NULL REFERENCES users ON DELETE CASCADE,
	id           text NOT NULL,
	name         text NOT NULL,
	price_minor  bigint NOT NULL,
	currency     text NOT NULL,
	category_key text NOT NULL,
	status       text NOT NULL CHECK (status IN ('planned', 'dropped', 'bought')),
	created_at   timestamptz NOT NULL,
	updated_at   timestamptz NOT NULL,
	PRIMARY KEY (psu_id, id)
);
CREATE TABLE IF NOT EXISTS alerts (
	psu_id     text NOT NULL REFERENCES users ON DELETE CASCADE,
	id         text NOT NULL,
	key        text NOT NULL,
	type       text NOT NULL,
	severity   text NOT NULL,
	created_at timestamptz NOT NULL,
	read       boolean NOT NULL DEFAULT false,
	params     jsonb NOT NULL,
	screen     text NOT NULL,
	target_id  text NOT NULL DEFAULT '',
	PRIMARY KEY (psu_id, id),
	UNIQUE (psu_id, key)
);
CREATE TABLE IF NOT EXISTS fx_rates (
	currency text PRIMARY KEY,
	rate     text NOT NULL
);
`

// childTables are rewritten on every save, in an order that respects nothing
// but the users foreign key, so any order works.
var childTables = []string{"connections", "accounts", "transactions", "category_overrides",
	"recurring_overrides", "manual_recurring", "loans", "budgets", "purchase_plans", "alerts"}

func OpenPostgres(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	// The database container may still be starting: retry for up to 30 s.
	for i := 0; i < 30; i++ {
		if pool, err = pgxpool.New(ctx, url); err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
			pool.Close()
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}
	return pool, nil
}

// dateArg turns "2026-10-19" into a value for a date column, or NULL when empty.
func dateArg(s string) any {
	if s == "" {
		return nil
	}
	if d, err := parseDate(s); err == nil {
		return d
	}
	return nil
}

func timeArg(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func dateStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, baku).Format(dateLayout)
}

func (s *State) saveDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for cur, rate := range s.FX {
		if _, err := tx.Exec(ctx, `INSERT INTO fx_rates VALUES ($1, $2) ON CONFLICT (currency) DO UPDATE SET rate = $2`, cur, rate); err != nil {
			return err
		}
	}
	for _, u := range s.Users {
		if err := saveUser(ctx, tx, u); err != nil {
			return fmt.Errorf("saving %s: %w", u.PSUID, err)
		}
	}
	return tx.Commit(ctx)
}

func saveUser(ctx context.Context, tx pgx.Tx, u *UserState) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO users (psu_id, name, language, householder, school_age_children, data_updated_at, seq)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (psu_id) DO UPDATE SET name = $2, language = $3, householder = $4,
			school_age_children = $5, data_updated_at = $6, seq = $7`,
		u.PSUID, u.Name, u.Language, u.Profile.Householder, u.Profile.SchoolAgeChildren, timeArg(u.DataUpdatedAt), u.Seq)
	if err != nil {
		return err
	}
	for _, t := range childTables {
		if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE psu_id = $1", u.PSUID); err != nil {
			return err
		}
	}

	copyRows := func(table string, cols []string, rows [][]any) error {
		if len(rows) == 0 {
			return nil
		}
		_, err := tx.CopyFrom(ctx, pgx.Identifier{table}, cols, pgx.CopyFromRows(rows))
		return err
	}

	var rows [][]any
	for _, c := range u.Connections {
		rows = append(rows, []any{u.PSUID, c.ID, c.BankCode, c.ConsentID, c.PendingConsentID, c.Status, dateArg(c.ValidUntil), timeArg(c.LastSyncedAt)})
	}
	if err := copyRows("connections", []string{"psu_id", "id", "bank_code", "consent_id", "pending_consent_id", "status", "valid_until", "last_synced_at"}, rows); err != nil {
		return err
	}

	rows = nil
	for _, a := range u.Accounts {
		rows = append(rows, []any{u.PSUID, a.ID, a.ConnectionID, a.BankCode, a.Name, a.Type, a.MaskedPAN, a.Currency, a.IBAN, a.BalanceMinor})
	}
	if err := copyRows("accounts", []string{"psu_id", "id", "connection_id", "bank_code", "name", "type", "masked_pan", "currency", "iban", "balance_minor"}, rows); err != nil {
		return err
	}

	rows = nil
	for _, t := range u.Txs {
		rows = append(rows, []any{u.PSUID, t.ID, t.AccountID, t.BankCode, dateArg(t.Date), t.Status, t.AmountMinor, t.Currency, t.Counterparty, t.Remittance, t.MCC})
	}
	if err := copyRows("transactions", []string{"psu_id", "id", "account_id", "bank_code", "booking_date", "status", "amount_minor", "currency", "counterparty", "remittance", "mcc"}, rows); err != nil {
		return err
	}

	rows = nil
	for target, key := range u.TxCategory {
		rows = append(rows, []any{u.PSUID, "transaction", target, key})
	}
	for target, key := range u.MerchantCategory {
		rows = append(rows, []any{u.PSUID, "merchant", target, key})
	}
	if err := copyRows("category_overrides", []string{"psu_id", "scope", "target", "category_key"}, rows); err != nil {
		return err
	}

	rows = nil
	for id, o := range u.RecurringOverrides {
		var next any
		if o.NextDate != nil {
			next = dateArg(*o.NextDate)
		}
		rows = append(rows, []any{u.PSUID, id, o.Name, o.Confirmed, o.TypicalAmount, next, o.Deleted})
	}
	if err := copyRows("recurring_overrides", []string{"psu_id", "recurring_id", "name", "confirmed", "typical_amount", "next_date", "deleted"}, rows); err != nil {
		return err
	}

	rows = nil
	for _, m := range u.ManualRecurring {
		rows = append(rows, []any{u.PSUID, m.ID, m.Type, m.Name, m.AmountMinor, m.Cadence, dateArg(m.NextDate), m.CategoryKey, m.Confirmed})
	}
	if err := copyRows("manual_recurring", []string{"psu_id", "id", "type", "name", "amount_minor", "cadence", "next_date", "category_key", "confirmed"}, rows); err != nil {
		return err
	}

	rows = nil
	for _, l := range u.Loans {
		rows = append(rows, []any{u.PSUID, l.ID, l.Manual, l.Deleted, l.Name, l.Lender, l.MonthlyPayment, l.Remaining, dateArg(l.NextPaymentDate)})
	}
	if err := copyRows("loans", []string{"psu_id", "id", "manual", "deleted", "name", "lender", "monthly_payment", "remaining", "next_payment_date"}, rows); err != nil {
		return err
	}

	rows = nil
	for _, b := range u.Budgets {
		rows = append(rows, []any{u.PSUID, b.ID, b.CategoryKey, b.Period, b.LimitMinor, b.AlreadySpentMinor})
	}
	if err := copyRows("budgets", []string{"psu_id", "id", "category_key", "period", "limit_minor", "already_spent_minor"}, rows); err != nil {
		return err
	}

	rows = nil
	for _, p := range u.PurchasePlans {
		rows = append(rows, []any{u.PSUID, p.ID, p.Name, p.PriceMinor, p.Currency, p.CategoryKey, p.Status, p.CreatedAt, p.UpdatedAt})
	}
	if err := copyRows("purchase_plans", []string{"psu_id", "id", "name", "price_minor", "currency", "category_key", "status", "created_at", "updated_at"}, rows); err != nil {
		return err
	}

	rows = nil
	for _, al := range u.Alerts {
		params, err := json.Marshal(al.Params)
		if err != nil {
			return err
		}
		rows = append(rows, []any{u.PSUID, al.ID, al.Key, al.Type, al.Severity, al.CreatedAt, al.Read, string(params), al.Screen, al.TargetID})
	}
	return copyRows("alerts", []string{"psu_id", "id", "key", "type", "severity", "created_at", "read", "params", "screen", "target_id"}, rows)
}

// LoadStateDB reads everything back into memory at startup.
func LoadStateDB(ctx context.Context, db *pgxpool.Pool) (*State, error) {
	s := &State{db: db, Users: map[string]*UserState{}, FX: map[string]string{"USD": "1.7000", "EUR": "1.9850"}}

	fx, err := db.Query(ctx, `SELECT currency, rate FROM fx_rates`)
	if err != nil {
		return nil, err
	}
	for fx.Next() {
		var cur, rate string
		if err := fx.Scan(&cur, &rate); err != nil {
			return nil, err
		}
		s.FX[cur] = rate
	}
	fx.Close()

	users, err := db.Query(ctx, `SELECT psu_id, name, language, householder, school_age_children, data_updated_at, seq FROM users`)
	if err != nil {
		return nil, err
	}
	for users.Next() {
		u := &UserState{TxCategory: map[string]string{}, MerchantCategory: map[string]string{},
			RecurringOverrides: map[string]*RecurringOverride{}, Loans: map[string]*LoanInfo{}}
		var updated *time.Time
		if err := users.Scan(&u.PSUID, &u.Name, &u.Language, &u.Profile.Householder, &u.Profile.SchoolAgeChildren, &updated, &u.Seq); err != nil {
			return nil, err
		}
		if updated != nil {
			u.DataUpdatedAt = *updated
		}
		s.Users[u.PSUID] = u
	}
	users.Close()

	// each runs a query and calls scan for every row.
	each := func(query string, scan func(rows pgx.Rows) error) error {
		rows, err := db.Query(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}

	err = each(`SELECT psu_id, id, bank_code, consent_id, pending_consent_id, status, valid_until, last_synced_at FROM connections ORDER BY id`,
		func(r pgx.Rows) error {
			var psu string
			c := &Connection{}
			var valid, synced *time.Time
			if err := r.Scan(&psu, &c.ID, &c.BankCode, &c.ConsentID, &c.PendingConsentID, &c.Status, &valid, &synced); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			c.ValidUntil = dateStr(valid)
			if synced != nil {
				c.LastSyncedAt = *synced
			}
			u.Connections = append(u.Connections, c)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, id, connection_id, bank_code, name, type, masked_pan, currency, iban, balance_minor FROM accounts ORDER BY id`,
		func(r pgx.Rows) error {
			var psu string
			a := &StoredAccount{}
			if err := r.Scan(&psu, &a.ID, &a.ConnectionID, &a.BankCode, &a.Name, &a.Type, &a.MaskedPAN, &a.Currency, &a.IBAN, &a.BalanceMinor); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			u.Accounts = append(u.Accounts, a)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, id, account_id, bank_code, booking_date, status, amount_minor, currency, counterparty, remittance, mcc FROM transactions ORDER BY id`,
		func(r pgx.Rows) error {
			var psu string
			t := &StoredTx{}
			var booked *time.Time
			if err := r.Scan(&psu, &t.ID, &t.AccountID, &t.BankCode, &booked, &t.Status, &t.AmountMinor, &t.Currency, &t.Counterparty, &t.Remittance, &t.MCC); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			t.Date = dateStr(booked)
			u.Txs = append(u.Txs, t)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, scope, target, category_key FROM category_overrides`,
		func(r pgx.Rows) error {
			var psu, scope, target, key string
			if err := r.Scan(&psu, &scope, &target, &key); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			if scope == "merchant" {
				u.MerchantCategory[target] = key
			} else {
				u.TxCategory[target] = key
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, recurring_id, name, confirmed, typical_amount, next_date, deleted FROM recurring_overrides`,
		func(r pgx.Rows) error {
			var psu, id string
			o := &RecurringOverride{}
			var next *time.Time
			if err := r.Scan(&psu, &id, &o.Name, &o.Confirmed, &o.TypicalAmount, &next, &o.Deleted); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			if next != nil {
				d := dateStr(next)
				o.NextDate = &d
			}
			u.RecurringOverrides[id] = o
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, id, type, name, amount_minor, cadence, next_date, category_key, confirmed FROM manual_recurring ORDER BY id`,
		func(r pgx.Rows) error {
			var psu string
			m := &ManualRecurring{}
			var next *time.Time
			if err := r.Scan(&psu, &m.ID, &m.Type, &m.Name, &m.AmountMinor, &m.Cadence, &next, &m.CategoryKey, &m.Confirmed); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			m.NextDate = dateStr(next)
			u.ManualRecurring = append(u.ManualRecurring, m)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, id, manual, deleted, name, lender, monthly_payment, remaining, next_payment_date FROM loans`,
		func(r pgx.Rows) error {
			var psu string
			l := &LoanInfo{}
			var next *time.Time
			if err := r.Scan(&psu, &l.ID, &l.Manual, &l.Deleted, &l.Name, &l.Lender, &l.MonthlyPayment, &l.Remaining, &next); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			l.NextPaymentDate = dateStr(next)
			u.Loans[l.ID] = l
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, id, name, price_minor, currency, category_key, status, created_at, updated_at FROM purchase_plans ORDER BY created_at, id`,
		func(r pgx.Rows) error {
			var psu string
			p := &PurchasePlan{}
			if err := r.Scan(&psu, &p.ID, &p.Name, &p.PriceMinor, &p.Currency, &p.CategoryKey, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			u.PurchasePlans = append(u.PurchasePlans, p)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, id, category_key, period, limit_minor, already_spent_minor FROM budgets ORDER BY id`,
		func(r pgx.Rows) error {
			var psu string
			b := &Budget{}
			if err := r.Scan(&psu, &b.ID, &b.CategoryKey, &b.Period, &b.LimitMinor, &b.AlreadySpentMinor); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			u.Budgets = append(u.Budgets, b)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = each(`SELECT psu_id, id, key, type, severity, created_at, read, params, screen, target_id FROM alerts ORDER BY created_at, id`,
		func(r pgx.Rows) error {
			var psu string
			al := &Alert{}
			var params []byte
			if err := r.Scan(&psu, &al.ID, &al.Key, &al.Type, &al.Severity, &al.CreatedAt, &al.Read, &params, &al.Screen, &al.TargetID); err != nil {
				return err
			}
			u := s.Users[psu]
			if u == nil {
				return nil
			}
			if err := json.Unmarshal(params, &al.Params); err != nil {
				return err
			}
			u.Alerts = append(u.Alerts, al)
			return nil
		})
	if err != nil {
		return nil, err
	}
	return s, nil
}
