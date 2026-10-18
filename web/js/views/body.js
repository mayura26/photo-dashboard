// Body: glanceable Garmin metrics — sleep, body battery, stress, activity.
(function () {
  'use strict';
  var D = window.DATA;
  var V = { el: null, bound: false };

  function g() {
    return D.garmin || null;
  }

  function hist() {
    var data = g();
    return (data && data.history) || {};
  }

  function dash(v) {
    return v == null || v === '' ? '—' : v;
  }

  function fmtDur(min) {
    if (min == null) return '—';
    var h = Math.floor(min / 60), m = Math.round(min % 60);
    if (!h) return m + 'm';
    return h + 'h ' + (m < 10 ? '0' : '') + m + 'm';
  }

  function fmtUpdated(iso) {
    if (!iso) return 'Seed data';
    var d = new Date(iso);
    if (isNaN(d.getTime())) return 'Updated recently';
    var now = App.now();
    var mins = Math.round((now - d) / 60000);
    if (mins < 1) return 'Just now';
    if (mins < 60) return mins + ' min ago';
    if (App.sameDay(d, now)) return 'Today · ' + App.fmt.time(d);
    return App.fmt.rel(d) + ' · ' + App.fmt.time(d);
  }

  function stressTone(n) {
    if (n == null) return '';
    if (n < 26) return 'is-ok';
    if (n < 51) return 'is-mid';
    return 'is-hot';
  }

  function batteryTone(n) {
    if (n == null) return '';
    if (n >= 50) return 'is-ok';
    if (n >= 25) return 'is-mid';
    return 'is-hot';
  }

  function sparkline(series) {
    var pts = (series || []).filter(function (p) { return p && p.v != null; });
    if (pts.length < 2) {
      return '<div class="b-spark b-spark--empty serif"><i>No day curve yet</i></div>';
    }
    var w = 420, h = 96, pad = 4;
    var min = Math.min.apply(null, pts.map(function (p) { return p.v; }));
    var max = Math.max.apply(null, pts.map(function (p) { return p.v; }));
    if (max === min) { max = min + 1; }
    var path = pts.map(function (p, i) {
      var x = pad + (i / (pts.length - 1)) * (w - pad * 2);
      var y = pad + (1 - (p.v - min) / (max - min)) * (h - pad * 2);
      return (i ? 'L' : 'M') + x.toFixed(1) + ' ' + y.toFixed(1);
    }).join(' ');
    var last = pts[pts.length - 1];
    var lx = pad + (w - pad * 2);
    var ly = pad + (1 - (last.v - min) / (max - min)) * (h - pad * 2);
    return '<svg class="b-spark" viewBox="0 0 ' + w + ' ' + h + '" preserveAspectRatio="none" aria-hidden="true">' +
      '<path class="b-spark-line" d="' + path + '"/>' +
      '<circle class="b-spark-dot" cx="' + lx.toFixed(1) + '" cy="' + ly.toFixed(1) + '" r="5"/></svg>';
  }

  function stageBar(sleep) {
    var parts = [
      { k: 'deep', n: sleep.deepMin, label: 'Deep' },
      { k: 'light', n: sleep.lightMin, label: 'Light' },
      { k: 'rem', n: sleep.remMin, label: 'REM' },
      { k: 'awake', n: sleep.awakeMin, label: 'Awake' }
    ];
    var total = parts.reduce(function (s, p) { return s + (p.n || 0); }, 0);
    if (!total) {
      return '<div class="b-stages b-stages--empty"></div>';
    }
    return '<div class="b-stages" aria-hidden="true">' + parts.map(function (p) {
      return '<i class="b-stage b-stage--' + p.k + '" style="flex:' + (p.n || 0) + '" title="' + p.label + '"></i>';
    }).join('') + '</div>' +
      '<div class="b-stage-legend">' + parts.map(function (p) {
        return '<span class="b-leg b-leg--' + p.k + '"><i></i>' + p.label + ' <b>' + dash(p.n != null ? p.n + 'm' : null) + '</b></span>';
      }).join('') + '</div>';
  }

  function ring(done, total, size) {
    if (typeof App.ring === 'function') return App.ring(done, total, size);
    var r = size / 2 - 5, c = 2 * Math.PI * r, f = total ? Math.min(1, done / total) : 0;
    return '<svg class="ring" width="' + size + '" height="' + size + '" viewBox="0 0 ' + size + ' ' + size + '">' +
      '<circle cx="' + size / 2 + '" cy="' + size / 2 + '" r="' + r + '" class="ring-bg"/>' +
      '<circle cx="' + size / 2 + '" cy="' + size / 2 + '" r="' + r + '" class="ring-fg" stroke-dasharray="' + (c * f) + ' ' + c + '" transform="rotate(-90 ' + size / 2 + ' ' + size / 2 + ')"/></svg>';
  }

  // ---- charts (SVG) --------------------------------------------------------
  function lineChart(series, opts) {
    opts = opts || {};
    var pts = (series || []).filter(function (p) { return p && p.v != null; });
    var w = opts.w || 560, h = opts.h || 220, padX = 8, padY = 16;
    if (pts.length < 2) {
      return '<div class="bxs-empty serif"><i>No day data yet</i></div>';
    }
    var min = opts.min != null ? opts.min : Math.min.apply(null, pts.map(function (p) { return p.v; }));
    var max = opts.max != null ? opts.max : Math.max.apply(null, pts.map(function (p) { return p.v; }));
    if (max === min) { max = min + 1; }
    var path = pts.map(function (p, i) {
      var x = padX + (i / (pts.length - 1)) * (w - padX * 2);
      var y = padY + (1 - (p.v - min) / (max - min)) * (h - padY * 2);
      return (i ? 'L' : 'M') + x.toFixed(1) + ' ' + y.toFixed(1);
    }).join(' ');
    var area = path + ' L' + (w - padX).toFixed(1) + ' ' + (h - padY) + ' L' + padX + ' ' + (h - padY) + ' Z';
    var last = pts[pts.length - 1];
    var lx = padX + (w - padX * 2);
    var ly = padY + (1 - (last.v - min) / (max - min)) * (h - padY * 2);
    return '<svg class="bxs-line" viewBox="0 0 ' + w + ' ' + h + '" preserveAspectRatio="none" aria-hidden="true">' +
      '<path class="bxs-area" d="' + area + '"/>' +
      '<path class="bxs-path" d="' + path + '"/>' +
      '<circle class="bxs-dot" cx="' + lx.toFixed(1) + '" cy="' + ly.toFixed(1) + '" r="6"/>' +
      '</svg>' +
      '<div class="bxs-axis"><span>' + dash(min) + '</span><span>' + dash(max) + '</span></div>';
  }

  function barChart(rows, opts) {
    opts = opts || {};
    var key = opts.key || 'v';
    var list = (rows || []).map(function (r) {
      return { d: r.d || '', v: r[key] != null ? r[key] : r.v };
    });
    var vals = list.filter(function (r) { return r.v != null; }).map(function (r) { return r.v; });
    if (!vals.length) {
      return '<div class="bxs-empty serif"><i>No week data yet</i></div>';
    }
    var max = opts.max != null ? opts.max : Math.max.apply(null, vals);
    if (!max) max = 1;
    return '<div class="bxs-bars">' + list.map(function (r) {
      var pct = r.v == null ? 0 : Math.max(4, Math.round((r.v / max) * 100));
      var cls = 'bxs-bar' + (r.v == null ? ' is-empty' : '');
      return '<div class="' + cls + '">' +
        '<div class="bxs-bar-col"><i style="height:' + pct + '%"></i></div>' +
        '<b class="num">' + dash(r.v) + '</b>' +
        '<span>' + App.esc(r.d) + '</span></div>';
    }).join('') + '</div>';
  }

  function rangeTabs(metric, range) {
    return '<div class="bxs-tabs" role="tablist">' +
      '<button type="button" class="bxs-tab' + (range === 'day' ? ' is-on' : '') + '" data-bxs-range="day" data-bxs-metric="' + metric + '">Day</button>' +
      '<button type="button" class="bxs-tab' + (range === 'week' ? ' is-on' : '') + '" data-bxs-range="week" data-bxs-metric="' + metric + '">Week</button>' +
      '</div>';
  }

  function sheetShell(title, heroHtml, statsHtml, chartHtml, metric, range) {
    return '<div class="bxs" data-bxs-metric="' + metric + '" data-bxs-range="' + range + '">' +
      '<div class="bxs-band">' +
      '  <div class="label">' + App.esc(title) + '</div>' +
      heroHtml +
      statsHtml +
      rangeTabs(metric, range) +
      '</div>' +
      '<div class="bxs-body" data-bxs-chart>' + chartHtml + '</div>' +
      '</div>';
  }

  function daySeries(metric) {
    var data = g() || {};
    var h = hist();
    if (metric === 'battery') {
      return (h.bodyBattery && h.bodyBattery.day && h.bodyBattery.day.length)
        ? h.bodyBattery.day
        : ((data.bodyBattery && data.bodyBattery.series) || []);
    }
    if (metric === 'stress') {
      return (h.stress && h.stress.day && h.stress.day.length)
        ? h.stress.day
        : ((data.stress && data.stress.series) || []);
    }
    if (metric === 'activity') {
      return (h.activity && h.activity.day) || [];
    }
    return [];
  }

  function weekRows(metric) {
    var h = hist();
    if (metric === 'battery') return (h.bodyBattery && h.bodyBattery.week) || [];
    if (metric === 'sleep') return (h.sleep && h.sleep.week) || [];
    if (metric === 'stress') return (h.stress && h.stress.week) || [];
    if (metric === 'activity') return (h.activity && h.activity.week) || [];
    return [];
  }

  function chartFor(metric, range) {
    if (metric === 'battery') {
      if (range === 'day') return lineChart(daySeries('battery'), { min: 0, max: 100 });
      return barChart(weekRows('battery'), { max: 100 });
    }
    if (metric === 'sleep') {
      if (range === 'day') {
        var sleep = (g() && g().sleep) || {};
        return '<div class="bxs-sleep-day">' +
          '<div class="bxs-sleep-dur serif">' + fmtDur(sleep.durationMin) + '</div>' +
          stageBar(sleep) +
          '</div>';
      }
      // Prefer score bars; many accounts have null scores — fall back to duration (hours).
      var sleepWeek = weekRows('sleep');
      var hasScore = sleepWeek.some(function (r) { return r && r.score != null; });
      if (hasScore) return barChart(sleepWeek, { key: 'score', max: 100 });
      var asHours = sleepWeek.map(function (r) {
        var mins = r && (r.durationMin != null ? r.durationMin : r.v);
        // Accept minutes or already-hours (< 24) so older payloads still chart.
        var hours = null;
        if (mins != null) {
          hours = mins > 24 ? Math.round(mins / 60 * 10) / 10 : +mins;
        }
        return { d: r.d, v: hours };
      });
      var chart = barChart(asHours, {});
      if (chart.indexOf('bxs-empty') !== -1) {
        return '<div class="bxs-empty serif"><i>No week data yet</i><br><span style="font-size:16px;color:var(--mute)">Restart the Garmin sidecar to refresh history</span></div>';
      }
      return '<div class="bxs-chart-cap label">Hours asleep</div>' + chart;
    }
    if (metric === 'stress') {
      if (range === 'day') return lineChart(daySeries('stress'), { min: 0, max: 100 });
      return barChart(weekRows('stress'), { max: 100 });
    }
    if (metric === 'activity') {
      if (range === 'day') return lineChart(daySeries('activity'), { min: 0 });
      return barChart(weekRows('activity'), { key: 'steps' });
    }
    return '<div class="bxs-empty serif"><i>Nothing to show</i></div>';
  }

  function openMetricSheet(metric, range) {
    range = range || 'day';
    var data = g();
    if (!data) return;

    var title = { battery: 'Body battery', sleep: 'Sleep', stress: 'Stress', activity: 'Activity' }[metric] || 'Body';
    var hero = '';
    var stats = '';

    if (metric === 'battery') {
      var bb = data.bodyBattery || {};
      hero = '<div class="bxs-head"><b class="num bxs-hero">' + dash(bb.current) + '</b><span class="bxs-unit">/ 100</span></div>';
      stats = '<div class="bxs-stats">' +
        '<div><b class="num">+' + dash(bb.charged) + '</b><span>Charged</span></div>' +
        '<div><b class="num">−' + dash(bb.drained) + '</b><span>Drained</span></div>' +
        '</div>';
    } else if (metric === 'sleep') {
      var sleep = data.sleep || {};
      hero = '<div class="bxs-head"><b class="num bxs-hero">' + dash(sleep.score) + '</b><span class="bxs-unit">score</span></div>';
      stats = '<div class="bxs-stats">' +
        '<div><b class="num">' + fmtDur(sleep.durationMin) + '</b><span>Duration</span></div>' +
        '<div><b class="num">' + dash(sleep.deepMin) + 'm</b><span>Deep</span></div>' +
        '<div><b class="num">' + dash(sleep.remMin) + 'm</b><span>REM</span></div>' +
        '</div>';
    } else if (metric === 'stress') {
      var stress = data.stress || {};
      hero = '<div class="bxs-head"><b class="num bxs-hero">' + dash(stress.current) + '</b><span class="bxs-unit">now</span></div>';
      stats = '<div class="bxs-stats">' +
        '<div><b class="num">' + dash(stress.avg) + '</b><span>Avg</span></div>' +
        '<div><b class="num">' + dash(stress.restMin) + 'm</b><span>Rest</span></div>' +
        '<div><b class="num">' + dash(stress.highMin) + 'm</b><span>High</span></div>' +
        '</div>';
    } else if (metric === 'activity') {
      var act = data.activity || {};
      hero = '<div class="bxs-head"><b class="num bxs-hero">' + (act.steps != null ? act.steps.toLocaleString() : '—') + '</b><span class="bxs-unit">steps</span></div>';
      stats = '<div class="bxs-stats">' +
        '<div><b class="num">' + (act.stepsGoal != null ? act.stepsGoal.toLocaleString() : '—') + '</b><span>Goal</span></div>' +
        '<div><b class="num">' + dash(act.activeMin) + 'm</b><span>Active</span></div>' +
        '<div><b class="num">' + dash(act.calories) + '</b><span>Calories</span></div>' +
        '</div>';
    }

    // Sleep has no useful continuous day series — default week when opening sleep? Keep day for stages.
    App.sheet(
      sheetShell(title, hero, stats, chartFor(metric, range), metric, range),
      {
        cls: 'sheet--body',
        onMount: function (sheet) {
          sheet.addEventListener('click', function (e) {
            var tab = e.target.closest('[data-bxs-range]');
            if (!tab) return;
            var m = tab.dataset.bxsMetric;
            var r = tab.dataset.bxsRange;
            var root = sheet.querySelector('.bxs');
            if (!root) return;
            root.dataset.bxsRange = r;
            App.$$('.bxs-tab', sheet).forEach(function (t) {
              t.classList.toggle('is-on', t.dataset.bxsRange === r);
            });
            var chart = sheet.querySelector('[data-bxs-chart]');
            if (chart) chart.innerHTML = chartFor(m, r);
          });
        }
      }
    );
  }

  function empty() {
    return '<div class="body body--empty">' +
      '<div class="b-head"><div class="label">Body</div></div>' +
      '<div class="b-empty serif">No Garmin data yet.<br><i>Connect the sidecar to fill this board.</i></div>' +
      '</div>';
  }

  function html() {
    var data = g();
    if (!data) return empty();

    var bb = data.bodyBattery || {};
    var sleep = data.sleep || {};
    var stress = data.stress || {};
    var act = data.activity || {};
    var steps = act.steps;
    var goal = act.stepsGoal || 10000;
    var goalHit = steps != null && steps >= goal;

    return '<div class="body">' +
      '<header class="b-head b-reveal" style="--d:0">' +
      '  <div class="label">Body</div>' +
      '  <h1 class="serif">' + App.esc(data.displayName || 'Garmin') + '</h1>' +
      '  <div class="b-updated">' + App.esc(fmtUpdated(data.updatedAt)) + '</div>' +
      '</header>' +

      '<button type="button" class="b-battery b-card b-reveal ' + batteryTone(bb.current) + '" style="--d:1" data-metric="battery" aria-label="Body battery details">' +
      '  <div class="label">Body battery</div>' +
      '  <div class="b-bat-main">' +
      '    <div class="b-dial">' + ring(bb.current || 0, 100, 220) +
      '      <div class="b-dial-num"><b class="num">' + dash(bb.current) + '</b><small>/ 100</small></div>' +
      '    </div>' +
      '    <div class="b-bat-meta">' +
      '      <div class="b-stat"><span class="label">Charged</span><b class="num">+' + dash(bb.charged) + '</b></div>' +
      '      <div class="b-stat"><span class="label">Drained</span><b class="num">−' + dash(bb.drained) + '</b></div>' +
      '      <div class="b-spark-wrap">' + sparkline(bb.series) + '<span class="b-spark-cap">Today · tap for more</span></div>' +
      '    </div>' +
      '  </div>' +
      '</button>' +

      '<button type="button" class="b-panel b-sleep b-card b-reveal" style="--d:2" data-metric="sleep" aria-label="Sleep details">' +
      '  <div class="label">Sleep</div>' +
      '  <div class="b-panel-top">' +
      '    <div class="b-big"><b class="num">' + dash(sleep.score) + '</b><small>score</small></div>' +
      '    <div class="b-sub serif">' + fmtDur(sleep.durationMin) + '</div>' +
      '  </div>' +
      stageBar(sleep) +
      '</button>' +

      '<button type="button" class="b-panel b-stress b-card b-reveal ' + stressTone(stress.current) + '" style="--d:3" data-metric="stress" aria-label="Stress details">' +
      '  <div class="label">Stress</div>' +
      '  <div class="b-panel-top">' +
      '    <div class="b-big"><b class="num">' + dash(stress.current) + '</b><small>now</small></div>' +
      '    <div class="b-sub">Avg <b class="num">' + dash(stress.avg) + '</b></div>' +
      '  </div>' +
      '  <div class="b-stress-row">' +
      '    <div class="b-stat"><span class="label">Rest</span><b class="num">' + dash(stress.restMin != null ? stress.restMin + 'm' : null) + '</b></div>' +
      '    <div class="b-stat"><span class="label">High</span><b class="num">' + dash(stress.highMin != null ? stress.highMin + 'm' : null) + '</b></div>' +
      '  </div>' +
      '</button>' +

      '<button type="button" class="b-panel b-act b-card b-reveal' + (goalHit ? ' is-goal' : '') + '" style="--d:4" data-metric="activity" aria-label="Activity details">' +
      '  <div class="label">Activity</div>' +
      '  <div class="b-act-main">' +
      '    <div class="b-steps-dial">' + ring(steps || 0, goal, 120) +
      '      <div class="b-steps-num"><b class="num">' + (steps != null ? Math.round(steps / 1000 * 10) / 10 + 'k' : '—') + '</b></div>' +
      '    </div>' +
      '    <div class="b-act-meta">' +
      '      <div class="b-stat"><span class="label">Steps</span><b class="num">' + (steps != null ? steps.toLocaleString() : '—') + '</b><small>/ ' + goal.toLocaleString() + '</small></div>' +
      '      <div class="b-stat"><span class="label">Active</span><b class="num">' + dash(act.activeMin != null ? act.activeMin + 'm' : null) + '</b></div>' +
      '      <div class="b-stat"><span class="label">Calories</span><b class="num">' + dash(act.calories) + '</b></div>' +
      '    </div>' +
      '  </div>' +
      '</button>' +
      '</div>';
  }

  function paint() {
    if (!V.el) return;
    V.el.innerHTML = html();
  }

  App.register('body', {
    show: function (el) {
      V.el = el;
      if (!V.bound) {
        V.bound = true;
        el.addEventListener('click', function (e) {
          var card = e.target.closest('[data-metric]');
          if (!card || !el.contains(card)) return;
          openMetricSheet(card.dataset.metric, 'day');
        });
      }
      paint();
    }
  });

  App.on('data', function () {
    if (App.current === 'body') paint();
  });
})();
