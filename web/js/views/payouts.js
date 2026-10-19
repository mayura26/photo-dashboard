// Payouts: monthly prop-firm totals, all-time firm bars, screenshot manage.
(function () {
  'use strict';
  var D = window.DATA;
  var V = { el: null, month: null, fp: '' };

  function firms() {
    return (D.payoutFirms || []).slice().sort(function (a, b) {
      return String(a.name || '').localeCompare(String(b.name || ''));
    });
  }

  function monthKeyOf(p) {
    var k = String((p && p.date) || '').slice(0, 7);
    return k.length < 7 ? '' : k;
  }

  function monthGroups() {
    var map = {};
    (D.payouts || []).forEach(function (p) {
      var k = monthKeyOf(p);
      (map[k] = map[k] || []).push(p);
    });
    var cur = App.payoutMonthKey(App.now());
    if (!map[cur]) map[cur] = [];
    return Object.keys(map).sort(function (a, b) {
      if (!a) return -1;
      if (!b) return 1;
      return b.localeCompare(a);
    }).map(function (k) {
      var items = map[k].slice().sort(function (a, b) {
        return String(b.date || '').localeCompare(String(a.date || '')) || String(a.account || '').localeCompare(String(b.account || ''));
      });
      return { key: k, items: items, sum: App.payoutSum(items) };
    });
  }

  function firmBars() {
    var list = D.payouts || [];
    var all = App.payoutSum(list);
    var cur = all.primary;
    var buckets = {};
    firms().forEach(function (f) {
      buckets[f.id] = { id: f.id, name: f.name, color: f.color, items: [] };
    });
    var unknown = [];
    list.forEach(function (p) {
      if (!p.firmId || !buckets[p.firmId]) unknown.push(p);
      else buckets[p.firmId].items.push(p);
    });
    var rows = Object.keys(buckets).map(function (id) {
      var b = buckets[id];
      var sum = App.payoutSum(b.items);
      return {
        name: b.name || 'Firm',
        color: b.color || '#111111',
        value: sum.by[cur] || 0,
        label: App.payoutMoney(sum.by[cur] || 0, cur),
        extra: extraLine(sum, cur)
      };
    });
    if (unknown.length) {
      var u = App.payoutSum(unknown);
      rows.push({
        name: 'Unknown',
        color: '#7A756B',
        value: u.by[cur] || 0,
        label: App.payoutMoney(u.by[cur] || 0, cur),
        extra: extraLine(u, cur)
      });
    }
    rows.sort(function (a, b) { return b.value - a.value || a.name.localeCompare(b.name); });
    var max = 0;
    rows.forEach(function (r) { if (r.value > max) max = r.value; });
    rows.forEach(function (r) { r.width = max > 0 ? Math.max(r.value > 0 ? 6 : 0, Math.round(r.value / max * 100)) : 0; });
    return { rows: rows, primary: cur };
  }

  function extraLine(sum, primary) {
    return Object.keys(sum.by).filter(function (c) { return c !== primary; })
      .map(function (c) { return App.payoutMoney(sum.by[c], c); }).join(' · ');
  }

  function firmName(id) {
    var f = App.payoutFirm(id);
    return f ? f.name : 'Unknown';
  }

  function toneClass(value) {
    if (value > 0) return ' is-up';
    if (value < 0) return ' is-down';
    return '';
  }

  function dayLabel(date) {
    if (!date) return 'No date';
    var p = String(date).split('-');
    var d = new Date(+p[0], (+p[1] || 1) - 1, +p[2] || 1);
    if (isNaN(d.getTime())) return date;
    return App.fmt.monthShort(d) + ' ' + d.getDate();
  }

  function empty() {
    return !(D.payouts || []).length;
  }

  function paint() {
    if (!V.el) return;
    var groups = monthGroups();
    if (!V.month || !groups.some(function (g) { return g.key === V.month; })) {
      var cur = App.payoutMonthKey(App.now());
      V.month = groups.some(function (g) { return g.key === cur; }) ? cur : (groups[0] ? groups[0].key : cur);
    }
    var selected = groups.filter(function (g) { return g.key === V.month; })[0] || groups[0];
    var bars = firmBars();
    var root = V.el.querySelector('.pay');
    root.classList.toggle('is-empty', empty());
    root.querySelector('.pay-months').innerHTML = groups.map(function (g) {
      var on = selected && g.key === selected.key;
      return '<button type="button" class="pay-month' + (on ? ' is-on' : '') + '" data-month="' + App.esc(g.key) + '">' +
        '<span class="pay-month-n"><b>' + App.esc(App.payoutMonthLabel(g.key)) + '</b>' +
        '<small>' + g.items.length + (g.items.length === 1 ? ' payout' : ' payouts') + '</small></span>' +
        '<span class="num' + toneClass(g.sum.value) + '">' + g.sum.main + '</span></button>';
    }).join('');
    var detail = '';
    if (empty()) {
      detail = '<div class="pay-empty"><h2 class="serif">No payouts yet</h2>' +
        '<p>Upload a screenshot in Manage. The amount, date, and firm can be corrected after.</p></div>';
    } else if (selected) {
      var series = App.payoutMonthSeries(selected.key);
      detail = '<div class="pay-hero' + toneClass(selected.sum.value) + '">' +
        '<div class="label">' + App.esc(App.payoutMonthLabel(selected.key)) + '</div>' +
        '<div class="pay-hero-num num">' + selected.sum.main + '</div>' +
        (selected.sum.extra ? '<div class="pay-hero-extra">' + App.esc(selected.sum.extra) + '</div>' : '') +
        (series ? '<div class="pay-spark-wrap">' + App.payoutSpark(series) +
          '<div class="pay-spark-scale"><span>1</span><span>' + series.days + '</span></div></div>' : '') +
        '</div><div class="pay-rows">' +
        (selected.items.length ? selected.items.map(function (p) {
          var amt = p.amount == null || p.amount === '' ? 'Needs amount' : App.payoutMoney(p.amount, 'USD');
          var rowTone = p.amount == null || p.amount === '' || isNaN(Number(p.amount)) ? '' : toneClass(Number(p.amount));
          return '<div class="pay-row">' +
            '<span class="pay-row-d">' + App.esc(dayLabel(p.date)) + '</span>' +
            '<span class="pay-row-m"><b>' + App.esc(firmName(p.firmId)) + '</b>' +
            (p.note ? '<small>' + App.esc(p.note) + '</small>' : '') +
            '</span>' +
            '<span class="pay-row-a num' + rowTone + '">' + App.esc(amt) + '</span></div>';
        }).join('') : '<div class="pay-none serif"><i>No payouts this month.</i></div>') +
        '</div>';
    }
    root.querySelector('.pay-detail').innerHTML = detail;
    var chart = '<div class="label">By firm</div><div class="pay-chart-sub">All time' +
      (bars.rows.length ? ' · ' + App.esc(bars.primary) : '') + '</div>';
    if (!bars.rows.length) {
      chart += '<div class="pay-none serif"><i>Nothing to chart yet.</i></div>';
    } else {
      chart += bars.rows.map(function (r) {
        return '<div class="pay-bar">' +
          '<span class="pay-bar-n">' + App.esc(r.name) + '</span>' +
          '<span class="pay-bar-v"><b class="num">' + r.label + '</b>' +
          (r.extra ? '<small>' + App.esc(r.extra) + '</small>' : '') +
          '</span>' +
          '<span class="pay-bar-track"><i style="width:' + r.width + '%;background:' + App.esc(r.color) + '"></i></span>' +
          '</div>';
      }).join('');
    }
    root.querySelector('.pay-chart').innerHTML = chart;
  }

  function api(url, opts) {
    return fetch(url, Object.assign({ credentials: 'same-origin' }, opts || {})).then(function (r) {
      if (!r.ok) return r.text().then(function (t) { throw new Error((t || r.statusText || 'error').trim()); });
      if (r.status === 204) return null;
      return r.json();
    });
  }

  function parseAmount(s) {
    s = String(s || '').replace(/,/g, '').replace(/[^0-9.\-]/g, '');
    if (!s || s === '-' || s === '.' || s === '-.') return null;
    var n = Number(s);
    return isNaN(n) ? null : n;
  }

  function firmOptions(selected) {
    var html = '<option value="">Unknown</option>';
    firms().forEach(function (f) {
      html += '<option value="' + App.esc(f.id) + '"' + (f.id === selected ? ' selected' : '') + '>' + App.esc(f.name) + '</option>';
    });
    return html;
  }

  function openEdit(id) {
    var p = (D.payouts || []).filter(function (x) { return x.id === id; })[0];
    if (!p) return;
    var initialFirm = p.firmId || '';
    App.sheet(
      '<div class="sheet-pad pay-edit">' +
      '<div class="label">Payout</div>' +
      '<h2 class="serif sheet-h">Details</h2>' +
      (p.image ? '<div class="pay-preview"><img src="/media/payouts/' + App.esc(p.image) + '.jpg" alt=""></div>' : '') +
      '<div class="pay-edit-grid">' +
      '  <label class="pay-field"><span class="label">Date</span><input class="pay-input" data-f="date" value="' + App.esc(p.date || '') + '" placeholder="YYYY-MM-DD" inputmode="numeric"></label>' +
      '  <label class="pay-field"><span class="label">Amount</span><input class="pay-input" data-f="amount" value="' + (p.amount == null || p.amount === '' ? '' : App.esc(p.amount)) + '" inputmode="decimal" placeholder="0"></label>' +
      '</div>' +
      '<label class="pay-field"><span class="label">Firm</span><select class="pay-select" data-f="firm">' + firmOptions(initialFirm) + '</select></label>' +
      '<label class="pay-field"><span class="label">Note</span><input class="pay-input" data-f="note" value="' + App.esc(p.note || '') + '" placeholder="Optional"></label>' +
      '<div class="sheet-actions"><button type="button" class="btn btn--ghost" data-del>Delete</button><button type="button" class="btn btn--solid" data-save>Save</button></div>' +
      '</div>',
      {
        cls: 'sheet--payouts',
        onMount: function (sheet, close) {
          var firmEl = sheet.querySelector('[data-f=firm]');
          sheet.addEventListener('click', function (e) {
            var t = e.target.closest('button');
            if (!t) return;
            if (t.hasAttribute('data-del')) {
              if (!confirm('Delete this payout?')) return;
              t.disabled = true;
              api('/api/payouts/' + id, { method: 'DELETE' }).then(function () {
                close();
                App.toast('Payout removed', { icon: 'payout', ms: 2200 });
              }).catch(function (err) {
                t.disabled = false;
                App.toast(App.esc(err.message), { tone: 'error' });
              });
              return;
            }
            if (!t.hasAttribute('data-save')) return;
            var patch = {
              date: sheet.querySelector('[data-f=date]').value.trim(),
              amount: parseAmount(sheet.querySelector('[data-f=amount]').value),
              currency: 'USD',
              firmId: firmEl.value,
              note: sheet.querySelector('[data-f=note]').value.trim()
            };
            t.disabled = true;
            api('/api/payouts/' + id, {
              method: 'PATCH',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(patch)
            }).then(function () {
              close();
              openManage();
              App.toast('Payout updated', { icon: 'payout', ms: 2000 });
            }).catch(function (err) {
              t.disabled = false;
              App.toast(App.esc(err.message), { tone: 'error' });
            });
          });
        }
      }
    );
  }

  function uploadFile(file) {
    var fd = new FormData();
    fd.append('file', file, file.name || 'payout.jpg');
    return api('/api/payouts', { method: 'POST', body: fd });
  }

  function fileFromBlob(blob) {
    var type = (blob && blob.type) || 'image/png';
    if (blob instanceof File && blob.name) return blob;
    var ext = (type.split('/')[1] || 'png').replace('jpeg', 'jpg');
    return new File([blob], 'clipboard.' + ext, { type: type });
  }

  function imageFromList(items) {
    if (!items) return null;
    for (var i = 0; i < items.length; i++) {
      var item = items[i];
      if (item.kind === 'file' && item.type && item.type.indexOf('image/') === 0) {
        var file = item.getAsFile && item.getAsFile();
        if (file) return fileFromBlob(file);
      }
    }
    return null;
  }

  function imageFromClipboard() {
    if (!navigator.clipboard || !navigator.clipboard.read) {
      return Promise.reject(new Error('Press Ctrl+V to paste a screenshot'));
    }
    return navigator.clipboard.read().then(function (items) {
      var found = null;
      items.forEach(function (item) {
        if (found) return;
        (item.types || []).forEach(function (type) {
          if (!found && type.indexOf('image/') === 0) found = item.getType(type);
        });
      });
      if (!found) throw new Error('No image on the clipboard');
      return found.then(fileFromBlob);
    });
  }

  function openManage() {
    var list = (D.payouts || []).slice().sort(function (a, b) {
      return String(b.date || '').localeCompare(String(a.date || '')) || String(b.id).localeCompare(String(a.id));
    });
    App.sheet(
      '<div class="sheet-pad pay-manage">' +
      '<div class="label">Payouts</div>' +
      '<h2 class="serif sheet-h">Manage</h2>' +
      '<p class="sheet-p">Upload a payout screenshot, or paste one. The amount, date, and firm are filled in when they can be read.</p>' +
      '<div class="pay-sec-h"><span class="label">Firms</span></div>' +
      '<form class="pay-firm-add" data-add-firm>' +
      '  <input class="pay-input" name="name" placeholder="Firm name" required>' +
      '  <input class="pay-input" name="prefixes" placeholder="Account starts with, e.g. FTMO">' +
      '  <button type="submit" class="btn btn--solid">Add</button>' +
      '</form>' +
      '<div class="pay-firms">' +
      (firms().length ? firms().map(function (f) {
        return '<form class="pay-firm" data-firm-id="' + App.esc(f.id) + '">' +
          '<i class="pay-swatch" style="background:' + App.esc(f.color || '#111') + '"></i>' +
          '<input class="pay-input" name="name" value="' + App.esc(f.name || '') + '" aria-label="Firm name">' +
          '<input class="pay-input" name="prefixes" value="' + App.esc((f.prefixes || []).join(', ')) + '" placeholder="Prefixes" aria-label="Account prefixes">' +
          '<button type="submit" class="btn btn--ghost">Save</button>' +
          '<button type="button" class="pay-icon" data-del-firm="' + App.esc(f.id) + '" aria-label="Delete firm">' + App.icon('x', 22) + '</button>' +
          '</form>';
      }).join('') : '<div class="pay-none">No firms yet. Add one so screenshots can be sorted.</div>') +
      '</div>' +
      '<div class="pay-sec-h"><span class="label">Payouts</span>' +
      '<span class="pay-add">' +
      '<button type="button" class="btn btn--ghost" data-paste>Paste</button>' +
      '<label class="btn btn--solid pay-upload">' + App.icon('plus', 22) + ' Upload' +
      '<input type="file" accept="image/*,.heic,.heif" data-upload></label></span></div>' +
      '<div class="pay-upload-status" data-status hidden></div>' +
      '<div class="pay-admin">' +
      (list.length ? list.map(function (p) {
        var amt = p.amount == null || p.amount === '' ? 'Needs amount' : App.payoutMoney(p.amount, 'USD');
        return '<article class="pay-admin-row" data-id="' + App.esc(p.id) + '">' +
          '<button type="button" class="pay-admin-thumb" data-edit="' + App.esc(p.id) + '">' +
          (p.image ? '<img src="/media/payouts/' + App.esc(p.image) + '.jpg" alt="">' : '') +
          '</button>' +
          '<button type="button" class="pay-admin-meta" data-edit="' + App.esc(p.id) + '">' +
          '<b>' + App.esc(amt) + '</b>' +
          '<span>' + App.esc(firmName(p.firmId)) + (p.date ? ' · ' + App.esc(p.date) : '') + '</span>' +
          '</button></article>';
      }).join('') : '<div class="pay-none">No payouts yet.</div>') +
      '</div></div>',
      (function () {
        var onPaste = null;
        return {
        cls: 'sheet--wide sheet--payouts',
        onClose: function () {
          if (onPaste) document.removeEventListener('paste', onPaste);
        },
        onMount: function (sheet, close) {
          var status = sheet.querySelector('[data-status]');
          var busy = false;
          function setStatus(msg) {
            status.hidden = !msg;
            status.textContent = msg || '';
          }
          function sendImage(file) {
            if (busy || !file) return;
            busy = true;
            setStatus('Reading ' + (file.name || 'screenshot') + '…');
            App.toast('Reading screenshot…', { icon: 'payout', ms: 120000 });
            uploadFile(file).then(function (body) {
              close();
              var firm = body && body.firmId ? firmName(body.firmId) : 'Unknown';
              if (body && body.extracted && body.amount != null) {
                App.toast('Read ' + App.esc(App.payoutMoney(body.amount, body.currency)) + ' · ' + App.esc(firm), { icon: 'payout', ms: 3500 });
              } else {
                App.toast('Couldn’t read the screenshot — fill it in', { icon: 'payout', ms: 4000 });
              }
              if (body && body.id) openEdit(body.id);
            }).catch(function (err) {
              busy = false;
              setStatus(err.message);
              App.toast(App.esc(err.message), { tone: 'error', ms: 5000 });
            });
          }
          onPaste = function (e) {
            var file = imageFromList(e.clipboardData && e.clipboardData.items);
            if (!file) return;
            e.preventDefault();
            sendImage(file);
          };
          document.addEventListener('paste', onPaste);
          sheet.querySelector('[data-add-firm]').addEventListener('submit', function (e) {
            e.preventDefault();
            var name = e.target.name.value.trim();
            var prefixes = e.target.prefixes.value.trim();
            if (!name) return;
            api('/api/payout-firms', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ name: name, prefixes: prefixes })
            }).then(function () {
              close();
              openManage();
              App.toast('Firm added', { icon: 'payout', ms: 2000 });
            }).catch(function (err) {
              App.toast(App.esc(err.message), { tone: 'error' });
            });
          });
          sheet.addEventListener('submit', function (e) {
            var form = e.target.closest('[data-firm-id]');
            if (!form) return;
            e.preventDefault();
            var id = form.getAttribute('data-firm-id');
            api('/api/payout-firms/' + id, {
              method: 'PATCH',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                name: form.name.value.trim(),
                prefixes: form.prefixes.value.trim()
              })
            }).then(function () {
              close();
              openManage();
              App.toast('Firm saved', { icon: 'payout', ms: 1800 });
            }).catch(function (err) {
              App.toast(App.esc(err.message), { tone: 'error' });
            });
          });
          var input = sheet.querySelector('[data-upload]');
          input.addEventListener('change', function () {
            var file = input.files && input.files[0];
            input.value = '';
            sendImage(file);
          });
          sheet.querySelector('[data-paste]').addEventListener('click', function () {
            imageFromClipboard().then(sendImage).catch(function (err) {
              var msg = err && err.name === 'NotAllowedError'
                ? 'Allow clipboard access, or press Ctrl+V'
                : ((err && err.message) || 'Couldn’t paste');
              setStatus(msg);
              App.toast(App.esc(msg), { tone: 'error', ms: 4000 });
            });
          });
          sheet.addEventListener('click', function (e) {
            var del = e.target.closest('[data-del-firm]');
            if (del) {
              var id = del.getAttribute('data-del-firm');
              var f = App.payoutFirm(id);
              if (!confirm('Remove ' + (f ? f.name : 'this firm') + '? Payouts stay, marked unknown.')) return;
              api('/api/payout-firms/' + id, { method: 'DELETE' }).then(function () {
                close();
                openManage();
                App.toast('Firm removed', { icon: 'payout', ms: 2000 });
              }).catch(function (err) {
                App.toast(App.esc(err.message), { tone: 'error' });
              });
              return;
            }
            var edit = e.target.closest('[data-edit]');
            if (!edit) return;
            var pid = edit.getAttribute('data-edit');
            close();
            openEdit(pid);
          });
        }
      };
      })()
    );
  }

  function fingerprint() {
    try { return JSON.stringify(D.payouts || []) + '|' + JSON.stringify(D.payoutFirms || []); }
    catch (e) { return ''; }
  }

  App.register('payouts', {
    show: function (el) {
      V.el = el;
      if (!el.firstChild) {
        el.innerHTML = '<div class="pay">' +
          '<div class="pay-head"><div><div class="label">Prop</div><h1 class="serif">Payouts</h1></div>' +
          '<button type="button" class="btn btn--solid" data-manage>' + App.icon('sliders', 22) + ' Manage</button></div>' +
          '<div class="pay-months"></div>' +
          '<div class="pay-detail"></div>' +
          '<div class="pay-chart"></div></div>';
        el.addEventListener('click', function (e) {
          var month = e.target.closest('[data-month]');
          if (month) {
            V.month = month.getAttribute('data-month');
            paint();
            return;
          }
          if (e.target.closest('[data-manage]')) openManage();
        });
      }
      V.fp = fingerprint();
      paint();
    }
  });

  App.on('data', function () {
    var fp = fingerprint();
    if (fp === V.fp) return;
    V.fp = fp;
    if (App.current === 'payouts' && V.el && V.el.firstChild) paint();
  });
})();
