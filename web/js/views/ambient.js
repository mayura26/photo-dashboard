// Ambient (idle photo frame) and Night (dim red clock) overlays.
(function () {
  'use strict';
  var D = window.DATA, $ = App.$;
  var amb = { show: null }, shiftTimer = null;

  function dash(v) {
    return v == null || v === '' ? '—' : v;
  }

  function fmtDur(min) {
    if (min == null) return '—';
    var h = Math.floor(min / 60), m = Math.round(min % 60);
    if (!h) return m + 'm';
    return h + 'h ' + (m < 10 ? '0' : '') + m + 'm';
  }

  function batteryTone(n) {
    if (n == null) return '';
    if (n >= 50) return ' is-ok';
    if (n >= 25) return ' is-mid';
    return ' is-hot';
  }

  function openAmbientTodos() {
    var r = App.currentRoutine();
    var today = App.dayKey(App.now());
    var order = D.people.map(function (p) { return p.id; }).concat(['any']);
    return (D.chores || []).filter(function (c) {
      if (c.done || c.r !== r.id) return false;
      var day = c.day || today;
      return day <= today;
    }).sort(function (a, b) {
      var ia = order.indexOf(a.who), ib = order.indexOf(b.who);
      if (ia < 0) ia = 99;
      if (ib < 0) ib = 99;
      return ia - ib;
    });
  }

  function ambientTask() {
    var todos = openAmbientTodos();
    if (!todos.length) return '';
    var c = todos[0], more = todos.length - 1;
    var r = App.currentRoutine();
    var who = c.who === 'any'
      ? '<span class="amb-task-who">Anyone</span>'
      : App.avatar(c.who, 28);
    return '<div class="amb-task" style="' + App.pstyle(c.who) + '">' +
      '<div class="amb-task-lab">' + App.esc(r.name || 'Todo') + '</div>' +
      '<div class="amb-task-row">' +
      '<span class="amb-task-mark">' + (c.emoji || '✓') + '</span>' +
      '<span class="amb-task-n">' + App.esc(c.title) + '</span>' +
      who +
      (more > 0 ? '<span class="amb-task-more">+' + more + '</span>' : '') +
      '</div></div>';
  }

  function ambientGarmin() {
    var data = D.garmin;
    if (!data) return '';
    var bb = data.bodyBattery || {};
    var sleep = data.sleep || {};
    var act = data.activity || {};
    var steps = act.steps;
    var goal = act.stepsGoal || 10000;
    var stepsLabel = steps != null ? (Math.round(steps / 1000 * 10) / 10) + 'k' : '—';
    var stepsPct = steps != null && goal ? Math.min(100, Math.round(steps / goal * 100)) : 0;
    var sleepHero = sleep.score != null ? dash(sleep.score) : fmtDur(sleep.durationMin);
    return '<div class="amb-g">' +
      '<div class="amb-g-chip' + batteryTone(bb.current) + '" style="--d:0">' +
      App.icon('bolt', 18) +
      '<div class="amb-g-meta"><b class="num">' + dash(bb.current) + '</b><small>Battery</small></div>' +
      '</div>' +
      '<div class="amb-g-chip" style="--d:1">' +
      App.icon('moon', 18) +
      '<div class="amb-g-meta"><b class="num">' + sleepHero + '</b><small>Sleep</small></div>' +
      '</div>' +
      '<div class="amb-g-chip" style="--d:2">' +
      App.icon('steps', 18) +
      '<div class="amb-g-meta"><b class="num">' + stepsLabel + '</b><small>Steps</small>' +
      '<i class="amb-g-bar"><i style="width:' + stepsPct + '%"></i></i></div>' +
      '</div></div>';
  }

  function ambientInfo() {
    var n = App.now(), w = App.weatherNow(), next = App.nextEvent();
    return '<div class="amb-clock"><div class="amb-time num">' + App.fmt.time(n, false) + '</div>' +
      '<div class="amb-date">' + App.fmt.day(n) + ', ' + App.fmt.month(n) + ' ' + n.getDate() + '</div></div>' +
      '<div class="amb-right">' +
      '<div class="amb-wx">' + App.icon(w.cond, 44) + '<span class="num">' + w.temp + '°</span><small>' + w.summary + '</small></div>' +
      (next ? '<div class="amb-next" style="' + App.pstyle(App.evColor(next)) + '"><i></i>' + App.esc(next.title) + ' · ' + App.fmt.time(next.start) + '</div>' : '') +
      ambientTask() +
      ambientGarmin() +
      '<div class="amb-who">' + D.people.map(function (p) { return App.avatar(p, 34, p.home ? '' : 'avatar--away'); }).join('') + '</div>' +
      '</div>';
  }
  function nightInfo() {
    var n = App.now(), tomorrow = App.addDays(n, n.getHours() < 6 ? 0 : 1);
    var first = App.timedOn(tomorrow)[0], w = App.weatherNow();
    return '<div class="nt-time num">' + App.fmt.time(n, false) + '</div>' +
      '<div class="nt-line">' + App.icon(w.cond, 26) + ' ' + w.temp + '° · ' + (first ? 'First up ' + App.fmt.rel(first.start).toLowerCase() + ': <b>' + App.esc(first.title) + '</b> ' + App.fmt.time(first.start) + ' · ' + App.person(first.people[0]).name : 'Nothing early tomorrow') + '</div>' +
      '<div class="nt-lock">' + App.icon('lock', 20) + ' House locked · alarm armed</div>';
  }

  function paint() {
    if (App.mode === 'ambient') $('#ambient .amb-info').innerHTML = ambientInfo();
    if (App.mode === 'night') $('#night .nt-inner').innerHTML = nightInfo();
  }

  // Nudge static content a few px every minute so nothing sits still for hours (image retention)
  function pixelShift() {
    var x = Math.round(Math.random() * 8 - 4), y = Math.round(Math.random() * 8 - 4);
    App.$$('.amb-info, .nt-inner').forEach(function (el) { el.style.transform = 'translate(' + x + 'px,' + y + 'px)'; });
  }

  App.on('ready', function () {
    $('#ambient').innerHTML = '<div class="amb-stage"></div><div class="amb-scrim"></div><div class="amb-info"></div>';
    $('#night').innerHTML = '<div class="nt-inner"></div>';
    amb.show = new App.Slideshow($('#ambient .amb-stage'), { layout: 'cover', captions: false, interval: 14000 });
    $('#ambient').addEventListener('click', function () { App.setMode('day'); App.go('home'); });
    $('#night').addEventListener('click', function () { App.setMode('nightdash'); });
  });

  App.on('mode', function (m) {
    clearInterval(shiftTimer);
    if (m === 'ambient') { amb.show.start(); paint(); shiftTimer = setInterval(pixelShift, 60000); }
    else amb.show.stop();
    if (m === 'night') { paint(); shiftTimer = setInterval(pixelShift, 60000); }
    App.resetIdle();
  });
  App.on('minute', paint);
  App.on('data', function () {
    if (App.mode === 'ambient' || App.mode === 'night') paint();
  });
})();
