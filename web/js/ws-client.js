// WebSocket client with reconnect. Speaks the dashboard JSON protocol.
import { checkVersion, fetchAndCheckVersion } from './version.js';

const listeners = new Set();

let socket = null;
let reconnectTimer = null;

export function onMessage(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function send(msg) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(msg));
    return true;
  }
  return false;
}

export function action(name, payload = {}) {
  return send({ type: 'action', name, payload });
}

export function connect() {
  return new Promise((resolve, reject) => {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = proto + '//' + location.host + '/ws';
    let settled = false;

    function open() {
      socket = new WebSocket(url);
      socket.addEventListener('open', () => {
        fetchAndCheckVersion();
      });
      socket.addEventListener('message', (ev) => {
        let msg;
        try { msg = JSON.parse(ev.data); } catch { return; }
        // Version gate before any snapshot/event handlers paint stale UI.
        if (msg.version && checkVersion(msg.version)) return;
        if (!settled && msg.type === 'snapshot') {
          settled = true;
          resolve(msg);
        }
        listeners.forEach((fn) => fn(msg));
      });
      socket.addEventListener('close', () => {
        clearTimeout(reconnectTimer);
        reconnectTimer = setTimeout(open, 1500);
      });
      socket.addEventListener('error', () => {
        if (!settled) {
          // allow reconnect; reject only after a long wait is handled by caller timeout
        }
      });
    }

    open();
    setTimeout(() => {
      if (!settled) reject(new Error('WebSocket snapshot timeout'));
    }, 10000);
  });
}
