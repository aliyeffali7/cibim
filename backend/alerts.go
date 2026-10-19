package main

import (
	"fmt"
	"time"
)

// generateAlerts looks at the user's current situation and adds any alert that
// hasn't fired before. Each alert has a dedup key, so re-syncing never repeats one.
// Callers hold app.state.mu.
func (app *App) generateAlerts(u *UserState) {
	now := app.now()
	a := analyze(u, app.state.FX, now, "az")
	today := a.today
	seen := map[string]bool{}
	for _, al := range u.Alerts {
		seen[al.Key] = true
	}
	add := func(key, typ, severity, screen, target string, params map[string]any) {
		if seen[key] {
			return
		}
		seen[key] = true
		u.Alerts = append(u.Alerts, &Alert{ID: u.nextID("alr"), Key: key, Type: typ, Severity: severity,
			CreatedAt: now, Params: params, Screen: screen, TargetID: target})
	}
	money := func(m int64) map[string]any { return map[string]any{"amountMinor": m, "currency": "AZN"} }

	// Budgets: warning at 80%, over at 100%.
	budgets := a.budgets(u)
	overCats := map[string]bool{}
	for _, b := range budgets {
		st := b.status()
		if st == "ok" {
			continue
		}
		if st == "over" {
			overCats[b.Category] = true
		}
		start, _ := periodRange(b.Period, today)
		name := "Ümumi xərclər"
		if b.Category != "" {
			name = categoryByKey(b.Category, "az").Name
		}
		typ, sev := "budget_warning", "info"
		if st == "over" {
			typ, sev = "budget_over", "warning"
		}
		add(fmt.Sprintf("%s:%s:%s", typ, b.ID, start.Format(dateLayout)), typ, sev, "budget", b.ID, map[string]any{
			"categoryKey": b.Category, "categoryName": name, "period": b.Period, "pct": b.pct(),
			"spent": money(b.Spent), "limit": money(b.Limit), "basis": b.Basis,
		})
	}

	// "Do you really need this?": a big day-to-day purchase while over today's limit or a budget.
	s2s := a.safeToSpend(u)
	for _, t := range a.txs {
		if !t.discretionary() || !t.date.Equal(today) || -t.amountAzn < 5000 {
			continue
		}
		if s2s.SpentToday > s2s.Today || overCats[t.category] || overCats[""] {
			add("purchase_check:"+t.ID, "purchase_check", "warning", "transaction", t.ID, map[string]any{
				"merchant": t.merchant, "amount": money(-t.amountAzn), "categoryName": categoryByKey(t.category, "az").Name,
				"spentToday": money(s2s.SpentToday), "safeToSpendToday": money(s2s.Today),
			})
		}
	}

	for _, r := range a.recurring {
		switch {
		case r.Status == "late" && r.Type == "income":
			add("income_late:"+r.ID+":"+r.NextDate.Format(dateLayout), "income_late", "warning", "recurring", r.ID, map[string]any{
				"name": r.Name, "expectedDate": r.NextDate.Format(dateLayout), "amount": money(r.Typical),
			})
		case r.Status == "changed" && r.Type == "expense":
			add("bill_changed:"+r.ID+":"+r.LastDate, "bill_changed", "info", "recurring", r.ID, map[string]any{
				"name": r.Name, "typicalAmount": money(-r.Typical), "lastAmount": money(-lastAmount(a, r)),
			})
		}
		if r.Category == "subscriptions" && r.Source == "detected" {
			if first, err := parseDate(r.FirstDate); err == nil && first.After(today.AddDate(0, 0, -40)) && first.After(a.firstDate.AddDate(0, 0, 7)) {
				add("new_subscription:"+r.ID, "new_subscription", "info", "recurring", r.ID, map[string]any{
					"name": r.Name, "amount": money(-r.Typical), "yearlyCost": money(-r.Typical * 12),
				})
			}
		}
		// A bill due within 3 days while the card that pays it can't cover it.
		if r.Type == "expense" && r.AccountID != "" && !r.NextDate.Before(today) && r.NextDate.Before(today.AddDate(0, 0, 4)) {
			if acc := u.account(r.AccountID); acc != nil && toAZN(acc.BalanceMinor, acc.Currency, a.fx) < -r.Typical {
				add("low_balance:"+r.ID+":"+r.NextDate.Format(dateLayout), "low_balance_before_payment", "critical", "accounts", acc.ID, map[string]any{
					"name": r.Name, "dueDate": r.NextDate.Format(dateLayout), "amount": money(-r.Typical),
					"accountName": acc.Name, "balance": money(toAZN(acc.BalanceMinor, acc.Currency, a.fx)),
				})
			}
		}
	}

	for _, c := range u.Connections {
		if dl := daysLeft(c.ValidUntil, now); c.Status == "active" && dl >= 0 && dl <= 7 {
			add("connection_expiring:"+c.ID+":"+c.ValidUntil, "connection_expiring", "warning", "connections", c.ID, map[string]any{
				"bankName": bankName(c.BankCode), "daysLeft": dl, "validUntil": c.ValidUntil,
			})
		}
	}

	for _, ev := range seasonalEvents(today) {
		if ev.forHouseholders && !u.Profile.Householder && !u.Profile.SchoolAgeChildren {
			continue
		}
		if ev.key == "school" && !u.Profile.SchoolAgeChildren {
			continue
		}
		if !today.Before(ev.date.AddDate(0, 0, -ev.leadDays)) && !today.After(ev.date) {
			add(fmt.Sprintf("seasonal:%s:%d", ev.key, ev.date.Year()), "seasonal_reminder", "info", "home", "", map[string]any{
				"event": ev.key, "eventName": ev.name, "date": ev.date.Format(dateLayout),
				"lastYearSpent": money(a.spending(ev.date.AddDate(-1, 0, -ev.leadDays), ev.date.AddDate(-1, 0, 0), nil)),
			})
		}
	}

	st := a.streak(u, s2s.Today)
	for _, m := range []int{3, 7, 14, 30} {
		if st.Current == m {
			add(fmt.Sprintf("streak:%d:%s", m, today.Format(dateLayout)), "streak_milestone", "info", "streak", "", map[string]any{"days": m})
		}
	}
}

