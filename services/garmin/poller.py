#!/usr/bin/env python3
"""Garmin Connect → Home HQ sidecar.

Polls one Garmin account every GARMIN_REFRESH (default 15m) and POSTs a
normalized JSON blob to the Go server's /internal/garmin ingest endpoint.
"""

from __future__ import annotations

import logging
import os
import sys
import time
from datetime import date, datetime, timedelta, timezone
from pathlib import Path
from typing import Any

import requests
from garminconnect import Garmin

log = logging.getLogger("garmin-sidecar")


def load_dotenv() -> None:
    """Load KEY=VALUE pairs from .env without overriding existing env vars.

    Searches (first match wins for each key): process env already set, then
    GARMIN_ENV_FILE, services/garmin/.env, repo-root .env, cwd .env.
    """
    here = Path(__file__).resolve().parent
    candidates = []
    if os.getenv("GARMIN_ENV_FILE"):
        candidates.append(Path(os.environ["GARMIN_ENV_FILE"]))
    candidates.extend(
        [
            here / ".env",
            here.parent.parent / ".env",  # repo root when at services/garmin/
            Path.cwd() / ".env",
        ]
    )
    seen: set[Path] = set()
    for path in candidates:
        try:
            path = path.resolve()
        except OSError:
            continue
        if path in seen or not path.is_file():
            continue
        seen.add(path)
        try:
            text = path.read_text(encoding="utf-8")
        except OSError as e:
            log.warning("could not read %s: %s", path, e)
            continue
        loaded = 0
        for line in text.splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("export "):
                line = line[7:].strip()
            if "=" not in line:
                continue
            key, _, val = line.partition("=")
            key = key.strip()
            val = val.strip()
            if len(val) >= 2 and val[0] == val[-1] and val[0] in "\"'":
                val = val[1:-1]
            if not key or key in os.environ:
                continue
            os.environ[key] = val
            loaded += 1
        if loaded:
            log.info("loaded %d vars from %s", loaded, path)


def env(name: str, default: str | None = None) -> str | None:
    v = os.getenv(name)
    if v is None or v == "":
        return default
    return v


def parse_refresh(raw: str | None) -> float:
    """Parse Go-style duration (15m, 1h, 900s) or bare seconds into seconds."""
    if not raw:
        return 15 * 60
    s = raw.strip().lower()
    try:
        if s.endswith("ms"):
            return max(1.0, float(s[:-2]) / 1000)
        if s.endswith("s") and not s.endswith("ms"):
            return max(1.0, float(s[:-1]))
        if s.endswith("m"):
            return max(1.0, float(s[:-1]) * 60)
        if s.endswith("h"):
            return max(1.0, float(s[:-1]) * 3600)
        return max(1.0, float(s))
    except ValueError:
        log.warning("invalid GARMIN_REFRESH %r; using 15m", raw)
        return 15 * 60


def dig(obj: Any, *path: str, default: Any = None) -> Any:
    cur = obj
    for key in path:
        if not isinstance(cur, dict) or key not in cur:
            return default
        cur = cur[key]
    return cur if cur is not None else default


def as_int(v: Any, default: int | None = None) -> int | None:
    if v is None:
        return default
    try:
        return int(round(float(v)))
    except (TypeError, ValueError):
        return default


def seconds_to_min(v: Any) -> int | None:
    n = as_int(v)
    if n is None:
        return None
    # Garmin sometimes returns minutes already for stage lengths; treat large
    # values as seconds (>= 3 hours of "minutes" is implausible for a stage).
    if n > 24 * 60:
        return n // 60
    return n


def downsample(points: list[dict[str, int]], max_n: int = 48) -> list[dict[str, int]]:
    if len(points) <= max_n:
        return points
    step = len(points) / max_n
    out: list[dict[str, int]] = []
    i = 0.0
    while len(out) < max_n:
        idx = min(int(i), len(points) - 1)
        out.append({"t": len(out), "v": points[idx]["v"]})
        i += step
    return out


def login_client(email: str, password: str, token_dir: str) -> Garmin:
    os.makedirs(token_dir, exist_ok=True)

    def no_mfa() -> str:
        raise RuntimeError(
            "Garmin MFA required; complete an interactive login once so tokens "
            f"are cached under {token_dir}"
        )

    client = Garmin(email, password, prompt_mfa=no_mfa)
    client.login(token_dir)
    return client


