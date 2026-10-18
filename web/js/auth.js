/** Shared-password login gate. Cookie is HttpOnly; status/login are HTTP-only. */

export async function ensureAuth() {
  const status = await fetchStatus();
  if (!status.required || status.ok) return;
  window.__hqAuthGate = true;
  await showLogin();
  window.__hqAuthGate = false;
}

async function fetchStatus() {
  const res = await fetch('/api/auth/status', { cache: 'no-store', credentials: 'same-origin' });
  if (!res.ok) throw new Error('auth status ' + res.status);
  return res.json();
}

function showLogin() {
  return new Promise((resolve) => {
    const root = document.createElement('div');
    root.className = 'auth-gate';
    root.innerHTML =
      '<form class="auth-card" autocomplete="current-password">' +
        '<p class="label">Home HQ</p>' +
        '<h1 class="serif">Enter password</h1>' +
        '<input class="auth-input" type="password" name="password" placeholder="Password" required autofocus>' +
        '<p class="auth-error" hidden></p>' +
        '<button class="auth-submit" type="submit">Unlock</button>' +
      '</form>';

    const form = root.querySelector('form');
    const input = root.querySelector('input');
    const errEl = root.querySelector('.auth-error');
    const btn = root.querySelector('.auth-submit');

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      errEl.hidden = true;
      btn.disabled = true;
      try {
        const res = await fetch('/api/login', {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ password: input.value }),
        });
        if (!res.ok) {
          errEl.textContent = res.status === 401 ? 'Wrong password' : 'Login failed';
          errEl.hidden = false;
          input.select();
          return;
        }
        root.remove();
        resolve();
      } catch (_) {
        errEl.textContent = 'Could not reach server';
        errEl.hidden = false;
      } finally {
        btn.disabled = false;
      }
    });

    document.body.appendChild(root);
    input.focus();
  });
}
