package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func (srv *Server) listBanks(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.RLock()
	defer srv.store.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string][]Bank{"banks": srv.store.banks})
}

type consentRequest struct {
	BankCode           string          `json:"bankCode"`
	Access             json.RawMessage `json:"access"`
	RecurringIndicator bool            `json:"recurringIndicator"`
	ValidUntil         string          `json:"validUntil"`
	FrequencyPerDay    int             `json:"frequencyPerDay"`
}

func (srv *Server) createConsent(w http.ResponseWriter, r *http.Request) {
	psuID := r.Header.Get("PSU-ID")
	redirect := r.Header.Get("TPP-Redirect-URI")
	if psuID == "" || redirect == "" {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "PSU-ID and TPP-Redirect-URI headers are required")
		return
	}
	var req consentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Invalid JSON body: "+err.Error())
		return
	}

	s := srv.store
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.user(psuID); !ok {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Unknown PSU-ID "+psuID+"; see GET /mock/users")
		return
	}
	if _, ok := s.bank(req.BankCode); !ok {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Unknown bankCode "+strconv.Quote(req.BankCode)+"; see GET /v1/banks")
		return
	}
	today := s.today()
	validUntil := today.AddDate(0, 0, 90)
	if req.ValidUntil != "" {
		d, err := parseDate(req.ValidUntil)
		if err != nil || d.Before(today) {
			writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "validUntil must be a date (YYYY-MM-DD) not in the past")
			return
		}
		validUntil = d
	}
	freq := req.FrequencyPerDay
	if freq <= 0 {
		freq = 4
	}

	c := &Consent{
		ID:              newConsentID(s.consents),
		PSUID:           psuID,
		BankCode:        req.BankCode,
		Status:          StatusReceived,
		ValidUntil:      validUntil,
		FrequencyPerDay: freq,
		LastActionDate:  today,
		RedirectURI:     redirect,
		reads:           map[string]int{},
	}
	s.consents[c.ID] = c

	writeJSON(w, http.StatusCreated, map[string]any{
		"consentId":     c.ID,
		"consentStatus": c.Status,
		"_links": map[string]any{
			"scaRedirect": map[string]string{"href": srv.baseURL(r) + "/mock-sca/" + c.ID},
			"status":      map[string]string{"href": "/v1/consents/" + c.ID + "/status"},
		},
	})
}

// lookupConsent finds a consent and checks it belongs to the caller when PSU-ID is sent.
// Callers hold the write lock.
func (srv *Server) lookupConsent(w http.ResponseWriter, r *http.Request, id string) *Consent {
	c, ok := srv.store.consents[id]
	if !ok || (r.Header.Get("PSU-ID") != "" && r.Header.Get("PSU-ID") != c.PSUID) {
		writeError(w, http.StatusForbidden, "CONSENT_UNKNOWN", "Consent "+id+" does not exist or belongs to another user")
		return nil
	}
	srv.store.refreshStatus(c)
	return c
}

func (srv *Server) consentStatus(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	if c := srv.lookupConsent(w, r, r.PathValue("consentId")); c != nil {
		writeJSON(w, http.StatusOK, map[string]string{"consentStatus": c.Status})
	}
}

func (srv *Server) getConsent(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	if c := srv.lookupConsent(w, r, r.PathValue("consentId")); c != nil {
		writeJSON(w, http.StatusOK, c.toJSON())
	}
}

func (srv *Server) deleteConsent(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	if c := srv.lookupConsent(w, r, r.PathValue("consentId")); c != nil {
		c.Status = StatusTerminated
		w.WriteHeader(http.StatusNoContent)
	}
}

