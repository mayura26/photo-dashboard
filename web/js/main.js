import { connect } from './ws-client.js';
import store from './store.js';
import { ensureAuth } from './auth.js';
import './components/hq-rail.js';
import './components/hq-views.js';

window.__hqAction = (name, payload) => store.dispatch(name, payload);

async function boot() {
  await ensureAuth();
  const first = await connect();
  store.applySnapshot(first.data);

  // Load shell + views only after DATA exists (they close over window.DATA).
  await import('./icons.js');
  await import('./app.js');
  await import('./slideshow.js');
  await import('./system.js');
  await import('./views/home.js');
  await import('./views/calendar.js');
  await import('./views/tasks.js');
  await import('./views/shopping.js');
  await import('./views/photos.js');
  await import('./views/body.js');
  await import('./views/ambient.js');

  patchMutations();
  App.start();
}

function patchMutations() {
  App.sync = {
    chore(id, done, doneBy) { store.dispatch('setChore', { id, done, doneBy }); },
    addChore(payload) { store.dispatch('addChore', payload); },
    grocery(id, patch) { store.dispatch('setGrocery', Object.assign({ id }, patch)); },
    addGrocery(payload) { store.dispatch('addGrocery', payload); },
    clearChecked() { store.dispatch('clearChecked', {}); },
    addEvent(ev) {
      const payload = Object.assign({}, ev);
      if (payload.start instanceof Date) payload.start = payload.start.toISOString();
      if (payload.end instanceof Date) payload.end = payload.end.toISOString();
      store.dispatch('addEvent', payload);
    },
    removeEvent(id) { store.dispatch('removeEvent', { id }); },
    fav(seed) { store.dispatch('toggleFav', { seed }); }
  };
}

boot().catch((err) => {
  console.error(err);
  document.body.innerHTML = '<pre style="padding:2rem;font:16px monospace">Failed to connect to Home HQ WebSocket.\n' +
    String(err) + '\n\nIs the Go server running? go run ./cmd/server</pre>';
});
