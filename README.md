# Home HQ

Family dashboard demo: Go backend + WebSockets + plain HTML/CSS/ES modules (web components). Look and feel ported from [wesbos/photo-frame-dashboard](https://github.com/wesbos/photo-frame-dashboard) `dashboard/`.

No Node, no Vite, no npm. Demo data only.

## Run

```bash
go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080).

Optional: `ADDR=:3000 go run ./cmd/server`

## Architecture

- **Go** serves `web/` as static files and owns in-memory demo state (`internal/demo`).
- **WebSocket** `/ws` sends a full JSON snapshot on connect and after every action.
- **Frontend** is plain ES modules + light-DOM custom elements (`hq-rail`, `hq-home`, …) that reuse the original CSS and view paint logic.

## Demo controls

Press and hold the sidebar clock (or press **D**) for time presets, ambient/night modes, and alert takeovers.