def parse_sleep(raw: Any) -> dict[str, Any]:
    dto = dig(raw, "dailySleepDTO", default=raw if isinstance(raw, dict) else {}) or {}
    score = as_int(
        dig(dto, "sleepScores", "overall", "value")
        or dig(dto, "sleepScore")
        or dig(raw, "sleepScore")
    )
    duration = seconds_to_min(
        dig(dto, "sleepTimeSeconds") or dig(raw, "sleepTimeSeconds")
    )
    deep = seconds_to_min(dig(dto, "deepSleepSeconds") or dig(raw, "deepSleepSeconds"))
    light = seconds_to_min(dig(dto, "lightSleepSeconds") or dig(raw, "lightSleepSeconds"))
    rem = seconds_to_min(dig(dto, "remSleepSeconds") or dig(raw, "remSleepSeconds"))
    awake = seconds_to_min(dig(dto, "awakeSleepSeconds") or dig(raw, "awakeSleepSeconds"))
    return {
        "score": score,
        "durationMin": duration,
        "deepMin": deep,
        "lightMin": light,
        "remMin": rem,
        "awakeMin": awake,
    }


def parse_body_battery(raw: Any) -> dict[str, Any]:
    """Normalize get_body_battery / events into current + charged/drained + series."""
    current = None
    charged = None
    drained = None
    series_pts: list[dict[str, int]] = []

    days = raw if isinstance(raw, list) else [raw] if isinstance(raw, dict) else []
    day = days[-1] if days else {}

    if isinstance(day, dict):
        charged = as_int(day.get("charged") or dig(day, "bodyBatteryChargedValue"))
        drained = as_int(day.get("drained") or dig(day, "bodyBatteryDrainedValue"))
        values = day.get("bodyBatteryValuesArray") or day.get("bodyBatteryValues") or []
        # valuesArray: [[timestamp_ms, value], ...]
        for item in values:
            if isinstance(item, (list, tuple)) and len(item) >= 2:
                v = as_int(item[1])
                if v is not None:
                    series_pts.append({"t": len(series_pts), "v": v})
            elif isinstance(item, dict):
                v = as_int(item.get("value") or item.get("bodyBattery"))
                if v is not None:
                    series_pts.append({"t": len(series_pts), "v": v})

        current = as_int(
            day.get("bodyBatteryMostRecentValue")
            or day.get("mostRecentValue")
            or dig(day, "bodyBattery", "value")
        )

    if current is None and series_pts:
        current = series_pts[-1]["v"]

    return {
        "current": current,
        "charged": charged,
        "drained": drained,
        "series": downsample(series_pts),
    }


def _series_from_values(values: Any) -> list[dict[str, int]]:
    pts: list[dict[str, int]] = []
    for item in values or []:
        if isinstance(item, (list, tuple)) and len(item) >= 2:
            v = as_int(item[1])
            if v is not None and v >= 0:
                pts.append({"t": len(pts), "v": v})
        elif isinstance(item, dict):
            v = as_int(item.get("value") or item.get("stressLevel"))
            if v is not None and v >= 0:
                pts.append({"t": len(pts), "v": v})
    return downsample(pts)


def parse_stress(raw: Any, stats: dict[str, Any]) -> dict[str, Any]:
    current = as_int(
        dig(raw, "overallStressLevel")
        or dig(raw, "avgStressLevel")
        or dig(stats, "averageStressLevel")
    )
    avg = as_int(
        dig(raw, "avgStressLevel")
        or dig(raw, "overallStressLevel")
        or dig(stats, "averageStressLevel")
    )
    rest = seconds_to_min(
        dig(raw, "restStressDuration")
        or dig(raw, "restDuration")
        or dig(stats, "restStressDuration")
    )
    high = seconds_to_min(
        dig(raw, "highStressDuration")
        or dig(stats, "highStressDuration")
    )
    values = dig(raw, "stressValuesArray", default=[]) or []
    series = _series_from_values(values)
    for item in reversed(values):
        if isinstance(item, (list, tuple)) and len(item) >= 2:
            v = as_int(item[1])
            if v is not None and v >= 0:
                current = v
                break
    return {
        "current": current,
        "avg": avg,
        "restMin": rest,
        "highMin": high,
        "series": series,
    }


def _weekday_label(d: date) -> str:
    return ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"][d.weekday()]


def parse_steps_day(raw: Any) -> list[dict[str, int]]:
    """Hourly (or finer) step samples → downsampled {t,v} series."""
    pts: list[dict[str, int]] = []
    rows = raw if isinstance(raw, list) else dig(raw, "values", default=[]) or []
    for item in rows:
        if isinstance(item, dict):
            v = as_int(item.get("steps") or item.get("value"))
            if v is not None:
                pts.append({"t": len(pts), "v": v})
        elif isinstance(item, (list, tuple)) and len(item) >= 2:
            v = as_int(item[1])
            if v is not None:
                pts.append({"t": len(pts), "v": v})
    return downsample(pts, 24)


