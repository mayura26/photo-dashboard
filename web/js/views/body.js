// Body: glanceable Garmin metrics — sleep, body battery, stress, activity.
(function () {
  'use strict';
  var D = window.DATA;
  var V = { el: null };

  function g() {
    return D.garmin || null;
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
    var lx = pad + ((pts.length - 1) / (pts.length - 1)) * (w - pad * 2);
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

      '<section class="b-battery b-reveal ' + batteryTone(bb.current) + '" style="--d:1">' +
      '  <div class="label">Body battery</div>' +
      '  <div class="b-bat-main">' +
      '    <div class="b-dial">' + ring(bb.current || 0, 100, 220) +
      '      <div class="b-dial-num"><b class="num">' + dash(bb.current) + '</b><small>/ 100</small></div>' +
      '    </div>' +
      '    <div class="b-bat-meta">' +
      '      <div class="b-stat"><span class="label">Charged</span><b class="num">+' + dash(bb.charged) + '</b></div>' +
      '      <div class="b-stat"><span class="label">Drained</span><b class="num">−' + dash(bb.drained) + '</b></div>' +
      '      <div class="b-spark-wrap">' + sparkline(bb.series) + '<span class="b-spark-cap">Today</span></div>' +
      '    </div>' +
      '  </div>' +
      '</section>' +

      '<section class="b-panel b-sleep b-reveal" style="--d:2">' +
      '  <div class="label">Sleep</div>' +
      '  <div class="b-panel-top">' +
      '    <div class="b-big"><b class="num">' + dash(sleep.score) + '</b><small>score</small></div>' +
      '    <div class="b-sub serif">' + fmtDur(sleep.durationMin) + '</div>' +
      '  </div>' +
      stageBar(sleep) +
      '</section>' +

      '<section class="b-panel b-stress b-reveal ' + stressTone(stress.current) + '" style="--d:3">' +
      '  <div class="label">Stress</div>' +
      '  <div class="b-panel-top">' +
      '    <div class="b-big"><b class="num">' + dash(stress.current) + '</b><small>now</small></div>' +
      '    <div class="b-sub">Avg <b class="num">' + dash(stress.avg) + '</b></div>' +
      '  </div>' +
      '  <div class="b-stress-row">' +
      '    <div class="b-stat"><span class="label">Rest</span><b class="num">' + dash(stress.restMin != null ? stress.restMin + 'm' : null) + '</b></div>' +
      '    <div class="b-stat"><span class="label">High</span><b class="num">' + dash(stress.highMin != null ? stress.highMin + 'm' : null) + '</b></div>' +
      '  </div>' +
      '</section>' +

      '<section class="b-panel b-act b-reveal' + (goalHit ? ' is-goal' : '') + '" style="--d:4">' +
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
      '</section>' +
      '</div>';
  }

  function paint() {
    if (!V.el) return;
    V.el.innerHTML = html();
  }

  App.register('body', {
    show: function (el) {
      V.el = el;
      paint();
    }
  });

  App.on('data', function () {
    if (App.current === 'body') paint();
  });
})();
