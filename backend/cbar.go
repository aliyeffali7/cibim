package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// CBARClient talks to the CBAR Open Banking gateway (the mock in the demo),
// following the Mock CBAR API Contract.
type CBARClient struct {
	baseURL string
	http    *http.Client
}

func NewCBARClient(baseURL string) *CBARClient {
	return &CBARClient{baseURL: baseURL, http: &http.Client{Timeout: 15 * time.Second}}
}

// CBARError is a tppMessages error from the gateway.
type CBARError struct {
	Status int
	Code   string
	Text   string
}

func (e *CBARError) Error() string { return fmt.Sprintf("cbar %d %s: %s", e.Status, e.Code, e.Text) }

func requestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (c *CBARClient) do(method, path string, headers map[string]string, body, out any) error {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.baseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("X-Request-ID", requestID())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cbar unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var e struct {
			TPPMessages []struct{ Code, Text string } `json:"tppMessages"`
		}
		_ = json.Unmarshal(raw, &e)
		ce := &CBARError{Status: resp.StatusCode}
		if len(e.TPPMessages) > 0 {
			ce.Code, ce.Text = e.TPPMessages[0].Code, e.TPPMessages[0].Text
		}
		return ce
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

type cbarBank struct {
	BankCode string `json:"bankCode"`
	Name     string `json:"name"`
}

func (c *CBARClient) Banks() ([]cbarBank, error) {
	var out struct{ Banks []cbarBank }
	err := c.do("GET", "/v1/banks", nil, nil, &out)
	return out.Banks, err
}

type cbarConsent struct {
	ConsentID     string `json:"consentId"`
	ConsentStatus string `json:"consentStatus"`
	ValidUntil    string `json:"validUntil"`
	Links         struct {
		SCARedirect struct {
			Href string `json:"href"`
		} `json:"scaRedirect"`
	} `json:"_links"`
}

func (c *CBARClient) CreateConsent(psuID, bankCode, redirect string) (cbarConsent, error) {
	var out cbarConsent
	err := c.do("POST", "/v1/consents",
		map[string]string{"PSU-ID": psuID, "TPP-Redirect-URI": redirect},
		map[string]any{"bankCode": bankCode, "access": map[string]string{"allPsd2": "allAccounts"}, "recurringIndicator": true, "frequencyPerDay": 4},
		&out)
	return out, err
}

func (c *CBARClient) Consent(id string) (cbarConsent, error) {
	var out cbarConsent
	err := c.do("GET", "/v1/consents/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

func (c *CBARClient) DeleteConsent(id string) error {
	return c.do("DELETE", "/v1/consents/"+url.PathEscape(id), nil, nil, nil)
}

type cbarAccount struct {
	ResourceID  string `json:"resourceId"`
	BankCode    string `json:"bankCode"`
	IBAN        string `json:"iban"`
	Currency    string `json:"currency"`
	Name        string `json:"name"`
	AccountType string `json:"accountType"`
	MaskedPAN   string `json:"maskedPan"`
}

func (c *CBARClient) Accounts(consentID string) ([]cbarAccount, error) {
	var out struct{ Accounts []cbarAccount }
	err := c.do("GET", "/v1/accounts", map[string]string{"Consent-ID": consentID}, nil, &out)
	return out.Accounts, err
}

type cbarAmount struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// AvailableBalance returns interimAvailable, i.e. "money I have now".
func (c *CBARClient) AvailableBalance(consentID, accountID string) (cbarAmount, error) {
	var out struct {
		Balances []struct {
			BalanceType   string     `json:"balanceType"`
			BalanceAmount cbarAmount `json:"balanceAmount"`
		}
	}
	err := c.do("GET", "/v1/accounts/"+url.PathEscape(accountID)+"/balances", map[string]string{"Consent-ID": consentID}, nil, &out)
	for _, b := range out.Balances {
		if b.BalanceType == "interimAvailable" {
			return b.BalanceAmount, err
		}
	}
	if err == nil && len(out.Balances) > 0 {
		return out.Balances[0].BalanceAmount, nil
	}
	return cbarAmount{}, err
}

type cbarTx struct {
	TransactionID     string     `json:"transactionId"`
	BookingDate       string     `json:"bookingDate"`
	TransactionAmount cbarAmount `json:"transactionAmount"`
	CreditorName      string     `json:"creditorName"`
	DebtorName        string     `json:"debtorName"`
	Remittance        string     `json:"remittanceInformationUnstructured"`
	MCC               string     `json:"merchantCategoryCode"`
}

func (c *CBARClient) Transactions(consentID, accountID, dateFrom string) (booked, pending []cbarTx, err error) {
	var out struct {
		Transactions struct {
			Booked  []cbarTx `json:"booked"`
			Pending []cbarTx `json:"pending"`
		}
	}
	path := "/v1/accounts/" + url.PathEscape(accountID) + "/transactions?bookingStatus=both&dateFrom=" + dateFrom
	err = c.do("GET", path, map[string]string{"Consent-ID": consentID}, nil, &out)
	return out.Transactions.Booked, out.Transactions.Pending, err
}

// The calls below use /mock demo tooling: demo login and CBAR's daily FX rates.

type cbarUser struct {
	PSUID   string   `json:"psuId"`
	Name    string   `json:"displayName"`
	Persona string   `json:"persona"`
	Banks   []string `json:"banks"`
}

func (c *CBARClient) DemoUsers() ([]cbarUser, error) {
	var out struct{ Users []cbarUser }
	err := c.do("GET", "/mock/users", nil, nil, &out)
	return out.Users, err
}

func (c *CBARClient) ApproveConsent(id string) error {
	return c.do("POST", "/mock/consents/"+url.PathEscape(id)+"/approve", nil, nil, nil)
}

func (c *CBARClient) FXRates() (map[string]string, error) {
	out := map[string]string{}
	err := c.do("GET", "/mock/fx-rates", nil, nil, &out)
	return out, err
}