def build_history(client: Garmin, today: date, stress: dict[str, Any], bb: dict[str, Any]) -> dict[str, Any]:
    start = today - timedelta(days=6)
    start_s, end_s = start.isoformat(), today.isoformat()
    week_days = [start + timedelta(days=i) for i in range(7)]

    # ---- body battery week (end-of-day / most recent per day)
    bb_week: list[dict[str, Any]] = []
    try:
        bb_range = client.get_body_battery(start_s, end_s) or []
        by_date: dict[str, int] = {}
        for day in bb_range if isinstance(bb_range, list) else []:
            if not isinstance(day, dict):
                continue
            dkey = str(day.get("date") or day.get("calendarDate") or "")[:10]
            v = as_int(
                day.get("bodyBatteryMostRecentValue")
                or day.get("mostRecentValue")
                or day.get("charged")
            )
            vals = day.get("bodyBatteryValuesArray") or []
            if v is None and vals:
                last = vals[-1]
                if isinstance(last, (list, tuple)) and len(last) >= 2:
                    v = as_int(last[1])
            if dkey and v is not None:
                by_date[dkey] = v
        for d in week_days:
            bb_week.append({"d": _weekday_label(d), "v": by_date.get(d.isoformat())})
    except Exception as e:  # noqa: BLE001
        log.warning("body battery week failed: %s", e)

    # ---- sleep week
    sleep_week: list[dict[str, Any]] = []
    try:
        sleep_range = client.get_sleep_daily(start_s, end_s) or []
        by_date = {}
        rows = sleep_range if isinstance(sleep_range, list) else []
        for row in rows:
            if not isinstance(row, dict):
                continue
            parsed = parse_sleep(row)
            dkey = str(
                row.get("calendarDate")
                or dig(row, "dailySleepDTO", "calendarDate")
                or ""
            )[:10]
            if dkey:
                by_date[dkey] = parsed
        for d in week_days:
            p = by_date.get(d.isoformat(), {})
            sleep_week.append(
                {
                    "d": _weekday_label(d),
                    "score": p.get("score"),
                    "durationMin": p.get("durationMin"),
                }
            )
    except Exception as e:  # noqa: BLE001
        log.warning("sleep week failed: %s", e)
        # Fallback: one call per day is heavy; leave empty on failure

    # ---- stress week
    stress_week: list[dict[str, Any]] = []
    try:
        stress_range = client.get_daily_stress_stats(start_s, end_s) or []
        by_date = {}
        rows = stress_range if isinstance(stress_range, list) else []
        for row in rows:
            if not isinstance(row, dict):
                continue
            dkey = str(row.get("calendarDate") or row.get("date") or "")[:10]
            v = as_int(
                row.get("overallStressLevel")
                or row.get("averageStressLevel")
                or row.get("avgStressLevel")
            )
            if dkey and v is not None:
                by_date[dkey] = v
        for d in week_days:
            stress_week.append({"d": _weekday_label(d), "v": by_date.get(d.isoformat())})
    except Exception as e:  # noqa: BLE001
        log.warning("stress week failed: %s", e)

    # ---- activity week + today hourly steps
    act_week: list[dict[str, Any]] = []
    try:
        steps_range = client.get_daily_steps(start_s, end_s) or []
        by_date = {}
        rows = steps_range if isinstance(steps_range, list) else []
        for row in rows:
            if not isinstance(row, dict):
                continue
            dkey = str(row.get("calendarDate") or row.get("date") or "")[:10]
            steps = as_int(row.get("totalSteps") or row.get("steps"))
            if dkey:
                by_date[dkey] = {
                    "steps": steps,
                    "activeMin": as_int(row.get("activeTime") or row.get("activeMinutes")),
                    "calories": as_int(row.get("totalKilocalories") or row.get("calories")),
                }
        for d in week_days:
            p = by_date.get(d.isoformat(), {})
            act_week.append(
                {
                    "d": _weekday_label(d),
                    "steps": p.get("steps"),
                    "activeMin": p.get("activeMin"),
                    "calories": p.get("calories"),
                }
            )
    except Exception as e:  # noqa: BLE001
        log.warning("steps week failed: %s", e)

    steps_day: list[dict[str, int]] = []
    try:
        steps_day = parse_steps_day(client.get_steps_data(end_s) or [])
    except Exception as e:  # noqa: BLE001
        log.warning("steps day failed: %s", e)

    return {
        "bodyBattery": {
            "day": bb.get("series") or [],
            "week": bb_week,
        },
        "sleep": {"week": sleep_week},
        "stress": {
            "day": stress.get("series") or [],
            "week": stress_week,
        },
        "activity": {
            "day": steps_day,
            "week": act_week,
        },
    }


