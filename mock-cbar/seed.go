package main

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

// historyDays is how far back seeded data goes. It matches the 90-day limit on
// GET /transactions, so the whole allowed window is always full.
const historyDays = 90

// minBalance is the lowest a seeded account's running balance may go (15 ₼).
const minBalance Money = 1500

var seedBanks = []Bank{
	{Code: "ABB", Name: "ABB", ibanCode: "MABB"},
	{Code: "XALQ", Name: "Xalq Bank", ibanCode: "MXLQ"},
	{Code: "PASHA", Name: "PAŞA Bank", ibanCode: "MPSH"},
	{Code: "LEO", Name: "Leobank", ibanCode: "MLEO"},
}

type merchant struct {
	name string // creditorName
	rem  string // remittance; "%d" is replaced by a random terminal/order number
	mcc  string
}

var (
	groceries = []merchant{
		{"BRAVO HYPERMARKET", "POS BRAVO HYPER 28 MAY BAKU AZ", "5411"},
		{"BRAVO SUPERMARKET", "BRAVO SUPERMARKET NARIMANOV %d", "5411"},
		{"ARAZ MARKET", "ARAZ MARKET %d BAKU AZ", "5411"},
		{"BAZARSTORE", "BAZARSTORE GANJLIK MALL BAKU", "5411"},
		{"OBA MARKET", "OBA MARKET %d AZ", "5411"},
	}
	dining = []merchant{
		{"WOLT", "WOLT AZ*REST %d", "5812"},
		{"BOLT FOOD", "BOLT FOOD AZ*%d", "5812"},
		{"CAFE CITY", "CAFE CITY NIZAMI BAKU", "5812"},
		{"STARBUCKS", "STARBUCKS PORT BAKU MALL", "5814"},
		{"KFC", "KFC 28 MALL BAKU AZ", "5814"},
	}
	coffee = []merchant{
		{"COFFEE MOFFIE", "COFFEE MOFFIE %d BAKU", "5814"},
		{"STARBUCKS", "STARBUCKS PORT BAKU MALL", "5814"},
		{"PAUL BAKERY", "PAUL BAKERY FOUNTAIN SQ", "5462"},
	}
	taxi = []merchant{
		{"BOLT", "BOLT.EU/O/%d BAKU", "4121"},
		{"UBER", "UBER *TRIP BAKU", "4121"},
	}
	metro    = merchant{"BAKIKART", "BAKIKART METRO ARTIRMA %d", "4111"}
	fuel     = merchant{"SOCAR PETROL", "SOCAR PETROL AZS %d", "5541"}
	shopping = []merchant{
		{"TRENDYOL", "TRENDYOL AZ ONLINE", "5311"},
		{"UMICO", "UMICO MARKET ONLINE BAKU", "5311"},
		{"ZARA", "POS ZARA PORT BAKU MALL", "5651"},
		{"KONTAKT HOME", "KONTAKT HOME 28 MALL", "5732"},
	}
	pharmacy = []merchant{
		{"ZEYTUN APTEK", "ZEYTUN APTEK %d", "5912"},
		{"AVROMED", "AVROMED APTEK SHEBEKESI", "5912"},
	}
	school = []merchant{
		{"ALI VE NINO", "KITAB EVI ALI VE NINO %d", "5942"},
		{"LC WAIKIKI", "LC WAIKIKI GANJLIK MALL", "5641"},
		{"OFIS MARKET", "OFIS MARKET DEFTERXANA %d", "5943"},
		{"BAKU BOOK CENTER", "BAKU BOOK CENTER", "5942"},
	}
)

type gen struct {
	r     *rand.Rand
	start time.Time // first seeded day
	today time.Time
	txs   []*Transaction
}

func (g *gen) chance(p float64) bool { return g.r.Float64() < p }

// amount returns a random amount in [lo, hi] manat, in steps of 10 qəpik.
func (g *gen) amount(lo, hi float64) Money {
	steps := int((hi - lo) * 10)
	return Money(lo*100) + Money(g.r.IntN(steps+1)*10)
}

