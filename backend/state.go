package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Everything the backend keeps per user. Bank data is a cache of the last sync;
// the rest (categories you changed, budgets, manual items, read alerts) is the user's own.

type Connection struct {
	ID               string    `json:"id"`
	BankCode         string    `json:"bankCode"`
	ConsentID        string    `json:"consentId"`
	PendingConsentID string    `json:"pendingConsentId,omitempty"` // a renewal waiting for approval
	Status           string    `json:"status"`                     // pending | active | expired | revoked | failed
	ValidUntil       string    `json:"validUntil,omitempty"`
	LastSyncedAt     time.Time `json:"lastSyncedAt"`
}

type StoredAccount struct {
	ID           string `json:"id"`
	ConnectionID string `json:"connectionId"`
	BankCode     string `json:"bankCode"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	MaskedPAN    string `json:"maskedPan,omitempty"`
	Currency     string `json:"currency"`
	IBAN         string `json:"iban"`
	BalanceMinor int64  `json:"balanceMinor"`
}

type StoredTx struct {
	ID           string `json:"id"`
	AccountID    string `json:"accountId"`
	BankCode     string `json:"bankCode"`
	Date         string `json:"date"` // empty for pending
	Status       string `json:"status"`
	AmountMinor  int64  `json:"amountMinor"`
	Currency     string `json:"currency"`
	Counterparty string `json:"counterparty"`
	Remittance   string `json:"remittance"`
	MCC          string `json:"mcc,omitempty"`
}

type RecurringOverride struct {
	Name          *string `json:"name,omitempty"`
	Confirmed     *bool   `json:"confirmed,omitempty"`
	TypicalAmount *int64  `json:"typicalAmount,omitempty"`
	NextDate      *string `json:"nextDate,omitempty"`
	Deleted       bool    `json:"deleted,omitempty"`
}

type ManualRecurring struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // income | expense
	Name        string `json:"name"`
	AmountMinor int64  `json:"amountMinor"` // signed: negative for expenses
	Cadence     string `json:"cadence"`
	NextDate    string `json:"nextDate"`
	CategoryKey string `json:"categoryKey"`
	Confirmed   bool   `json:"confirmed"`
}

// LoanInfo holds what the user told us about a loan: details for a detected one,
// or the whole record for a manual one.
type LoanInfo struct {
	ID              string `json:"id"`
	Manual          bool   `json:"manual"`
	Deleted         bool   `json:"deleted,omitempty"`
	Name            string `json:"name,omitempty"`
	Lender          string `json:"lender,omitempty"`
	MonthlyPayment  *int64 `json:"monthlyPayment,omitempty"`
	Remaining       *int64 `json:"remaining,omitempty"`
	NextPaymentDate string `json:"nextPaymentDate,omitempty"`
}

type Budget struct {
	ID          string `json:"id"`
	CategoryKey string `json:"categoryKey"` // empty = total spending
	Period      string `json:"period"`      // week | month
	LimitMinor  int64  `json:"limitMinor"`
}

type Alert struct {
	ID        string         `json:"id"`
	Key       string         `json:"key"` // dedup key: the same situation never alerts twice
	Type      string         `json:"type"`
	Severity  string         `json:"severity"`
	CreatedAt time.Time      `json:"createdAt"`
	Read      bool           `json:"read"`
	Params    map[string]any `json:"params"`
	Screen    string         `json:"screen"`
	TargetID  string         `json:"targetId,omitempty"`
}

type Profile struct {
	Householder       bool `json:"householder"`
	SchoolAgeChildren bool `json:"schoolAgeChildren"`
}

type UserState struct {
	PSUID         string    `json:"psuId"`
	Name          string    `json:"name"`
	Language      string    `json:"language"`
	Profile       Profile   `json:"profile"`
	DataUpdatedAt time.Time `json:"dataUpdatedAt"`

	Connections []*Connection    `json:"connections"`
	Accounts    []*StoredAccount `json:"accounts"`
	Txs         []*StoredTx      `json:"txs"`

	TxCategory         map[string]string             `json:"txCategory"`       // transaction ID → category key
	MerchantCategory   map[string]string             `json:"merchantCategory"` // merchant name → category key
	RecurringOverrides map[string]*RecurringOverride `json:"recurringOverrides"`
	ManualRecurring    []*ManualRecurring            `json:"manualRecurring"`
	Loans              map[string]*LoanInfo          `json:"loans"`
	Budgets            []*Budget                     `json:"budgets"`
	Alerts             []*Alert                      `json:"alerts"`
	Seq                int                           `json:"seq"`
}

func (u *UserState) nextID(prefix string) string {
	u.Seq++
	return fmt.Sprintf("%s_%d", prefix, u.Seq)
}

func (u *UserState) connection(id string) *Connection {
	for _, c := range u.Connections {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (u *UserState) connectionForBank(bank string) *Connection {
	for _, c := range u.Connections {
		if c.BankCode == bank {
			return c
		}
	}
	return nil
}

func (u *UserState) account(id string) *StoredAccount {
	for _, a := range u.Accounts {
		if a.ID == id {
			return a
		}
	}
	return nil
}

// State is the whole backend's data, saved to one JSON file after every change.
type State struct {
	mu    sync.Mutex
	path  string
	Users map[string]*UserState `json:"users"`
	FX    map[string]string     `json:"fx"` // AZN per unit, e.g. USD → "1.7000"
}

func LoadState(path string) (*State, error) {
	s := &State{path: path, Users: map[string]*UserState{}, FX: map[string]string{"USD": "1.7000", "EUR": "1.9850"}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if s.Users == nil {
		s.Users = map[string]*UserState{}
	}
	return s, nil
}

// save writes atomically: a crash mid-write never leaves a half-written file.
// Callers hold s.mu.
func (s *State) save() error {
	if s.path == "" {
		return nil
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// user returns the state for psuID, creating it on first login. Callers hold s.mu.
func (s *State) user(psuID, name string) *UserState {
	u, ok := s.Users[psuID]
	if !ok {
		u = &UserState{
			PSUID: psuID, Name: name, Language: "az",
			TxCategory: map[string]string{}, MerchantCategory: map[string]string{},
			RecurringOverrides: map[string]*RecurringOverride{}, Loans: map[string]*LoanInfo{},
		}
		// Seed the profile from the persona, so seasonal reminders work in the demo.
		if psuID == "u-rashad" {
			u.Profile = Profile{Householder: true, SchoolAgeChildren: true}
		}
		s.Users[psuID] = u
	}
	return u
}