// usableConsent validates the Consent-ID header for an account read and counts the read.
// Callers hold the write lock.
func (srv *Server) usableConsent(w http.ResponseWriter, r *http.Request) *Consent {
	id := r.Header.Get("Consent-ID")
	if id == "" {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Missing Consent-ID header")
		return nil
	}
	c := srv.lookupConsent(w, r, id)
	if c == nil {
		return nil
	}
	switch c.Status {
	case StatusValid:
	case StatusExpired:
		writeError(w, http.StatusUnauthorized, "CONSENT_EXPIRED", "Consent "+c.ID+" expired on "+c.ValidUntil.Format(dateLayout))
		return nil
	default:
		writeError(w, http.StatusUnauthorized, "CONSENT_INVALID", "Consent "+c.ID+" is "+c.Status+", not valid")
		return nil
	}
	today := srv.store.today()
	key := today.Format(dateLayout)
	c.reads[key]++
	if srv.enforceFreq && c.reads[key] > c.FrequencyPerDay {
		writeError(w, http.StatusTooManyRequests, "ACCESS_EXCEEDED", "More than "+strconv.Itoa(c.FrequencyPerDay)+" reads today for consent "+c.ID)
		return nil
	}
	c.LastActionDate = today
	return c
}

// consentAccount resolves {resourceId} and checks the consent covers it.
func (srv *Server) consentAccount(w http.ResponseWriter, r *http.Request) (*Account, *Consent) {
	c := srv.usableConsent(w, r)
	if c == nil {
		return nil, nil
	}
	a := srv.store.account(r.PathValue("resourceId"))
	if a == nil || a.PSUID != c.PSUID || a.BankCode != c.BankCode {
		writeError(w, http.StatusNotFound, "RESOURCE_UNKNOWN", "Account "+r.PathValue("resourceId")+" is not covered by consent "+c.ID)
		return nil, nil
	}
	return a, c
}

func (srv *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	c := srv.usableConsent(w, r)
	if c == nil {
		return
	}
	out := []accountJSON{}
	for _, a := range srv.store.consentAccounts(c) {
		out = append(out, a.toJSON())
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

func (srv *Server) getBalances(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	a, _ := srv.consentAccount(w, r)
	if a == nil {
		return
	}
	b := srv.store.balances(a)
	today := srv.store.today()
	bal := func(typ string, m Money, ref time.Time) map[string]any {
		return map[string]any{
			"balanceType":   typ,
			"balanceAmount": amountJSON{Currency: a.Currency, Amount: m.String()},
			"referenceDate": ref.Format(dateLayout),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account": map[string]string{"iban": a.IBAN},
		"balances": []any{
			bal("interimAvailable", b.interimAvailable, today),
			bal("closingBooked", b.closingBooked, today.AddDate(0, 0, -1)),
		},
	})
}

func (srv *Server) getTransactions(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	a, _ := srv.consentAccount(w, r)
	if a == nil {
		return
	}

	q := r.URL.Query()
	today := srv.store.today()
	if q.Get("dateFrom") == "" {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "dateFrom query parameter is required (YYYY-MM-DD)")
		return
	}
	from, err := parseDate(q.Get("dateFrom"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "dateFrom must be YYYY-MM-DD")
		return
	}
	to := today
	if v := q.Get("dateTo"); v != "" {
		if to, err = parseDate(v); err != nil {
			writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "dateTo must be YYYY-MM-DD")
			return
		}
	}
	if earliest := today.AddDate(0, 0, -historyDays); from.Before(earliest) {
		writeError(w, http.StatusBadRequest, "PERIOD_INVALID", "dateFrom cannot be earlier than "+earliest.Format(dateLayout)+" (90 days)")
		return
	}
	if from.After(to) {
		writeError(w, http.StatusBadRequest, "PERIOD_INVALID", "dateFrom is after dateTo")
		return
	}
	status := q.Get("bookingStatus")
	if status == "" {
		status = "booked"
	}
	if status != "booked" && status != "pending" && status != "both" {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "bookingStatus must be booked, pending or both")
		return
	}

	booked, pending := srv.store.transactions(a, from, to)
	list := map[string][]txJSON{}
	if status != "pending" {
		list["booked"] = toJSONList(booked, a.Currency)
	}
	if status != "booked" {
		list["pending"] = toJSONList(pending, a.Currency)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account":      map[string]string{"iban": a.IBAN},
		"transactions": list,
	})
}

func toJSONList(txs []*Transaction, currency string) []txJSON {
	out := make([]txJSON, 0, len(txs))
	for _, t := range txs {
		out = append(out, t.toJSON(currency))
	}
	return out
}
