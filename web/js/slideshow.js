// Photo slideshow used by the Photos view, ambient mode and the home tile.
(function () {
  'use strict';
  var D = window.DATA;

  // Display-sized JPEGs from the local library (one size only — 1GB frame budget).
  function src(p) {
    if (!p || !p.id) return '';
    return '/media/photos/' + p.id + '.jpg';
  }
  App.photoSrc = src;

  function parseDate(p) { var a = (p.date || '').split('-'); return new Date(+a[0], +a[1] - 1, +a[2]); }
  App.photoMeta = function (p) {
    if (!p || !p.date) {
      return { date: '', monthYear: '', kicker: '', onThisDay: false, place: (p && p.place) || '', from: App.person(p && p.from) };
    }
    var d = parseDate(p), now = App.now();
    var years = now.getFullYear() - d.getFullYear();
    var onThisDay = d.getMonth() === now.getMonth() && d.getDate() === now.getDate();
    var kicker;
    if (onThisDay) kicker = years > 0 ? years + (years === 1 ? ' year' : ' years') + ' ago today' : 'Today';
    else kicker = years > 0 ? years + (years === 1 ? ' year ago' : ' years ago') : 'This year';
    return {
      date: App.fmt.month(d) + ' ' + d.getDate() + ', ' + d.getFullYear(),
      monthYear: App.fmt.month(d) + ' ' + d.getFullYear(),
      kicker: kicker, onThisDay: onThisDay, place: p.place || '', from: App.person(p.from)
    };
  };

  function shuffle(a) {
    for (var i = a.length - 1; i > 0; i--) { var j = Math.floor(Math.random() * (i + 1)), t = a[i]; a[i] = a[j]; a[j] = t; }
    return a;
  }

  function caption(ps, size) {
    var m = App.photoMeta(ps[0]);
    var kick = m.kicker ? ((m.onThisDay && m.kicker !== 'Today' ? '<b>On this day</b> · ' : '') + m.kicker) : '';
    if (m.place) kick += (kick ? ' · ' : '') + App.esc(m.place);
    return '<div class="cap cap--' + (size || 'l') + '">' +
      (kick ? '<div class="cap-kicker">' + kick + '</div>' : '') +
      (m.date || m.monthYear ? '<div class="cap-date serif">' + (ps.length > 1 ? m.monthYear : m.date) + '</div>' : '') +
      '</div>';
  }
  function kb(p, still) {
    var dx = (Math.random() > 0.5 ? 1 : -1) * (1 + Math.random() * 2.5) + '%';
    var dy = (Math.random() > 0.5 ? 1 : -1) * (0.5 + Math.random() * 1.5) + '%';
    return '<div class="kb' + (still ? ' kb--still' : '') + '" style="--dx:' + dx + ';--dy:' + dy + '"><img src="' + src(p) + '" alt=""></div>';
  }

  var BUILD = {
    cover: function (ps, o) {
      var p = ps[0];
      if (p.o === 'p') {
        return '<div class="slide slide--fit"><img class="fitimg" src="' + src(p) + '" alt="">' + (o.captions ? caption(ps) : '') + '</div>';
      }
      return '<div class="slide slide--cover">' + kb(p) + (o.captions ? caption(ps) : '') + '</div>';
    },
    pair: function (ps, o) {
      return '<div class="slide slide--pair"><div class="pane">' + kb(ps[0], true) + '</div><div class="pane">' + kb(ps[1], true) + '</div>' + (o.captions ? caption(ps, 'm') : '') + '</div>';
    },
    collage: function (ps, o) {
      return '<div class="slide slide--collage"><div class="pane pane--a">' + kb(ps[0], true) + '</div><div class="pane pane--b">' + kb(ps[1], true) + '</div><div class="pane pane--c">' + kb(ps[2], true) + '</div>' + (o.captions ? caption(ps, 'm') : '') + '</div>';
    },
    polaroid: function (ps) {
      var p = ps[0], m = App.photoMeta(p), rot = (Math.random() * 5 - 2.5).toFixed(1);
      return '<div class="slide slide--polaroid">' +
        '<figure class="polaroid polaroid--' + p.o + '" style="transform:rotate(' + rot + 'deg)"><div class="polaroid-img"><img src="' + src(p) + '" alt=""></div>' +
        '<figcaption class="serif"><i>' + App.esc(m.place || 'Untitled') + (m.monthYear ? ', ' + m.monthYear : '') + '</i></figcaption></figure></div>';
    }
  };

  // Keep at most 2 decoded prefetches to stay inside the ~1GB kiosk budget.
  var prefetchRing = [];
  function prefetchUrls(urls) {
    var want = {};
    urls.forEach(function (u) { if (u) want[u] = true; });
    prefetchRing = prefetchRing.filter(function (im) {
      if (want[im.src]) { delete want[im.src]; return true; }
      im.src = '';
      return false;
    });
    Object.keys(want).forEach(function (u) {
      if (prefetchRing.length >= 2) return;
      var im = new Image();
      im.src = u;
      prefetchRing.push(im);
    });
  }

  App.Slideshow = function (el, opts) {
    this.el = el;
    this.o = Object.assign({ layout: 'auto', interval: 15000, captions: true }, opts || {});
    this.queue = [];
    this.history = [];
    this.count = 0;
    this.timer = null;
    this.current = null;
  };
  var S = App.Slideshow.prototype;

  S.refill = function () {
    var q = [];
    (D.photos || []).forEach(function (p) { q.push(p); if (p.fav) q.push(p); });
    // On-this-day photos always come first in a fresh cycle
    shuffle(q).sort(function (a, b) { return (App.photoMeta(b).onThisDay ? 1 : 0) - (App.photoMeta(a).onThisDay ? 1 : 0); });
    this.queue = q;
  };
  S.take = function (pred) {
    if (!this.queue.length) this.refill();
    for (var i = 0; i < this.queue.length; i++) if (!pred || pred(this.queue[i])) return this.queue.splice(i, 1)[0];
    return null;
  };

  // YYYY-MM month index for proximity ranking; null if undated.
  function monthIndex(p) {
    var s = (p && (p.date || p.tag)) || '';
    if (s.length < 7) return null;
    var a = s.split('-');
    var y = +a[0], m = +a[1];
    if (!y || !m) return null;
    return y * 12 + m;
  }
  function sameCalendarDay(a, b) {
    var da = (a && a.date) || '', db = (b && b.date) || '';
    return da.length >= 10 && db.length >= 10 && da.slice(5) === db.slice(5);
  }
  // Lower score = closer period. Same day < same month < nearby months < undated.
  function pairScore(first, cand) {
    if (sameCalendarDay(first, cand)) return 0;
    var fi = monthIndex(first), ci = monthIndex(cand);
    if (fi != null && ci != null) {
      var dist = Math.abs(fi - ci);
      return dist === 0 ? 100 : 200 + dist;
    }
    return 100000;
  }

  S.pick = function () {
    var mode = this.o.layout, layout = mode, n = this.count++;
    var first = this.take();
    if (!first) return null;
    var self = this;
    function sibling(pred) {
      return self.take(pred) || (D.photos || []).filter(function (x) { return x && x.id !== first.id && pred(x); })[0];
    }
    function pairPartner(lead) {
      var cands = (D.photos || []).filter(function (x) {
        return x && x.id && lead.id && x.id !== lead.id && x.o === 'p';
      });
      if (!cands.length) return null;
      cands.sort(function (a, b) { return pairScore(lead, a) - pairScore(lead, b); });
      var best = pairScore(lead, cands[0]);
      var top = cands.filter(function (x) { return pairScore(lead, x) === best; });
      var want = {};
      top.forEach(function (x) { want[x.id] = true; });
      return self.take(function (x) { return x && want[x.id]; }) || top[0];
    }
    if (layout === 'auto') {
      if (first.o === 'p') layout = (D.photos || []).some(function (x) { return x.id !== first.id && x.o === 'p' && x.tag && x.tag === first.tag; }) ? 'pair' : 'polaroid';
      else layout = n % 5 === 3 ? 'collage' : n % 5 === 4 ? 'polaroid' : 'cover';
    }
    if (layout === 'ambient') {
      if (first.o === 'p') {
        layout = (D.photos || []).some(function (x) { return x.id !== first.id && x.o === 'p'; }) ? 'pair' : 'collage';
      } else {
        layout = n % 4 === 3 ? 'collage' : 'cover';
      }
    }
    var ps = [first];
    if (layout === 'pair') {
      if (first.o !== 'p') { first = sibling(function (x) { return x.o === 'p'; }); ps = [first]; }
      if (!first) return null;
      ps.push(pairPartner(first));
      if (!ps[1]) {
        if (mode === 'ambient') { layout = 'collage'; ps = [first]; }
        else return { layout: 'polaroid', photos: [ps[0]] };
      }
    }
    if (layout === 'collage') {
      ps.push(sibling(function (x) { return x.id !== first.id; }));
      ps.push(sibling(function (x) { return ps.every(function (p) { return !p || p.id !== x.id; }); }));
      if (!ps[1] || !ps[2]) return { layout: 'cover', photos: [ps[0]] };
    }
    return { layout: layout, photos: ps };
  };

  S.prefetchNext = function () {
    var urls = [];
    for (var i = 0; i < this.queue.length && urls.length < 2; i++) {
      var u = src(this.queue[i]);
      if (u && urls.indexOf(u) < 0) urls.push(u);
    }
    prefetchUrls(urls);
  };

  S.render = function (slide) {
    var self = this;
    this.current = slide;
    var wrap = document.createElement('div');
    wrap.innerHTML = BUILD[slide.layout](slide.photos, this.o);
    var node = wrap.firstChild;
    node.style.opacity = '0';
    Array.prototype.forEach.call(node.querySelectorAll('img'), function (im) {
      im.addEventListener('error', function () { im.classList.add('is-broken'); });
    });
    var old = Array.prototype.slice.call(this.el.children);
    this.el.appendChild(node);
    var first = node.querySelector('img');
    var shown = false;
    function show() {
      if (shown) return; shown = true;
      node.classList.add('is-in');
      node.style.opacity = '';
      old.forEach(function (o) { o.remove(); });
      self.prefetchNext();
    }
    if (first && !first.complete) { first.addEventListener('load', show); first.addEventListener('error', show); setTimeout(show, 2500); }
    else show();
    if (this.o.onChange) this.o.onChange(slide);
  };

  S.next = function () {
    if (!(D.photos || []).length) {
      this.current = null;
      this.el.innerHTML = '';
      return;
    }
    var s = this.pick();
    if (!s) return;
    if (this.current) this.history.push(this.current);
    this.render(s);
    this.restart();
  };
  S.prev = function () {
    if (!this.history.length) return;
    this.render(this.history.pop());
    this.restart();
  };
  S.setLayout = function (l) { this.o.layout = l; this.next(); };
  S.restart = function () {
    var self = this;
    clearInterval(this.timer);
    this.timer = setInterval(function () { self.next(); }, this.o.interval);
  };
  S.start = function () {
    if (!(D.photos || []).length) {
      this.stop();
      this.current = null;
      this.el.innerHTML = '';
      return;
    }
    if (!this.current) this.next();
    else this.restart();
  };
  S.stop = function () { clearInterval(this.timer); this.timer = null; };
  S.reset = function () {
    this.queue = [];
    this.history = [];
    this.count = 0;
    this.current = null;
    this.el.innerHTML = '';
  };
})();