func (g *gen) rem(m merchant) string {
	if strings.Contains(m.rem, "%d") {
		return fmt.Sprintf(m.rem, 10000+g.r.IntN(90000))
	}
	return m.rem
}

func (g *gen) add(acc string, d time.Time, amt Money, counterparty, rem, mcc string) {
	g.txs = append(g.txs, &Transaction{AccountID: acc, Booked: true, Date: d, Amount: amt, Counterparty: counterparty, Remittance: rem, MCC: mcc})
}

func (g *gen) pending(acc string, amt Money, counterparty, rem, mcc string) {
	g.txs = append(g.txs, &Transaction{AccountID: acc, Amount: amt, Counterparty: counterparty, Remittance: rem, MCC: mcc})
}

// buy records a card purchase. About half carry an MCC, so the app's
// categoriser gets tested with and without it.
func (g *gen) buy(acc string, d time.Time, m merchant, lo, hi float64) {
	mcc := ""
	if g.chance(0.5) {
		mcc = m.mcc
	}
	g.add(acc, d, -g.amount(lo, hi), m.name, g.rem(m), mcc)
}

func (g *gen) pick(ms []merchant) merchant { return ms[g.r.IntN(len(ms))] }

// monthly calls f for the given day of every month inside the seeded window.
func (g *gen) monthly(day int, f func(d time.Time)) {
	for m := time.Date(g.start.Year(), g.start.Month(), 1, 0, 0, 0, 0, baku); !m.After(g.today); m = m.AddDate(0, 1, 0) {
		last := m.AddDate(0, 1, -1).Day()
		d := time.Date(m.Year(), m.Month(), min(day, last), 0, 0, 0, 0, baku)
		if !d.Before(g.start) && !d.After(g.today) {
			f(d)
		}
	}
}

func (g *gen) daily(f func(d time.Time)) {
	for d := g.start; !d.After(g.today); d = d.AddDate(0, 0, 1) {
		f(d)
	}
}

func (g *gen) inRange(d time.Time, fromMonth time.Month, fromDay int, toMonth time.Month, toDay int) bool {
	from := time.Date(d.Year(), fromMonth, fromDay, 0, 0, 0, 0, baku)
	to := time.Date(d.Year(), toMonth, toDay, 0, 0, 0, 0, baku)
	return !d.Before(from) && !d.After(to)
}

func monthTag(d time.Time) string { return strings.ToUpper(d.Format("Jan 2006")) }

