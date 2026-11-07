'use strict';

(function () {
  const CHATS_KEY = 'zbChats';
  const ACTIVE_KEY = 'zbChatActive';
  const MODEL_KEY = 'zbModel';
  const state = {
    chats: [],
    active: null,
    query: '',
    streaming: false,
    abort: null,
    mounted: false,
    saveTimer: null
  };

  function uid() { return Date.now().toString(36) + Math.random().toString(36).slice(2, 7); }
  function esc(v) { return ZB.esc(v); }
  function load() {
    try { state.chats = JSON.parse(localStorage.getItem(CHATS_KEY) || '[]') || []; } catch (e) { state.chats = []; }
    state.chats = state.chats.filter(c => c && c.id);
    state.active = localStorage.getItem(ACTIVE_KEY) || (state.chats[0] && state.chats[0].id) || null;
    if (!state.chats.find(c => c.id === state.active)) state.active = state.chats[0] ? state.chats[0].id : null;
  }
  function save() {
    try { localStorage.setItem(CHATS_KEY, JSON.stringify(state.chats)); localStorage.setItem(ACTIVE_KEY, state.active || ''); }
    catch (e) { ZB.toast('Riwayat penuh: hapus percakapan lama', 'bad'); }
  }
  function saveSoon() {
    clearTimeout(state.saveTimer);
    state.saveTimer = setTimeout(save, 350);
  }
  function chat() { return state.chats.find(c => c.id === state.active) || null; }
  function touch(c) { c.updated = Date.now(); saveSoon(); }

  function newChat() {
    const c = { id: uid(), title: 'Percakapan baru', created: Date.now(), updated: Date.now(), messages: [] };
    state.chats.unshift(c);
    state.active = c.id;
    save();
    renderAll();
    setTimeout(() => { const i = document.getElementById('cinput'); if (i) i.focus(); }, 0);
  }

  function selectChat(id) { state.active = id; save(); renderAll(); }

  function deleteChat(id) {
    const c = state.chats.find(x => x.id === id);
    if (!c) return;
    if (c.messages.length && !confirm('Hapus percakapan "' + c.title + '"?')) return;
    state.chats = state.chats.filter(x => x.id !== id);
    if (state.active === id) state.active = state.chats[0] ? state.chats[0].id : null;
    save();
    if (!state.chats.length) newChat(); else renderAll();
  }

  function renameChat(id) {
    const c = state.chats.find(x => x.id === id); if (!c) return;
    const name = prompt('Nama percakapan', c.title);
    if (name == null) return;
    c.title = name.trim() || c.title;
    touch(c); renderAll();
  }

  /* ---------- markdown ---------- */
  function inline(s) {
    s = s.replace(/`([^`]+)`/g, '<code>$1</code>');
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/(^|[\s(])\*([^*\n]+)\*/g, '$1<em>$2</em>');
    s = s.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (m, text, url) => {
      if (!/^https?:\/\//i.test(url)) return m;
      return '<a href="' + url + '" target="_blank" rel="noopener noreferrer">' + text + '</a>';
    });
    return s;
  }
  function md(src) {
    const lines = String(src || '').replace(/\r\n/g, '\n').split('\n');
    let out = '', para = [], list = null, i = 0;
    const flushPara = () => { if (para.length) { out += '<p>' + inline(para.join('<br>')) + '</p>'; para = []; } };
    const flushList = () => { if (list) { out += '</' + list + '>'; list = null; } };
    while (i < lines.length) {
      const line = lines[i];
      const fence = line.match(/^```(\w*)\s*$/);
      if (fence) {
        flushPara(); flushList();
        const buf = []; i++;
        while (i < lines.length && !/^```\s*$/.test(lines[i])) buf.push(lines[i++]);
        i++;
        out += '<pre><code>' + esc(buf.join('\n')) + '</code></pre>';
        continue;
      }
      if (/^\s*$/.test(line)) { flushPara(); flushList(); i++; continue; }
      const h = line.match(/^(#{1,6})\s+(.*)$/);
      if (h) { flushPara(); flushList(); out += '<h' + h[1].length + '>' + inline(esc(h[2])) + '</h' + h[1].length + '>'; i++; continue; }
      const ul = line.match(/^\s*[-*]\s+(.*)$/), ol = line.match(/^\s*\d+[.)]\s+(.*)$/);
      if (ul || ol) {
        flushPara();
        const kind = ul ? 'ul' : 'ol';
        if (list !== kind) { flushList(); out += '<' + kind + '>'; list = kind; }
        out += '<li>' + inline(esc((ul || ol)[1])) + '</li>';
        i++; continue;
      }
      if (list) { flushList(); }
      para.push(esc(line));
      i++;
    }
    flushPara(); flushList();
    return out || '<p class="muted">(kosong)</p>';
  }
  window.ZB.md = md;

  function plainText(s) { return String(s || ''); }

  function thinkBlock(text) {
    if (!text) return '';
    return `<details class="think"><summary>Proses berpikir</summary><div>${esc(text).replace(/\n/g, '<br>')}</div></details>`;
  }

  /* ---------- rendering ---------- */
  function renderList() {
    const box = document.getElementById('clist');
    if (!box) return;
    const q = state.query.toLowerCase();
    const rows = state.chats.filter(c => !q || c.title.toLowerCase().includes(q) || c.messages.some(m => String(m.content).toLowerCase().includes(q)));
    box.innerHTML = rows.length ? rows.map(c => `
      <div class="chat-item ${c.id === state.active ? 'active' : ''}" data-id="${c.id}">
        <div class="chat-item-main"><b>${esc(c.title)}</b><span>${c.messages.length ? c.messages.length + ' pesan · ' + new Date(c.updated || c.created).toLocaleDateString('id-ID') : 'kosong'}</span></div>
        <div class="chat-item-tools">
          <button type="button" data-act="rename" title="Ganti nama">Ubah</button>
          <button type="button" data-act="delete" title="Hapus">Hapus</button>
        </div>
      </div>`).join('') : '<div class="empty small">Belum ada percakapan.</div>';
    box.onclick = e => {
      const item = e.target.closest('.chat-item'); if (!item) return;
      const id = item.dataset.id;
      const act = e.target.closest('button') && e.target.closest('button').dataset.act;
      if (act === 'rename') { e.stopPropagation(); renameChat(id); return; }
      if (act === 'delete') { e.stopPropagation(); deleteChat(id); return; }
      if (state.streaming) { ZB.toast('Hentikan stream dulu', 'bad'); return; }
      selectChat(id);
    };
  }

  function renderMessages() {
    const box = document.getElementById('cmsgs');
    if (!box) return;
    const c = chat();
    if (!c || !c.messages.length) {
      box.innerHTML = `<div class="chat-welcome">
        <h2>Mulai percakapan</h2>
        <p class="muted">Pesan dikirim lewat lane plan bridge. Model, suhu, dan batas token ada di header.</p>
        <div class="chat-samples">
          ${['Jelaskan arsitektur bridge ini', 'Buat fungsi Go untuk parsing SSE', 'Ringkas log error terakhir'].map(t => `<button type="button" class="btn ghost sm" data-sample="${esc(t)}">${esc(t)}</button>`).join('')}
        </div></div>`;
      box.querySelectorAll('button[data-sample]').forEach(b => b.onclick = () => { const i = document.getElementById('cinput'); i.value = b.dataset.sample; autosize(i); i.focus(); });
      return;
    }
    box.innerHTML = c.messages.map((m, idx) => {
      const isUser = m.role === 'user';
      const isStreaming = m._stream && state.streaming;
      return `<div class="msg ${isUser ? 'user' : 'assistant'}" data-idx="${idx}">
        <div class="msg-meta"><span>${isUser ? 'Kamu' : 'Asisten'}</span>${m.model && !isUser ? '<span class="badge">' + esc(m.model) + '</span>' : ''}${m.usage && m.usage.output_tokens > 0 ? '<span class="muted small">' + ZB.fmtN(m.usage.output_tokens) + ' token</span>' : ''}<span class="spacer"></span>
          ${isStreaming ? '<span class="badge warn">streaming…</span>' : ''}
          <button type="button" class="msgbtn" data-copy="${idx}" title="Salin pesan">Salin</button>
          ${!isUser && idx === c.messages.length - 1 && !state.streaming ? '<button type="button" class="msgbtn" data-regen="' + idx + '" title="Ulangi balasan">Ulangi</button>' : ''}
        </div>
        <div class="msg-body">${isUser ? '<p>' + esc(m.content).replace(/\n/g, '<br>') + '</p>' : thinkBlock(m.reasoning) + md(m.content)}</div>
      </div>`;
    }).join('');
    box.querySelectorAll('button[data-copy]').forEach(b => b.onclick = () => {
      const m = c.messages[Number(b.dataset.copy)];
      navigator.clipboard && navigator.clipboard.writeText(plainText(m.content)).then(() => ZB.toast('Disalin', 'ok'), () => ZB.toast('Gagal menyalin', 'bad'));
    });
    box.querySelectorAll('button[data-regen]').forEach(b => b.onclick = () => regenerate());
    box.scrollTop = box.scrollHeight;
  }

  function renderHeader() {
    const sel = document.getElementById('cmodel');
    if (!sel) return;
    const models = state.models || [];
    const current = localStorage.getItem(MODEL_KEY) || (models[0] && (models[0].id || models[0].model)) || '';
    sel.innerHTML = models.length
      ? models.map(m => { const id = m.id || m.model || m.name; return `<option value="${esc(id)}" ${id === current ? 'selected' : ''}>${esc(id)}</option>`; }).join('')
      : '<option value="">(model belum dimuat)</option>';
    if (!sel.value && current) sel.innerHTML = `<option value="${esc(current)}">${esc(current)}</option>`;
  }

  function renderAll() { renderList(); renderMessages(); renderHeader(); updateControls(); }

  function updateControls() {
    const send = document.getElementById('csend'), stop = document.getElementById('cstop'), input = document.getElementById('cinput');
    if (!send) return;
    send.classList.toggle('hidden', state.streaming);
    stop.classList.toggle('hidden', !state.streaming);
    input.disabled = false;
  }

  function autosize(ta) { ta.style.height = 'auto'; ta.style.height = Math.min(220, ta.scrollHeight) + 'px'; }

  function addMessage(role, content) {
    const c = chat() || newChat();
    const m = { role, content, at: Date.now() };
    c.messages.push(m);
    if (role === 'user' && (c.title === 'Percakapan baru' || !c.title)) {
      c.title = content.trim().slice(0, 46) || 'Percakapan baru';
    }
    touch(c);
    renderAll();
    return m;
  }

  function buildWire(c, upto) {
    return c.messages.slice(0, upto).filter(m => m.role === 'user' || m.role === 'assistant').map(m => ({ role: m.role, content: m.content }));
  }

  async function streamReply() {
    const c = chat();
    if (!c || state.streaming) return;
    const modelSel = document.getElementById('cmodel');
    const model = (modelSel && modelSel.value) || localStorage.getItem(MODEL_KEY) || '';
    if (!model) { ZB.toast('Model belum tersedia', 'bad'); return; }
    localStorage.setItem(MODEL_KEY, model);
    const temp = Number(document.getElementById('ctemp').value);
    const maxTok = Number(document.getElementById('cmax').value);
    const wire = buildWire(c, c.messages.length);
    const assistant = { role: 'assistant', content: '', model, at: Date.now(), _stream: true };
    c.messages.push(assistant);

    state.streaming = true;
    state.abort = new AbortController();
    renderAll();
    const box = document.getElementById('cmsgs');

    try {
      const headers = { 'Content-Type': 'application/json', Accept: 'text/event-stream' };
      if (ZB.token) headers.Authorization = 'Bearer ' + ZB.token;
      const body = { model, messages: wire, stream: true };
      if (!isNaN(temp) && temp >= 0) body.temperature = temp;
      if (!isNaN(maxTok) && maxTok > 0) body.max_tokens = maxTok;
      const res = await fetch('/v1/chat/completions', { method: 'POST', headers, body: JSON.stringify(body), signal: state.abort.signal });
      if (res.status === 401) { ZB.toast('Token diperlukan', 'bad'); throw new Error('401'); }
      if (!res.ok) {
        let detail = 'HTTP ' + res.status;
        try { const j = await res.json(); detail = (j.error && (j.error.message || j.error)) || detail; } catch (e) {}
        throw new Error(detail);
      }
      const reader = res.body.getReader();
      const dec = new TextDecoder();
      let buf = '', raw = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        let idx;
        while ((idx = buf.indexOf('\n')) >= 0) {
          const line = buf.slice(0, idx).trim(); buf = buf.slice(idx + 1);
          if (!line.startsWith('data:')) continue;
          const data = line.slice(5).trim();
          if (!data || data === '[DONE]') continue;
          let chunk; try { chunk = JSON.parse(data); } catch (e) { continue; }
          if (chunk.error) throw new Error(chunk.error.message || 'upstream error');
          const choice = chunk.choices && chunk.choices[0];
          if (!choice) continue;
          const delta = choice.delta || {};
          if (delta.reasoning_content) assistant.reasoning = (assistant.reasoning || '') + delta.reasoning_content;
          const piece = delta.content || '';
          if (piece) { raw += piece; assistant.content = raw; saveSoon(); }
          if (chunk.usage) assistant.usage = chunk.usage;
          if ((piece || delta.reasoning_content) && box) {
            const el = box.querySelector('.msg:last-child .msg-body');
            if (el) { el.innerHTML = thinkBlock(assistant.reasoning) + md(raw); box.scrollTop = box.scrollHeight; }
          }
        }
      }
      if (!assistant.content) assistant.content = '(tidak ada balasan)';
    } catch (e) {
      if (e.name === 'AbortError') {
        if (!assistant.content) assistant.content = '(dihentikan)';
        else assistant.content += '\n\n_(dihentikan)_';
        ZB.toast('Stream dihentikan', '');
      } else {
        assistant.content = assistant.content || ('**Gagal:** ' + e.message);
        ZB.toast(e.message, 'bad');
      }
    } finally {
      delete assistant._stream;
      assistant.at = Date.now();
      state.streaming = false;
      state.abort = null;
      if (c) touch(c);
      save();
      renderAll();
      const i = document.getElementById('cinput'); if (i) i.focus();
    }
  }

  function regenerate() {
    const c = chat();
    if (!c || state.streaming) return;
    while (c.messages.length && c.messages[c.messages.length - 1].role === 'assistant') c.messages.pop();
    if (!c.messages.length) return;
    save(); renderAll();
    streamReply();
  }

  function send() {
    if (state.streaming) return;
    const input = document.getElementById('cinput');
    const text = input.value.trim();
    if (!text) return;
    if (!chat()) newChat();
    input.value = ''; autosize(input);
    addMessage('user', text);
    streamReply();
  }

  function escape() {
    if (state.streaming && state.abort) { state.abort.abort(); }
  }

  function exportChats() {
    const blob = new Blob([JSON.stringify(state.chats, null, 2)], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'zcode-chats-' + new Date().toISOString().slice(0, 10) + '.json';
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000);
  }

  function importChats(file) {
    const fr = new FileReader();
    fr.onload = () => {
      try {
        const data = JSON.parse(fr.result);
        if (!Array.isArray(data)) throw new Error('format tidak dikenal');
        const clean = data.filter(c => c && c.id && Array.isArray(c.messages)).map(c => ({ id: c.id, title: c.title || 'Impor', created: c.created || Date.now(), updated: c.updated || Date.now(), messages: c.messages.map(m => ({ role: m.role, content: String(m.content || ''), model: m.model, usage: m.usage })) }));
        state.chats = clean.concat(state.chats.filter(c => !clean.some(x => x.id === c.id)));
        state.active = clean[0] ? clean[0].id : state.active;
        save(); renderAll();
        ZB.toast(clean.length + ' percakapan diimpor', 'ok');
      } catch (e) { ZB.toast('Impor gagal: ' + e.message, 'bad'); }
    };
    fr.readAsText(file);
  }

  async function loadModels() {
    try {
      const r = await ZB.api('/v1/models');
      state.models = (r.data || r.models || []).map(m => typeof m === 'string' ? { id: m } : m);
    } catch (e) {
      state.models = [];
      ZB.toast('Model gagal dimuat: ' + e.message, 'bad');
    }
    renderHeader();
  }

  function mount(view) {
    load();
    if (!state.chats.length) {
      state.chats = [{ id: uid(), title: 'Percakapan baru', created: Date.now(), updated: Date.now(), messages: [] }];
      state.active = state.chats[0].id;
      save();
    }
    view.innerHTML = `<div class="chat" id="chatroot">
      <aside class="chat-side">
        <div class="chat-side-head">
          <button class="btn primary sm" id="cnew" type="button">+ Baru</button>
          <input id="csearch" placeholder="Cari percakapan…" value="${esc(state.query)}">
        </div>
        <div id="clist" class="chat-list"></div>
        <div class="chat-side-foot">
          <button class="btn ghost sm" id="cexport" type="button">Ekspor</button>
          <button class="btn ghost sm" id="cimport" type="button">Impor</button>
          <input id="cfile" type="file" accept="application/json" hidden>
        </div>
      </aside>
      <section class="chat-main">
        <header class="chat-head">
          <button class="btn ghost sm only-mobile" id="cside" type="button" aria-label="Daftar chat">Daftar</button>
          <select id="cmodel" title="Model"></select>
          <input id="ctemp" type="number" step="0.1" min="0" max="2" value="${esc(localStorage.getItem('zbTemp') || '0.7')}" title="Temperature" class="chat-num">
          <input id="cmax" type="number" step="256" min="0" value="${esc(localStorage.getItem('zbMax') || '0')}" title="Max token (0 = default)" class="chat-num">
          <span class="small muted" id="cstate"></span>
          <span class="spacer"></span>
          <button class="btn ghost sm" id="cclear" type="button">Kosongkan</button>
        </header>
        <div id="cmsgs" class="chat-msgs"></div>
        <form id="cform" class="chat-composer">
          <textarea id="cinput" rows="1" placeholder="Tulis pesan… Enter kirim, Shift+Enter baris baru"></textarea>
          <button class="btn primary" id="csend" type="submit">Kirim</button>
          <button class="btn danger hidden" id="cstop" type="button">Stop</button>
        </form>
      </section>
    </div>`;
    state.mounted = true;

    document.getElementById('cnew').onclick = newChat;
    const search = document.getElementById('csearch');
    search.oninput = e => { state.query = e.target.value; renderList(); };
    document.getElementById('cexport').onclick = exportChats;
    document.getElementById('cimport').onclick = () => document.getElementById('cfile').click();
    document.getElementById('cfile').onchange = e => { if (e.target.files[0]) importChats(e.target.files[0]); e.target.value = ''; };
    document.getElementById('cclear').onclick = () => {
      const c = chat(); if (!c) return;
      if (c.messages.length && !confirm('Kosongkan pesan di percakapan ini?')) return;
      c.messages = []; c.title = 'Percakapan baru'; touch(c); renderAll();
    };
    document.getElementById('cmodel').onchange = e => localStorage.setItem(MODEL_KEY, e.target.value);
    document.getElementById('ctemp').onchange = e => localStorage.setItem('zbTemp', e.target.value);
    document.getElementById('cmax').onchange = e => localStorage.setItem('zbMax', e.target.value);
    document.getElementById('cstop').onclick = () => { if (state.abort) state.abort.abort(); };
    document.getElementById('cside').onclick = () => { const r = document.getElementById('chatroot'); if (r) r.classList.toggle('side-open'); };
    const form = document.getElementById('cform');
    const input = document.getElementById('cinput');
    form.onsubmit = e => { e.preventDefault(); send(); };
    input.oninput = () => autosize(input);
    input.onkeydown = e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } };
    renderAll();
    loadModels();
    setTimeout(() => input.focus(), 20);
  }

  window.ZChat = { mount, escape, md };
})();
