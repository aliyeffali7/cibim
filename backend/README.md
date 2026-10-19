# Cibim backend

Cibim's own API for the mobile app, following **App API Contract v0.1**. It reads bank data from the CBAR gateway (our mock, see `../mock-cbar`), categorises it and computes what each screen shows.

Go, with one dependency (`pgx`, vendored). Data lives in **PostgreSQL** (`DATABASE_URL`); without it the backend falls back to a JSON file (`DATA_FILE`), handy for local runs and tests.

```
[Mobile app] ⇄ /api/v1 [backend :8080] ⇄ /v1 [mock CBAR :4000]
```

## Run locally

```bash
cd ../mock-cbar && go run . &                  # bank gateway on :4000
cd ../backend && MOCK_CBAR_URL=http://localhost:4000 go run .   # API on :8080, JSON file storage
go test ./...
```

With PostgreSQL: start one (e.g. `docker compose up -d postgres`) and add
`DATABASE_URL=postgres://cibim:cibim-dev-password@127.0.0.1:5433/cibim?sslmode=disable`.

## Storage

The API works on an in-memory copy of the data. After every change it rewrites the user's rows in PostgreSQL inside one transaction, and on start it loads everything back. The schema is created automatically (`pgstore.go`):

| Table | Holds |
|---|---|
| `users` | demo users, language, profile (householder, school-age children) |
| `connections` | bank links: consent ID, status, valid until, last sync |
| `accounts` | synced card and current accounts with balances |
| `transactions` | 90 days of synced bank entries (raw, as the bank sent them) |
| `category_overrides` | categories the user changed, per transaction or per merchant |
| `recurring_overrides`, `manual_recurring` | edits to detected recurring items, and items added by hand |
| `loans` | loan details the user entered, and manual loans |
| `budgets` | the user's budgets (week, month or year) with the manual "already spent" figure |
| `purchase_plans` | purchase planner entries: planned, dropped ("Don't buy") or bought |
| `alerts` | generated alerts with read state (`params` is JSONB) |
| `fx_rates` | the latest CBAR rates |

Categories, recurring items, safe-to-spend and so on are computed from these on each request, so they always reflect the current rules.

On the first start with an empty database, an existing JSON state file is imported automatically.

## Quick start for the app

```bash
API=http://localhost:8080/api/v1

curl $API/auth/demo-users                                   # Aysel, Rəşad, Nigar
curl -X POST $API/auth/demo-login -d '{"psuId":"u-aysel"}'  # → {"token":"demo.u-aysel", ...}
T='Authorization: Bearer demo.u-aysel'

# Normal flow: create a connection, open authUrl, approve, then poll the connection.
curl -X POST -H "$T" $API/connections -d '{"bankCode":"ABB"}'   # → {connectionId, status:"pending", authUrl}
curl -H "$T" $API/connections/con_1                              # → status "active" after approval

# Shortcut for the stage: link all of the user's banks without the approval page.
curl -X POST -H "$T" $API/demo/connect-all

curl -H "$T" $API/home
```

Tokens are `demo.<psuId>`, so the app can also hard-code one.

## Endpoints

Everything in the contract is implemented:
auth (demo), banks, connections (create, list, poll, renew, delete), sync, home, accounts, transactions (filters incl. `bankCode` and `recurringId`, cursor paging, `GET /transactions/{id}`, re-categorise with `applyToMerchant`), categories, summary, recurring (list, calendar, create, edit, delete), loans, budgets (week, month, year; `alreadySpent`; `dataSince`), purchase planner, safe-to-spend, streak, alerts, me.

### Purchase planner (contract v0.2–v0.3)

```bash
# Dry run: nothing is saved.
curl -X POST -H "$T" $API/purchase-plans/check -d '{"name":"Washing machine","price":{"amountMinor":120000,"currency":"AZN"},"categoryId":"cat-assets"}'
# → status no_limit until the user has a limit for that category:
curl -X POST -H "$T" $API/budgets -d '{"categoryId":"cat-assets","period":"year","limit":{"amountMinor":500000,"currency":"AZN"},"alreadySpent":{"amountMinor":410000,"currency":"AZN"}}'
# → check again: projected 5300 ₼, pct 106, status exceeds, overage 300 ₼

curl -X POST -H "$T" $API/purchase-plans -d '{"name":"Washing machine","price":{"amountMinor":120000,"currency":"AZN"},"categoryId":"cat-assets","status":"dropped"}'  # "Don't buy"
curl -H "$T" "$API/purchase-plans?status=planned"
curl -X PATCH -H "$T" $API/purchase-plans/plan_3 -d '{"status":"bought"}'
```