func lastAmount(a *Analysis, r *recurringItem) int64 {
	for _, t := range a.txs { // newest first
		if t.recurringID == r.ID {
			return t.amountAzn
		}
	}
	return r.Typical
}

type seasonalEvent struct {
	key, name       string
	date            time.Time
	leadDays        int
	forHouseholders bool
}

// seasonalEvents returns this year's and next year's dates, so a reminder for
// early next year can fire in December.
func seasonalEvents(today time.Time) []seasonalEvent {
	var out []seasonalEvent
	for _, y := range []int{today.Year(), today.Year() + 1} {
		out = append(out,
			seasonalEvent{"school", "Məktəbə hazırlıq", time.Date(y, time.September, 15, 0, 0, 0, 0, baku), 30, true},
			seasonalEvent{"novruz", "Novruz bayramı", time.Date(y, time.March, 20, 0, 0, 0, 0, baku), 21, true},
			seasonalEvent{"new_year", "Yeni il", time.Date(y, time.December, 31, 0, 0, 0, 0, baku), 30, true},
		)
	}
	return out
}

// fallbackText is the alert text for when the app has no template for its type yet.
func fallbackText(al *Alert, lang string) map[string]string {
	p := al.Params
	str := func(k string) string { v, _ := p[k].(string); return v }
	amt := func(k string) string {
		m, _ := p[k].(map[string]any)
		return formatAZN(toInt64(m["amountMinor"]))
	}
	en := lang == "en"
	t := func(az, eng string) string {
		if en {
			return eng
		}
		return az
	}
	switch al.Type {
	case "budget_warning":
		return map[string]string{"title": t("Büdcənin 80%-nə çatdın: ", "You've used 80% of your budget: ") + str("categoryName"),
			"body": t(fmt.Sprintf("%s / %s xərcləndi.", amt("spent"), amt("limit")), fmt.Sprintf("Spent %s of %s.", amt("spent"), amt("limit")))}
	case "budget_over":
		return map[string]string{"title": t("Büdcəni keçdin: ", "Over budget: ") + str("categoryName"),
			"body": t(fmt.Sprintf("%s xərclədin, limit %s idi.", amt("spent"), amt("limit")), fmt.Sprintf("You spent %s against a %s limit.", amt("spent"), amt("limit")))}
	case "purchase_check":
		return map[string]string{"title": t("Bu alış həqiqətən lazım idi?", "Did you really need this?"),
			"body": t(fmt.Sprintf("%s: %s. Bu günün limiti %s idi, artıq %s xərcləmisən.", str("merchant"), amt("amount"), amt("safeToSpendToday"), amt("spentToday")),
				fmt.Sprintf("%s: %s. Today's limit was %s and you've spent %s.", str("merchant"), amt("amount"), amt("safeToSpendToday"), amt("spentToday")))}
	case "income_late":
		return map[string]string{"title": t("Gözlənilən gəlir gecikir: ", "Expected income is late: ") + str("name"),
			"body": t("Gözlənilən tarix: ", "Expected on ") + str("expectedDate")}
	case "bill_changed":
		return map[string]string{"title": t("Hesab dəyişib: ", "Bill changed: ") + str("name"),
			"body": t(fmt.Sprintf("Adətən %s, bu dəfə %s.", amt("typicalAmount"), amt("lastAmount")), fmt.Sprintf("Usually %s, this time %s.", amt("typicalAmount"), amt("lastAmount")))}
	case "new_subscription":
		return map[string]string{"title": t("Yeni abunə: ", "New subscription: ") + str("name"),
			"body": t(fmt.Sprintf("Ayda %s, ildə %s.", amt("amount"), amt("yearlyCost")), fmt.Sprintf("%s a month, %s a year.", amt("amount"), amt("yearlyCost")))}
	case "low_balance_before_payment":
		return map[string]string{"title": t("Ödənişə pul çatmaya bilər: ", "Not enough for an upcoming payment: ") + str("name"),
			"body": t(fmt.Sprintf("%s tarixində %s ödəniləcək, %s kartında %s var.", str("dueDate"), amt("amount"), str("accountName"), amt("balance")),
				fmt.Sprintf("%s is due on %s; %s has %s.", amt("amount"), str("dueDate"), str("accountName"), amt("balance")))}
	case "connection_expiring":
		return map[string]string{"title": t("Bank bağlantısının müddəti bitir: ", "Bank connection expiring: ") + str("bankName"),
			"body": t(fmt.Sprintf("%v gün qalıb. Bir toxunuşla yenilə.", p["daysLeft"]), fmt.Sprintf("%v days left. Renew in one tap.", p["daysLeft"]))}
	case "seasonal_reminder":
		return map[string]string{"title": t("Yaxınlaşır: ", "Coming up: ") + str("eventName"),
			"body": t("Keçən il bu dövrdə xərcin: ", "Last year you spent: ") + amt("lastYearSpent")}
	case "streak_milestone":
		return map[string]string{"title": t(fmt.Sprintf("🔥 %v gün ardıcıl limit daxilində!", p["days"]), fmt.Sprintf("🔥 %v days in a row within your limit!", p["days"])),
			"body": t("Belə davam et.", "Keep it up.")}
	}
	return map[string]string{"title": al.Type, "body": ""}
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}

func formatAZN(minor int64) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	return fmt.Sprintf("%s%d.%02d ₼", sign, minor/100, minor%100)
}