// seedData builds users, accounts and 90 days of transactions ending today.
// Same day in, same data out: the RNG seed is fixed.
func seedData(now time.Time) ([]User, []*Account, []*Transaction) {
	today := dateOf(now)
	g := &gen{r: rand.New(rand.NewPCG(2026, 10)), start: today.AddDate(0, 0, -(historyDays - 1)), today: today}

	users := []User{
		{PSUID: "u-aysel", Name: "Aysel", Persona: "Salaried employee: salary, rent, utilities, subscriptions, a loan instalment, dining over budget", Banks: []string{"ABB", "LEO"}},
		{PSUID: "u-rashad", Name: "Rəşad", Persona: "Householder with school-age children: family bills, groceries, school supplies in late Aug and Sep", Banks: []string{"XALQ", "PASHA"}},
		{PSUID: "u-nigar", Name: "Nigar", Persona: "Student: monthly stipend, transfer from family, small daily spending", Banks: []string{"LEO"}},
	}

	// target is the interimAvailable balance today; Opening is solved for below.
	type acct struct {
		*Account
		target Money
	}
	accts := []acct{
		{&Account{ResourceID: "acc-aysel-abb-1", PSUID: "u-aysel", BankCode: "ABB", Currency: "AZN", Name: "ABB Debit *4812", Type: "CARD", MaskedPAN: "4169********4812"}, 125040},
		{&Account{ResourceID: "acc-aysel-leo-1", PSUID: "u-aysel", BankCode: "LEO", Currency: "AZN", Name: "Leobank Card *7731", Type: "CARD", MaskedPAN: "5239********7731"}, 8630},
		{&Account{ResourceID: "acc-aysel-abb-usd", PSUID: "u-aysel", BankCode: "ABB", Currency: "USD", Name: "ABB Current USD", Type: "CURRENT"}, 134000},
		{&Account{ResourceID: "acc-rashad-xalq-1", PSUID: "u-rashad", BankCode: "XALQ", Currency: "AZN", Name: "Xalq Bank Kart *3307", Type: "CARD", MaskedPAN: "4098********3307"}, 91275},
		{&Account{ResourceID: "acc-rashad-pasha-1", PSUID: "u-rashad", BankCode: "PASHA", Currency: "AZN", Name: "PAŞA Visa *2201", Type: "CARD", MaskedPAN: "4847********2201"}, 23310},
		{&Account{ResourceID: "acc-nigar-leo-1", PSUID: "u-nigar", BankCode: "LEO", Currency: "AZN", Name: "Leobank Card *5590", Type: "CARD", MaskedPAN: "5239********5590"}, 4785},
	}

	seedAysel(g)
	seedRashad(g)
	seedNigar(g)

	// Order by date and number IDs in that order, so newer means higher.
	sort.SliceStable(g.txs, func(i, j int) bool {
		a, b := g.txs[i], g.txs[j]
		if a.Booked != b.Booked {
			return a.Booked // pending last
		}
		return a.Date.Before(b.Date)
	})
	for i, t := range g.txs {
		t.seq = i + 1
		t.ID = fmt.Sprintf("tx-%06d", i+1)
	}

	accounts := make([]*Account, 0, len(accts))
	bankByCode := map[string]Bank{}
	for _, b := range seedBanks {
		bankByCode[b.Code] = b
	}
	for i, a := range accts {
		a.Status = "enabled"
		a.IBAN = fmt.Sprintf("AZ00%s%020d", bankByCode[a.BankCode].ibanCode, i+1)
		// Aim for the target balance today, but never let the running balance
		// dip below minBalance on any day: a debit card can't go negative.
		var sum, lowest Money
		for _, t := range g.txs {
			if t.AccountID == a.ResourceID {
				sum += t.Amount
				lowest = min(lowest, sum)
			}
		}
		a.Opening = max(a.target-sum, minBalance-lowest)
		accounts = append(accounts, a.Account)
	}
	return users, accounts, g.txs
}

