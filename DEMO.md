# Placeholder data (replace later)

The UI shell is real; most content is still fake until feeds are wired.

| Surface | Where | Notes |
|---------|--------|--------|
| Seed world | `internal/demo/seed.json` | People, calendar, weather (shopping/TODO/photos from SQLite) |
| Weather | Open-Meteo via `WEATHER_LAT`/`WEATHER_LON` | Live when configured; otherwise seed weather |
| Photos | `data/photos/` + SQLite `photos` kv | Upload via Photos → Manage; display-sized JPEG |
| Alert helpers | `web/js/system.js` | Doorbell, leak, garage, package — **no trigger UI**; await real events |
| Home devices | seed `devices` / `climate` / `cameras` / `scenes` | Left over after Home view removal |
| Agents | seed `agents` | Left over after Agents view removal |
| `busyExtras` | seed + Go snapshot | Old calendar density demo; unused by UI |

Server package path `internal/demo` is still named for the placeholder store; rename when real data services land.
