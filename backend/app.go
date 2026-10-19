package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type App struct {
	state       *State
	cbar        *CBARClient
	now         func() time.Time
	redirectURL string        // where the bank sends the user back; "{id}" becomes the connection ID
	staleAfter  time.Duration // auto-sync when data is older than this
	demoUsers   []cbarUser    // cached from the mock
}

var staticBanks = map[string]string{"ABB": "ABB", "XALQ": "Xalq Bank", "PASHA": "PAŞA Bank", "LEO": "Leobank"}

func bankName(code string) string {
	if n, ok := staticBanks[code]; ok {
		return n
	}
	return code
}

// handler is an API handler that runs with the state locked and the caller's user loaded.
type handler func(w http.ResponseWriter, r *http.Request, u *UserState)

func (app *App) routes() http.Handler {
	mux := http.NewServeMux()
	p := "/api/v1"

	mux.HandleFunc("GET "+p+"/auth/demo-users", app.demoUsersHandler)
	mux.HandleFunc("POST "+p+"/auth/demo-login", app.demoLogin)

	auth := func(pattern string, h handler) { mux.HandleFunc(pattern, app.withUser(h)) }

	auth("GET "+p+"/banks", app.listBanks)
	auth("POST "+p+"/connections", app.createConnection)
	auth("GET "+p+"/connections", app.listConnections)
	auth("GET "+p+"/connections/{id}", app.getConnection)
	auth("POST "+p+"/connections/{id}/renew", app.renewConnection)
	auth("DELETE "+p+"/connections/{id}", app.deleteConnection)
	auth("POST "+p+"/sync", app.syncHandler)

	auth("GET "+p+"/home", app.home)
	auth("GET "+p+"/accounts", app.listAccounts)
	auth("GET "+p+"/accounts/{id}", app.getAccount)
	auth("GET "+p+"/transactions", app.listTransactions)
	auth("GET "+p+"/transactions/{id}", app.getTransaction)
	auth("PATCH "+p+"/transactions/{id}", app.patchTransaction)
	auth("GET "+p+"/categories", app.listCategories)
	auth("GET "+p+"/summary", app.summary)

	auth("GET "+p+"/recurring", app.listRecurring)
	auth("GET "+p+"/recurring/calendar", app.recurringCalendar)
	auth("POST "+p+"/recurring", app.createRecurring)
	auth("PATCH "+p+"/recurring/{id}", app.patchRecurring)
	auth("DELETE "+p+"/recurring/{id}", app.deleteRecurring)

	auth("GET "+p+"/loans", app.listLoans)
	auth("POST "+p+"/loans", app.createLoan)
	auth("PATCH "+p+"/loans/{id}", app.patchLoan)
	auth("DELETE "+p+"/loans/{id}", app.deleteLoan)

	auth("GET "+p+"/budgets", app.listBudgets)
	auth("POST "+p+"/budgets", app.createBudget)
	auth("PATCH "+p+"/budgets/{id}", app.patchBudget)
	auth("DELETE "+p+"/budgets/{id}", app.deleteBudget)

	auth("POST "+p+"/purchase-plans/check", app.checkPurchasePlan)
	auth("POST "+p+"/purchase-plans", app.createPurchasePlan)
	auth("GET "+p+"/purchase-plans", app.listPurchasePlans)
	auth("PATCH "+p+"/purchase-plans/{id}", app.patchPurchasePlan)

	auth("GET "+p+"/safe-to-spend", app.safeToSpendHandler)
	auth("GET "+p+"/streak", app.streakHandler)

	auth("GET "+p+"/alerts", app.listAlerts)
	auth("POST "+p+"/alerts/read-all", app.readAllAlerts)
	auth("POST "+p+"/alerts/{id}/read", app.readAlert)

	auth("GET "+p+"/me", app.getMe)
	auth("PATCH "+p+"/me", app.patchMe)

	// Demo shortcut: link every bank of the user without the approval page.
	auth("POST "+p+"/demo/connect-all", app.demoConnectAll)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No endpoint "+r.Method+" "+r.URL.Path)
	})
	return logRequests(cors(mux))
}

// withUser checks the bearer token, locks the state and loads the user.
// Demo tokens are "demo.<psuId>", so the app can also hard-code one.
func (app *App) withUser(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		psuID, ok := strings.CutPrefix(token, "demo.")
		if !ok || psuID == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Send Authorization: Bearer <token> from POST /api/v1/auth/demo-login")
			return
		}
		app.state.mu.Lock()
		defer app.state.mu.Unlock()
		u, ok := app.state.Users[psuID]
		if !ok {
			name, known := app.demoUserName(psuID)
			if !known {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unknown user "+psuID)
				return
			}
			u = app.state.user(psuID, name)
		}
		h(w, r, u)
	}
}

func (app *App) loadDemoUsers() []cbarUser {
	if len(app.demoUsers) == 0 {
		if users, err := app.cbar.DemoUsers(); err == nil {
			app.demoUsers = users
		}
	}
	return app.demoUsers
}

func (app *App) demoUserName(psuID string) (string, bool) {
	for _, du := range app.loadDemoUsers() {
		if du.PSUID == psuID {
			return du.Name, true
		}
	}
	return "", false
}

// lang picks az or en from Accept-Language, defaulting to the user's setting.
func lang(r *http.Request, u *UserState) string {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		return "en"
	}
	if r.Header.Get("Accept-Language") == "" && u != nil && u.Language == "en" {
		return "en"
	}
	return "az"
}

// ensureFresh syncs when the cached bank data is older than staleAfter.
func (app *App) ensureFresh(u *UserState) {
	hasActive := false
	for _, c := range u.Connections {
		if c.Status == "active" {
			hasActive = true
		}
	}
	if hasActive && app.now().Sub(u.DataUpdatedAt) > app.staleAfter {
		if err := app.syncUser(u); err != nil {
			log.Printf("auto-sync %s: %v", u.PSUID, err)
		}
	}
}

func (app *App) analysis(r *http.Request, u *UserState) *Analysis {
	return analyze(u, app.state.FX, app.now(), lang(r, u))
}

func (app *App) persist() {
	if err := app.state.save(); err != nil {
		log.Printf("save: %v", err)
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept-Language")
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
