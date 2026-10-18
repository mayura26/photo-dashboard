# Home HQ

Family dashboard shell: Go backend + WebSockets + plain HTML/CSS/ES modules (web components). Look and feel ported from [wesbos/photo-frame-dashboard](https://github.com/wesbos/photo-frame-dashboard) `dashboard/`.

No Node, no Vite, no npm. Content is still placeholder — see [DEMO.md](DEMO.md).

## Run

```bash
cp .env.example .env   # optional: WEATHER_*, HQ_PASSWORD, …
make run
```

Open [http://localhost:8080](http://localhost:8080).

| Target | What it does |
|--------|----------------|
| `make run` | `go run` with `.env` loaded if present |
| `make build` | compile to `bin/server` |
| `make test` | `go test ./...` |
| `make clean` | remove `bin/` |

Or without Make: `go run ./cmd/server` (pass env vars yourself). Optional `ADDR=:3000`.

### Live weather

Forecasts from [Open-Meteo](https://open-meteo.com/) (CC BY 4.0; no API key for personal use). See [`.env.example`](.env.example). Without lat/lon, seed weather is used.
### Coolify

Use the repo `Dockerfile`. Set **Ports Exposes** to `8080` (must match `PORT`). Domain should proxy to container port `8080`. Redeploy after changing ports. Add `WEATHER_LAT` / `WEATHER_LON` (and optional `WEATHER_REFRESH`) as environment variables for live weather.

Set `HQ_PASSWORD` to require a shared login before the dashboard connects (long-lived cookie; optional `HQ_AUTH_SECRET`). Leave unset for open local use.

## Architecture

- **Go** serves `web/` as static files and owns in-memory state (`internal/demo`).
- **WebSocket** `/ws` sends a full JSON snapshot on connect and after every action.
- **Frontend** is plain ES modules + light-DOM custom elements (`hq-rail`, `hq-home`, …).

## Shell shortcuts

- **A** ambient · **N** night · **1–5** switch views · **Esc** close sheet
- Hold **Today** 2s in night/nightdash → day (idle can return to ambient/night)