- Planner categories: `cat-clothes`, `cat-home`, `cat-tech`, `cat-school`, `cat-health`, `cat-assets`, `cat-other`. They are regular categories, so bank transactions land in them too (Zara, LC Waikiki → clothes; Kontakt Home → tech; Ali & Nino, Ofis Market → school).
- The limit is the user's own budget for that category; with several, the longest period wins (year over month over week). Baselines don't count as limits.
- `spentSoFar` = spending in the limit's period + the budget's `alreadySpent`. `plannedOther` = other plans with status `planned` in the same category.
- `fits` ≤ 80% of the limit < `close` ≤ 100% < `exceeds`. `pct` is a whole number.
- Rəşad's demo data has ~1 244 ₼ of school supplies (Aug–Sep), so a yearly school limit shows the planner working on real transactions.

### Additions to contract v0.1 (for the next version)

- `POST /connections` and `POST /connections/{id}/renew` accept an optional `redirectUri` (`{id}` is replaced). Use it in Expo Go, where the deep link is `exp://…/--/connections/{id}`.
- `POST /demo/connect-all`: demo-only shortcut, links every bank of the user and approves through the mock.
- `POST /sync` finishes the sync before answering `202`, so the app can refetch immediately. Response also carries `dataUpdatedAt`.
- `GET /safe-to-spend` also returns `spentToday`; `GET /streak` also returns `today`.
- Pending transactions carry today's date in `date`.
- Errors from the bank gateway return `502 BANK_UNAVAILABLE`; creating a connection for an already linked bank returns `409 VALIDATION_ERROR`.

## How the numbers are computed

- **Categories:** rules on the raw bank text first (`POS BRAVO HYPER…` → Bravo · Ərzaq), then the MCC, then income/transfer defaults. The user's own choices always win. Rules live in `categorize.go`.
- **Internal transfers:** both legs visible (same amount, opposite sign, different accounts, ≤ 2 days apart), or an outgoing transfer naming one of the user's own cards (`TO CARD *7731`). They show in the feed but never count as spending or income.
- **Recurring:** same merchant, amount within ±10%, interval 27–34 days (monthly, 2+ hits) or 6–8 days (weekly, 3+ hits). Utility bills skip the amount check; a jump over 10% marks them `changed`. An item 3+ days overdue is `late`.
- **Loans:** recurring payments in the loans category (credit, taksit). Remaining and payoff stay `null` until the user fills them in.
- **Safe-to-spend:** card balances at the start of today − bills due before the next income − 10% buffer, divided by the days left. Next income = the largest expected income in the next 5 weeks.
- **Streak:** a day counts as under when its day-to-day spending (not bills or subscriptions) stayed within today's safe-to-spend.
- **Budgets:** the user's own, plus baselines for total spending and the month's top 3 day-to-day categories: the average of the last up to 3 full months. Warning at 80%, over at 100%.
- **Alerts:** generated after every sync and deduplicated: budget warning/over, purchase check (a ≥ 50 ₼ day-to-day purchase while over today's limit or a budget), income late, bill changed, new subscription, low balance before a payment, connection expiring, seasonal reminders (school, Novruz, New Year; from the profile), streak milestones.

## On-stage demo

```bash
# 1. Before the pitch: clean slate.
curl -X POST https://cibim.farahmobilya.az/mock/reset

# 2. Live: Aysel buys something big. Then pull to refresh in the app (POST /sync).
curl -X POST https://cibim.farahmobilya.az/mock/transactions -H 'Content-Type: application/json' -d '{
  "resourceId":"acc-aysel-abb-1","status":"pending",
  "transactionAmount":{"currency":"AZN","amount":"-120.00"},
  "creditorName":"ZARA","remittanceInformationUnstructured":"POS ZARA PORT BAKU MALL","merchantCategoryCode":"5651"}'
# → alert "Bu alış həqiqətən lazım idi?" plus a shopping budget alert.
```

After a mock reset, old connections show as `failed`; link the banks again.

## Deploy on the VPS

Next to the mock, behind the same domain: nginx sends `/api/` to the backend and everything else to the mock. `docker compose` starts two containers: `cibim-postgres` (port 5433, localhost only) and `cibim-backend`.

```bash
cd /opt/cibim && git pull
cd backend && cp .env.example .env     # set POSTGRES_PASSWORD
docker compose up -d --build
curl -s localhost:8080/health
docker exec -it cibim-postgres psql -U cibim -c '\dt'   # look at the tables
```

Add the `/api/` route to the existing nginx site (in front of `location / {`, in every server block that has one), then reload:

```nginx
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
```

The app then uses `https://cibim.farahmobilya.az/api/v1`.

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | listen port |
| `MOCK_CBAR_URL` | `http://localhost:4000` | bank gateway base URL |
| `APP_REDIRECT_URL` | `cibim://connections/{id}` | where the bank's approval page returns |
| `SYNC_STALE_AFTER` | `10m` | auto-sync when data is older |
| `DATABASE_URL` | empty | PostgreSQL connection string; set by docker compose |
| `DATA_FILE` | `data/state.json` | JSON state when there is no database; imported into an empty database (`/data/state.json` in Docker) |