func seedAysel(g *gen) {
	const abb, leo, usd = "acc-aysel-abb-1", "acc-aysel-leo-1", "acc-aysel-abb-usd"

	// Recurring income
	g.monthly(15, func(d time.Time) { g.add(abb, d, 180000, "AZERTECH MMC", "SALARY "+monthTag(d), "") })
	g.monthly(5, func(d time.Time) { g.add(usd, d, 20000, "UPWORK ESCROW INC", "FREELANCE PAYMENT "+monthTag(d), "") })

	// Recurring expenses
	g.monthly(1, func(d time.Time) { g.add(abb, d, -60000, "RAUF HASANOV", "KIRAYE HAQQI "+monthTag(d), "") })
	g.monthly(5, func(d time.Time) {
		g.add(abb, d, -g.amount(18, 34), "AZERISHIQ ASC", "AZERISHIQ ONLINE ODENIS 1102334", "4900")
	})
	g.monthly(7, func(d time.Time) { g.add(abb, d, -g.amount(8, 12), "AZERSU ASC", "AZERSU ASC ABONENT 44120", "") })
	g.monthly(8, func(d time.Time) {
		lo, hi := 9.0, 15.0
		if d.Month() >= time.October {
			lo, hi = 28, 45 // heating season
		}
		g.add(abb, d, -g.amount(lo, hi), "AZERIQAZ IB", "AZERIQAZ IB ABONENT 22931", "4900")
	})
	g.monthly(10, func(d time.Time) { g.add(abb, d, -1500, "AZERCELL", "AZERCELL *MOBILE ODENIS 50 2214567", "4814") })
	g.monthly(20, func(d time.Time) { g.add(abb, d, -18500, "ABB KREDIT", "KREDIT ODENISI MUQ 2025/44871", "") })
	g.monthly(28, func(d time.Time) { g.add(abb, d, -100, "ABB", "SMS XIDMET HAQQI", "") })

	// Subscriptions
	g.monthly(9, func(d time.Time) { g.add(leo, d, -1699, "NETFLIX.COM", "NETFLIX.COM AMSTERDAM NL", "4899") })
	g.monthly(12, func(d time.Time) { g.add(leo, d, -599, "SPOTIFY", "SPOTIFY P2A1B3C4D5 STOCKHOLM", "5815") })
	g.monthly(3, func(d time.Time) { g.add(leo, d, -799, "WOLT", "WOLT+ MEMBERSHIP", "5812") })
	g.monthly(20, func(d time.Time) { g.add(abb, d, -199, "APPLE.COM/BILL", "APPLE.COM/BILL CORK IE", "5818") })

	// Moves money to the Leobank card; both legs appear, for transfer matching.
	g.monthly(16, func(d time.Time) {
		g.add(abb, d, -30000, "AYSEL GULIYEVA", "C2C TRANSFER TO CARD *7731", "")
		g.add(leo, d, 30000, "AYSEL GULIYEVA", "C2C TRANSFER FROM CARD *4812", "")
	})

	// Day-to-day spending. Dining picks up in the last month: the over-budget story.
	lastMonth := g.today.AddDate(0, 0, -30)
	g.daily(func(d time.Time) {
		p := 0.45
		if d.After(lastMonth) {
			p = 0.8
		}
		if g.chance(p) {
			g.buy(leo, d, g.pick(dining), 9, 38)
		}
		if g.chance(0.35) {
			g.buy(abb, d, g.pick(groceries), 12, 75)
		}
		if g.chance(0.4) {
			g.buy(abb, d, g.pick(taxi), 3, 9)
		}
		if d.Weekday() == time.Monday {
			g.buy(abb, d, metro, 10, 10)
		}
		if g.chance(0.06) {
			g.buy(leo, d, g.pick(shopping), 25, 140)
		}
		if g.chance(0.04) {
			g.buy(abb, d, g.pick(pharmacy), 6, 30)
		}
		if g.chance(0.03) {
			g.add(abb, d, -10000, "ATM ABB 0283", "ATM ABB 0283 BAKU NAGD PUL", "6011")
			g.add(abb, d, -50, "ABB", "NAGD PUL CIXARMA KOMISSIYASI", "")
		}
	})
	g.add(usd, g.today.AddDate(0, 0, -40), -4900, "AMAZON.COM", "AMAZON.COM*MK2RT4 SEATTLE US", "5942")

	g.pending(leo, -1490, "WOLT", "WOLT AZ*REST 99450", "")
}

