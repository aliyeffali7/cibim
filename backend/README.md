# Cibim backend

Cibim's own API for the mobile app, following **App API Contract v0.1**. It reads bank data from the CBAR gateway (our mock, see `../mock-cbar`), categorises it and computes what each screen shows.

Go standard library only. State lives in one JSON file (`DATA_FILE`), written after every change.

```
[Mobile app] ⇄ /api/v1 [backend :8080] ⇄ /v1 [mock CBAR :4000]
```

## Run locally

```bash
cd ../mock-cbar && go run . &                  # bank gateway on :4000
cd ../backend && MOCK_CBAR_URL=http://localhost:4000 go run .   # API on :8080
go test ./...
```

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
auth (demo), banks, connections (create, list, poll, renew, delete), sync, home, accounts, transactions (filters, cursor paging, re-categorise with `applyToMerchant`), categories, summary, recurring (list, calendar, create, edit, delete), loans, budgets, safe-to-spend, streak, alerts, me.

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

Next to the mock, behind the same domain: nginx sends `/api/` to the backend and everything else to the mock.

```bash
cd /opt/cibim && git pull
cd backend && cp .env.example .env && docker compose up -d --build
curl -s localhost:8080/health
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
| `DATA_FILE` | `data/state.json` | state file (`/data/state.json` in Docker) |
