# Cibim mobile

React Native (Expo SDK 57, Expo Router) app built from the Figma file "Cibim Main App Design". It talks only to the Cibim backend (`https://cibim.farahmobilya.az/api/v1` by default).

## Run on a phone (Expo Go)

```bash
cd mobile
npm install
npx expo start            # scan the QR code with Expo Go (Android) or the Camera app (iOS)
```

Phone and computer on the same Wi-Fi. If they aren't, use `npx expo start --tunnel`.

Other API: `EXPO_PUBLIC_API_URL=http://192.168.1.10:8080/api/v1 npx expo start`.

## Screens (Figma frame → route)

| Figma | Route |
|---|---|
| 1.1 Splash · 1.2 Login · 1.3 Connect banks | `/` · `/login` · `/connect` |
| 2.1 Home · 2.2 Feed · 2.3 Overview · 2.4 Bills · 2.5 Profile | `/home` `/feed` `/overview` `/bills` `/profile` |
| 3.x Account | `/account/[id]` |
| 4.x Transaction (re-categorise) | `/transaction/[id]` |
| 5.1 Plan a purchase · 5.2 Budgets · 5.3 Edit limit | `/plan` · `/budgets` · `/budget-edit` |
| 6.x Recurring · 6.9 Loans | `/recurring/[id]` · `/loans` |
| 7.1 Alerts · 7.2 Connected banks | `/alerts` · `/banks` |
| 8.x States | inside the screens above (no banks, loading, errors, approval cancelled, no limit) |

1.4 Bank approval is the mock bank's own page (served by `mock-cbar`), opened in the in-app browser.

## Demo flow

1. Pick a demo user. A new user lands on **Connect banks**.
2. **Connect** opens the bank's approval page → **Təsdiqlə** → back in the app the bank shows as connected. "Demo shortcut: connect all my banks" links everything in one tap.
3. Pull down on any tab to sync (`POST /sync`). After injecting a transaction on the mock server, pull to refresh on Home to see the new alert.
