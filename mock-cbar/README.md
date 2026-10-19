# Mock CBAR server

Imitates CBAR's Open Banking gateway for the Cibim demo, following **Mock CBAR API Contract v0.2**.
Go standard library only, all data in memory, seeded on start with 3 users, 4 banks and 90 days of transactions ending today.

## Run locally

```bash
go run .                 # http://localhost:4000
go test ./...            # end-to-end tests of the consent flow, reads, errors and seed
```

## Deploy on the VPS (Docker + existing nginx)

The container is isolated: its own folder, its own port bound to `127.0.0.1` only, and capped at 128 MB RAM / half a CPU.

```bash
cd /opt && git clone -b claude/peaceful-archimedes-k4a2q1 https://github.com/aliyeffali7/cibim.git
cd /opt/cibim/mock-cbar
cp .env.example .env          # set PUBLIC_BASE_URL to your subdomain
docker compose up -d --build
curl -s localhost:4000/health # {"status":"ok"}
```

Then point a subdomain at the VPS (DNS `A` record → VPS IP) and add an nginx site:

```bash
sudo cp nginx.conf.example /etc/nginx/sites-available/cibim-api   # edit server_name
sudo ln -s /etc/nginx/sites-available/cibim-api /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d cibim-api.example.az                       # free HTTPS
```

Update: `git pull && docker compose up -d --build`. Remove completely: `docker compose down --rmi local`.

## Seed data

| psuId | Persona | Accounts (resourceId) |
|---|---|---|
| `u-aysel` | Salaried employee | `acc-aysel-abb-1` ABB card AZN, `acc-aysel-abb-usd` ABB current USD, `acc-aysel-leo-1` Leobank card AZN |
| `u-rashad` | Householder, school-age kids | `acc-rashad-xalq-1` Xalq card, `acc-rashad-pasha-1` PAŞA card |
| `u-nigar` | Student | `acc-nigar-leo-1` Leobank card |

- **Aysel:** salary on the 15th (AZERTECH MMC), rent on the 1st, Azərişıq/Azərsu/Azəriqaz, Azercell, a loan instalment on the 20th (`ABB KREDIT`), Netflix, Spotify, Wolt+, Apple, monthly freelance income in USD, an ABB→Leobank transfer on the 16th (both legs present), and dining that jumps in the last 30 days (overspend story).
- **Rəşad:** salary on the 25th to Xalq, transfer to PAŞA on the 26th, family utilities, internet, two phones, tutoring, a Kontakt Home taksit, groceries, weekly fuel, and school supplies from 20 Aug to 14 Sep.
- **Nigar:** stipend on the 5th, family transfer on the 1st, coffee, metro, small groceries, Spotify Student.
- About half of card purchases carry `merchantCategoryCode`. Each account has realistic pending items where noted in the contract (Aysel: Wolt, Rəşad: Bravo).
- Data is deterministic for a given day. `POST /mock/reset` reseeds and drops all consents.

## Endpoints

All `/v1` calls need `X-Request-ID`. See the contract for full shapes.

| Method | Path | Notes |
|---|---|---|
| GET | `/v1/banks` | |
| POST | `/v1/consents` | headers `PSU-ID`, `TPP-Redirect-URI`; body `bankCode` |
| GET | `/v1/consents/{id}/status` · `/v1/consents/{id}` | |
| DELETE | `/v1/consents/{id}` | → `terminatedByTpp` |
| GET | `/v1/accounts` | header `Consent-ID` |
| GET | `/v1/accounts/{resourceId}/balances` | `interimAvailable` = booked + pending |
| GET | `/v1/accounts/{resourceId}/transactions` | `dateFrom` (≤ 90 days back), `dateTo`, `bookingStatus` |
| GET/POST | `/mock-sca/{id}` | approve page; the form redirects to `TPP-Redirect-URI?consentId=…&status=valid\|rejected` |
| GET | `/mock/users` | |
| POST | `/mock/consents/{id}/approve` · `/reject` · `/expire` | |
| POST | `/mock/transactions` | inject live (default `pending`) |
| POST | `/mock/reset` | |
| GET | `/mock/fx-rates` | `{"USD":"1.7000","EUR":"1.9850"}` (mock values) |
| GET | `/health` | for Docker and uptime checks |

### Small additions to contract v0.2 (for the next contract version)

- `GET /health` and the `POST /mock-sca/{id}` form target are new; neither is under `/v1`.
- `GET /mock/users` also lists each user's `banks`.
- `transactions` contains only the lists asked for: `booked` for `bookingStatus=booked`, `pending` for `pending`, both for `both`.

## Demo recipes

```bash
API=https://cibim-api.example.az   # or http://localhost:4000

# On stage: Aysel overspends, the app should raise the budget alert on its next sync.
curl -X POST $API/mock/transactions -H 'Content-Type: application/json' -d '{
  "resourceId":"acc-aysel-abb-1","status":"pending",
  "transactionAmount":{"currency":"AZN","amount":"-120.00"},
  "creditorName":"ZARA","remittanceInformationUnstructured":"POS ZARA PORT BAKU MALL","merchantCategoryCode":"5651"}'

# Show consent renewal.
curl -X POST $API/mock/consents/<consentId>/expire

# Clean slate before the pitch.
curl -X POST $API/mock/reset
```

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `4000` | listen port inside the container |
| `PUBLIC_BASE_URL` | derived from request | origin used in `scaRedirect` links |
| `MOCK_ENFORCE_FREQUENCY` | `false` | `true` enforces `frequencyPerDay` (429 `ACCESS_EXCEEDED`) |
