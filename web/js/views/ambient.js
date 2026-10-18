// Ambient (idle photo frame) and Night (dim red clock) overlays.
(function () {
  'use strict';
  var D = window.DATA, $ = App.$;
  var amb = { show: null }, shiftTimer = null;

  // Placeholder fixtures until a D.sports snapshot lands (same as home).
  var SPORTS = [
    { team: 'Sydney FC', league: 'A-League', opp: 'Melbourne City', when: 'Sat', status: 'final', score: [2, 1], home: true },
    { team: 'Swans', league: 'AFL', opp: 'Giants', when: 'Sun 1:20', status: 'upcoming', score: null, home: true },
    { team: 'Roosters', league: 'NRL', opp: 'Rabbitohs', when: 'Fri', status: 'final', score: [18, 24], home: false }
  ];

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

  function eventSourceLabel(e) {
    if (!e) return 'Calendar';
    if (e.source === 'pmday') return 'Plan My Day';
    if (e.source === 'family') return 'Family';
    return 'Calendar';
  }

  function currentEvent() {
    var now = App.now();
    return App.timedOn(now).filter(function (e) { return e.start <= now && e.end > now; })[0] || null;
  }

  function openAmbientTodos() {
    var r = App.currentRoutine();
    var today = App.dayKey(App.now());
    var order = D.people.map(function (p) { return p.id; }).concat(['any']);
    return (D.chores || []).filter(function (c) {
      if (c.done) return false;
      var day = c.day || today;
      return day <= today;
    }).sort(function (a, b) {
      // Prefer current-routine todos, then household order.
      var ra = a.r === r.id ? 0 : 1, rb = b.r === r.id ? 0 : 1;
      if (ra !== rb) return ra - rb;
      var ia = order.indexOf(a.who), ib = order.indexOf(b.who);
      if (ia < 0) ia = 99;
      if (ib < 0) ib = 99;
      return ia - ib;
    });
  }

  function schedulePill(opts) {
    return '<div class="amb-task" style="' + (opts.style || '') + '">' +
      '<div class="amb-task-lab">' + App.esc(opts.lab) + '</div>' +
      '<div class="amb-task-row">' +
      (opts.mark ? '<span class="amb-task-mark">' + opts.mark + '</span>' : '<i class="amb-task-dot"></i>') +
      '<span class="amb-task-n">' + App.esc(opts.title) + '</span>' +
      (opts.meta ? '<span class="amb-task-meta">' + App.esc(opts.meta) + '</span>' : '') +
      (opts.who || '') +
      (opts.more ? '<span class="amb-task-more">+' + opts.more + '</span>' : '') +
      '</div></div>';
  }

  // Prefer in-progress calendar/PMD, then next upcoming, then open todos.
  function ambientSchedule() {
    var now = App.now();
    var cur = currentEvent();
    if (cur) {
      return schedulePill({
        lab: 'Now · ' + eventSourceLabel(cur),
        title: cur.title,
        meta: App.fmt.time(cur.start, false) + '–' + App.fmt.time(cur.end, false),
        style: App.pstyle(App.evColor(cur)),
        who: cur.people && cur.people[0] ? App.avatar(cur.people[0], 28) : ''
      });
    }
    var next = App.nextEvent();
    if (next) {
      var mins = Math.round((next.start - now) / 60000);
      var when = mins > 0 && mins < 24 * 60
        ? (mins < 60 ? 'in ' + mins + 'm' : App.fmt.time(next.start))
        : App.fmt.time(next.start);
      return schedulePill({
        lab: 'Up next · ' + eventSourceLabel(next),
        title: next.title,
        meta: when,
        style: App.pstyle(App.evColor(next)),
        who: next.people && next.people[0] ? App.avatar(next.people[0], 28) : ''
      });
    }
    var todos = openAmbientTodos();
    if (!todos.length) return '';
    var c = todos[0], more = todos.length - 1;
    var who = c.who === 'any'
      ? '<span class="amb-task-who">Anyone</span>'
      : App.avatar(c.who, 28);
    return schedulePill({
      lab: 'Todo',
      title: c.title,
      mark: c.emoji || '✓',
      style: App.pstyle(c.who),
      who: who,
      more: more > 0 ? more : 0
    });
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

  function upcomingSport() {
    var list = (D.sports && D.sports.fixtures) || SPORTS;
    for (var i = 0; i < list.length; i++) {
      if (list[i].status === 'upcoming') return list[i];
    }
    return null;
  }

  function ambientSports() {
    var g = upcomingSport();
    if (!g) return '';
    var a = g.home ? g.team : g.opp;
    var b = g.home ? g.opp : g.team;
    return '<div class="amb-sp">' +
      '<span class="amb-sp-league">' + App.esc(g.league) + '</span>' +
      '<span class="amb-sp-match"><b>' + App.esc(a) + '</b><i>vs</i><b>' + App.esc(b) + '</b></span>' +
      '<span class="amb-sp-when">' + App.esc(g.when) + '</span>' +
      '</div>';
  }

  function ambientPhoto(slide) {
    var el = $('#ambient .amb-photo');
    if (!el) return;
    var p = slide && slide.photos && slide.photos[0];
    if (!p) {
      el.innerHTML = '';
      return;
    }
    var m = App.photoMeta(p);
    var kicker = m.kicker
      ? (m.onThisDay ? 'On this day · ' : '') + m.kicker
      : '';
    if (!kicker && !m.place) {
      el.innerHTML = '';
      return;
    }
    el.innerHTML = '<div class="amb-ph">' +
      (kicker ? '<span class="amb-ph-ago">' + App.esc(kicker) + '</span>' : '') +
      (m.place ? '<span class="amb-ph-place">' + App.esc(m.place) + '</span>' : '') +
      '</div>';
  }

  function ambientInfo() {
    var n = App.now(), w = App.weatherNow();
    return '<div class="amb-clock"><div class="amb-time num">' + App.fmt.time(n, false) + '</div>' +
      '<div class="amb-date">' + App.fmt.day(n) + ', ' + App.fmt.month(n) + ' ' + n.getDate() + '</div></div>' +
      '<div class="amb-right">' +
      '<div class="amb-wx">' + App.icon(w.cond, 44) + '<span class="num">' + w.temp + '°</span><small>' + w.summary + '</small></div>' +
      ambientSchedule() +
      ambientGarmin() +
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
    if (App.mode === 'ambient') {
      $('#ambient .amb-info').innerHTML = ambientInfo();
      $('#ambient .amb-sports').innerHTML = ambientSports();
    }
    if (App.mode === 'night') $('#night .nt-inner').innerHTML = nightInfo();
  }

  // Nudge static content a few px every minute so nothing sits still for hours (image retention)
  function pixelShift() {
    var x = Math.round(Math.random() * 8 - 4), y = Math.round(Math.random() * 8 - 4);
    App.$$('.amb-info, .amb-sports, .amb-photo, .nt-inner').forEach(function (el) { el.style.transform = 'translate(' + x + 'px,' + y + 'px)'; });
  }

  App.on('ready', function () {
    $('#ambient').innerHTML = '<div class="amb-stage"></div><div class="amb-scrim"></div><div class="amb-sports"></div><div class="amb-photo"></div><div class="amb-info"></div>';
    $('#night').innerHTML = '<div class="nt-inner"></div>';
    amb.show = new App.Slideshow($('#ambient .amb-stage'), {
      layout: 'cover',
      captions: false,
      interval: 14000,
      onChange: ambientPhoto
    });
    $('#ambient').addEventListener('click', function () { App.setMode('day'); App.go('home'); });
    $('#night').addEventListener('click', function () { App.setMode('nightdash'); });
  });

  App.on('mode', function (m) {
    clearInterval(shiftTimer);
    if (m === 'ambient') {
      amb.show.start();
      paint();
      ambientPhoto(amb.show.current);
      shiftTimer = setInterval(pixelShift, 60000);
    }
    else amb.show.stop();
    if (m === 'night') { paint(); shiftTimer = setInterval(pixelShift, 60000); }
    App.resetIdle();
  });
  App.on('minute', paint);
  App.on('data', function () {
    if (App.mode === 'ambient' || App.mode === 'night') paint();
  });
})();
