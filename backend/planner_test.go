package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPurchasePlanCheck(t *testing.T) {
	s := &State{Users: map[string]*UserState{}, FX: map[string]string{"USD": "1.7000"}}
	u := s.user("u-test", "Test")
	u.Txs = []*StoredTx{tx("z1", "a1", "2026-10-10", -20000, "ZARA", "POS ZARA PORT BAKU MALL")}
	app := &App{state: s, now: func() time.Time { return testNow }, staleAfter: time.Hour}
	h := app.routes()

	call := func(method, path, body string) map[string]any {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer demo.u-test")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	check := func(price int) map[string]any {
		return call("POST", "/api/v1/purchase-plans/check",
			`{"name":"Coat","price":{"amountMinor":`+itoa(price)+`,"currency":"AZN"},"categoryId":"cat-clothes"}`)
	}

	if got := check(10000)["status"]; got != "no_limit" {
		t.Fatalf("without a limit: %v", got)
	}
	u.Budgets = []*Budget{{ID: "b1", CategoryKey: "clothes", Period: "month", LimitMinor: 50000}}

	// 200 ₼ already spent at Zara this month, limit 500 ₼.
	for price, want := range map[int]string{19000: "fits", 20000: "fits", 25000: "close", 30000: "close", 30001: "exceeds"} {
		r := check(price)
		if r["status"] != want {
			t.Errorf("price %d: status %v, want %s (pct %v)", price, r["status"], want, r["pct"])
		}
	}
	r := check(40000)
	if r["overage"].(map[string]any)["amountMinor"].(float64) != 10000 || r["pct"].(float64) != 120 {
		t.Errorf("overage/pct: %v %v", r["overage"], r["pct"])
	}

	// A planned purchase counts in plannedOther; a dropped one doesn't.
	call("POST", "/api/v1/purchase-plans", `{"name":"Shoes","price":{"amountMinor":15000,"currency":"AZN"},"categoryId":"cat-clothes"}`)
	call("POST", "/api/v1/purchase-plans", `{"name":"Bag","price":{"amountMinor":99000,"currency":"AZN"},"categoryId":"cat-clothes","status":"dropped"}`)
	if got := check(10000)["plannedOther"].(map[string]any)["amountMinor"].(float64); got != 15000 {
		t.Errorf("plannedOther %v, want 15000", got)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
