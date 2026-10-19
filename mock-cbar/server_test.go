package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 10, 19, 12, 0, 0, 0, baku)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := &Server{store: NewStore(func() time.Time { return fixedNow })}
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return ts
}

type call struct {
	method, path, body string
	headers            map[string]string
}

func do(t *testing.T, ts *httptest.Server, c call) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(c.method, ts.URL+c.path, strings.NewReader(c.body))
	req.Header.Set("X-Request-ID", "6f1c2a8e-0000-4000-8000-000000000001")
	for k, v := range c.headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func errCode(body map[string]any) string {
	msgs, _ := body["tppMessages"].([]any)
	if len(msgs) == 0 {
		return ""
	}
	return msgs[0].(map[string]any)["code"].(string)
}

// createConsent returns a consent ID for user at bank, approved unless approve is false.
func createConsent(t *testing.T, ts *httptest.Server, user, bank string, approve bool) string {
	t.Helper()
	code, body := do(t, ts, call{method: "POST", path: "/v1/consents",
		body:    `{"bankCode":"` + bank + `","access":{"allPsd2":"allAccounts"},"recurringIndicator":true,"frequencyPerDay":4}`,
		headers: map[string]string{"PSU-ID": user, "TPP-Redirect-URI": "cibim://consent"}})
	if code != 201 {
		t.Fatalf("create consent: %d %v", code, body)
	}
	id := body["consentId"].(string)
	if approve {
		if code, body := do(t, ts, call{method: "POST", path: "/mock/consents/" + id + "/approve"}); code != 200 {
			t.Fatalf("approve: %d %v", code, body)
		}
	}
	return id
}

