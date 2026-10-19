package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Baku is UTC+4 all year. A fixed zone avoids depending on tzdata in the container.
var baku = time.FixedZone("AZT", 4*60*60)

const dateLayout = "2006-01-02"

// Money is an amount in qəpik (1/100 of a manat). Floats never touch money.
type Money int64

func (m Money) String() string {
	sign := ""
	v := int64(m)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// parseMoney accepts "-42.50", "120", "14.9". More than two decimals is an error.
func parseMoney(s string) (Money, error) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	v := Money(w*100 + f)
	if neg {
		v = -v
	}
	return v, nil
}

// dateOf truncates a time to midnight in Baku.
func dateOf(t time.Time) time.Time {
	t = t.In(baku)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, baku)
}

func parseDate(s string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, s, baku)
}

type Bank struct {
	Code     string `json:"bankCode"`
	Name     string `json:"name"`
	ibanCode string // 4-char code inside the IBAN; mock codes start with M
}

type User struct {
	PSUID   string   `json:"psuId"`
	Name    string   `json:"displayName"`
	Persona string   `json:"persona"`
	Banks   []string `json:"banks"`
}

type Account struct {
	ResourceID string
	PSUID      string
	BankCode   string
	IBAN       string
	Currency   string
	Name       string
	Type       string // CARD, CURRENT, SAVINGS
	MaskedPAN  string
	Status     string
	Opening    Money // balance before the first seeded transaction
}

type Transaction struct {
	ID           string
	AccountID    string
	Booked       bool
	Date         time.Time // zero for pending
	Amount       Money
	Counterparty string // creditor for outgoing, debtor for incoming
	Remittance   string
	MCC          string
	seq          int // stable ordering within a day
}

const (
	StatusReceived   = "received"
	StatusValid      = "valid"
	StatusRejected   = "rejected"
	StatusExpired    = "expired"
	StatusTerminated = "terminatedByTpp"
)

type Consent struct {
	ID              string
	PSUID           string
	BankCode        string
	Status          string
	ValidUntil      time.Time
	FrequencyPerDay int
	LastActionDate  time.Time
	RedirectURI     string
	reads           map[string]int // reads per day, keyed by date
}

// JSON shapes from the contract.

type amountJSON struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type txJSON struct {
	TransactionID     string     `json:"transactionId"`
	BookingDate       string     `json:"bookingDate,omitempty"`
	ValueDate         string     `json:"valueDate,omitempty"`
	TransactionAmount amountJSON `json:"transactionAmount"`
	CreditorName      string     `json:"creditorName,omitempty"`
	DebtorName        string     `json:"debtorName,omitempty"`
	Remittance        string     `json:"remittanceInformationUnstructured,omitempty"`
	MCC               string     `json:"merchantCategoryCode,omitempty"`
}

func (t *Transaction) toJSON(currency string) txJSON {
	j := txJSON{
		TransactionID:     t.ID,
		TransactionAmount: amountJSON{Currency: currency, Amount: t.Amount.String()},
		Remittance:        t.Remittance,
		MCC:               t.MCC,
	}
	if t.Booked {
		j.BookingDate = t.Date.Format(dateLayout)
		j.ValueDate = j.BookingDate
	}
	if t.Amount < 0 {
		j.CreditorName = t.Counterparty
	} else {
		j.DebtorName = t.Counterparty
	}
	return j
}

type accountJSON struct {
	ResourceID  string `json:"resourceId"`
	BankCode    string `json:"bankCode"`
	IBAN        string `json:"iban"`
	Currency    string `json:"currency"`
	Name        string `json:"name"`
	AccountType string `json:"accountType"`
	MaskedPAN   string `json:"maskedPan,omitempty"`
	Status      string `json:"status"`
}

func (a *Account) toJSON() accountJSON {
	return accountJSON{a.ResourceID, a.BankCode, a.IBAN, a.Currency, a.Name, a.Type, a.MaskedPAN, a.Status}
}

type consentJSON struct {
	ConsentID       string `json:"consentId"`
	PSUID           string `json:"psuId"`
	BankCode        string `json:"bankCode"`
	ConsentStatus   string `json:"consentStatus"`
	ValidUntil      string `json:"validUntil"`
	FrequencyPerDay int    `json:"frequencyPerDay"`
	LastActionDate  string `json:"lastActionDate"`
}

func (c *Consent) toJSON() consentJSON {
	return consentJSON{c.ID, c.PSUID, c.BankCode, c.Status, c.ValidUntil.Format(dateLayout), c.FrequencyPerDay, c.LastActionDate.Format(dateLayout)}
}
