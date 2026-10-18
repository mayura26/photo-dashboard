// Photos: full-bleed editorial slideshow with layout modes + local library manage.
(function () {
  'use strict';
  var D = window.DATA;
  var V = { el: null, show: null, layout: 'auto', loud: false };

  var LAYOUTS = [['auto', 'Auto'], ['cover', 'Cover'], ['pair', 'Pair'], ['collage', 'Collage'], ['polaroid', 'Polaroid']];

  function heart() {
    var s = V.show && V.show.current, on = s && s.photos[0] && s.photos[0].fav;
    var b = V.el && V.el.querySelector('[data-fav]');
    if (!b) return;
    b.classList.toggle('is-on', !!on);
    b.disabled = !(s && s.photos[0]);
  }

  function empty() {
    return !(D.photos && D.photos.length);
  }

  function syncChrome() {
    if (!V.el) return;
    var root = V.el.querySelector('.photos');
    if (!root) return;
    root.classList.toggle('is-empty', empty());
    var emptyEl = root.querySelector('.ph-empty');
    if (emptyEl) emptyEl.hidden = !empty();
    heart();
  }

  function ensureBanner() {
    if (!V.el) return null;
    var b = V.el.querySelector('.ph-upload-banner');
    if (b) return b;
    b = document.createElement('div');
    b.className = 'ph-upload-banner';
    b.hidden = true;
    V.el.querySelector('.photos').appendChild(b);
    return b;
  }

  function setUploadStatus(msg) {
    var sheetEl = document.querySelector('.sheet .ph-upload-status');
    if (sheetEl) {
      sheetEl.hidden = !msg;
      sheetEl.textContent = msg || '';
    }
    var banner = ensureBanner();
    if (banner) {
      banner.hidden = !msg;
      banner.textContent = msg || '';
    }
  }

  function uploadFiles(fileList) {
    // Must copy FileList before the input is cleared — it is a live reference.
    var files = Array.prototype.slice.call(fileList || []);
    if (!files.length) return Promise.resolve(0);
    var ok = 0;
    var fail = 0;
    var i = 0;
    var total = files.length;
    function next() {
      if (i >= total) {
        if (V.show) { V.show.reset(); V.show.start(); }
        syncChrome();
        var done = ok ? ('Uploaded ' + ok + ' of ' + total + (fail ? ' (' + fail + ' failed)' : '')) : (fail ? total + ' uploads failed' : '');
        setUploadStatus(done);
        if (ok) App.toast(done, { icon: 'photo', ms: 3500 });
        setTimeout(function () { setUploadStatus(''); }, 4000);
        return Promise.resolve(ok);
      }
      var file = files[i++];
      var label = file.name || ('photo ' + i);
      var progress = 'Uploading ' + i + ' of ' + total + ' — ' + label;
      setUploadStatus(progress);
      App.toast(progress, { icon: 'photo', ms: 120000 });
      var fd = new FormData();
      fd.append('file', file, file.name || 'photo.jpg');
      return fetch('/api/photos', { method: 'POST', body: fd, credentials: 'same-origin' })
        .then(function (r) {
          if (!r.ok) return r.text().then(function (t) { throw new Error(t || r.statusText); });
          ok++;
          return r.json();
        })
        .then(next)
        .catch(function (err) {
          fail++;
          var msg = 'Failed ' + i + ' of ' + total + ' — ' + label + ' — ' + (err && err.message ? err.message : 'error');
          setUploadStatus(msg);
          App.toast(msg, { tone: 'error', ms: 5000 });
          return next();
        });
    }
    return next();
  }

  function patchPhoto(id, patch) {
    return fetch('/api/photos/' + id, {
      method: 'PATCH',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch)
    }).then(function (r) {
      if (!r.ok) return r.text().then(function (t) { throw new Error(t || r.statusText); });
      return r.json();
    });
  }

  function deletePhoto(id) {
    return fetch('/api/photos/' + id, { method: 'DELETE', credentials: 'same-origin' })
      .then(function (r) {
        if (!r.ok && r.status !== 204) return r.text().then(function (t) { throw new Error(t || r.statusText); });
      });
  }

  function openManage() {
    var photos = (D.photos || []).slice().sort(function (a, b) {
      return String(b.date || '').localeCompare(String(a.date || ''));
    });
    App.sheet(
      '<div class="sheet-pad ph-manage">' +
      '<div class="label">Photo library</div>' +
      '<h2 class="serif sheet-h">Manage</h2>' +
      '<p class="sheet-p">Date and orientation come from the file. Place uses GPS when present, otherwise tags/filename via GPT. Google Photos web downloads often strip GPS — originals/Takeout keep more metadata.</p>' +
      '<div class="ph-manage-bar">' +
      '  <label class="btn btn--solid ph-upload-btn">' + App.icon('plus', 22) + ' Upload' +
      '    <input type="file" accept="image/*,.heic,.heif" multiple data-upload>' +
      '  </label>' +
      '  <span class="ph-manage-count">' + photos.length + ' photo' + (photos.length === 1 ? '' : 's') + '</span>' +
      '</div>' +
      '<div class="ph-upload-status" hidden></div>' +
      '<div class="ph-grid" data-grid>' +
      (photos.length ? photos.map(function (p) {
        return '<article class="ph-card" data-id="' + App.esc(p.id) + '">' +
          '<button class="ph-card-thumb" data-edit type="button"><img src="' + App.photoSrc(p) + '" alt=""></button>' +
          '<div class="ph-card-meta">' +
          '  <b>' + App.esc(p.place || 'No place') + '</b>' +
          '  <span>' + App.esc(p.date || '') + (p.from ? ' · ' + App.esc(App.person(p.from).name) : '') + '</span>' +
          '</div>' +
          '<div class="ph-card-actions">' +
          '  <button type="button" class="ph-card-act' + (p.fav ? ' is-on' : '') + '" data-fav-id="' + App.esc(p.id) + '" aria-label="Favourite">' + App.icon('heart', 22) + '</button>' +
          '  <button type="button" class="ph-card-act" data-del-id="' + App.esc(p.id) + '" aria-label="Delete">' + App.icon('x', 22) + '</button>' +
          '</div></article>';
      }).join('') : '<div class="ph-grid-empty">No photos yet — upload from your phone or computer.</div>') +
      '</div></div>',
      {
        cls: 'sheet--wide sheet--photos',
        onMount: function (sheet) {
          var input = sheet.querySelector('[data-upload]');
          input.addEventListener('change', function () {
            var list = Array.prototype.slice.call(input.files || []);
            input.value = '';
            if (!list.length) return;
            uploadFiles(list).then(function () {
              App.closeSheet();
              openManage();
            });
          });
          sheet.addEventListener('click', function (e) {
            var t = e.target.closest('button');
            if (!t) return;
            if (t.dataset.favId) {
              if (App.sync) App.sync.fav(t.dataset.favId);
              t.classList.toggle('is-on');
              return;
            }
            if (t.dataset.delId) {
              var id = t.dataset.delId;
              deletePhoto(id).then(function () {
                var card = sheet.querySelector('[data-id="' + id + '"]');
                if (card) card.remove();
                if (V.show) { V.show.reset(); V.show.start(); }
                syncChrome();
                App.toast('Photo removed', { icon: 'photo', ms: 2000 });
              }).catch(function (err) {
                App.toast('Delete failed · ' + err.message, { tone: 'error' });
              });
              return;
            }
            if (t.hasAttribute('data-edit')) {
              var card = t.closest('[data-id]');
              if (!card) return;
              openEdit(card.getAttribute('data-id'));
            }
          });
        }
      }
    );
  }

  function openEdit(id) {
    var p = (D.photos || []).filter(function (x) { return x.id === id; })[0];
    if (!p) return;
    App.sheet(
      '<div class="sheet-pad add">' +
      '<div class="label">Edit photo</div>' +
      '<h2 class="serif sheet-h">Details</h2>' +
      '<div class="ph-edit-preview"><img src="' + App.photoSrc(p) + '" alt=""></div>' +
      '<div class="label">Place</div><input class="big-input" data-f="place" value="' + App.esc(p.place || '') + '" placeholder="Where was this?">' +
      '<div class="label">Date</div><input class="big-input" data-f="date" value="' + App.esc(p.date || '') + '" placeholder="YYYY-MM-DD">' +
      '<div class="label">Who</div><div class="add-who">' + D.people.map(function (person) {
        return '<button type="button" class="who-btn' + (p.from === person.id ? ' is-on' : '') + '" data-who="' + person.id + '" style="' + App.pstyle(person) + '">' +
          App.avatar(person, 56) + '<span>' + person.name + '</span></button>';
      }).join('') + '</div>' +
      '<div class="sheet-actions"><button class="btn btn--ghost" data-close>Cancel</button><button class="btn btn--solid" data-save>Save</button></div>' +
      '</div>',
      {
        onMount: function (sheet, close) {
          var from = p.from;
          sheet.addEventListener('click', function (e) {
            var t = e.target.closest('button');
            if (!t) return;
            if (t.dataset.who) {
              from = t.dataset.who;
              App.$$('[data-who]', sheet).forEach(function (b) { b.classList.toggle('is-on', b === t); });
            }
            if (t.hasAttribute('data-save')) {
              var place = sheet.querySelector('[data-f=place]').value.trim();
              var date = sheet.querySelector('[data-f=date]').value.trim();
              var patch = { place: place, from: from, date: date };
              if (/^\d{4}-\d{2}/.test(date)) patch.tag = date.slice(0, 7);
              patchPhoto(id, patch).then(function () {
                close();
                openManage();
                if (V.show && V.show.current) heart();
                App.toast('Photo updated', { icon: 'photo', ms: 2000 });
              }).catch(function (err) {
                App.toast('Save failed · ' + err.message, { tone: 'error' });
              });
            }
          });
        }
      }
    );
  }

  App.register('photos', {
    show: function (el) {
      if (!V.el) {
        V.el = el;
        el.innerHTML = '<div class="photos' + (empty() ? ' is-empty' : '') + '">' +
          '<div class="ph-stage"></div>' +
          '<div class="ph-empty" ' + (empty() ? '' : 'hidden') + '>' +
          '  <h2 class="serif">Your photos live here</h2>' +
          '  <p>Upload from this kiosk or your phone — images are sized for the frame.</p>' +
          '  <button class="btn btn--solid" data-manage>' + App.icon('plus', 22) + ' Add photos</button>' +
          '</div>' +
          '<button class="ph-zone ph-zone--l" data-prev aria-label="Previous"></button>' +
          '<button class="ph-zone ph-zone--c" data-loud aria-label="Show details"></button>' +
          '<button class="ph-zone ph-zone--r" data-next aria-label="Next"></button>' +
          '<div class="ph-top">' +
          '  <div class="seg seg--glass">' + LAYOUTS.map(function (l) {
            return '<button data-layout="' + l[0] + '" class="' + (l[0] === V.layout ? 'is-on' : '') + '">' + l[1] + '</button>';
          }).join('') + '</div>' +
          '  <button class="ph-btn ph-btn--manage" data-manage>' + App.icon('photo', 26) + '<span>Manage</span></button>' +
          '</div>' +
          '<div class="ph-bottom">' +
          '  <button class="ph-btn" data-prev>' + App.icon('left', 30) + '</button>' +
          '  <button class="ph-btn ph-btn--fav" data-fav>' + App.icon('heart', 30) + '<span>Favourite</span></button>' +
          '  <button class="ph-btn" data-next>' + App.icon('right', 30) + '</button>' +
          '</div></div>';
        V.show = new App.Slideshow(el.querySelector('.ph-stage'), { layout: V.layout, interval: 15000, onChange: function () { setTimeout(heart); } });
        el.addEventListener('click', function (e) {
          var t = e.target.closest('button');
          if (!t) return;
          if (t.hasAttribute('data-manage')) { openManage(); return; }
          if (t.hasAttribute('data-next')) V.show.next();
          if (t.hasAttribute('data-prev')) V.show.prev();
          if (t.hasAttribute('data-loud')) { V.loud = !V.loud; el.querySelector('.photos').classList.toggle('is-loud', V.loud); }
          if (t.dataset.layout) {
            V.layout = t.dataset.layout;
            App.$$('[data-layout]', el).forEach(function (b) { b.classList.toggle('is-on', b === t); });
            V.show.setLayout(V.layout);
          }
          if (t.hasAttribute('data-fav')) {
            var p = V.show.current && V.show.current.photos[0];
            if (!p) return;
            p.fav = !p.fav;
            heart();
            if (App.sync) App.sync.fav(p.id);
            App.toast(p.fav ? 'Added to favourites · it’ll show up more often' : 'Removed from favourites', { icon: 'heart', ms: 2500 });
          }
        });
        App.on('data', function () {
          syncChrome();
          if (!empty() && V.show && !V.show.current && V.el && !V.el.hidden) V.show.start();
          if (empty() && V.show) V.show.reset();
        });
      }
      syncChrome();
      V.show.start();
    },
    resume: function () { syncChrome(); V.show.start(); },
    hide: function () { if (V.show) V.show.stop(); }
  });
})();