def _active_min(stats: dict[str, Any]) -> int | None:
    sec = as_int(stats.get("activeSeconds"))
    if sec is not None:
        return sec // 60
    mod = as_int(stats.get("moderateIntensityMinutes"), 0) or 0
    vig = as_int(stats.get("vigorousIntensityMinutes"), 0) or 0
    if mod or vig:
        return mod + vig
    return as_int(stats.get("activeMinutes") or stats.get("totalActiveMinutes"))


def parse_activity(stats: dict[str, Any]) -> dict[str, Any]:
    return {
        "steps": as_int(stats.get("totalSteps") or stats.get("steps")),
        "stepsGoal": as_int(stats.get("dailyStepGoal") or stats.get("stepGoal")),
        "activeMin": _active_min(stats),
        "calories": as_int(
            stats.get("totalKilocalories")
            or stats.get("calories")
            or stats.get("netCalorieIntake")
        ),
    }


def build_payload(client: Garmin, today: str) -> dict[str, Any]:
    display = None
    try:
        display = client.get_full_name()
    except Exception as e:  # noqa: BLE001
        log.debug("display name unavailable: %s", e)

    sleep_raw: Any = {}
    try:
        sleep_raw = client.get_sleep_data(today) or {}
    except Exception as e:  # noqa: BLE001
        log.warning("sleep fetch failed: %s", e)

    bb_raw: Any = []
    try:
        bb_raw = client.get_body_battery(today, today) or []
    except Exception as e:  # noqa: BLE001
        log.warning("body battery fetch failed: %s", e)

    stress_raw: Any = {}
    try:
        stress_raw = client.get_all_day_stress(today) or {}
    except Exception as e:  # noqa: BLE001
        log.warning("stress fetch failed: %s", e)

    stats: dict[str, Any] = {}
    try:
        stats = client.get_stats(today) or {}
    except Exception as e:  # noqa: BLE001
        log.warning("stats fetch failed: %s", e)

    sleep = parse_sleep(sleep_raw)
    body_battery = parse_body_battery(bb_raw)
    stress = parse_stress(stress_raw, stats)
    activity = parse_activity(stats)
    today_d = date.fromisoformat(today)
    history = build_history(client, today_d, stress, body_battery)

    return {
        "updatedAt": datetime.now(timezone.utc).isoformat(),
        "displayName": display or env("GARMIN_DISPLAY_NAME") or "Garmin",
        "sleep": sleep,
        "bodyBattery": body_battery,
        "stress": stress,
        "activity": activity,
        "history": history,
    }


def push(payload: dict[str, Any], hq_url: str, token: str) -> None:
    url = hq_url.rstrip("/") + "/internal/garmin"
    r = requests.post(
        url,
        json=payload,
        headers={"X-HQ-Token": token, "Content-Type": "application/json"},
        timeout=30,
    )
    if r.status_code not in (200, 204):
        raise RuntimeError(f"ingest failed HTTP {r.status_code}: {r.text[:200]}")


def tick(client: Garmin, hq_url: str, token: str) -> None:
    today = date.today().isoformat()
    payload = build_payload(client, today)
    push(payload, hq_url, token)
    bb = dig(payload, "bodyBattery", "current")
    steps = dig(payload, "activity", "steps")
    log.info("pushed garmin snapshot (bodyBattery=%s steps=%s)", bb, steps)


def main() -> int:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    load_dotenv()

    email = env("GARMIN_EMAIL")
    password = env("GARMIN_PASSWORD")
    token = env("HQ_INGEST_TOKEN")
    hq_url = env("HQ_URL", "http://localhost:8080") or "http://localhost:8080"
    token_dir = env("GARMIN_TOKEN_DIR", "./.garmin-tokens") or "./.garmin-tokens"
    interval = parse_refresh(env("GARMIN_REFRESH", "15m"))

    if not email or not password:
        log.error("GARMIN_EMAIL and GARMIN_PASSWORD are required")
        return 1
    if not token:
        log.error("HQ_INGEST_TOKEN is required")
        return 1

    log.info("logging in to Garmin (token dir %s)", token_dir)
    try:
        client = login_client(email, password, token_dir)
    except Exception as e:  # noqa: BLE001
        log.error("login failed: %s", e)
        return 1

    log.info("polling every %ss → %s", int(interval), hq_url)
    while True:
        try:
            tick(client, hq_url, token)
        except Exception as e:  # noqa: BLE001
            log.error("tick failed: %s", e)
            # Attempt re-login on auth-ish failures
            msg = str(e).lower()
            if "auth" in msg or "login" in msg or "401" in msg or "403" in msg:
                try:
                    log.info("re-login after auth error")
                    client = login_client(email, password, token_dir)
                except Exception as e2:  # noqa: BLE001
                    log.error("re-login failed: %s", e2)
        time.sleep(interval)


if __name__ == "__main__":
    sys.exit(main())
