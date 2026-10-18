// Ambient (idle photo frame) and Night (dim red clock) overlays.
(function () {
  'use strict';
  var D = window.DATA, $ = App.$;
  var amb = { show: null, infoKey: null, sportsKey: null, photoKey: null, nightKey: null };
  var shiftTimer = null;

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

  function nextEventMeta(next, now) {
    var mins = Math.round((next.start - now) / 60000);
    return mins > 0 && mins < 24 * 60
      ? (mins < 60 ? 'in ' + mins + 'm' : App.fmt.time(next.start))
      : App.fmt.time(next.start);
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
      return schedulePill({
        lab: 'Up next · ' + eventSourceLabel(next),
        title: next.title,
        meta: nextEventMeta(next, now),
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

  function ambientInfo() {
    var n = App.now(), w = App.weatherNow();
    return '<div class="amb-clock"><div class="amb-time num">' + App.fmt.time(n, false) + '</div>' +
      '<div class="amb-date">' + App.fmt.day(n) + ', ' + App.fmt.month(n) + ' ' + n.getDate() + '</div></div>' +
      '<div class="amb-right">' +
      '<div class="amb-wx">' + App.icon(w.cond, 44) + '<span class="num">' + w.temp + '°</span>' +
      '<div class="amb-wx-meta"><small>' + w.summary + '</small>' +
      '<span class="amb-wx-day">H ' + w.hi + '° · L ' + w.lo + '° · ' + App.fmtMm(w.rain) + '</span></div></div>' +
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

  function markEnter(root) {
    if (!root) return;
    root.querySelectorAll('.amb-task, .amb-g-chip, .amb-sp, .amb-ph').forEach(function (n) {
      n.classList.add('amb-enter');
    });
  }

  function scheduleKey() {
    var cur = currentEvent();
    if (cur) {
      return 'now|' + (cur.id || '') + '|' + cur.title + '|' + +cur.start + '|' + +cur.end + '|' +
        ((cur.people && cur.people[0]) || '') + '|' + (cur.source || '');
    }
    var next = App.nextEvent();
    if (next) {
      return 'next|' + (next.id || '') + '|' + next.title + '|' + +next.start + '|' +
        ((next.people && next.people[0]) || '') + '|' + (next.source || '');
    }
    var todos = openAmbientTodos();
    if (!todos.length) return 'todo|';
    var c = todos[0];
    return 'todo|' + (c.id || c.title) + '|' + c.title + '|' + c.who + '|' + (c.emoji || '') + '|' + todos.length;
  }

  function garminKey() {
    var data = D.garmin;
    if (!data) return '';
    var bb = data.bodyBattery || {};
    var sleep = data.sleep || {};
    var act = data.activity || {};
    return [bb.current, sleep.score, sleep.durationMin, act.steps, act.stepsGoal || 10000].join('|');
  }

  function weatherKey() {
    var w = App.weatherNow();
    return [w.cond, w.temp, w.summary, w.hi, w.lo, w.rain].join('|');
  }

  function infoFingerprint() {
    return weatherKey() + '//' + scheduleKey() + '//' + garminKey();
  }

  function sportsFingerprint() {
    var g = upcomingSport();
    if (!g) return '';
    return [g.league, g.team, g.opp, g.when, g.home ? 1 : 0].join('|');
  }

  function nightFingerprint() {
    var n = App.now(), tomorrow = App.addDays(n, n.getHours() < 6 ? 0 : 1);
    var first = App.timedOn(tomorrow)[0], w = App.weatherNow();
    return [
      w.cond, w.temp,
      first ? (first.id || first.title) : '',
      first ? +first.start : '',
      first && first.people ? first.people[0] : ''
    ].join('|');
  }

  function paintClock() {
    var n = App.now();
    var time = App.fmt.time(n, false);
    var date = App.fmt.day(n) + ', ' + App.fmt.month(n) + ' ' + n.getDate();
    var t = $('#ambient .amb-time');
    var d = $('#ambient .amb-date');
    if (t) t.textContent = time;
    if (d) d.textContent = date;
    var nt = $('#night .nt-time');
    if (nt) nt.textContent = time;
  }

  // Countdown / range range can change without a card rebuild.
  function syncScheduleMeta() {
    var meta = $('#ambient .amb-task-meta');
    if (!meta) return;
    var now = App.now();
    var cur = currentEvent();
    if (cur) {
      meta.textContent = App.fmt.time(cur.start, false) + '–' + App.fmt.time(cur.end, false);
      return;
    }
    var next = App.nextEvent();
    if (next) meta.textContent = nextEventMeta(next, now);
  }

  function setInfo(html) {
    var el = $('#ambient .amb-info');
    if (!el) return;
    el.innerHTML = html;
    markEnter(el);
  }

  function setSports(html) {
    var el = $('#ambient .amb-sports');
    if (!el) return;
    el.innerHTML = html;
    markEnter(el);
  }

  function paintAmbient(force) {
    paintClock();
    var ik = infoFingerprint();
    if (force || ik !== amb.infoKey) {
      amb.infoKey = ik;
      setInfo(ambientInfo());
    } else {
      syncScheduleMeta();
    }
    var sk = sportsFingerprint();
    if (force || sk !== amb.sportsKey) {
      amb.sportsKey = sk;
      setSports(ambientSports());
    }
  }

  function paintNight(force) {
    paintClock();
    var nk = nightFingerprint();
    if (force || nk !== amb.nightKey) {
      amb.nightKey = nk;
      var el = $('#night .nt-inner');
      if (el) el.innerHTML = nightInfo();
    }
  }

  function paint(force) {
    if (App.mode === 'ambient') paintAmbient(force);
    if (App.mode === 'night') paintNight(force);
  }

  function ambientPhoto(slide) {
    var el = $('#ambient .amb-photo');
    if (!el) return;
    var p = slide && slide.photos && slide.photos[0];
    if (!p) {
      amb.photoKey = '';
      el.innerHTML = '';
      return;
    }
    var m = App.photoMeta(p);
    var kicker = m.kicker
      ? ((m.onThisDay && m.kicker !== 'Today' ? 'On this day · ' : '') + m.kicker)
      : '';
    var key = kicker + '|' + (m.place || '');
    if (!kicker && !m.place) {
      if (amb.photoKey === '') return;
      amb.photoKey = '';
      el.innerHTML = '';
      return;
    }
    if (key === amb.photoKey) return;
    amb.photoKey = key;
    el.innerHTML = '<div class="amb-ph">' +
      (kicker ? '<span class="amb-ph-ago">' + App.esc(kicker) + '</span>' : '') +
      (m.place ? '<span class="amb-ph-place">' + App.esc(m.place) + '</span>' : '') +
      '</div>';
    markEnter(el);
  }

  // Nudge static content a few px every minute so nothing sits still for hours (image retention)
  function pixelShift() {
    var x = Math.round(Math.random() * 8 - 4), y = Math.round(Math.random() * 8 - 4);
    App.$$('.amb-info, .amb-sports, .amb-photo, .nt-inner').forEach(function (el) { el.style.transform = 'translate(' + x + 'px,' + y + 'px)'; });
  }

  function resetKeys() {
    amb.infoKey = amb.sportsKey = amb.photoKey = amb.nightKey = null;
  }

  App.on('ready', function () {
    $('#ambient').innerHTML = '<div class="amb-stage"></div><div class="amb-scrim"></div><div class="amb-sports"></div><div class="amb-photo"></div><div class="amb-info"></div>';
    $('#night').innerHTML = '<div class="nt-inner"></div>';
    amb.show = new App.Slideshow($('#ambient .amb-stage'), {
      layout: 'ambient',
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
      resetKeys();
      amb.show.start();
      paint(true);
      ambientPhoto(amb.show.current);
      shiftTimer = setInterval(pixelShift, 60000);
    }
    else if (amb.show) amb.show.stop();
    if (m === 'night') {
      resetKeys();
      paint(true);
      shiftTimer = setInterval(pixelShift, 60000);
    }
    App.resetIdle();
  });
  App.on('minute', function () { paint(false); });
  App.on('data', function () {
    if (App.mode === 'ambient' || App.mode === 'night') paint(false);
  });
})();
