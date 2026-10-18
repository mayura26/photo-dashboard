// Build-version gate for long-lived kiosk tabs.
// When the server version changes after deploy, force a full reload so
// cached JS/CSS are revalidated (paired with Cache-Control: no-cache).

const STORAGE_KEY = 'hq_version';

export function checkVersion(version) {
  if (!version || typeof version !== 'string') return false;
  let stored = null;
  try {
    stored = sessionStorage.getItem(STORAGE_KEY);
  } catch {
    // private mode / blocked storage — still allow first paint
  }
  if (stored && stored !== version) {
    try {
      sessionStorage.setItem(STORAGE_KEY, version);
    } catch { /* ignore */ }
    location.reload();
    return true;
  }
  if (stored !== version) {
    try {
      sessionStorage.setItem(STORAGE_KEY, version);
    } catch { /* ignore */ }
  }
  return false;
}

export async function fetchAndCheckVersion() {
  try {
    const res = await fetch('/version.json', { cache: 'no-store' });
    if (!res.ok) return false;
    const meta = await res.json();
    return checkVersion(meta && meta.version);
  } catch {
    return false;
  }
}
