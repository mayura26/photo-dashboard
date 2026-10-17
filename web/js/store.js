// Local mirror of server demo state. Revives ISO date strings into Date objects.
// Mutates window.DATA in place so view modules that closed over DATA stay live.
import { onMessage, action as wsAction } from './ws-client.js';

const DATE_KEYS = new Set(['start', 'end']);

function revive(value, key) {
  if (Array.isArray(value)) return value.map((v) => revive(v));
  if (value && typeof value === 'object') {
    const out = {};
    for (const [k, v] of Object.entries(value)) out[k] = revive(v, k);
    return out;
  }
  if (typeof value === 'string' && DATE_KEYS.has(key) && /^\d{4}-\d{2}-\d{2}T/.test(value)) {
    return new Date(value);
  }
  return value;
}

function attachCounters(data) {
  if (typeof data._gid !== 'function') {
    let g = 0;
    (data.groceries || []).forEach((x) => {
      const n = parseInt(String(x.id).replace(/\D/g, ''), 10);
      if (n > g) g = n;
    });
    data._gid = () => 'g' + (++g);
  }
  if (typeof data._eid !== 'function') {
    let e = 0;
    (data.events || []).forEach((x) => {
      const n = parseInt(String(x.id).replace(/\D/g, ''), 10);
      if (n > e) e = n;
    });
    data._eid = () => 'e' + (++e);
  }
}

const store = {
  data: null,
  calMode: 'normal',

  get() { return this.data; },

  applySnapshot(raw) {
    const fresh = revive(raw);
    this.calMode = fresh.calMode || 'normal';
    attachCounters(fresh);

    if (!this.data) {
      this.data = fresh;
      window.DATA = fresh;
    } else {
      const target = this.data;
      Object.keys(fresh).forEach((k) => {
        if (k === '_gid' || k === '_eid') return;
        target[k] = fresh[k];
      });
      attachCounters(target);
    }

    if (window.App) {
      // update calMode without re-dispatching
      if (App._setCalModeLocal) App._setCalModeLocal(this.calMode);
      else App.calMode = this.calMode;
      if (typeof fresh.timeOffset === 'number') App._setOffset(fresh.timeOffset);
      App.emit('data');
    }
  },

  dispatch(name, payload) {
    return wsAction(name, payload);
  }
};

onMessage((msg) => {
  if (msg.type === 'snapshot' && msg.data) store.applySnapshot(msg.data);
  if (msg.type === 'event' && window.App) {
    const map = {
      doorbell: () => App.doorbell && App.doorbell(),
      leak: () => App.leak && App.leak(),
      garage: () => App.garageOpenAlert && App.garageOpenAlert(),
      package: () => App.packageNotice && App.packageNotice()
    };
    if (map[msg.name]) map[msg.name]();
  }
});

export default store;
