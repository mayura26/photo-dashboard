// Home / Today: the glanceable overview. Never scrolls.
(function () {
  'use strict';
  var D = window.DATA, V = {};

  // Placeholder fixtures until a D.sports snapshot lands.
  var SPORTS = [
    { team: 'Sydney FC', league: 'A-League', opp: 'Melbourne City', when: 'Sat', status: 'final', score: [2, 1], home: true },
    { team: 'Swans', league: 'AFL', opp: 'Giants', when: 'Sun 1:20', status: 'upcoming', score: null, home: true },
    { team: 'Roosters', league: 'NRL', opp: 'Rabbitohs', when: 'Fri', status: 'final', score: [18, 24], home: false }
  ];

  function clock() {
    var n = App.now();
    return '<div class="h-time num">' + App.fmt.time(n, false) + '<span class="h-ampm">' + App.fmt.ampm(n) + '</span></div>' +
      '<div class="h-date"><b>' + App.fmt.day(n) + '</b> ' + App.fmt.month(n) + ' ' + n.getDate() + '</div>';
  }

  function wetClass(p) {
    if (p > App.REAL_RAIN_MM) return ' is-wet';
    if (p > 0) return ' is-maybe-wet';
    return '';
  }

  function weather() {
    var w = App.weatherNow();
    var hours = App.hourly(7);
    var tmin = hours.length ? Math.min.apply(null, hours.map(function (h) { return h.t; })) : 0;
    var tmax = hours.length ? Math.max.apply(null, hours.map(function (h) { return h.t; })) : 0;
    return '<div class="h-wx-top h-reveal" style="--d:0">' +
      '<span class="h-wx-icon">' + App.icon(w.cond, 60) + '</span>' +
      '<span class="h-wx-temp num">' + w.temp + '°</span>' +
      '<span class="h-wx-meta"><b>' + w.label + '</b><span>H ' + w.hi + '°  L ' + w.lo + '°</span></span>' +
      '<span class="h-wx-side">' +
      '  <span><b class="num">' + w.humidity + '%</b><small>Humidity</small></span>' +
      '  <span><b class="num">' + w.wind + '</b><small>Wind</small></span>' +
      '  <span><b class="num">' + App.fmtMm(w.rain) + '</b><small>Rain</small></span>' +
      '</span>' +
      '</div>' +
      '<div class="h-wx-say h-reveal" style="--d:1"><b>' + w.summary + '.</b> ' + w.advice + '.</div>' +
      '<div class="h-wx-hours">' + hours.map(function (h, i) {
        var lift = tmax === tmin ? 0 : Math.round((h.t - tmin) / (tmax - tmin) * 8);
        var mm = h.p > App.REAL_RAIN_MM ? '<span class="h-hr-mm">' + App.fmtMm(h.p) + '</span>' : '<span class="h-hr-mm"></span>';
        return '<div class="h-hr h-reveal' + wetClass(h.p) + '" style="--d:' + (2 + i) + '"><span class="h-hr-l">' + h.label + '</span>' +
          '<span class="h-hr-i" style="transform:translateY(' + -lift + 'px)">' + App.icon(h.c, 22) + '</span><b>' + h.t + '°</b>' + mm + '</div>';
      }).join('') + '</div>';
  }

  App.weatherSheet = function () {
    var w = App.weatherNow();
    var hours = App.hourlyRest();
    App.sheet(
      '<div class="wxs">' +
      '<div class="wxs-band">' +
      '  <div class="label">Today</div>' +
      '  <div class="wxs-head">' +
      '    <span class="wxs-icon">' + App.icon(w.cond, 56) + '</span>' +
      '    <span class="wxs-temp num">' + w.temp + '°</span>' +
      '    <span class="wxs-meta"><b>' + w.label + '</b><span>H ' + w.hi + '°  L ' + w.lo + '°</span></span>' +
      '  </div>' +
      '  <p class="wxs-say serif"><b>' + w.summary + '.</b> ' + w.advice + '.</p>' +
      '</div>' +
      '<div class="wxs-body">' +
      '<div class="wxs-stats">' +
      '  <div><b>' + w.feels + '°</b><span>Feels</span></div>' +
      '  <div><b>' + w.humidity + '%</b><span>Humidity</span></div>' +
      '  <div><b>' + w.wind + '</b><span>Wind km/h</span></div>' +
      '  <div><b>' + App.fmtMm(w.rain) + '</b><span>Rain today</span></div>' +
      '  <div><b>' + w.hi + '° / ' + w.lo + '°</b><span>High / Low</span></div>' +
      '</div>' +
      '<div class="wxs-hours">' + hours.map(function (h) {
        var mm = h.p > 0
          ? '<span class="wxs-mm' + (h.p > App.REAL_RAIN_MM ? ' is-real' : '') + '">' + App.fmtMm(h.p) + '</span>'
          : '<span class="wxs-mm wxs-mm--none">—</span>';
        return '<div class="wxs-hr' + wetClass(h.p) + '">' +
          '<span class="wxs-hr-l">' + h.label + '</span>' +
          '<span class="wxs-hr-i">' + App.icon(h.c, 26) + '</span>' +
          '<b class="num">' + h.t + '°</b>' + mm + '</div>';
      }).join('') + '</div>' +
      '</div></div>',
      { cls: 'sheet--weather' }
    );
  };

  function fixtures() {
    return (D.sports && D.sports.fixtures) || SPORTS;
  }

  function sports() {
    var list = fixtures().slice(0, 3);
    return '<div class="label">Sports</div>' +
      '<div class="h-sports-list">' + list.map(function (g, i) {
        var a = App.esc(g.home ? g.team : g.opp);
        var b = App.esc(g.home ? g.opp : g.team);
        var right;
        var tone = '';
        if (g.status === 'final' && g.score) {
          var us = g.home ? g.score[0] : g.score[1];
          var them = g.home ? g.score[1] : g.score[0];
          if (us > them) tone = ' is-win';
          else if (us < them) tone = ' is-loss';
          else tone = ' is-draw';
          right = '<b class="num">' + g.score[0] + '–' + g.score[1] + '</b><small>' + App.esc(g.when) + '</small>';
        } else {
          right = '<small class="h-sp-when">' + App.esc(g.when) + '</small>';
        }
        return '<div class="h-sp h-reveal' + tone + '" style="--d:' + i + '">' +
          '<div class="h-sp-main">' +
          '  <span class="h-sp-league">' + App.esc(g.league) + '</span>' +
          '  <span class="h-sp-match" title="' + a + ' vs ' + b + '">' +
          '    <span class="h-sp-side">' + a + '</span>' +
          '    <span class="h-sp-vs" aria-hidden="true">vs</span>' +
          '    <span class="h-sp-side">' + b + '</span>' +
          '  </span>' +
          '</div>' +
          '<div class="h-sp-score">' + right + '</div>' +
          '</div>';
      }).join('') + '</div>';
  }

  function hero() {
    var e = App.nextEvent(), now = App.now();
    if (!e) {
      var tomorrow = App.timedOn(App.addDays(now, 1))[0];
      return '<div class="h-hero h-hero--empty" data-go="calendar">' +
        '<div class="label label--light">Rest of today</div>' +
        '<div class="h-hero-title serif">Nothing else scheduled.<br><i>Enjoy the quiet.</i></div>' +
        (tomorrow ? '<div class="h-hero-meta">Tomorrow starts with <b>' + App.esc(tomorrow.title) + '</b> at ' + App.fmt.time(tomorrow.start) + '</div>' : '') +
        '</div>';
    }
    var p = App.evColor(e);
    var mins = (e.start - now) / 60000;
    var leave = (App.leaveBy(e) - now) / 60000;
    var who = e.people.map(function (id) { return App.person(id).name; });
    var whoText = App.isFamily(e) ? 'Everyone' : who.join(' + ');
    var big = mins < 100
      ? '<div class="h-hero-count num">' + Math.round(mins) + '<small>min</small></div>'
      : '<div class="h-hero-count num">' + App.fmt.time(e.start, false) + '<small>' + App.fmt.ampm(e.start) + '</small></div>';
    var leavePill = '';
    if (e.travel) {
      leavePill = leave <= 0
        ? '<span class="pill pill--alert"><i class="pulse"></i>Leave now</span>'
        : '<span class="pill' + (leave <= 15 ? ' pill--soon' : '') + '">' + App.icon('car', 20) + 'Leave in ' + App.fmt.mins(leave) + '</span>';
    }
    return '<button class="h-hero" style="' + App.pstyle(p) + '" data-ev="' + e.id + '">' +
      '<div class="h-hero-main">' +
      '  <div class="label label--on">Next up · ' + App.esc(whoText) + '</div>' +
      '  <div class="h-hero-title serif">' + App.esc(e.title) + '</div>' +
      '  <div class="h-hero-meta">' + App.fmt.range(e.start, e.end) + (e.loc ? ' · ' + App.esc(e.loc) : '') + '</div>' +
      '  <div class="h-hero-foot">' + (e.people.length > 1 ? App.dots(e.people, 22) : '') + leavePill + '</div>' +
      '</div>' + big + '</button>';
  }

  function openTodos(who) {
    var r = App.currentRoutine();
    var today = App.dayKey(App.now());
    return (D.chores || []).filter(function (c) {
      if (c.done || c.r !== r.id || c.who !== who) return false;
      var day = c.day || today;
      return day <= today;
    });
  }

  function todoRow(c) {
    return '<button class="h-todo" data-ch="' + c.id + '">' +
      '<span class="h-todo-mark">' + (c.emoji || '✓') + '</span>' +
      '<span class="h-todo-n">' + App.esc(c.title) + '</span></button>';
  }

  function lanes() {
    var now = App.now();
    return D.people.map(function (p) {
      var events = App.timedOn(now, p.id).filter(function (e) { return e.end > now; });
      var todos = openTodos(p.id);
      var shownEv = events.slice(0, 2);
      var shownTd = todos.slice(0, 2);
      var moreEv = events.length - shownEv.length;
      var moreTd = todos.length - shownTd.length;
      var empty = !events.length && !todos.length;
      return '<div class="h-lane" style="' + App.pstyle(p) + '">' +
        '<div class="h-lane-head">' + App.avatar(p, 30) + '<b>' + p.name + '</b></div>' +
        (empty ? '<div class="h-free serif"><i>Free</i> the rest of the day</div>' : '') +
        shownEv.map(function (e) {
          var cur = e.start <= now;
          return '<button class="h-ev' + (cur ? ' is-now' : '') + '" data-ev="' + e.id + '">' +
            '<span class="h-ev-t">' + (cur ? 'Now' : App.fmt.time(e.start, false)) + '</span>' +
            '<span class="h-ev-n">' + App.esc(e.title) + '</span></button>';
        }).join('') +
        shownTd.map(todoRow).join('') +
        (moreEv > 0 ? '<button class="h-more" data-go="calendar">+' + moreEv + ' more</button>' : '') +
        (moreTd > 0 ? '<button class="h-more" data-go="tasks">+' + moreTd + ' todo</button>' : '') +
        '</div>';
    }).join('');
  }

  function anyoneStrip() {
    var any = openTodos('any');
    if (!any.length) return '';
    var shown = any.slice(0, 4);
    var more = any.length - shown.length;
    return '<div class="h-anyone-head"><span class="label">Anyone</span></div>' +
      '<div class="h-anyone-list">' + shown.map(todoRow).join('') +
      (more > 0 ? '<button class="h-more" data-go="tasks">+' + more + ' more</button>' : '') +
      '</div>';
  }

  function completeTodo(c, by) {
    c.done = true;
    c.doneOn = App.dayKey(App.now());
    if (by) c.doneBy = by;
    var earner = D.stars[c.who] != null ? c.who : (by && D.stars[by] != null ? by : null);
    if (earner && c.stars) D.stars[earner] += c.stars;
    if (App.sync) App.sync.chore(c.id, true, by);
    App.emit('data');
  }

  function whoDidTodo(c) {
    App.sheet('<div class="sheet-pad"><div class="label">' + (c.emoji || '✓') + ' ' + App.esc(c.title) + '</div><h2 class="serif sheet-h">Who did it?</h2>' +
      '<div class="add-who add-who--big">' + D.people.map(function (p) {
        return '<button class="who-btn" data-by="' + p.id + '" style="' + App.pstyle(p) + '">' + App.avatar(p, 88) + '<span>' + p.name + '</span></button>';
      }).join('') + '</div></div>', {
      onMount: function (s, close) {
        s.addEventListener('click', function (e) {
          var b = e.target.closest('[data-by]');
          if (!b) return;
          close();
          completeTodo(c, b.dataset.by);
        });
      }
    });
  }

  function paintDay() {
    if (!V.el) return;
    V.el.querySelector('.h-lanes').innerHTML = lanes();
    var strip = V.el.querySelector('.h-anyone');
    strip.innerHTML = anyoneStrip();
    strip.style.cssText = strip.firstChild ? App.pstyle(D.family) : '';
  }

  function ring(done, total, size) {
    var r = size / 2 - 5, c = 2 * Math.PI * r, f = total ? Math.min(1, done / total) : 0;
    return '<svg class="ring" width="' + size + '" height="' + size + '" viewBox="0 0 ' + size + ' ' + size + '">' +
      '<circle cx="' + size / 2 + '" cy="' + size / 2 + '" r="' + r + '" class="ring-bg"/>' +
      '<circle cx="' + size / 2 + '" cy="' + size / 2 + '" r="' + r + '" class="ring-fg" stroke-dasharray="' + (c * f) + ' ' + c + '" transform="rotate(-90 ' + size / 2 + ' ' + size / 2 + ')"/></svg>';
  }
  App.ring = ring;

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

  function stressTone(n) {
    if (n == null) return '';
    if (n < 26) return ' is-ok';
    if (n < 51) return ' is-mid';
    return ' is-hot';
  }

  function health() {
    var data = D.garmin;
    if (!data) {
      return '<div class="label label--light">Body</div>' +
        '<div class="h-health-empty serif"><i>No Garmin data yet</i></div>';
    }
    var bb = data.bodyBattery || {};
    var sleep = data.sleep || {};
    var stress = data.stress || {};
    var act = data.activity || {};
    var steps = act.steps;
    var goal = act.stepsGoal || 10000;
    var stepsLabel = steps != null ? (Math.round(steps / 1000 * 10) / 10) + 'k' : '—';
    var activeLabel = act.activeMin != null ? act.activeMin + 'm' : '—';
    var weekLabel = act.weekActiveMin != null
      ? (act.weekActiveMin + '/' + (act.intensityGoal || 150))
      : '';
    var sleepHero = sleep.score != null ? dash(sleep.score) : fmtDur(sleep.durationMin);
    var sleepSub = sleep.score != null ? fmtDur(sleep.durationMin) : '';
    return '<div class="label label--light">Body · ' + App.esc(data.displayName || 'Garmin') + '</div>' +
      '<div class="h-health-grid">' +
      '<div class="h-hm h-reveal' + batteryTone(bb.current) + '" style="--d:0">' +
      '  <div class="h-hm-head"><span class="h-hm-ico">' + App.icon('bolt', 16) + '</span><span>Battery</span></div>' +
      '  <div class="h-hm-body">' +
      '    <div class="h-hm-dial">' + ring(bb.current || 0, 100, 52) +
      '      <b class="num">' + dash(bb.current) + '</b></div>' +
      '  </div>' +
      '</div>' +
      '<div class="h-hm h-reveal" style="--d:1">' +
      '  <div class="h-hm-head"><span class="h-hm-ico">' + App.icon('moon', 16) + '</span><span>Sleep</span></div>' +
      '  <div class="h-hm-body">' +
      '    <div class="h-hm-meta"><b class="num">' + sleepHero + '</b>' +
      (sleepSub ? '<small>' + sleepSub + '</small>' : '') + '</div>' +
      '  </div>' +
      '</div>' +
      '<div class="h-hm h-reveal' + stressTone(stress.current) + '" style="--d:2">' +
      '  <div class="h-hm-head"><span class="h-hm-ico">' + App.icon('heart', 16) + '</span><span>Stress</span></div>' +
      '  <div class="h-hm-body">' +
      '    <div class="h-hm-meta"><b class="num">' + dash(stress.current) + '</b></div>' +
      '  </div>' +
      '</div>' +
      '<div class="h-hm h-reveal" style="--d:3">' +
      '  <div class="h-hm-head"><span class="h-hm-ico">' + App.icon('steps', 16) + '</span><span>Steps</span></div>' +
      '  <div class="h-hm-body">' +
      '    <div class="h-hm-dial">' + ring(steps || 0, goal, 52) +
      '      <b class="num">' + stepsLabel + '</b></div>' +
      '    <div class="h-hm-meta"><b class="num">' + activeLabel + '</b>' +
      (weekLabel ? '<small>' + weekLabel + '</small>' : '') + '</div>' +
      '  </div>' +
      '</div>' +
      '</div>';
  }

  function paint() {
    var el = V.el;
    el.querySelector('.h-clock').innerHTML = clock();
    el.querySelector('.h-weather').innerHTML = weather();
    el.querySelector('.h-sports').innerHTML = sports();
    el.querySelector('.h-hero-slot').innerHTML = hero();
    paintDay();
    el.querySelector('.h-health').innerHTML = health();
  }

  function photoCaption(s) {
    var cap = V.el && V.el.querySelector('.h-photo-cap');
    if (!cap) return;
    if (!s || !s.photos || !s.photos[0]) {
      cap.innerHTML = '<b>Photos</b><span>Add some in Photos</span>';
      return;
    }
    var m = App.photoMeta(s.photos[0]);
    var kick = m.kicker
      ? ((m.onThisDay && m.kicker !== 'Today' ? 'On this day · ' : '') + m.kicker)
      : '';
    if (!kick && !m.place) {
      cap.innerHTML = '<b>Photos</b><span></span>';
      return;
    }
    cap.innerHTML = '<b>' + App.esc(kick || 'Photos') + '</b><span>' + App.esc(m.place || '') + '</span>';
  }

  App.register('home', {
    show: function (el) {
      V.el = el;
      if (!el.firstChild) {
        el.innerHTML = '<div class="home">' +
          '<div class="h-clock" data-go="calendar"></div>' +
          '<button type="button" class="h-weather" data-wx aria-label="Weather details"></button>' +
          '<div class="h-sports" aria-label="Sports scores"></div>' +
          '<div class="h-main"><div class="h-hero-slot"></div><div class="h-day"><div class="h-lanes"></div><div class="h-anyone"></div></div></div>' +
          '<div class="h-side">' +
          '  <button class="h-photo" data-go="photos"><div class="h-photo-stage"></div><div class="h-photo-cap"></div></button>' +
          '  <button type="button" class="h-health" data-go="body" aria-label="Body health summary"></button>' +
          '</div></div>';
        el.addEventListener('click', function (e) {
          var ch = e.target.closest('[data-ch]');
          if (ch) {
            var c = (D.chores || []).filter(function (x) { return x.id === ch.dataset.ch; })[0];
            if (!c || c.done) return;
            if (c.who === 'any') whoDidTodo(c);
            else completeTodo(c);
            return;
          }
          var ev = e.target.closest('[data-ev]');
          if (ev) { App.eventSheet(App.findEvent(ev.dataset.ev)); return; }
          var wx = e.target.closest('[data-wx]');
          if (wx) { App.weatherSheet(); return; }
          var g = e.target.closest('[data-go]');
          if (g) App.go(g.dataset.go);
        });
        V.slides = new App.Slideshow(el.querySelector('.h-photo-stage'), { layout: 'cover', captions: false, interval: 12000, onChange: photoCaption });
      }
      paint();
      V.slides.start();
    },
    resume: function () { V.slides.start(); },
    hide: function () { if (V.slides) V.slides.stop(); }
  });
  // Every minute: just the clock and next-up countdown (cheap, even while hidden). Weather on the hour.
  App.on('minute', function (n) {
    if (!V.el || !V.el.firstChild) return;
    V.el.querySelector('.h-clock').innerHTML = clock();
    V.el.querySelector('.h-hero-slot').innerHTML = hero();
    if (n.getMinutes() === 0) {
      V.el.querySelector('.h-weather').innerHTML = weather();
      paintDay();
    }
  });
  App.on('data', function () { if (App.current === 'home') paint(); });
})();
