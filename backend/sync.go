package main

import (
	"errors"
	"log"
	"time"
)

// connStatusFromConsent maps CBAR consent statuses to the app's connection statuses.
func connStatusFromConsent(s string) string {
	switch s {
	case "valid":
		return "active"
	case "received":
		return "pending"
	case "expired":
		return "expired"
	case "terminatedByTpp":
		return "revoked"
	default:
		return "failed"
	}
}

// refreshConnection asks CBAR for the consent's current status. It returns true
// when the connection just became active (a fresh approval or renewal).
func (app *App) refreshConnection(c *Connection) bool {
	// A renewal in progress: switch over once the new consent is approved.
	if c.PendingConsentID != "" {
		if cs, err := app.cbar.Consent(c.PendingConsentID); err == nil {
			switch cs.ConsentStatus {
			case "valid":
				c.ConsentID, c.PendingConsentID = c.PendingConsentID, ""
				c.Status, c.ValidUntil = "active", cs.ValidUntil
				return true
			case "rejected", "expired", "terminatedByTpp":
				c.PendingConsentID = ""
			}
		}
	}
	if c.Status == "revoked" || c.ConsentID == "" {
		return false
	}
	cs, err := app.cbar.Consent(c.ConsentID)
	if err != nil {
		var ce *CBARError
		if errors.As(err, &ce) && ce.Code == "CONSENT_UNKNOWN" {
			c.Status = "failed" // the bank no longer knows this consent (e.g. mock reset)
		}
		return false
	}
	was := c.Status
	c.Status = connStatusFromConsent(cs.ConsentStatus)
	c.ValidUntil = cs.ValidUntil
	return was != "active" && c.Status == "active"
}

// syncUser pulls accounts, balances and 90 days of transactions for every active
// connection, then regenerates alerts. Callers hold app.state.mu.
func (app *App) syncUser(u *UserState) error {
	if rates, err := app.cbar.FXRates(); err == nil && len(rates) > 0 {
		app.state.FX = rates
	}
	dateFrom := dateOf(app.now()).AddDate(0, 0, -89).Format(dateLayout)
	var firstErr error

	for _, c := range u.Connections {
		app.refreshConnection(c)
		if c.Status != "active" {
			continue
		}
		accounts, err := app.cbar.Accounts(c.ConsentID)
		if err != nil {
			app.handleSyncError(c, err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		var newAccounts []*StoredAccount
		var newTxs []*StoredTx
		failed := false
		for _, ca := range accounts {
			acc := &StoredAccount{ID: ca.ResourceID, ConnectionID: c.ID, BankCode: c.BankCode, Name: ca.Name,
				Type: ca.AccountType, MaskedPAN: ca.MaskedPAN, Currency: ca.Currency, IBAN: ca.IBAN}
			bal, err := app.cbar.AvailableBalance(c.ConsentID, ca.ResourceID)
			if err != nil {
				failed = true
				firstErr = errors.Join(firstErr, err)
				break
			}
			acc.BalanceMinor = parseMinor(bal.Amount)
			booked, pending, err := app.cbar.Transactions(c.ConsentID, ca.ResourceID, dateFrom)
			if err != nil {
				failed = true
				firstErr = errors.Join(firstErr, err)
				break
			}
			add := func(t cbarTx, status string) {
				cp := t.CreditorName
				if cp == "" {
					cp = t.DebtorName
				}
				newTxs = append(newTxs, &StoredTx{
					ID: "txn_" + t.TransactionID, AccountID: ca.ResourceID, BankCode: c.BankCode, Date: t.BookingDate,
					Status: status, AmountMinor: parseMinor(t.TransactionAmount.Amount), Currency: t.TransactionAmount.Currency,
					Counterparty: cp, Remittance: t.Remittance, MCC: t.MCC,
				})
			}
			for _, t := range booked {
				add(t, "booked")
			}
			for _, t := range pending {
				add(t, "pending")
			}
			newAccounts = append(newAccounts, acc)
		}
		if failed {
			continue
		}
		replaceConnectionData(u, c.ID, newAccounts, newTxs)
		c.LastSyncedAt = app.now()
		u.DataUpdatedAt = c.LastSyncedAt
	}

	app.generateAlerts(u)
	if err := app.state.save(); err != nil {
		log.Printf("save: %v", err)
	}
	return firstErr
}

func (app *App) handleSyncError(c *Connection, err error) {
	var ce *CBARError
	if errors.As(err, &ce) {
		switch ce.Code {
		case "CONSENT_EXPIRED":
			c.Status = "expired"
		case "CONSENT_INVALID":
			c.Status = "failed"
		case "CONSENT_UNKNOWN":
			c.Status = "failed"
		}
	}
	log.Printf("sync %s (%s): %v", c.ID, c.BankCode, err)
}

// replaceConnectionData swaps one bank's accounts and transactions for fresh ones.
func replaceConnectionData(u *UserState, connID string, accounts []*StoredAccount, txs []*StoredTx) {
	removeConnectionData(u, connID)
	u.Accounts = append(u.Accounts, accounts...)
	u.Txs = append(u.Txs, txs...)
}

func removeConnectionData(u *UserState, connID string) {
	gone := map[string]bool{}
	keptAcc := u.Accounts[:0]
	for _, a := range u.Accounts {
		if a.ConnectionID == connID {
			gone[a.ID] = true
		} else {
			keptAcc = append(keptAcc, a)
		}
	}
	u.Accounts = keptAcc
	keptTx := u.Txs[:0]
	for _, t := range u.Txs {
		if !gone[t.AccountID] {
			keptTx = append(keptTx, t)
		}
	}
	u.Txs = keptTx
}

// daysLeft until a consent's validUntil, or -1 if unknown.
func daysLeft(validUntil string, now time.Time) int {
	d, err := parseDate(validUntil)
	if err != nil {
		return -1
	}
	return daysBetween(dateOf(now), d)
}