func seedRashad(g *gen) {
	const xalq, pasha = "acc-rashad-xalq-1", "acc-rashad-pasha-1"

	g.monthly(25, func(d time.Time) { g.add(xalq, d, 240000, "KASPI LOGISTIKA MMC", "EMEK HAQQI "+monthTag(d), "") })

	g.monthly(4, func(d time.Time) {
		g.add(xalq, d, -g.amount(40, 70), "AZERISHIQ ASC", "AZERISHIQ ONLINE ODENIS 2208815", "4900")
	})
	g.monthly(6, func(d time.Time) { g.add(xalq, d, -g.amount(15, 25), "AZERSU ASC", "AZERSU ASC ABONENT 70331", "") })
	g.monthly(8, func(d time.Time) {
		lo, hi := 18.0, 30.0
		if d.Month() >= time.October {
			lo, hi = 60, 95
		}
		g.add(xalq, d, -g.amount(lo, hi), "AZERIQAZ IB", "AZERIQAZ IB ABONENT 51207", "4900")
	})
	g.monthly(11, func(d time.Time) { g.add(xalq, d, -2500, "AZERTELECOM", "CITYNET INTERNET ABONENT 33018", "4814") })
	g.monthly(12, func(d time.Time) { g.add(xalq, d, -2000, "BAKCELL", "BAKCELL ONLINE ODENIS 0552231144", "4814") })
	g.monthly(12, func(d time.Time) { g.add(xalq, d, -1000, "NAR MOBILE", "NAR MOBILE TOPUP 0702231144", "4814") })
	g.monthly(2, func(d time.Time) {
		g.add(pasha, d, -15000, "XEYAL TEDRIS MERKEZI", "REPETITOR ODENISI "+monthTag(d), "8299")
	})
	g.monthly(18, func(d time.Time) { g.add(pasha, d, -8900, "KONTAKT HOME", "KONTAKT HOME TAKSIT ODENISI 4/12", "") })
	g.monthly(14, func(d time.Time) { g.add(pasha, d, -799, "YOUTUBE PREMIUM", "GOOGLE *YOUTUBEPREMIUM", "5815") })
	g.monthly(28, func(d time.Time) { g.add(xalq, d, -60, "XALQ BANK", "SMS XIDMET HAQQI", "") })

	// Salary lands on Xalq; the family budget moves to PAŞA.
	g.monthly(26, func(d time.Time) {
		g.add(xalq, d, -160000, "RASHAD MAMMADOV", "C2C TRANSFER TO CARD *2201", "")
		g.add(pasha, d, 160000, "RASHAD MAMMADOV", "C2C TRANSFER FROM CARD *3307", "")
	})

	g.daily(func(d time.Time) {
		if g.chance(0.55) {
			g.buy(pasha, d, g.pick(groceries), 25, 150)
		}
		if d.Weekday() == time.Saturday {
			g.buy(xalq, d, fuel, 30, 60)
		}
		if g.chance(0.08) {
			g.buy(pasha, d, g.pick(pharmacy), 8, 45)
		}
		if g.chance(0.1) {
			g.buy(pasha, d, g.pick(dining), 20, 60)
		}
		// School supplies: late August to mid September.
		if g.inRange(d, time.August, 20, time.September, 14) && g.chance(0.45) {
			g.buy(pasha, d, g.pick(school), 30, 180)
		}
	})
	g.add(xalq, g.today.AddDate(0, 0, -55), -1000, "XALQ BANK", "KART ILLIK XIDMET HAQQI", "")

	g.pending(pasha, -6740, "BRAVO HYPERMARKET", "POS BRAVO HYPER 28 MAY BAKU AZ", "")
}

func seedNigar(g *gen) {
	const leo = "acc-nigar-leo-1"

	g.monthly(5, func(d time.Time) { g.add(leo, d, 15000, "UNIVERSITET TEQAUD FONDU", "TEQAUD "+monthTag(d), "") })
	g.monthly(1, func(d time.Time) { g.add(leo, d, 12000, "LEYLA HUSEYNOVA", "C2C TRANSFER", "") })

	g.monthly(10, func(d time.Time) { g.add(leo, d, -800, "NAR MOBILE", "NAR MOBILE TOPUP 0705559911", "4814") })
	g.monthly(12, func(d time.Time) { g.add(leo, d, -299, "SPOTIFY", "SPOTIFY STUDENT P7Q2W STOCKHOLM", "5815") })

	g.daily(func(d time.Time) {
		if g.chance(0.6) {
			g.buy(leo, d, g.pick(coffee), 2, 7)
		}
		if d.Weekday() == time.Monday {
			g.buy(leo, d, metro, 5, 5)
		}
		if g.chance(0.15) {
			g.buy(leo, d, g.pick(groceries), 4, 18)
		}
		if g.chance(0.1) {
			g.buy(leo, d, g.pick(dining), 8, 20)
		}
		if g.chance(0.05) {
			g.buy(leo, d, g.pick(taxi), 3, 7)
		}
	})
}
