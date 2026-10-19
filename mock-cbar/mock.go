package main

import (
	"encoding/json"
	"net/http"
)

func (srv *Server) mockUsers(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.RLock()
	defer srv.store.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string][]User{"users": srv.store.users})
}

// mockDecide approves or rejects a consent without the SCA page, for scripts and tests.
func (srv *Server) mockDecide(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		srv.store.mu.Lock()
		defer srv.store.mu.Unlock()
		c := srv.lookupConsent(w, r, r.PathValue("consentId"))
		if c == nil {
			return
		}
		if c.Status != StatusReceived {
			writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Consent "+c.ID+" is "+c.Status+"; only a received consent can be approved or rejected")
			return
		}
		c.Status = status
		writeJSON(w, http.StatusOK, c.toJSON())
	}
}

func (srv *Server) mockExpire(w http.ResponseWriter, r *http.Request) {
	s := srv.store
	s.mu.Lock()
	defer s.mu.Unlock()
	c := srv.lookupConsent(w, r, r.PathValue("consentId"))
	if c == nil {
		return
	}
	c.Status = StatusExpired
	c.ValidUntil = s.today().AddDate(0, 0, -1)
	writeJSON(w, http.StatusOK, c.toJSON())
}

type injectRequest struct {
	ResourceID        string     `json:"resourceId"`
	Status            string     `json:"status"`
	TransactionAmount amountJSON `json:"transactionAmount"`
	CreditorName      string     `json:"creditorName"`
	DebtorName        string     `json:"debtorName"`
	Remittance        string     `json:"remittanceInformationUnstructured"`
	MCC               string     `json:"merchantCategoryCode"`
}

func (srv *Server) mockInject(w http.ResponseWriter, r *http.Request) {
	var req injectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Invalid JSON body: "+err.Error())
		return
	}
	s := srv.store
	s.mu.Lock()
	defer s.mu.Unlock()

	a := s.account(req.ResourceID)
	if a == nil {
		writeError(w, http.StatusNotFound, "RESOURCE_UNKNOWN", "No account "+req.ResourceID)
		return
	}
	if req.Status == "" {
		req.Status = "pending"
	}
	if req.Status != "pending" && req.Status != "booked" {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "status must be pending or booked")
		return
	}
	amt, err := parseMoney(req.TransactionAmount.Amount)
	if err != nil || amt == 0 {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "transactionAmount.amount must be a non-zero amount like \"-120.00\"")
		return
	}
	if req.TransactionAmount.Currency != "" && req.TransactionAmount.Currency != a.Currency {
		writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Account "+a.ResourceID+" is in "+a.Currency)
		return
	}
	counterparty := req.CreditorName
	if amt > 0 || counterparty == "" {
		counterparty = req.DebtorName
	}
	if counterparty == "" {
		counterparty = req.CreditorName
	}

	t := &Transaction{AccountID: a.ResourceID, Booked: req.Status == "booked", Amount: amt, Counterparty: counterparty, Remittance: req.Remittance, MCC: req.MCC}
	if t.Booked {
		t.Date = s.today()
	}
	s.inject(t)
	writeJSON(w, http.StatusCreated, t.toJSON(a.Currency))
}

func (srv *Server) mockReset(w http.ResponseWriter, r *http.Request) {
	srv.store.Reset()
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// mockFXRates stands in for CBAR's official daily rates: manat per one unit.
func (srv *Server) mockFXRates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"USD": "1.7000", "EUR": "1.9850"})
}
