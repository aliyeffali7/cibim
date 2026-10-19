package main

import (
	"net/http"
	"strings"
)

// Purchase planner (strategy doc §17): the user checks a purchase before making
// it, against their own limit for that category. Read-only: nothing is paid or blocked.

var plannerCategories = map[string]bool{"clothes": true, "home": true, "tech": true, "school": true, "health": true, "assets": true, "other": true}

var planStatuses = map[string]bool{"planned": true, "dropped": true, "bought": true}

type planInput struct {
	Name       string     `json:"name"`
	Price      *moneyJSON `json:"price"`
	CategoryID string     `json:"categoryId"`
	Status     string     `json:"status"`
}

// validate checks the input and returns the category key and the price in AZN.
func (in planInput) validate(w http.ResponseWriter, fx map[string]string) (string, int64, bool) {
	key, ok := categoryKeyFromID(in.CategoryID)
	if !ok || !plannerCategories[key] {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "categoryId must be one of cat-clothes, cat-home, cat-tech, cat-school, cat-health, cat-assets, cat-other")
		return "", 0, false
	}
	if strings.TrimSpace(in.Name) == "" || in.Price == nil || in.Price.AmountMinor == 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name and a non-zero price are required")
		return "", 0, false
	}
	return key, toAZN(abs64(in.Price.AmountMinor), in.Price.Currency, fx), true
}

// planLimit finds the user's own limit for a category. When there are several,
// the longest period wins (a yearly limit fits big purchases best).
func planLimit(u *UserState, category string) *Budget {
	rank := map[string]int{"week": 1, "month": 2, "year": 3}
	var best *Budget
	for _, b := range u.Budgets {
		if b.CategoryKey == category && (best == nil || rank[b.Period] > rank[best.Period]) {
			best = b
		}
	}
	return best
}

func (app *App) checkPurchasePlan(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in planInput
	if !decode(w, r, &in) {
		return
	}
	key, price, ok := in.validate(w, app.state.FX)
	if !ok {
		return
	}
	app.ensureFresh(u)
	a := app.analysis(r, u)

	out := map[string]any{
		"name": strings.TrimSpace(in.Name), "category": categoryByKey(key, a.lang), "price": azn(price),
		"limit": nil, "period": nil, "spentSoFar": nil, "plannedOther": nil, "projected": nil,
		"pct": nil, "status": "no_limit", "overage": nil, "dataSince": a.dataSince(),
	}
	b := planLimit(u, key)
	if b == nil || b.LimitMinor <= 0 {
		writeJSON(w, http.StatusOK, out) // the UI offers to set a limit
		return
	}

	spent := a.userBudget(b).Spent
	var planned int64
	for _, p := range u.PurchasePlans {
		if p.Status == "planned" && p.CategoryKey == key {
			planned += toAZN(p.PriceMinor, p.Currency, a.fx)
		}
	}
	projected := spent + planned + price
	pct := projected * 100 / b.LimitMinor
	status := "fits"
	switch {
	case projected > b.LimitMinor:
		status = "exceeds"
	case projected*10 > b.LimitMinor*8:
		status = "close"
	}
	out["limit"], out["period"] = azn(b.LimitMinor), b.Period
	out["spentSoFar"], out["plannedOther"], out["projected"] = azn(spent), azn(planned), azn(projected)
	out["pct"], out["status"], out["overage"] = pct, status, azn(max(projected-b.LimitMinor, 0))
	writeJSON(w, http.StatusOK, out)
}

func planView(p *PurchasePlan, lang string) map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "price": moneyJSON{p.PriceMinor, p.Currency},
		"category": categoryByKey(p.CategoryKey, lang), "status": p.Status,
		"createdAt": p.CreatedAt.In(baku), "updatedAt": p.UpdatedAt.In(baku),
	}
}

// createPurchasePlan saves a plan. status defaults to planned; "Don't buy" sends dropped.
func (app *App) createPurchasePlan(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in planInput
	if !decode(w, r, &in) {
		return
	}
	key, _, ok := in.validate(w, app.state.FX)
	if !ok {
		return
	}
	status := in.Status
	if status == "" {
		status = "planned"
	}
	if !planStatuses[status] {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be planned, dropped or bought")
		return
	}
	currency := in.Price.Currency
	if currency == "" {
		currency = "AZN"
	}
	now := app.now()
	p := &PurchasePlan{ID: u.nextID("plan"), Name: strings.TrimSpace(in.Name), PriceMinor: abs64(in.Price.AmountMinor),
		Currency: currency, CategoryKey: key, Status: status, CreatedAt: now, UpdatedAt: now}
	u.PurchasePlans = append(u.PurchasePlans, p)
	app.persist()
	writeJSON(w, http.StatusCreated, planView(p, lang(r, u)))
}

func (app *App) listPurchasePlans(w http.ResponseWriter, r *http.Request, u *UserState) {
	status := r.URL.Query().Get("status")
	out := []map[string]any{}
	for i := len(u.PurchasePlans) - 1; i >= 0; i-- { // newest first
		if p := u.PurchasePlans[i]; status == "" || p.Status == status {
			out = append(out, planView(p, lang(r, u)))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *App) patchPurchasePlan(w http.ResponseWriter, r *http.Request, u *UserState) {
	var in struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !planStatuses[in.Status] {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be planned, dropped or bought")
		return
	}
	for _, p := range u.PurchasePlans {
		if p.ID == r.PathValue("id") {
			p.Status, p.UpdatedAt = in.Status, app.now()
			app.persist()
			writeJSON(w, http.StatusOK, planView(p, lang(r, u)))
			return
		}
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "No purchase plan "+r.PathValue("id"))
}
