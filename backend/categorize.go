package main

import (
	"regexp"
	"strings"
)

// Category is the shared Category object from the App API Contract.
type Category struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Kind string `json:"kind"` // expense | income | transfer
}

type categoryDef struct {
	key, az, en, icon, kind string
}

var categoryDefs = []categoryDef{
	{"groceries", "Ərzaq", "Groceries", "cart", "expense"},
	{"dining", "Restoran və kafe", "Dining", "utensils", "expense"},
	{"transport", "Nəqliyyat", "Transport", "car", "expense"},
	{"utilities", "Kommunal", "Utilities", "bolt", "expense"},
	{"telecom", "Mobil və internet", "Mobile & internet", "phone", "expense"},
	{"subscriptions", "Abunələr", "Subscriptions", "repeat", "expense"},
	{"shopping", "Alış-veriş", "Shopping", "bag", "expense"},
	{"health", "Səhiyyə", "Health", "pill", "expense"},
	{"education", "Təhsil", "Education", "book", "expense"},
	{"housing", "Kirayə və ev", "Housing", "home", "expense"},
	{"loans", "Kredit və taksit", "Loans & instalments", "landmark", "expense"},
	{"fees", "Bank komissiyaları", "Bank fees", "receipt", "expense"},
	{"cash", "Nağd pul", "Cash", "banknote", "expense"},
	{"transfers_out", "Köçürmələr", "Transfers", "arrow-up", "expense"},
	{"other_expense", "Digər xərclər", "Other", "dots", "expense"},
	{"salary", "Maaş", "Salary", "briefcase", "income"},
	{"transfers_in", "Daxil olan köçürmələr", "Incoming transfers", "arrow-down", "income"},
	{"other_income", "Digər gəlir", "Other income", "plus", "income"},
	{"internal_transfer", "Öz hesablarım arası", "Between my accounts", "swap", "transfer"},
}

func categoryByKey(key, lang string) Category {
	for _, d := range categoryDefs {
		if d.key == key {
			name := d.az
			if lang == "en" {
				name = d.en
			}
			return Category{ID: "cat-" + d.key, Key: d.key, Name: name, Icon: d.icon, Kind: d.kind}
		}
	}
	return categoryByKey("other_expense", lang)
}

func categoryKeyFromID(id string) (string, bool) {
	key := strings.TrimPrefix(id, "cat-")
	for _, d := range categoryDefs {
		if d.key == key {
			return key, true
		}
	}
	return "", false
}

func allCategories(lang string) []Category {
	out := make([]Category, 0, len(categoryDefs))
	for _, d := range categoryDefs {
		out = append(out, categoryByKey(d.key, lang))
	}
	return out
}

// rule maps a pattern in the bank's raw text to a clean merchant and a category.
// Order matters: the first match wins, so specific rules come before general ones.
type rule struct {
	re       *regexp.Regexp
	merchant string // empty = use the counterparty name, tidied up
	category string
}

func r(pattern, merchant, category string) rule {
	return rule{regexp.MustCompile(pattern), merchant, category}
}

