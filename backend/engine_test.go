package main

import (
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 19, 12, 0, 0, 0, baku)

func TestCategorize(t *testing.T) {
	cases := []struct {
		cp, rem, mcc string
		amount       int64
		merchant     string
		category     string
	}{
		{"BRAVO HYPERMARKET", "POS BRAVO HYPER 28 MAY BAKU AZ", "", -4250, "Bravo", "groceries"},
		{"WOLT", "WOLT+ MEMBERSHIP", "5812", -799, "Wolt+", "subscriptions"},
		{"WOLT", "WOLT AZ*REST 99450", "", -1490, "Wolt", "dining"},
		{"BOLT FOOD", "BOLT FOOD AZ*12345", "", -1800, "Bolt Food", "dining"},
		{"BOLT", "BOLT.EU/O/12345 BAKU", "", -500, "Bolt", "transport"},
		{"ABB", "SMS XIDMET HAQQI", "", -100, "SMS xidmət haqqı", "fees"},
		{"ABB KREDIT", "KREDIT ODENISI MUQ 2025/44871", "", -18500, "ABB Kredit", "loans"},
		{"AZERTECH MMC", "SALARY OCT 2026", "", 180000, "Azertech MMC", "salary"},
		{"UNKNOWN SHOP", "UNKNOWN SHOP 123", "5411", -900, "Unknown Shop", "groceries"}, // MCC fallback
		{"SOMEONE", "SOMETHING", "", 5000, "Someone", "transfers_in"},
	}
	for _, c := range cases {
		m, cat := categorize(c.cp, c.rem, c.mcc, c.amount)
		if m != c.merchant || cat != c.category {
			t.Errorf("%s: got %q/%q, want %q/%q", c.rem, m, cat, c.merchant, c.category)
		}
	}
}

func tx(id, acc, date string, amount int64, cp, rem string) *StoredTx {
	return &StoredTx{ID: id, AccountID: acc, BankCode: "ABB", Date: date, Status: "booked", AmountMinor: amount, Currency: "AZN", Counterparty: cp, Remittance: rem}
}

func testUser() *UserState {
	s := &State{Users: map[string]*UserState{}}
	u := s.user("u-test", "Test")
	u.Accounts = []*StoredAccount{
		{ID: "a1", ConnectionID: "c1", BankCode: "ABB", Type: "CARD", Currency: "AZN", MaskedPAN: "4169********4812", BalanceMinor: 100000},
		{ID: "a2", ConnectionID: "c2", BankCode: "LEO", Type: "CARD", Currency: "AZN", MaskedPAN: "5239********7731", BalanceMinor: 20000},
	}
	u.Txs = []*StoredTx{
		tx("s1", "a1", "2026-08-15", 180000, "ACME MMC", "SALARY AUG 2026"),
		tx("s2", "a1", "2026-09-15", 180000, "ACME MMC", "SALARY SEP 2026"),
		tx("s3", "a1", "2026-10-15", 180000, "ACME MMC", "SALARY OCT 2026"),
		tx("n1", "a2", "2026-08-09", -1699, "NETFLIX.COM", "NETFLIX.COM AMSTERDAM"),
		tx("n2", "a2", "2026-09-09", -1699, "NETFLIX.COM", "NETFLIX.COM AMSTERDAM"),
		tx("g1", "a1", "2026-09-08", -1400, "AZERIQAZ IB", "AZERIQAZ IB ABONENT"),
		tx("g2", "a1", "2026-10-08", -4200, "AZERIQAZ IB", "AZERIQAZ IB ABONENT"), // heating season jump
		tx("t1", "a1", "2026-10-16", -30000, "TEST USER", "C2C TRANSFER TO CARD *7731"),
		tx("t2", "a2", "2026-10-16", 30000, "TEST USER", "C2C TRANSFER FROM CARD *4812"),
		tx("w1", "a2", "2026-10-18", -2500, "WOLT", "WOLT AZ*REST 1"),
		tx("w2", "a2", "2026-10-19", -2500, "WOLT", "WOLT AZ*REST 2"),
	}
	return u
}

func TestAnalyze(t *testing.T) {
	u := testUser()
	a := analyze(u, map[string]string{"USD": "1.7000"}, testNow, "az")

	if !a.txByID["t1"].internal || !a.txByID["t2"].internal {
		t.Error("own-card transfer legs should be internal")
	}

	rec := map[string]*recurringItem{}
	for _, r := range a.recurring {
		rec[r.Name] = r
	}
	if s := rec["Acme MMC"]; s == nil || s.Type != "income" || s.NextDate.Format(dateLayout) != "2026-11-15" {
		t.Errorf("salary not detected right: %+v", s)
	}
	if n := rec["Netflix"]; n == nil || n.Cadence != "monthly" || n.NextDate.Format(dateLayout) != "2026-10-09" || n.Status != "late" {
		t.Errorf("netflix should be monthly and late (Oct 9 missing): %+v", n)
	}
	if g := rec["Azəriqaz"]; g == nil || g.Status != "changed" {
		t.Errorf("gas bill jump should be 'changed': %+v", g)
	}
	if _, ok := rec["Wolt"]; ok {
		t.Error("daily Wolt orders are not recurring")
	}

	s := a.safeToSpend(u)
	if s.NextIncome == nil || s.DaysLeft != 27 {
		t.Fatalf("next income / days left: %+v", s)
	}
	// Start-of-day cards: 1000 + 200 + today's 25 Wolt. No bills before Nov 15 except Azəriqaz (Nov 8).
	if s.Available != 122500 {
		t.Errorf("available %d", s.Available)
	}
	if s.Today != (s.Available-s.UpcomingBills-s.Buffer)/27 {
		t.Errorf("today %d", s.Today)
	}
	if spent := a.spending(a.today, a.today, nil); spent != 2500 {
		t.Errorf("today's spending %d, transfers must not count", spent)
	}
}

func TestOverridesWin(t *testing.T) {
	u := testUser()
	u.MerchantCategory["Wolt"] = "groceries"
	u.TxCategory["w2"] = "shopping"
	a := analyze(u, nil, testNow, "en")
	if a.txByID["w1"].category != "groceries" || a.txByID["w2"].category != "shopping" {
		t.Errorf("got %s / %s", a.txByID["w1"].category, a.txByID["w2"].category)
	}
}
