// Alerts (takeovers, banners, notices). Shell keyboard shortcuts for modes/views.
(function () {
  'use strict';
  var D = window.DATA, $ = App.$;

  // TEMP: picsum placeholders until real photo URLs
  function img(seed, w, h) { return 'https://picsum.photos/seed/' + seed + '/' + w + '/' + h; }
  App.img = img;

  // TEMP: device helpers left over from removed Home control; used only by alert helpers below
  function device(id) {
    return (D.devices || []).filter(function (d) { return d.id === id; })[0];
  }
  App.setDevice = function (id, patch) {
    var d = device(id);
    if (!d) return;
    Object.keys(patch).forEach(function (k) { d[k] = patch[k]; });
    App.emit('data');
    if (window.__hqAction) window.__hqAction('setDevice', { id: id, patch: patch });
  };
  App.openGarageSheet = function () {
    App.setDevice('g-door', { state: 'closed' });
    App.toast('Garage closed', { icon: 'garage' });
  };
  App.cameraSheet = function (c) {
    if (!c) return;
    App.toast(App.esc(c.name) + ' · camera', { icon: 'camera' });
  };

  // ------------------------------------------------------------ full-screen takeovers
  function takeover(html, cls) {
    var el = $('#takeover');
    el.className = 'takeover ' + (cls || '');
    el.innerHTML = html;
    el.classList.add('is-open');
    App.closeSheet();
    if (App.mode !== 'day') App.setMode('day');
    return el;
  }
  function closeTakeover() {
    var el = $('#takeover');
    el.classList.remove('is-open');
    el.innerHTML = '';
  }
  App.closeTakeover = closeTakeover;

  // TEMP: demo-only until real event sources (doorbell, leak sensors, etc.)
  var doorbellTimer;
  App.doorbell = function () {
    var now = App.now();
    var el = takeover(
      '<div class="tk-cam"><img src="' + img('visitor-porch', 1280, 800) + '" alt=""><span class="tk-live"><i></i>LIVE · Front Porch</span></div>' +
      '<div class="tk-door">' +
      '  <div class="label label--light">' + App.icon('bell', 20) + ' Doorbell · ' + App.fmt.time(now) + '</div>' +
      '  <h1 class="serif tk-title">Someone’s at<br>the front door</h1>' +
      '  <div class="tk-actions">' +
      '    <button class="tk-btn" data-talk>' + App.icon('mic', 30) + '<span>Talk</span></button>' +
      '    <button class="tk-btn tk-btn--hold" data-unlock>' + App.icon('unlock', 30) + '<span>Hold to unlock</span></button>' +
      '    <button class="tk-btn tk-btn--solid" data-dismiss>' + App.icon('x', 30) + '<span>Dismiss</span></button>' +
      '  </div>' +
      '</div>', 'takeover--door');
    el.querySelector('[data-dismiss]').onclick = function () { clearTimeout(doorbellTimer); closeTakeover(); };
    el.querySelector('[data-talk]').onclick = function () {
      this.classList.toggle('is-on');
      this.querySelector('span').textContent = this.classList.contains('is-on') ? 'Talking…' : 'Talk';
    };
    var u = el.querySelector('[data-unlock]');
    App.hold(u, 1000, {
      done: function () {
        App.setDevice('e-door', { locked: false });
        u.classList.add('is-on');
        u.querySelector('span').textContent = 'Unlocked';
        App.toast('Front door unlocked · relocks in 2 min', { icon: 'unlock' });
      },
      cancel: function () { App.toast('Keep holding to unlock', { ms: 2000, icon: 'lock' }); }
    });
    clearTimeout(doorbellTimer);
    doorbellTimer = setTimeout(closeTakeover, 45000);
  };

  // TEMP: demo-only until real event sources
  App.leak = function () {
    var now = App.now();
    var el = takeover(
      '<div class="tk-alarm">' +
      '  <div class="tk-alarm-icon">' + App.icon('drop', 88) + '</div>' +
      '  <div class="label label--light">Critical alert · ' + App.fmt.time(now) + '</div>' +
      '  <h1 class="tk-huge">Water leak</h1>' +
      '  <p class="tk-sub">Laundry room sensor detected water on the floor.</p>' +
      '  <div class="tk-actions">' +
      '    <button class="tk-btn tk-btn--hold tk-btn--light" data-valve>' + App.icon('drop', 30) + '<span>Hold to shut off water</span></button>' +
      '    <button class="tk-btn tk-btn--solid tk-btn--light" data-ack>' + App.icon('check', 30) + '<span>Acknowledge</span></button>' +
      '  </div>' +
      '</div>', 'takeover--alarm');
    var v = el.querySelector('[data-valve]');
    App.hold(v, 1200, {
      done: function () { v.classList.add('is-on'); v.querySelector('span').textContent = 'Water main closed'; },
      cancel: function () { App.toast('Keep holding to shut off the water main', { ms: 2000, icon: 'drop' }); }
    });
    el.querySelector('[data-ack]').onclick = function () {
      closeTakeover();
      App.toast('Leak acknowledged by the kitchen display', { icon: 'check' });
    };
  };

  // ------------------------------------------------------------ banners (amber, actionable)
  App.banner = function (opts) {
    var box = $('#banners');
    var b = document.createElement('div');
    b.className = 'banner banner--' + (opts.tone || 'amber');
    b.innerHTML = App.icon(opts.icon || 'bell', 30) + '<div class="banner-text"><b>' + opts.title + '</b><span>' + (opts.sub || '') + '</span></div>' +
      (opts.action ? '<button class="banner-btn banner-btn--solid" data-act>' + opts.action + '</button>' : '') +
      '<button class="banner-btn" data-x>' + (opts.dismiss || 'Dismiss') + '</button>';
    box.appendChild(b);
    b.classList.add('is-in');
    function kill() { b.remove(); }
    b.querySelector('[data-x]').onclick = kill;
    if (opts.action) b.querySelector('[data-act]').onclick = function () { kill(); opts.onAction(); };
    if (opts.ms) setTimeout(kill, opts.ms);
    return kill;
  };

  // TEMP: demo-only until real event sources
  App.garageOpenAlert = function () {
    App.setDevice('g-door', { state: 'open' });
    App.banner({
      icon: 'garage', title: 'Garage open for 15 min', sub: 'No one is in the garage · Wes is 12 min away',
      action: 'Close garage', dismiss: 'Leave open',
      onAction: function () { App.openGarageSheet(); }
    });
  };

  // TEMP: demo-only until real event sources
  App.packageNotice = function () {
    App.banner({
      tone: 'ink', icon: 'box', title: 'Package delivered', sub: 'Front porch · ' + App.fmt.time(App.now()),
      action: 'View camera', dismiss: 'OK', ms: 9000,
      onAction: function () { App.cameraSheet(D.cameras[0]); }
    });
  };

  App.on('ready', function () {
    document.addEventListener('keydown', function (e) {
      if (e.target.tagName === 'INPUT') return;
      var k = e.key.toLowerCase();
      if (k === 'a') App.setMode('ambient');
      if (k === 'n') App.setMode('night');
      if (k === 'escape') App.closeSheet();
      var views = ['home', 'calendar', 'tasks', 'shopping', 'photos'];
      if (k >= '1' && k <= '5') App.go(views[+k - 1]);
    });
  });
})();