var rules = []rule{
	// Income
	r(`SALARY|EMEK HAQQI|MAAS`, "", "salary"),
	r(`TEQAUD`, "Təqaüd", "other_income"),
	r(`UPWORK|FREELANCE`, "Upwork", "other_income"),

	// Bank fees and cash (before anything that matches the bank's name)
	r(`SMS XIDMET`, "SMS xidmət haqqı", "fees"),
	r(`ILLIK XIDMET`, "Kartın illik xidməti", "fees"),
	r(`KOMISSIYA`, "Bank komissiyası", "fees"),
	r(`\bATM\b|NAGD PUL`, "Bankomat", "cash"),

	// Loans and instalments
	r(`KREDIT ODENIS`, "", "loans"),
	r(`KONTAKT HOME.*TAKSIT|TAKSIT ODENIS`, "Kontakt Home taksit", "loans"),

	// Housing
	r(`KIRAYE`, "Kirayə", "housing"),

	// Subscriptions (Wolt+ before Wolt)
	r(`WOLT\+`, "Wolt+", "subscriptions"),
	r(`NETFLIX`, "Netflix", "subscriptions"),
	r(`SPOTIFY`, "Spotify", "subscriptions"),
	r(`APPLE\.COM`, "Apple", "subscriptions"),
	r(`YOUTUBE`, "YouTube Premium", "subscriptions"),

	// Utilities and telecom
	r(`AZERISHIQ`, "Azərişıq", "utilities"),
	r(`AZERSU`, "Azərsu", "utilities"),
	r(`AZERIQAZ`, "Azəriqaz", "utilities"),
	r(`AZERCELL`, "Azercell", "telecom"),
	r(`BAKCELL`, "Bakcell", "telecom"),
	r(`NAR MOBILE`, "Nar", "telecom"),
	r(`CITYNET|AZERTELECOM`, "CityNet", "telecom"),

	// Groceries
	r(`BRAVO`, "Bravo", "groceries"),
	r(`ARAZ MARKET`, "Araz", "groceries"),
	r(`BAZARSTORE`, "Bazarstore", "groceries"),
	r(`OBA MARKET`, "Oba", "groceries"),

	// Dining (Bolt Food before Bolt)
	r(`WOLT`, "Wolt", "dining"),
	r(`BOLT FOOD`, "Bolt Food", "dining"),
	r(`CAFE CITY`, "Cafe City", "dining"),
	r(`STARBUCKS`, "Starbucks", "dining"),
	r(`\bKFC\b`, "KFC", "dining"),
	r(`COFFEE MOFFIE`, "Coffee Moffie", "dining"),
	r(`PAUL BAKERY`, "Paul", "dining"),

	// Transport
	r(`BOLT`, "Bolt", "transport"),
	r(`UBER`, "Uber", "transport"),
	r(`BAKIKART`, "BakıKart", "transport"),
	r(`SOCAR PETROL|\bAZS\b`, "SOCAR Petroleum", "transport"),

	// Shopping
	r(`TRENDYOL`, "Trendyol", "shopping"),
	r(`UMICO`, "Umico", "shopping"),
	r(`ZARA`, "Zara", "shopping"),
	r(`KONTAKT HOME`, "Kontakt Home", "shopping"),
	r(`AMAZON`, "Amazon", "shopping"),

	// Health and education
	r(`ZEYTUN APTEK`, "Zeytun Aptek", "health"),
	r(`AVROMED`, "Avromed", "health"),
	r(`APTEK`, "Aptek", "health"),
	r(`LC WAIKIKI`, "LC Waikiki", "shopping"),
	r(`ALI VE NINO`, "Ali & Nino", "education"),
	r(`OFIS MARKET`, "Ofis Market", "education"),
	r(`BAKU BOOK CENTER`, "Baku Book Center", "education"),
	r(`REPETITOR|TEDRIS`, "", "education"),
}

// mccCategories is the fallback when no rule matches but the bank sent an MCC.
var mccCategories = map[string]string{
	"5411": "groceries", "5812": "dining", "5814": "dining", "5462": "dining",
	"4121": "transport", "4111": "transport", "5541": "transport",
	"4900": "utilities", "4814": "telecom",
	"4899": "subscriptions", "5815": "subscriptions", "5818": "subscriptions",
	"5311": "shopping", "5651": "shopping", "5641": "shopping", "5732": "shopping",
	"5912": "health", "5942": "education", "5943": "education", "8299": "education",
	"6011": "cash",
}

// categorize turns a raw bank entry into a clean merchant name and a category key.
func categorize(counterparty, remittance, mcc string, amountMinor int64) (merchant, category string) {
	text := strings.ToUpper(remittance + " " + counterparty)
	for _, rl := range rules {
		if rl.re.MatchString(text) {
			merchant = rl.merchant
			if merchant == "" {
				merchant = tidyName(counterparty)
			}
			return merchant, rl.category
		}
	}
	merchant = tidyName(counterparty)
	if merchant == "" {
		merchant = tidyName(remittance)
	}
	if c, ok := mccCategories[mcc]; ok && amountMinor < 0 {
		return merchant, c
	}
	if amountMinor > 0 {
		return merchant, "transfers_in"
	}
	if strings.Contains(text, "C2C") || strings.Contains(text, "TRANSFER") {
		return merchant, "transfers_out"
	}
	return merchant, "other_expense"
}

// tidyName turns "AZERTECH MMC" into "Azertech MMC".
func tidyName(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		switch w {
		case "MMC", "ASC", "QSC", "LLC", "INC", "ABB":
			continue
		}
		rs := []rune(strings.ToLower(w))
		words[i] = strings.ToUpper(string(rs[:1])) + string(rs[1:])
	}
	return strings.Join(words, " ")
}
