package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	store         *Store
	enforceFreq   bool
	publicBaseURL string // used for absolute scaRedirect links; derived from the request when empty
}

func (srv *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// /v1 imitates CBAR.
	mux.HandleFunc("GET /v1/banks", srv.listBanks)
	mux.HandleFunc("POST /v1/consents", srv.createConsent)
	mux.HandleFunc("GET /v1/consents/{consentId}/status", srv.consentStatus)
	mux.HandleFunc("GET /v1/consents/{consentId}", srv.getConsent)
	mux.HandleFunc("DELETE /v1/consents/{consentId}", srv.deleteConsent)
	mux.HandleFunc("GET /v1/accounts", srv.listAccounts)
	mux.HandleFunc("GET /v1/accounts/{resourceId}/balances", srv.getBalances)
	mux.HandleFunc("GET /v1/accounts/{resourceId}/transactions", srv.getTransactions)

	// Demo tooling. The app's normal flows never call these.
	mux.HandleFunc("GET /mock-sca/{consentId}", srv.scaPage)
	mux.HandleFunc("POST /mock-sca/{consentId}", srv.scaDecision)
	mux.HandleFunc("GET /mock/users", srv.mockUsers)
	mux.HandleFunc("POST /mock/consents/{consentId}/approve", srv.mockDecide(StatusValid))
	mux.HandleFunc("POST /mock/consents/{consentId}/reject", srv.mockDecide(StatusRejected))
	mux.HandleFunc("POST /mock/consents/{consentId}/expire", srv.mockExpire)
	mux.HandleFunc("POST /mock/transactions", srv.mockInject)
	mux.HandleFunc("POST /mock/reset", srv.mockReset)
	mux.HandleFunc("GET /mock/fx-rates", srv.mockFXRates)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "RESOURCE_UNKNOWN", "No endpoint "+r.Method+" "+r.URL.Path)
	})

	return logRequests(cors(requireRequestID(mux)))
}

// requireRequestID enforces X-Request-ID on every /v1 call and echoes it back.
func requireRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				writeError(w, http.StatusBadRequest, "FORMAT_ERROR", "Missing X-Request-ID header")
				return
			}
			w.Header().Set("X-Request-ID", id)
		}
		next.ServeHTTP(w, r)
	})
}

// cors lets the app call the mock from a browser too (Expo web, Swagger tools).
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, PSU-ID, Consent-ID, TPP-Redirect-URI")
		h.Set("Access-Control-Expose-Headers", "X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), rec.status, time.Since(start).Round(time.Microsecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

type tppMessage struct {
	Category string `json:"category"`
	Code     string `json:"code"`
	Text     string `json:"text"`
}

func writeError(w http.ResponseWriter, status int, code, text string) {
	writeJSON(w, status, map[string][]tppMessage{"tppMessages": {{Category: "ERROR", Code: code, Text: text}}})
}

// baseURL is the server's public origin, e.g. https://cibim-api.example.az.
func (srv *Server) baseURL(r *http.Request) string {
	if srv.publicBaseURL != "" {
		return strings.TrimRight(srv.publicBaseURL, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host
}
