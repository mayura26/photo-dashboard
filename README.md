# Home HQ

Family dashboard shell: Go backend + WebSockets + plain HTML/CSS/ES modules (web components). Look and feel ported from [wesbos/photo-frame-dashboard](https://github.com/wesbos/photo-frame-dashboard) `dashboard/`.

No Node, no Vite, no npm. Content is still placeholder — see [DEMO.md](DEMO.md).

## Run

```bash
go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080).

Optional: `ADDR=:3000 go run ./cmd/server`

## Architecture

- **Go** serves `web/` as static files and owns in-memory state (`internal/demo`).
- **WebSocket** `/ws` sends a full JSON snapshot on connect and after every action.
- **Frontend** is plain ES modules + light-DOM custom elements (`hq-rail`, `hq-home`, …).

## Shell shortcuts

- **A** ambient · **N** night · **1–5** switch views · **Esc** close sheet