func TestConsentFlowAndReads(t *testing.T) {
	ts := newTestServer(t)
	id := createConsent(t, ts, "u-aysel", "ABB", false)

	// Not approved yet.
	code, body := do(t, ts, call{method: "GET", path: "/v1/accounts", headers: map[string]string{"Consent-ID": id}})
	if code != 401 || errCode(body) != "CONSENT_INVALID" {
		t.Fatalf("unapproved read: %d %v", code, body)
	}

	// Approve through the SCA page and follow the redirect target.
	req, _ := http.NewRequest("POST", ts.URL+"/mock-sca/"+id, strings.NewReader("decision=approve"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if loc := resp.Header.Get("Location"); resp.StatusCode != 303 || loc != "cibim://consent?consentId="+id+"&status=valid" {
		t.Fatalf("sca redirect: %d %q", resp.StatusCode, loc)
	}

	if _, body := do(t, ts, call{method: "GET", path: "/v1/consents/" + id + "/status"}); body["consentStatus"] != "valid" {
		t.Fatalf("status: %v", body)
	}

	// ABB covers Aysel's AZN card and USD account, not her Leobank card.
	_, body = do(t, ts, call{method: "GET", path: "/v1/accounts", headers: map[string]string{"Consent-ID": id}})
	accts := body["accounts"].([]any)
	if len(accts) != 2 {
		t.Fatalf("want 2 ABB accounts, got %v", accts)
	}

	code, body = do(t, ts, call{method: "GET", path: "/v1/accounts/acc-aysel-abb-1/balances", headers: map[string]string{"Consent-ID": id}})
	bals := body["balances"].([]any)
	avail, _ := parseMoney(bals[0].(map[string]any)["balanceAmount"].(map[string]any)["amount"].(string))
	closing, _ := parseMoney(bals[1].(map[string]any)["balanceAmount"].(map[string]any)["amount"].(string))
	if code != 200 || avail <= 0 || closing <= 0 {
		t.Fatalf("balances: %d %v", code, body)
	}

	code, body = do(t, ts, call{method: "GET", path: "/v1/accounts/acc-aysel-leo-1/balances", headers: map[string]string{"Consent-ID": id}})
	if code != 404 || errCode(body) != "RESOURCE_UNKNOWN" {
		t.Fatalf("other bank's account: %d %v", code, body)
	}

	code, body = do(t, ts, call{method: "GET", path: "/v1/accounts/acc-aysel-abb-1/transactions?dateFrom=2026-07-21&bookingStatus=both", headers: map[string]string{"Consent-ID": id}})
	txs := body["transactions"].(map[string]any)
	booked := txs["booked"].([]any)
	if code != 200 || len(booked) < 100 {
		t.Fatalf("transactions: %d, %d booked", code, len(booked))
	}
	first := booked[0].(map[string]any)
	if first["bookingDate"] != "2026-10-19" && first["bookingDate"] != "2026-10-18" {
		t.Fatalf("newest first expected, got %v", first)
	}

	// Revoke.
	if code, _ := do(t, ts, call{method: "DELETE", path: "/v1/consents/" + id}); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if code, body := do(t, ts, call{method: "GET", path: "/v1/accounts", headers: map[string]string{"Consent-ID": id}}); code != 401 {
		t.Fatalf("after delete: %d %v", code, body)
	}
}

func TestErrors(t *testing.T) {
	ts := newTestServer(t)
	id := createConsent(t, ts, "u-nigar", "LEO", true)
	acct := "/v1/accounts/acc-nigar-leo-1/transactions"
	h := map[string]string{"Consent-ID": id}

	cases := []struct {
		name string
		c    call
		code int
		err  string
	}{
		{"missing request id", call{method: "GET", path: "/v1/banks", headers: map[string]string{"X-Request-ID": ""}}, 400, "FORMAT_ERROR"},
		{"missing consent header", call{method: "GET", path: "/v1/accounts"}, 400, "FORMAT_ERROR"},
		{"unknown consent", call{method: "GET", path: "/v1/accounts", headers: map[string]string{"Consent-ID": "cns-nope"}}, 403, "CONSENT_UNKNOWN"},
		{"other user's consent", call{method: "GET", path: "/v1/accounts", headers: map[string]string{"Consent-ID": id, "PSU-ID": "u-aysel"}}, 403, "CONSENT_UNKNOWN"},
		{"no dateFrom", call{method: "GET", path: acct, headers: h}, 400, "FORMAT_ERROR"},
		{"older than 90 days", call{method: "GET", path: acct + "?dateFrom=2026-07-01", headers: h}, 400, "PERIOD_INVALID"},
		{"from after to", call{method: "GET", path: acct + "?dateFrom=2026-10-10&dateTo=2026-10-01", headers: h}, 400, "PERIOD_INVALID"},
		{"unknown bank", call{method: "POST", path: "/v1/consents", body: `{"bankCode":"XYZ"}`, headers: map[string]string{"PSU-ID": "u-nigar", "TPP-Redirect-URI": "x://y"}}, 400, "FORMAT_ERROR"},
	}
	for _, tc := range cases {
		code, body := do(t, ts, tc.c)
		if code != tc.code || errCode(body) != tc.err {
			t.Errorf("%s: got %d %s, want %d %s", tc.name, code, errCode(body), tc.code, tc.err)
		}
	}

	// Force-expire for the renewal demo.
	do(t, ts, call{method: "POST", path: "/mock/consents/" + id + "/expire"})
	if code, body := do(t, ts, call{method: "GET", path: "/v1/accounts", headers: h}); code != 401 || errCode(body) != "CONSENT_EXPIRED" {
		t.Errorf("expired: %d %v", code, body)
	}
}

func TestInjectAffectsBalanceAndFeed(t *testing.T) {
	ts := newTestServer(t)
	id := createConsent(t, ts, "u-aysel", "ABB", true)
	h := map[string]string{"Consent-ID": id}

	available := func() string {
		_, body := do(t, ts, call{method: "GET", path: "/v1/accounts/acc-aysel-abb-1/balances", headers: h})
		return body["balances"].([]any)[0].(map[string]any)["balanceAmount"].(map[string]any)["amount"].(string)
	}
	before, _ := parseMoney(available())

	code, body := do(t, ts, call{method: "POST", path: "/mock/transactions", body: `{"resourceId":"acc-aysel-abb-1","status":"pending","transactionAmount":{"currency":"AZN","amount":"-120.00"},"creditorName":"ZARA","remittanceInformationUnstructured":"POS ZARA PORT BAKU MALL","merchantCategoryCode":"5651"}`})
	if code != 201 || body["creditorName"] != "ZARA" {
		t.Fatalf("inject: %d %v", code, body)
	}

	if after, _ := parseMoney(available()); after != before-12000 {
		t.Fatalf("available went from %s to %s after a -120.00 pending", before, after)
	}

	_, body = do(t, ts, call{method: "GET", path: "/v1/accounts/acc-aysel-abb-1/transactions?dateFrom=2026-10-01&bookingStatus=pending", headers: h})
	pending := body["transactions"].(map[string]any)["pending"].([]any)
	if len(pending) != 1 || pending[0].(map[string]any)["bookingDate"] != nil {
		t.Fatalf("pending feed: %v", pending)
	}

	do(t, ts, call{method: "POST", path: "/mock/reset"})
	if code, _ := do(t, ts, call{method: "GET", path: "/v1/accounts", headers: h}); code != 403 {
		t.Fatalf("consents should be gone after reset, got %d", code)
	}
}

// TestSeedShape checks the personas carry what the contract promises.
func TestSeedShape(t *testing.T) {
	users, accounts, txs := seedData(fixedNow)
	if len(users) != 3 || len(accounts) != 6 {
		t.Fatalf("users %d accounts %d", len(users), len(accounts))
	}
	find := func(acc, needle string) int {
		n := 0
		for _, tx := range txs {
			if tx.AccountID == acc && strings.Contains(tx.Remittance+" "+tx.Counterparty, needle) {
				n++
			}
		}
		return n
	}
	checks := []struct{ acc, needle string }{
		{"acc-aysel-abb-1", "SALARY"}, {"acc-aysel-abb-1", "KIRAYE"}, {"acc-aysel-abb-1", "AZERISHIQ"},
		{"acc-aysel-abb-1", "AZERSU"}, {"acc-aysel-abb-1", "AZERIQAZ"}, {"acc-aysel-abb-1", "KREDIT"},
		{"acc-aysel-leo-1", "NETFLIX"}, {"acc-aysel-leo-1", "SPOTIFY"}, {"acc-aysel-abb-usd", "UPWORK"},
		{"acc-rashad-xalq-1", "EMEK HAQQI"}, {"acc-rashad-pasha-1", "ALI VE NINO"},
		{"acc-nigar-leo-1", "TEQAUD"}, {"acc-nigar-leo-1", "C2C TRANSFER"},
	}
	for _, c := range checks {
		if find(c.acc, c.needle) == 0 {
			t.Errorf("%s has no %q transactions", c.acc, c.needle)
		}
	}

	withMCC, cards := 0, 0
	for _, tx := range txs {
		if tx.Amount < 0 && strings.Contains(tx.Remittance, "WOLT AZ*REST") {
			cards++
			if tx.MCC != "" {
				withMCC++
			}
		}
	}
	if withMCC == 0 || withMCC == cards {
		t.Errorf("MCC should be present on some card purchases only: %d of %d", withMCC, cards)
	}

	// Running balances should never go negative on these debit accounts.
	for _, a := range accounts {
		bal := a.Opening
		for _, tx := range txs {
			if tx.AccountID == a.ResourceID && tx.Booked {
				bal += tx.Amount
				if bal < 0 {
					t.Errorf("%s goes negative (%s) on %s", a.ResourceID, bal, tx.Date.Format(dateLayout))
					break
				}
			}
		}
	}
}
