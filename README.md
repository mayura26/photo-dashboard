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
| `make watch` | auto-restart the Go server on `.go` changes ([Air](https://github.com/air-verse/air)) |
| `make build` | compile to `bin/server` |
| `make test` | `go test ./...` |
| `make clean` | remove `bin/` and `tmp/` |
| `make garmin-run` | run the Garmin sidecar (`services/garmin/poller.py`) |

Or without Make: `go run ./cmd/server` (pass env vars yourself). Optional `ADDR=:3000`.

For day-to-day Go work, prefer `make watch` so edits under `cmd/` / `internal/` rebuild and restart automatically. `web/` is served from disk with `Cache-Control: no-cache` — refresh the browser after HTML/JS/CSS changes (no server restart).

### Garmin sidecar

One-time setup, then `make garmin-run` from the repo root (needs the Go server running and `HQ_INGEST_TOKEN` / Garmin creds in `.env`):

```bash
cd services/garmin
python -m venv .venv
# Windows: .venv/Scripts/activate
# Unix:    source .venv/bin/activate
pip install -r requirements.txt
cd ../..
make garmin-run
```

### Live weather

Forecasts from [Open-Meteo](https://open-meteo.com/) (CC BY 4.0; no API key for personal use). See [`.env.example`](.env.example). Without lat/lon, seed weather is used.
### Coolify

Use the repo `Dockerfile`. Set **Ports Exposes** to `8080` (must match `PORT`). Domain should proxy to container port `8080`. Redeploy after changing ports. Add `WEATHER_LAT` / `WEATHER_LON` (and optional `WEATHER_REFRESH`) as environment variables for live weather.

Set `HQ_PASSWORD` to require a shared login before the dashboard connects (long-lived cookie; optional `HQ_AUTH_SECRET`). Leave unset for open local use.

Mount a persistent volume at `/data` and set `HQ_DB_PATH=/data/hq.db` so shopping and TODO survive redeploys.

### Persistent shopping / TODO

Grocery list and chore completion (plus star totals) are stored in SQLite (`HQ_DB_PATH`, default `./data/hq.db`). First run creates an empty DB; later edits survive restarts. On Coolify, mount a volume at `/data` and set `HQ_DB_PATH=/data/hq.db`.

## Architecture

- **Go** serves `web/` as static files and owns in-memory state (`internal/demo`), with shopping/TODO write-through to SQLite (`internal/persist`).
- **WebSocket** `/ws` sends a full JSON snapshot on connect and after every action.
- **Frontend** is plain ES modules + light-DOM custom elements (`hq-rail`, `hq-home`, …).

## Shell shortcuts

- **A** ambient · **N** night · **1–5** switch views · **Esc** close sheet
- Hold **Today** 2s in night/nightdash → day (idle can return to ambient/night)
