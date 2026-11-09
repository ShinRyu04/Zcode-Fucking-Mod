'use strict';

const ZB = window.ZB = {
  token: sessionStorage.getItem('zbToken') || '',
  health: null,
  usage: null,
  net: null,
  quota: null,
  models: null,
  page: 'overview',
  range: '24h',
  live: false,
  esAbort: null,
  lang: localStorage.getItem('zbLang') === 'en' ? 'en' : 'id'
};

const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));

function esc(v) {
  return String(v == null ? '' : v).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch (e) {
    const t = document.createElement('textarea');
    t.value = text; t.style.position = 'fixed'; t.style.opacity = '0';
    document.body.appendChild(t); t.select();
    try { document.execCommand('copy'); } catch (_) {}
    t.remove();
  }
  toast('Disalin', 'ok');
}
function fmtN(n) {
  n = Number(n) || 0;
  if (n >= 1e9) return (n / 1e9).toFixed(2) + ' B';
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' M';
  if (n >= 1e4) return (n / 1e3).toFixed(1) + ' K';
  return n.toLocaleString(ZB.lang === 'en' ? 'en-US' : 'id-ID');
}
function fmtTps(n) { n = Number(n) || 0; return n.toFixed(n >= 100 ? 0 : 1); }
function fmtBps(n) {
  n = Number(n) || 0;
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB/s';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB/s';
  return Math.round(n) + ' B/s';
}
function fmtBytes(n) {
  n = Number(n) || 0;
  if (n >= 1e9) return (n / 1e9).toFixed(2) + ' GB';
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB';
  return Math.round(n) + ' B';
}
function pct(a, b) { b = Number(b) || 0; return b <= 0 ? 0 : Math.max(0, Math.min(100, (Number(a) || 0) / b * 100)); }
function when(v) {
  if (!v) return '-';
  let d = new Date(v);
  if (typeof v === 'number' && v < 1e12) d = new Date(v * 1000);
  if (isNaN(d)) return '-';
  return d.toLocaleString(ZB.lang === 'en' ? 'en-GB' : 'id-ID', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' });
}
function toast(msg, kind) {
  msg = tr(msg);
  const el = document.createElement('div');
  el.className = 'toast ' + (kind || '');
  el.textContent = msg;
  $('#toasts').appendChild(el);
  setTimeout(() => el.remove(), 4200);
}

const EN_TEXT = {
  'Ringkasan': 'Overview',
  'Akun': 'Accounts',
  'Penggunaan': 'Usage',
  'Jaringan': 'Network',
  'Klaim plan': 'Plan claim',
  'Pengaturan': 'Settings',
  'Lane, kredensial, kuota, dan lalu lintas proses lokal.': 'Lane, credentials, quota, and local process traffic.',
  'Kecepatan terakhir': 'Latest speed',
  't/s dari request terakhir': 't/s from last request',
  'Request hari ini': 'Requests today',
  'Token hari ini': 'Tokens today',
  'Kuota plan': 'Plan quota',
  'memuat…': 'loading…',
  'Memuat…': 'Loading…',
  'Memuat...': 'Loading...',
  'Memuat kuota…': 'Loading quota…',
  'ring plan lokal': 'local plan ring',
  'Ring plan lokal: koneksi, akun aktif, dan masa berlaku.': 'Local plan ring: connection, active accounts, and expiry.',
  'tanpa masa aktif': 'no expiry',
  'aktif': 'active',
  'siap': 'ready',
  'kedaluwarsa': 'expired',
  'perlu login': 'reauth',
  'jeda': 'cooldown',
  'Hubungkan akun': 'Connect account',
  'Label akun baru (mis. kerja)': 'New account label (e.g. work)',
  'Belum ada akun tersimpan. Hubungkan akun Z.ai pertama.': 'No stored accounts yet. Connect your first Z.ai account.',
  'Akun di pool': 'Account pool',
  'kuota per model': 'per-model quota',
  'Segarkan': 'Refresh',
  'Pool akun ring plan: status tiap akun dan sisa kuota per model.': 'Account pool: per-account status and remaining quota by model.',
  'Belum ada akun': 'No accounts yet',
  'Hubungkan akun Z.ai pertama lewat form di bawah.': 'Connect your first Z.ai account with the form below.',
  'Kuota dibaca langsung dari billing tiap akun; cache 45 detik. Kolom model muncul otomatis dari plan yang terdeteksi.': 'Quota is read from each account billing; 45s cache. Model columns come from detected plans.',
  'Post baru': 'New post',
  'tersimpan lokal': 'stored locally',
  'Plan terdeteksi': 'Plans found',
  'Pemindaian terakhir': 'Last scan',
  'Riwayat klaim': 'Claim history',
  'dari pemindaian terakhir': 'from the last scan',
  'waktu lokal': 'local time',
  'percobaan tersimpan': 'stored attempts',
  'Aktifkan': 'Activate',
  'Setujui tautan ini di browser yang sudah login ke Z.ai:': 'Approve this link in a browser already signed in to Z.ai:',
  'Buka halaman izin': 'Open approval page',
  'Salin tautan': 'Copy link',
  'Batal': 'Cancel',
  'Menunggu persetujuan...': 'Waiting for approval...',
  'Akun terhubung.': 'Account connected.',
  'Login dibatalkan.': 'Login cancelled.',
  'Login gagal.': 'Login failed.',
  'Tutup': 'Close',
  'Akun berhasil terhubung': 'Account connected',
  'Akun aktif diganti': 'Active account changed',
  'Akun dihapus': 'Account deleted',
  'Kunci baru dibuat dan dipakai tab ini': 'New key created and used by this tab',
  'Kunci dihapus': 'Key deleted',
  'Request terakhir': 'Recent requests',
  'Waktu': 'Time',
  'Metode': 'Method',
  'Rute': 'Route',
  'Durasi': 'Duration',
  'Ukuran': 'Size',
  'Token': 'Tokens',
  'Jaringan proses': 'Process network',
  'Baca': 'Read',
  'Tulis': 'Write',
  'Total': 'Total',
  'Adapter': 'Adapters',
  'Turun': 'Down',
  'Naik': 'Up',
  'Unduh': 'Download',
  'Unggah': 'Upload',
  'semua adapter': 'all adapters',
  'Hari ini': 'Today',
  'hari ini': 'today',
  'Sepanjang waktu': 'All time',
  'Kecepatan': 'Speed',
  'Puncak': 'Peak',
  'Rata-rata': 'Average',
  'Rata-rata 7 hari': '7-day average',
  'Rata-rata 7h': '7h average',
  'Puncak 7h': '7h peak',
  'Terakhir': 'Last',
  'Tertinggi': 'Highest',
  'Per model': 'Per model',
  'Aplikasi ZCode': 'ZCode app',
  'Sesi': 'Sessions',
  'Aktivitas': 'Activity',
  '7 hari': '7 days',
  'Keluar': 'Out',
  'Masuk': 'In',
  'Token keluar': 'Output tokens',
  'Token masuk': 'Input tokens',
  'token keluar': 'output tokens',
  'Info lane': 'Lane info',
  'Versi app': 'App version',
  'Versi': 'Version',
  'Token plan': 'Plan token',
  'ada': 'present',
  'tidak ada': 'none',
  'tidak': 'no',
  'ya': 'yes',
  'tidak tersedia': 'unavailable',
  'tidak didukung': 'unsupported',
  'nonaktif': 'disabled',
  'sampler lokal': 'local sampler',
  'Klaim': 'Claim',
  'Token, kecepatan, dan tren per model.': 'Tokens, speed, and per-model trends.',
  'Klaim sekarang': 'Claim now',
  'Cek plan weekend/trial yang tersedia lalu klaim.': 'Check available weekend/trial plans, then claim.',
  'Tidak ada plan yang bisa diklaim.': 'No plans available to claim.',
  'Tekan “Cek plan” untuk memindai penawaran.': 'Press “Check plans” to scan offers.',
  'Belum ada plan aktif': 'No active plans',
  'Tekan “Cek plan” untuk memindai penawaran weekend/trial.': 'Press “Check plans” to scan weekend/trial offers.',
  'Tidak ada plan yang bisa diklaim': 'No plans available to claim',
  'Coba pindai lagi nanti; penawaran biasanya muncul akhir pekan.': 'Scan again later; offers usually appear on weekends.',
  'Riwayat klaim': 'Claim history',
  'Belum ada riwayat.': 'No history yet.',
  'Buka halaman klaim': 'Open claim page',
  'Butuh plan JWT / device MID.': 'A plan JWT / device MID is required.',
  'Permintaan klaim dikirim': 'Claim request sent',
  'Mulai': 'Start',
  'Selesai': 'End',
  'Inspector': 'Inspector',
  'Rekaman request/response lokal, replay, dan ekspor HAR.': 'Local request/response capture, replay, and HAR export.',
  'Muat ulang': 'Reload',
  'Ekspor HAR': 'Export HAR',
  'Cari model, path, akun…': 'Search model, path, account…',
  'Semua metode': 'All methods',
  'Semua status': 'All statuses',
  'Pilih satu baris untuk melihat detail.': 'Select a row to view details.',
  'Tidak ada hasil.': 'No results.',
  'Memuat detail…': 'Loading details…',
  'Replay terkirim': 'Replay sent',
  'Entri dihapus': 'Entry deleted',
  'Sharing': 'Sharing',
  'Papan sharing lokal: post, link, gambar, dan komentar tersimpan di bridge ini.': 'Local sharing board: posts, links, images, and comments are stored on this bridge.',
  'Judul': 'Title',
  'Tulis pesan...': 'Write a message...',
  'Gambar': 'Image',
  'Link (opsional)': 'Link (optional)',
  'Nama': 'Name',
  'Nama tampilan': 'Display name',
  'Lampiran': 'Attachment',
  'Belum ada komentar.': 'No comments yet.',
  'Tulis komentar...': 'Write a comment...',
  'Suka': 'Like',
  'Komentar': 'Comments',
  'Belum ada post': 'No posts yet',
  'Mulai dari form di atas.': 'Start with the form above.',
  'Memuat feed...': 'Loading feed...',
  'Terbaru': 'Newest',
  'Terpopuler': 'Top',
  'Judul wajib diisi': 'Title is required',
  'Isi, link, atau gambar wajib ada': 'Add a message, link, or image',
  'Gambar maksimal 120 KB': 'Image must be under 120 KB',
  'Hanya file gambar yang didukung': 'Only image files are supported',
  'Gambar gagal dibaca': 'Image could not be read',
  'Hapus gambar': 'Remove image',
  'Pratinjau gambar': 'Image preview',
  'Post terkirim': 'Posted',
  'Post dihapus': 'Post deleted',
  'Hapus post ini?': 'Delete this post?',
  'Jangan posting token privat. Siapa pun yang membuka bridge ini bisa membaca papan.': 'Do not post private tokens. Anyone who opens this bridge can read the board.',
  'Urutkan post': 'Sort posts',
  'judul wajib diisi': 'title is required',
  'judul terlalu panjang': 'title is too long',
  'isi post terlalu panjang': 'post body is too long',
  'link terlalu panjang': 'link is too long',
  'link harus http/https': 'link must be http or https',
  'gambar tidak didukung atau terlalu besar': 'image is unsupported or too large',
  'isi, link, atau gambar wajib ada': 'add a message, link, or image',
  'komentar kosong': 'comment is empty',
  'komentar terlalu panjang': 'comment is too long',
  'post tidak ditemukan': 'post not found',
  'hanya pembuat post yang bisa menghapus': 'only the post creator can delete it',
  'body bukan JSON yang valid': 'request body is not valid JSON',
  'id wajib diisi': 'id is required',
  '(dipotong)': '(truncated)',
  'Jaringan': 'Network',
  'Lalu lintas adapter, koneksi TCP, dan proses ZCode.': 'Adapter traffic, TCP connections, and the ZCode process.',
  'Proses ZCode': 'ZCode process',
  'Proses ZCode tidak ditemukan.': 'ZCode process not found.',
  'Menunggu sampel jaringan…': 'Waiting for network samples…',
  'Tidak ada adapter.': 'No adapters.',
  'Nilai efektif proses ini. Perubahan env butuh restart.': 'Effective values for this process. Env changes need a restart.',
  'Token & tema': 'Token & theme',
  'Bearer token (sesi tab)': 'Bearer token (tab session)',
  'Simpan token': 'Save token',
  'Tema': 'Theme',
  'Ganti tema': 'Change theme',
  'Integrasi agen': 'Agent integration',
  'Kunci API (Bearer hilir)': 'API key (downstream Bearer)',
  'belum di-set': 'not set',
  'belum di-set, bridge terbuka di loopback': 'not set, bridge is open on loopback',
  'Buat kunci': 'Create key',
  'Ganti kunci': 'Rotate key',
  'Contoh permintaan': 'Example request',
  'Salin contoh': 'Copy example',
  'Klien lain memakai kunci ini sebagai': 'Other clients use this key as',
  '. Kunci env tidak bisa diubah dari dashboard.': '. Env keys cannot be changed from the dashboard.',
  'Variabel': 'Variable',
  'Nilai': 'Value',
  'Data & alat': 'Data & tools',
  'Dashboard klasik': 'Classic dashboard',
  'Capture kapasitas': 'Capture capacity',
  'Capture dan flag lain dibaca saat start; ubah env lalu restart bridge.': 'Capture and other flags are read at startup; change env then restart the bridge.',
  'Tidak ada override env yang terbaca.': 'No environment overrides found.',
  'Token bridge': 'Bridge token',
  'Atur token bridge': 'Set bridge token',
  'Diperlukan saat ZCODE_BRIDGE_TOKEN di-set. Disimpan di sesi tab ini.': 'Required when ZCODE_BRIDGE_TOKEN is set. Stored in this tab session.',
  'Token disimpan': 'Token saved',
  'Token dihapus': 'Token deleted',
  'Status stream langsung': 'Live stream status',
  'Navigasi utama': 'Main navigation',
  'Buka menu': 'Open menu',
  'lane lokal': 'local lane',
  'memuat lane': 'loading lane',
  'Buka overlay': 'Open overlay',
  'Lewati ke konten': 'Skip to content',
  '+ Baru': '+ New',
  'Ekspor': 'Export',
  'Impor': 'Import',
  'Daftar': 'List',
  'Daftar chat': 'Chat list',
  'Kosongkan': 'Clear',
  'Cari percakapan…': 'Search chats…',
  'Tulis pesan… Enter kirim, Shift+Enter baris baru': 'Write a message… Enter sends, Shift+Enter for a new line',
  'Kirim': 'Send',
  'Stop': 'Stop',
  'Mulai percakapan': 'Start a conversation',
  'Pesan dikirim lewat lane plan bridge. Model, suhu, dan batas token ada di header.': 'Messages go through the bridge plan lane. Model, temperature and token limits are in the header.',
  'Jelaskan arsitektur bridge ini': 'Explain this bridge architecture',
  'Buat fungsi Go untuk parsing SSE': 'Write a Go function to parse SSE',
  'Ringkas log error terakhir': 'Summarize the latest error log',
  'Kamu': 'You',
  'Asisten': 'Assistant',
  'Salin': 'Copy',
  'Salin pesan': 'Copy message',
  'Ulangi': 'Retry',
  'Proses berpikir': 'Reasoning',
  '(kosong)': '(empty)',
  'kosong': 'empty',
  '(tidak ada balasan)': '(no reply)',
  '(dihentikan)': '(stopped)',
  'Percakapan baru': 'New chat',
  'Belum ada percakapan.': 'No chats yet.',
  'Ubah': 'Rename',
  'Ganti nama': 'Rename',
  'Hapus': 'Delete',
  'Max token (0 = default)': 'Max tokens (0 = default)',
  '(model belum dimuat)': '(model not loaded yet)',
  'streaming…': 'streaming…',
  'Riwayat penuh: hapus percakapan lama': 'History full: delete old chats',
  'Hentikan stream dulu': 'Stop the stream first',
  'Model belum tersedia': 'Model not available',
  'Token diperlukan': 'Token required',
  'token diperlukan': 'token required',
  'Stream dihentikan': 'Stream stopped',
  'Gagal menyalin': 'Copy failed',
  'Disalin': 'Copied',
  'format tidak dikenal': 'unknown format',
  'Hapus kunci? Setelah ini bridge terbuka untuk siapa pun di loopback.': 'Delete the key? The bridge will then be open to anyone on loopback.',
  'Ganti kunci? Klien yang memakai kunci lama langsung ditolak.': 'Rotate the key? Clients using the old key will be rejected immediately.',
  'Nama percakapan': 'Chat name',
  'Kosongkan pesan di percakapan ini?': 'Clear the messages in this chat?',
  'Model gagal dimuat: ': 'Failed to load models: ',
  'Impor gagal: ': 'Import failed: ',
  'Database aplikasi tidak terbaca.': 'Application database could not be read.',
  'Belum ada request hari ini.': 'No requests today yet.',
  'Belum ada request pada rentang ini': 'No requests in this range'
};

const EN_PATTERNS = [
  [/^(\S+) koneksi TCP, (.+) established$/, '$1 TCP connections, $2 established'],
  [/^(.+) token keluar · (.+) masuk$/, '$1 output tokens · $2 input tokens'],
  [/^(\d+) pesan · (.+)$/, '$1 messages · $2'],
  [/^sisa (.+)$/, '$1 left'],
  [/^berakhir (.+)$/, 'ends $1'],
  [/^akun (.+) · (.+)$/, 'account $1 · $2'],
  [/^ada \((\d+) akun\)$/, 'present ($1 account)'],
  [/^(.+) \/ (.+) unit$/, '$1 / $2 units'],
  [/^Model default (.+)$/, 'Default model $1'],
  [/^Tampilan: (.+)$/, 'Preview: $1'],
  [/^Kalibrasi (.+)$/, 'Calibration $1'],
  [/^(.+) token$/, '$1 tokens'],
  [/^Hapus akun (.+)\?$/, 'Delete account $1?'],
  [/^Hapus percakapan "(.+)"\?$/, 'Delete chat "$1"?'],
  [/^(\d+) percakapan diimpor$/, '$1 chats imported'],
  [/^Impor gagal: (.+)$/, 'Import failed: $1'],
  [/^Model gagal dimuat: (.+)$/, 'Failed to load models: $1'],
  [/^(\d+) akun$/, '$1 accounts']
];

function tr(text) {
  if (ZB.lang !== 'en' || !text) return text;
  if (Object.prototype.hasOwnProperty.call(EN_TEXT, text)) return EN_TEXT[text];
  const tcp = text.match(/^(\d+) koneksi TCP, (.+) established$/);
  if (tcp) return tcp[1] + ' TCP connection' + (tcp[1] === '1' ? '' : 's') + ', ' + tcp[2] + ' established';
  for (const [re, rep] of EN_PATTERNS) {
    if (re.test(text)) return text.replace(re, rep);
  }
  return text;
}
ZB.tr = tr;

function translateTextNode(node) {
  const raw = node.nodeValue;
  if (!raw) return;
  const core = raw.trim();
  if (!core) return;
  const next = tr(core);
  if (next === core) return;
  const at = raw.indexOf(core);
  node.nodeValue = raw.slice(0, at) + next + raw.slice(at + core.length);
}

const I18N_SKIP = '.msg-body,.snippet,.share-title,.share-body,.share-author,.share-link,.share-comment-body,.share-comment-author,script,style,textarea';

function translateTree(root) {
  if (ZB.lang !== 'en' || !root) return;
  if (root.nodeType === 3) { translateTextNode(root); return; }
  if (root.nodeType !== 1 && root.nodeType !== 9) return;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    if (!n.parentElement) continue;
    if (n.parentElement.closest(I18N_SKIP) && !n.parentElement.closest('.think summary')) continue;
    translateTextNode(n);
  }
  const els = [];
  if (root.nodeType === 1) els.push(root);
  if (root.querySelectorAll) els.push(...root.querySelectorAll('[placeholder],[title],[aria-label]'));
  for (const el of els) {
    for (const attr of ['placeholder', 'title', 'aria-label']) {
      const v = el.getAttribute(attr);
      if (!v) continue;
      const next = tr(v);
      if (next !== v) el.setAttribute(attr, next);
    }
  }
}

let i18nObserver = null;
const nativeConfirm = window.confirm.bind(window);
const nativePrompt = window.prompt.bind(window);
window.confirm = message => nativeConfirm(tr(message));
window.prompt = (message, value) => nativePrompt(tr(message), value);

function applyLanguage() {
  document.documentElement.lang = ZB.lang;
  const btn = $('#langbtn');
  if (btn) {
    btn.textContent = ZB.lang === 'en' ? 'ID' : 'EN';
    btn.title = ZB.lang === 'en' ? 'Ganti ke Bahasa Indonesia' : 'Switch to English';
    btn.setAttribute('aria-label', btn.title);
    btn.onclick = () => {
      localStorage.setItem('zbLang', ZB.lang === 'en' ? 'id' : 'en');
      location.reload();
    };
  }
  translateTree(document.body);
  if (!i18nObserver && document.body) {
    i18nObserver = new MutationObserver(mutations => {
      for (const m of mutations) {
        for (const n of m.addedNodes) translateTree(n);
      }
    });
    i18nObserver.observe(document.body, { childList: true, subtree: true });
  }
}
ZB.esc = esc; ZB.fmtN = fmtN; ZB.toast = toast;

function openToken() {
  $('#tokeninput').value = ZB.token;
  const d = $('#tokendlg');
  if (typeof d.showModal === 'function') d.showModal(); else d.setAttribute('open', '');
}

async function api(path, opts = {}) {
  const headers = Object.assign({}, opts.headers || {});
  if (ZB.token) headers['Authorization'] = 'Bearer ' + ZB.token;
  if (opts.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json';
  const res = await fetch(path, Object.assign({ cache: 'no-store' }, opts, { headers }));
  if (res.status === 401) { openToken(); throw new Error('token diperlukan'); }
  const ct = res.headers.get('content-type') || '';
  const body = ct.includes('json') ? await res.json() : await res.text();
  if (!res.ok) {
    const msg = body && body.error ? (typeof body.error === 'string' ? body.error : body.error.message) : ('HTTP ' + res.status);
    throw new Error(msg);
  }
  return body;
}
ZB.api = api;

function route() {
  let hash = location.hash.replace(/^#\/?/, '').split('?')[0];
  if (!hash) hash = location.pathname === '/chat' ? 'chat' : 'overview';
  if (!['overview', 'accounts', 'chat', 'sharing', 'inspector', 'usage', 'network', 'claim', 'settings'].includes(hash)) hash = 'overview';
  ZB.page = hash;
  $$('#nav a').forEach(a => a.classList.toggle('active', a.dataset.page === hash));
  document.body.classList.toggle('navopen', false);
  document.body.classList.toggle('chat-mode', hash === 'chat');
  const view = $('#view');
  view.innerHTML = '';
  if (hash === 'chat' && window.ZChat) window.ZChat.mount(view);
  else if (hash === 'chat') view.innerHTML = '<div class="empty">Memuat chat…</div>';
  else ({ overview: pageOverview, accounts: pageAccounts, sharing: pageSharing, inspector: pageInspector, usage: pageUsage, network: pageNetwork, claim: pageClaim, settings: pageSettings })[hash](view);
  view.scrollTop = 0;
}
window.addEventListener('hashchange', route);

function skeleton(title, sub) {
  return `<div class="page-head"><div><h1>${esc(title)}</h1><p>${esc(sub)}</p></div></div>`;
}

/* ---------------- overview ---------------- */
function pageOverview(view) {
  const h = ZB.health || {};
  const u = ZB.usage;
  const n = ZB.net;
  view.innerHTML = skeleton('Ringkasan', 'Lane, kredensial, kuota, dan lalu lintas proses lokal.') + `
  <div class="grid g3" id="ovtiles">
    <div class="card stat"><span class="k">Kecepatan terakhir</span><span class="v" id="ov-tps">-</span><span class="s">t/s dari request terakhir</span></div>
    <div class="card stat"><span class="k">Request hari ini</span><span class="v" id="ov-req">-</span><span class="s" id="ov-tok">-</span></div>
    <div class="card stat"><span class="k">Jaringan</span><span class="v smallv" id="ov-net">-</span><span class="s" id="ov-net2">-</span></div>
  </div>
  <div class="grid g2 mt14">
    <div class="card" id="ov-quota"><h2>Kuota plan <span class="muted" id="ov-qs">memuat…</span></h2><div class="empty">Memuat kuota…</div></div>
    <div class="card" id="ov-accounts"><h2>Akun <span class="muted">ring plan lokal</span></h2><div class="empty">Memuat…</div></div>
  </div>
  <div class="grid g2 mt14">
    <div class="card" id="ov-recent"><h2>Request terakhir</h2><div class="empty">Belum ada data.</div></div>
    <div class="card" id="ov-netcard"><h2>Jaringan proses</h2><div class="empty">Menunggu sampel…</div></div>
  </div>
  <div class="grid g2 mt14">
    <div class="card" id="ov-claim"><h2>Klaim plan <span class="muted">weekend / trial</span></h2><div class="empty">Memuat…</div></div>
    <div class="card"><h2>Info lane</h2>
      <dl class="kv">
        <dt>Lane</dt><dd id="ov-lane">-</dd>
        <dt>Upstream</dt><dd id="ov-up" class="mono smallmono">-</dd>
        <dt>Versi app</dt><dd id="ov-ver">-</dd>
        <dt>Device MID</dt><dd id="ov-mid">-</dd>
        <dt>Auth</dt><dd id="ov-authmode">-</dd>
        <dt>Token plan</dt><dd id="ov-jwt">-</dd>
      </dl>
    </div>
  </div>`;
  patchOverview();
  loadOverviewExtras();
}

function patchOverview() {
  const h = ZB.health || {}, u = ZB.usage, n = ZB.net;
  if (!$('#ov-tps')) return;
  if (u) {
    $('#ov-tps').textContent = fmtTps(u.rings && u.rings.last_tps);
    $('#ov-req').textContent = fmtN(u.today && u.today.requests);
    $('#ov-tok').textContent = fmtN((u.today && u.today.output_tokens)) + ' token keluar · ' + fmtN((u.today && u.today.input_tokens)) + ' masuk';
  }
  const acct = (h.accounts || []).find(a => a.active) || (h.accounts || [])[0];
  const acctEl = $('#ov-acct');
  if (acctEl) acctEl.textContent = acct ? (acct.label || acct.id || 'default') : '-';
  const authEl = $('#ov-auth');
  if (authEl) authEl.textContent = h.auth || '-';
  $('#ov-lane').textContent = h.lane || '-';
  $('#ov-up').textContent = h.upstream || '-';
  $('#ov-ver').textContent = h.version || '-';
  $('#ov-mid').textContent = h.deviceMid || '-';
  $('#ov-authmode').textContent = h.auth || '-';
  $('#ov-jwt').textContent = h.planJwt && h.planJwt.present ? ('ada (' + (h.planJwt.count || 1) + ' akun)') : 'tidak ada';
  if (n) {
    $('#ov-net').textContent = '↓ ' + fmtBps(n.total_in_bps) + ' · ↑ ' + fmtBps(n.total_out_bps);
    $('#ov-net2').textContent = (n.tcp_connections || 0) + ' koneksi TCP, ' + (n.tcp_established || 0) + ' established';
  }
  if (u && u.recent && u.recent.length) {
    const rows = u.recent.slice(-8).reverse();
    $('#ov-recent').innerHTML = '<h2>Request terakhir</h2><div class="tablewrap"><table><thead><tr><th>Waktu</th><th>Model</th><th class="num">Token</th><th class="num">t/s</th><th class="num">Status</th></tr></thead><tbody>' +
      rows.map(r => `<tr><td>${when(r.at)}</td><td>${esc(r.model)}</td><td class="num">${fmtN(r.output_tokens)}</td><td class="num">${fmtTps(r.tps)}</td><td class="num"><span class="badge ok">ok</span></td></tr>`).join('') +
      '</tbody></table></div>';
  }
  if (n && n.process) {
    const p = n.process;
    $('#ov-netcard').innerHTML = `<h2>Jaringan proses</h2>
      <div class="grid g3">
        <div class="stat"><span class="k">Baca</span><span class="v smallv">${fmtBps(p.read_bps)}</span><span class="s">${fmtBytes(p.read_bytes)} total</span></div>
        <div class="stat"><span class="k">Tulis</span><span class="v smallv">${fmtBps(p.write_bps)}</span><span class="s">${fmtBytes(p.write_bytes)} total</span></div>
        <div class="stat"><span class="k">CPU</span><span class="v smallv">${(p.cpu_percent || 0).toFixed(1)}%</span><span class="s">${p.found ? esc(p.name) : 'proses tidak ditemukan'}</span></div>
      </div>`;
  }
  if (u && u.rings) {
    const t = $('#sidetps'); if (t) t.textContent = fmtTps(u.rings.last_tps) + ' t/s';
    const r = $('#sidereq'); if (r) r.textContent = fmtN(u.today && u.today.requests);
  }
  if (n) { const e = $('#sidenet'); if (e) e.textContent = '↓' + fmtBps(n.total_in_bps) + ' ↑' + fmtBps(n.total_out_bps); }
}

function patchSideStats() {
  const u = ZB.usage, n = ZB.net;
  if (u && u.rings) {
    const t = $('#sidetps'); if (t) t.textContent = fmtTps(u.rings.last_tps) + ' t/s';
    const r = $('#sidereq'); if (r) r.textContent = fmtN(u.today && u.today.requests);
  }
  if (n) { const e = $('#sidenet'); if (e) e.textContent = '↓' + fmtBps(n.total_in_bps) + ' ↑' + fmtBps(n.total_out_bps); }
}

let accountFlow = null, accountTimer = null;

function accountBadges(a) {
  const out = [];
  if (a.default) out.push('<span class="badge ok">aktif</span>');
  if (a.expired) out.push('<span class="badge bad">kedaluwarsa</span>');
  if (a.needsReauth) out.push('<span class="badge bad">perlu login</span>');
  if (a.cooldownUntil) out.push('<span class="badge warn">jeda</span>');
  if (a.hasRefresh) out.push('<span class="badge">refresh</span>');
  return out.join('') || '<span class="badge">siap</span>';
}

function renderAccountCard() {
  const accounts = (ZB.health && ZB.health.accounts) || [];
  const box = $('#ov-accounts');
  if (!box) return;
  box.innerHTML = '<h2>Akun <span class="muted">ring plan lokal</span></h2>' +
    (accounts.length ? '<div class="acct-list">' + accounts.map(a => `
      <div class="acct-row">
        <div class="acct-main"><b>${esc(a.label || a.id)}</b><span class="small muted mono">${esc(a.id)}${a.email ? ' · ' + esc(a.email) : ''}</span>
          <span class="small muted">${a.expiresAt ? 'berakhir ' + when(a.expiresAt) : 'tanpa masa aktif'}</span></div>
        <div class="acct-badges">${accountBadges(a)}</div>
        <div class="acct-act">${a.default ? '' : `<button class="btn ghost sm" data-acc-use="${esc(a.id)}" type="button">Aktifkan</button>`}<button class="btn ghost sm danger" data-acc-del="${esc(a.id)}" data-acc-label="${esc(a.label || a.id)}" type="button">Hapus</button></div>
      </div>`).join('') + '</div>'
      : '<div class="empty">Belum ada akun tersimpan. Hubungkan akun Z.ai pertama.</div>') +
    `<form id="acc-add" class="row mt10"><input id="acc-label" class="grow" maxlength="40" placeholder="Label akun baru (mis. kerja)" autocomplete="off"><button class="btn" id="acc-addbtn" type="submit">Hubungkan akun</button></form>` +
    '<div id="acc-flow"></div>';

  bindAccountActions(box);
  bindAccountForm(box);
  renderAccountFlow();
}

function bindAccountActions(root) {
  root.querySelectorAll('[data-acc-use]').forEach(btn => {
    btn.onclick = async () => {
      btn.disabled = true;
      try {
        await api('/v1/accounts/switch', { method: 'POST', body: JSON.stringify({ id: btn.dataset.accUse }) });
        toast('Akun aktif diganti', 'ok');
        await refreshHealth();
        if (ZB.page === 'accounts') loadPool(true);
      } catch (e) { toast(e.message, 'bad'); btn.disabled = false; }
    };
  });
  root.querySelectorAll('[data-acc-del]').forEach(btn => {
    btn.onclick = async () => {
      if (!confirm('Hapus akun ' + btn.dataset.accLabel + '? Token plan ikut dihapus dari penyimpanan lokal.')) return;
      btn.disabled = true;
      try {
        await api('/v1/accounts/remove', { method: 'POST', body: JSON.stringify({ id: btn.dataset.accDel }) });
        toast('Akun dihapus', 'ok');
        await refreshHealth();
        if (ZB.page === 'accounts') loadPool(true);
      } catch (e) { toast(e.message, 'bad'); btn.disabled = false; }
    };
  });
}

function bindAccountForm(root) {
  const form = root.querySelector('#acc-add');
  if (!form) return;
  form.onsubmit = async ev => {
    ev.preventDefault();
    const label = ($('#acc-label').value || '').trim();
    const addBtn = $('#acc-addbtn');
    addBtn.disabled = true;
    try {
      const r = await api('/v1/accounts/login', { method: 'POST', body: JSON.stringify({ label }) });
      accountFlow = { id: r.flow_id, url: r.authorize_url, status: 'pending', error: '' };
      renderAccountFlow();
      const wait = Math.max(1200, Number(r.poll_interval_ms) || 2000);
      accountTimer = setInterval(pollAccountLogin, wait);
      pollAccountLogin();
    } catch (e) {
      toast(e.message, 'bad');
      addBtn.disabled = false;
    }
  };
}

/* pool akun: status + sisa kuota per model untuk SEMUA akun */
function poolErrText(e) {
  if (/empty plan list/i.test(e)) return 'tidak ada grant di plan akun ini';
  if (/status 429/i.test(e)) return 'billing sibuk (429), coba Segarkan';
  if (/status 5\d\d/i.test(e)) return 'billing error, coba Segarkan';
  if (/^balance:/i.test(e)) return e.replace(/^balance:\s*/i, 'billing: ');
  return e;
}

function prettyModel(m) {
  return String(m).replace(/^glm/i, 'GLM').replace(/-([a-z])/g, (_, c) => '-' + c.toUpperCase());
}

function poolUnits(n) {
  n = Number(n) || 0;
  if (n >= 1e9) return (n / 1e9).toFixed(1) + ' B';
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' M';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' K';
  return String(Math.round(n));
}

async function loadPool(force) {
  const box = $('#acc-pool');
  if (!box) return;
  if (force) box.setAttribute('aria-busy', 'true');
  try {
    ZB.pool = await api('/v1/accounts/pool' + (force ? '?force=1' : ''));
    renderPool();
  } catch (e) {
    box.innerHTML = `<div class="empty">${esc(e.message)}</div>`;
  }
}

function renderPool() {
  const box = $('#acc-pool');
  const p = ZB.pool;
  if (!box || !p) return;
  const accs = p.accounts || [], models = p.models || [];
  if (!accs.length) {
    box.innerHTML = `<div class="empty"><b>Belum ada akun</b><span class="empty-hint">Hubungkan akun Z.ai pertama lewat form di bawah.</span></div>`;
    return;
  }
  const cell = (a, model) => {
    if (a.error) return '<td class="num muted">—</td>';
    const rows = (a.rows || []).filter(r => r.model === model);
    if (!rows.length) return '<td class="num muted">—</td>';
    const rem = rows.reduce((s, r) => s + Number(r.remaining || 0), 0);
    const tot = rows.reduce((s, r) => s + Number(r.total || 0), 0);
    const used = rows.reduce((s, r) => s + Number(r.used || 0), 0);
    const wide = tot > 0 ? pct(used, tot) : 0;
    return `<td class="num" title="terpakai ${poolUnits(used)} dari ${poolUnits(tot)}">
      <span class="pool-rem">${poolUnits(rem)}</span><span class="pool-tot">/ ${poolUnits(tot)}</span>
      <span class="pool-bar"><i style="width:${wide.toFixed(1)}%"></i></span>
    </td>`;
  };
  box.innerHTML = `<div class="tablewrap pool-wrap"><table class="pool">
    <thead><tr><th>Akun</th><th>Status</th>${models.map(m => `<th class="num model" title="${esc(m)}">${esc(prettyModel(m))}</th>`).join('')}<th class="num">Aksi</th></tr></thead>
    <tbody>${accs.map(a => `<tr class="${a.default ? 'sel' : ''}">
      <td><div class="pool-name"><b>${esc(a.label || a.id)}</b><span class="small muted mono">${esc(a.email || a.id)}</span>${a.error ? `<span class="small pool-err" title="${esc(a.error)}">${esc(poolErrText(a.error))}</span>` : ''}</div></td>
      <td>${accountBadges(a)}</td>
      ${models.map(m => cell(a, m)).join('')}
      <td class="num"><div class="pool-act">${a.default ? '' : `<button class="btn ghost sm" data-acc-use="${esc(a.id)}" type="button">Aktifkan</button>`}<button class="btn ghost sm danger" data-acc-del="${esc(a.id)}" data-acc-label="${esc(a.label || a.id)}" type="button">Hapus</button></div></td>
    </tr>`).join('')}</tbody>
  </table></div>
  <p class="small muted pool-note">Kuota dibaca langsung dari billing tiap akun; cache 45 detik. Kolom model muncul otomatis dari plan yang terdeteksi.</p>`;
  bindAccountActions(box);
}

function pageAccounts(view) {
  view.innerHTML = skeleton('Akun', 'Pool akun ring plan: status tiap akun dan sisa kuota per model.') + `
  <div class="card">
    <h2>Akun di pool <span class="muted">kuota per model</span><span class="spacer"></span><button class="btn ghost sm" id="pool-refresh" type="button">Segarkan</button></h2>
    <div id="acc-pool" aria-live="polite"><div class="empty">Membaca kuota semua akun…</div></div>
  </div>
  <div class="card mt14">
    <h2>Hubungkan akun</h2>
    <form id="acc-add" class="row"><input id="acc-label" class="grow" maxlength="40" placeholder="Label akun baru (mis. kerja)" autocomplete="off"><button class="btn primary" id="acc-addbtn" type="submit">Hubungkan akun</button></form>
    <div id="acc-flow"></div>
  </div>`;
  $('#pool-refresh').onclick = () => loadPool(true);
  bindAccountForm(view);
  renderAccountFlow();
  loadPool(false);
}

function renderAccountFlow() {
  const el = $('#acc-flow');
  if (!el) return;
  if (!accountFlow) { el.innerHTML = ''; return; }
  if (accountFlow.status === 'pending') {
    el.innerHTML = `<div class="flow"><div class="small muted">Setujui tautan ini di browser yang sudah login ke Z.ai:</div>
      <div class="row mt6"><a class="btn sm" href="${esc(accountFlow.url)}" target="_blank" rel="noopener">Buka halaman izin</a><button class="btn ghost sm" id="accflow-copy" type="button">Salin tautan</button><button class="btn ghost sm" id="accflow-cancel" type="button">Batal</button></div>
      <div id="accflow-stat" class="mt6">Menunggu persetujuan...</div></div>`;
    const copy = $('#accflow-copy');
    if (copy) copy.onclick = () => copyText(accountFlow.url);
    const cancel = $('#accflow-cancel');
    if (cancel) cancel.onclick = cancelAccountLogin;
  } else {
    el.innerHTML = `<div class="flow"><div class="small ${accountFlow.status === 'ok' ? 'muted' : 'danger'}">${esc(accountFlow.error || (accountFlow.status === 'ok' ? 'Akun terhubung.' : 'Login dibatalkan.'))}</div><div class="row mt6"><button class="btn ghost sm" id="accflow-close" type="button">Tutup</button></div></div>`;
    const close = $('#accflow-close');
    if (close) close.onclick = () => { accountFlow = null; renderAccountFlow(); };
  }
}

async function pollAccountLogin() {
  if (!accountFlow || accountFlow.status !== 'pending') return;
  try {
    const s = await api('/v1/accounts/login/status?flow_id=' + encodeURIComponent(accountFlow.id));
    if (s.status === 'ok') {
      clearInterval(accountTimer); accountTimer = null;
      accountFlow = { status: 'ok', error: '' };
      renderAccountFlow();
      toast('Akun berhasil terhubung', 'ok');
      await refreshHealth();
    } else if (s.status === 'error' || s.status === 'cancelled') {
      clearInterval(accountTimer); accountTimer = null;
      accountFlow = { status: s.status, error: s.error || 'Login gagal.' };
      renderAccountFlow();
      const addBtn = $('#acc-addbtn'); if (addBtn) addBtn.disabled = false;
    }
  } catch (e) {
    accountFlow.error = e.message;
    const stat = $('#accflow-stat'); if (stat) stat.textContent = e.message;
  }
}

async function cancelAccountLogin() {
  if (!accountFlow) return;
  clearInterval(accountTimer); accountTimer = null;
  try { await api('/v1/accounts/login/cancel', { method: 'POST', body: JSON.stringify({ flow_id: accountFlow.id }) }); } catch (e) {}
  accountFlow = { status: 'cancelled', error: 'Login dibatalkan.' };
  renderAccountFlow();
  const addBtn = $('#acc-addbtn'); if (addBtn) addBtn.disabled = false;
}

async function loadOverviewExtras() {
  api('/v1/quota').then(q => {
    ZB.quota = q;
    const box = $('#ov-quota'); if (!box) return;
    const rows = q.rows || [];
    if (!rows.length) {
      box.innerHTML = `<h2>Kuota plan <span class="muted">${esc(q.source || '')}</span></h2><div class="empty">${esc((q.errors && q.errors[0]) || 'Tidak ada data kuota.')}</div>`;
      return;
    }
    box.innerHTML = `<h2>Kuota plan <span class="muted">akun ${esc(q.account || '-')} · ${esc(q.source || '')}</span></h2>` + rows.slice(0, 3).map(r => {
      const total = Number(r.total_units) || 0, used = Number(r.used_units) || 0, rem = Number(r.remaining_units) || (total - used);
      return `<div class="gapq">
        <div class="row"><b>${esc(r.model || r.plan_name || 'plan')}</b><span class="badge">${esc(r.plan_name || r.plan || '')}</span><span class="spacer"></span><span class="muted small">sisa ${fmtN(rem)}</span></div>
        <div class="bar gapbar"><i style="width:${pct(used, total)}%"></i></div>
        <div class="row small muted"><span>${fmtN(used)} / ${fmtN(total)} unit</span><span class="spacer"></span><span>${r.ends_at ? ('berakhir ' + when(r.ends_at)) : ''}</span></div>
      </div>`;
    }).join('');
  }).catch(e => {
    const box = $('#ov-quota'); if (box) box.innerHTML = `<h2>Kuota plan</h2><div class="empty">${esc(e.message)}</div>`;
  });

  renderAccountCard();

  api('/v1/claim').then(c => {
    const el = $('#ov-claim'); if (!el) return;
    const plans = c.plans || c.candidates || [];
    el.innerHTML = `<h2>Klaim plan <span class="muted">weekend / trial</span></h2>` +
      (plans.length ? `<div class="tablewrap"><table><thead><tr><th>Plan</th><th>Status</th><th>Mulai</th><th>Selesai</th></tr></thead><tbody>` +
        plans.slice(0, 4).map(p => `<tr><td>${esc(p.name || p.plan || p.id || 'plan')}</td><td>${esc(p.status || '-')}</td><td>${when(p.starts_at || p.start)}</td><td>${when(p.ends_at || p.end)}</td></tr>`).join('') + '</tbody></table></div>'
        : `<div class="empty">${esc(c.message || 'Tidak ada plan yang bisa diklaim.')}</div>`) +
      `<div class="row mt10"><a class="btn ghost sm" href="#/claim">Buka halaman klaim</a></div>`;
  }).catch(() => { const el = $('#ov-claim'); if (el) el.innerHTML = '<h2>Klaim plan</h2><div class="empty">Butuh plan JWT / device MID.</div>'; });
}

/* ---------------- usage ---------------- */
function pageUsage(view) {
  view.innerHTML = skeleton('Penggunaan', 'Token, kecepatan, dan tren per model.') + `
  <div class="toolbar">
    <div class="tabs" id="urange">
      ${['1h', '6h', '24h', '7d'].map(r => `<button type="button" data-r="${r}" class="${ZB.range === r ? 'active' : ''}">${r === '7d' ? '7 hari' : r}</button>`).join('')}
    </div>
    <span class="spacer"></span>
    <button class="btn ghost sm" id="urefresh" type="button">Muat ulang</button>
  </div>
  <div class="ringgrid">
    <div class="ring"><div class="k">Terakhir</div><div class="v" id="ur-last">-</div></div>
    <div class="ring"><div class="k">Tertinggi</div><div class="v" id="ur-high">-</div></div>
    <div class="ring"><div class="k">Rata-rata 7 hari</div><div class="v" id="ur-avg">-</div></div>
  </div>
  <div class="grid g2 mt14">
    <div class="card"><h2>Hari ini</h2><div class="grid g2" id="u-today"></div></div>
    <div class="card"><h2>Sepanjang waktu</h2><div class="grid g2" id="u-all"></div></div>
  </div>
  <div class="card mt14"><h2>Kecepatan <span class="muted" id="u-range-label"></span></h2>
    <div class="chartbox"><svg class="chart" id="uchart" preserveAspectRatio="none"></svg></div>
    <div class="legend"><span><i class="tps"></i>t/s</span><span><i class="out"></i>token keluar</span></div>
  </div>
  <div class="grid g2 mt14">
    <div class="card"><h2>Per model <span class="muted">hari ini</span></h2><div id="umodels"></div></div>
    <div class="card"><h2>Aplikasi ZCode <span class="muted" id="udnote"></span></h2><div id="udesktop"></div></div>
  </div>`;
  $('#urange').onclick = e => {
    const b = e.target.closest('button[data-r]'); if (!b) return;
    ZB.range = b.dataset.r;
    $$('#urange button').forEach(x => x.classList.toggle('active', x === b));
    drawUsageChart();
  };
  $('#urefresh').onclick = () => loadUsage(true);
  renderUsage();
}

async function loadUsage(force) {
  if (ZB.usage && !force) return;
  try { ZB.usage = await api('/v1/usage'); } catch (e) { toast(e.message, 'bad'); }
}

function renderUsage() {
  const u = ZB.usage;
  if (!u) return;
  const set = (id, html) => { const el = document.getElementById(id); if (el) el.innerHTML = html; };
  set('ur-last', fmtTps(u.rings.last_tps) + ' <small>t/s</small>');
  set('ur-high', fmtTps(u.rings.high_tps) + ' <small>t/s</small>');
  set('ur-avg', fmtTps(u.rings.seven_day_avg_tps) + ' <small>t/s</small>');
  const statBlocks = st => `
    <div class="stat"><span class="k">Request</span><span class="v">${fmtN(st.requests)}</span></div>
    <div class="stat"><span class="k">Token keluar</span><span class="v">${fmtN(st.output_tokens)}</span></div>
    <div class="stat"><span class="k">Token masuk</span><span class="v">${fmtN(st.input_tokens)}</span></div>
    <div class="stat"><span class="k">Cache</span><span class="v">${fmtN(st.cache_read_tokens)}</span></div>
    <div class="stat"><span class="k">Rata-rata</span><span class="v">${fmtTps(st.avg_tps)} t/s</span></div>
    <div class="stat"><span class="k">Puncak</span><span class="v">${fmtTps(st.high_tps)} t/s</span></div>`;
  set('u-today', statBlocks(u.today || {}));
  set('u-all', statBlocks(u.all_time || {}));
  const pm = (u.today && u.today.per_model) || {};
  const models = Object.entries(pm).sort((a, b) => b[1].requests - a[1].requests);
  set('umodels', models.length ? `<div class="tablewrap"><table><thead><tr><th>Model</th><th class="num">Req</th><th class="num">Keluar</th><th class="num">t/s</th></tr></thead><tbody>${models.map(([m, s]) => `<tr><td>${esc(m)}</td><td class="num">${fmtN(s.requests)}</td><td class="num">${fmtN(s.output_tokens)}</td><td class="num">${fmtTps(s.avg_tps)}</td></tr>`).join('')}</tbody></table></div>` : '<div class="empty">Belum ada request hari ini.</div>');
  drawUsageChart();
  renderDesktop(u.desktop);
}

function drawUsageChart() {
  const u = ZB.usage; if (!u) return;
  const svg = document.getElementById('uchart'); if (!svg) return;
  const label = document.getElementById('u-range-label');
  if (label) label.textContent = '· ' + ZB.range;
  const now = Date.now();
  const span = { '1h': 36e5, '6h': 216e5, '24h': 864e5, '7d': 6048e5 }[ZB.range] || 864e5;
  const rows = (u.recent || []).filter(r => now - new Date(r.at).getTime() <= span);
  const W = 640, H = 180, pad = 8;
  if (!rows.length) {
    svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
    svg.innerHTML = `<text x="${W / 2}" y="${H / 2}" text-anchor="middle" fill="var(--muted)" font-size="12">Belum ada request pada rentang ini</text>`;
    return;
  }
  let maxTps = 1, maxTok = 1;
  rows.forEach(r => { maxTps = Math.max(maxTps, r.tps || 0); maxTok = Math.max(maxTok, r.output_tokens || 0); });
  const t0 = Math.min(...rows.map(r => new Date(r.at).getTime()));
  const t1 = Math.max(...rows.map(r => new Date(r.at).getTime()), t0 + 1);
  const x = t => pad + (W - pad * 2) * ((t - t0) / (t1 - t0 || 1));
  const yTps = v => H - pad - (H - pad * 2) * ((v || 0) / maxTps);
  const yTok = v => H - pad - (H - pad * 2) * ((v || 0) / maxTok);
  const tpsPts = rows.map(r => `${x(new Date(r.at).getTime()).toFixed(1)},${yTps(r.tps).toFixed(1)}`).join(' ');
  const tokPts = rows.map(r => `${x(new Date(r.at).getTime()).toFixed(1)},${yTok(r.output_tokens).toFixed(1)}`).join(' ');
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  svg.innerHTML = `<polyline points="${tokPts}" fill="none" stroke="var(--muted)" stroke-width="1.4" stroke-dasharray="3 3" opacity=".7"/>` +
    `<polyline points="${tpsPts}" fill="none" stroke="var(--accent)" stroke-width="2.2" stroke-linejoin="round"/>` +
    rows.map(r => `<circle cx="${x(new Date(r.at).getTime())}" cy="${yTps(r.tps)}" r="1.8" fill="var(--accent)"><title>${esc(r.model)} · ${fmtTps(r.tps)} t/s · ${fmtN(r.output_tokens)} token</title></circle>`).join('') +
    `<text x="${pad}" y="12" fill="var(--muted)" font-size="10">${fmtTps(maxTps)} t/s</text>`;
}

function renderDesktop(d) {
  const note = document.getElementById('udnote'), box = document.getElementById('udesktop');
  if (!box) return;
  if (!d || !d.available) {
    if (note) note.textContent = 'tidak tersedia';
    box.innerHTML = `<div class="empty">${esc((d && d.error) || 'Database aplikasi tidak terbaca.')}</div>`;
    return;
  }
  const s = d.snapshot || {};
  if (note) note.textContent = s.in_flight ? (s.in_flight + ' in-flight') : '';
  box.innerHTML = `<div class="grid g2">
    <div class="stat"><span class="k">Request hari ini</span><span class="v">${fmtN(s.today && s.today.requests)}</span></div>
    <div class="stat"><span class="k">Token hari ini</span><span class="v">${fmtN(s.today && s.today.tokens)}</span></div>
    <div class="stat"><span class="k">Rata-rata 7h</span><span class="v">${fmtTps(s.avg_7d)} t/s</span></div>
    <div class="stat"><span class="k">Puncak 7h</span><span class="v">${fmtTps(s.high_7d)} t/s</span></div>
    <div class="stat"><span class="k">Sesi</span><span class="v">${fmtN(s.sessions)}</span></div>
    <div class="stat"><span class="k">Aktivitas</span><span class="v smallv">${when(s.last_activity ? s.last_activity * 1000 : 0)}</span></div>
  </div><p class="small muted u-path">${esc(d.path || '')}</p>`;
}

/* ---------------- inspector ---------------- */
const insState = { list: [], sel: null, q: '', method: '', status: '' };

function pageInspector(view) {
  view.innerHTML = skeleton('Inspector', 'Rekaman request/response lokal, replay, dan ekspor HAR.') + `
  <div class="toolbar">
    <input class="grow" id="iq" placeholder="Cari model, path, akun…" value="${esc(insState.q)}">
    <select id="imethod"><option value="">Semua metode</option>${['GET', 'POST'].map(m => `<option>${m}</option>`).join('')}</select>
    <select id="istatus"><option value="">Semua status</option><option value="ok">2xx</option><option value="err">non-2xx</option></select>
    <button class="btn ghost sm" id="irefresh" type="button">Muat ulang</button>
    <a class="btn ghost sm" id="ihar" href="/inspect?format=har" target="_blank" rel="noopener">Ekspor HAR</a>
  </div>
  <div class="split">
    <div class="card flat flat-card"><div class="tablewrap table-tall"><table id="itable"><thead><tr><th>Waktu</th><th>Metode</th><th>Rute</th><th>Model</th><th class="num">Status</th><th class="num">Durasi</th></tr></thead><tbody><tr><td colspan="6" class="empty">Memuat…</td></tr></tbody></table></div></div>
    <div class="card" id="idetail"><div class="empty">Pilih satu baris untuk melihat detail.</div></div>
  </div>`;
  $('#iq').oninput = e => { insState.q = e.target.value; renderInspectRows(); };
  $('#imethod').onchange = e => { insState.method = e.target.value; renderInspectRows(); };
  $('#istatus').onchange = e => { insState.status = e.target.value; renderInspectRows(); };
  $('#irefresh').onclick = () => loadInspect(true);
  loadInspect(true);
}

async function loadInspect(force) {
  if (insState.list.length && !force) return;
  try {
    const r = await api('/inspect?limit=200');
    insState.list = r.entries || [];
    if (!r.enabled) {
      const tb = $('#itable tbody');
      if (tb) tb.innerHTML = `<tr><td colspan="6" class="empty">${esc(r.hint || 'Capture nonaktif.')}</td></tr>`;
      return;
    }
    renderInspectRows();
  } catch (e) {
    const tb = $('#itable tbody'); if (tb) tb.innerHTML = `<tr><td colspan="6" class="empty">${esc(e.message)}</td></tr>`;
  }
}

function renderInspectRows() {
  const tb = $('#itable tbody'); if (!tb) return;
  const q = insState.q.toLowerCase();
  const rows = insState.list.filter(r => {
    if (insState.method && r.method !== insState.method) return false;
    if (insState.status === 'ok' && (r.status < 200 || r.status >= 300)) return false;
    if (insState.status === 'err' && r.status >= 200 && r.status < 300) return false;
    if (!q) return true;
    return [r.model, r.path, r.route, r.account, String(r.status)].some(v => String(v || '').toLowerCase().includes(q));
  });
  if (!rows.length) { tb.innerHTML = '<tr><td colspan="6" class="empty">Tidak ada hasil.</td></tr>'; return; }
  tb.innerHTML = rows.map(r => `<tr class="clickable ${insState.sel === r.id ? 'sel' : ''}" data-id="${r.id}">
    <td>${when(r.started_at)}</td><td>${esc(r.method)}</td><td>${esc(r.route || r.path)}</td><td>${esc(r.model || '-')}</td>
    <td class="num"><span class="badge ${r.status >= 200 && r.status < 300 ? 'ok' : 'bad'}">${r.status}</span></td>
    <td class="num">${r.duration_ms} ms</td></tr>`).join('');
  tb.onclick = e => {
    const tr = e.target.closest('tr[data-id]'); if (!tr) return;
    insState.sel = Number(tr.dataset.id);
    $$('#itable tbody tr').forEach(x => x.classList.toggle('sel', x === tr));
    loadInspectDetail(insState.sel);
  };
}

async function loadInspectDetail(id) {
  const box = $('#idetail'); if (!box) return;
  box.innerHTML = '<div class="empty">Memuat detail…</div>';
  try {
    const ex = await api('/inspect?id=' + id);
    box.innerHTML = `
      <div class="detail-head"><b>#${ex.id}</b><span class="badge ${ex.status >= 200 && ex.status < 300 ? 'ok' : 'bad'}">${ex.status}</span>
        <span class="badge">${esc(ex.route || ex.path)}</span><span class="spacer"></span>
        <button class="btn sm" id="ireplay" type="button">Replay</button>
        <button class="btn sm ghost danger" id="idelete" type="button">Hapus</button></div>
      <dl class="kv">
        <dt>Waktu</dt><dd>${when(ex.started_at)}</dd>
        <dt>Model</dt><dd>${esc(ex.model || '-')}</dd>
        <dt>Akun</dt><dd>${esc(ex.account || '-')}</dd>
        <dt>Durasi</dt><dd>${ex.duration_ms} ms${ex.first_token_ms ? ' · TTFT ' + ex.first_token_ms + ' ms' : ''}</dd>
        <dt>Ukuran</dt><dd>${fmtBytes(ex.request_bytes)} → ${fmtBytes(ex.response_bytes)}${ex.truncated ? ' (dipotong)' : ''}</dd>
        <dt>Stream</dt><dd>${ex.stream ? 'ya' : 'tidak'}</dd>
      </dl>
      <h3 class="detail-h">Request</h3><pre class="body">${esc(ex.request_body || '')}</pre>
      <h3>Response</h3><pre class="body">${esc(ex.response_body || '')}</pre>`;
    $('#ireplay').onclick = async () => {
      try { await api('/inspect/replay', { method: 'POST', body: JSON.stringify({ id: ex.id }) }); toast('Replay terkirim', 'ok'); }
      catch (e) { toast(e.message, 'bad'); }
    };
    $('#idelete').onclick = async () => {
      try { await api('/inspect?id=' + ex.id, { method: 'DELETE' }); toast('Entri dihapus', 'ok'); insState.sel = null; loadInspect(true); $('#idetail').innerHTML = '<div class="empty">Pilih satu baris untuk melihat detail.</div>'; }
      catch (e) { toast(e.message, 'bad'); }
    };
  } catch (e) { box.innerHTML = `<div class="empty">${esc(e.message)}</div>`; }
}

/* ---------------- network ---------------- */
function pageNetwork(view) {
  view.innerHTML = skeleton('Jaringan', 'Lalu lintas adapter, koneksi TCP, dan proses ZCode.') + `
  <div class="grid g4" id="ntiles"></div>
  <div class="grid g2 mt14">
    <div class="card"><h2>Adapter</h2><div id="nifs"></div></div>
    <div class="card"><h2>Proses ZCode</h2><div id="nproc"></div></div>
  </div>`;
  renderNetwork();
}

function renderNetwork() {
  const n = ZB.net;
  const tiles = document.getElementById('ntiles');
  if (!tiles) return;
  if (!n) { tiles.innerHTML = '<div class="card empty">Menunggu sampel jaringan…</div>'; return; }
  tiles.innerHTML = `
    <div class="card stat"><span class="k">Unduh</span><span class="v">${fmtBps(n.total_in_bps)}</span><span class="s">semua adapter</span></div>
    <div class="card stat"><span class="k">Unggah</span><span class="v">${fmtBps(n.total_out_bps)}</span><span class="s">semua adapter</span></div>
    <div class="card stat"><span class="k">TCP</span><span class="v">${n.tcp_connections || 0}</span><span class="s">${n.tcp_established || 0} established</span></div>
    <div class="card stat"><span class="k">Status</span><span class="v smallv">${n.supported ? 'aktif' : 'tidak didukung'}</span><span class="s">${esc(n.error || 'sampler lokal')}</span></div>`;
  const ifs = (n.interfaces || []).slice().sort((a, b) => (b.in_bps + b.out_bps) - (a.in_bps + a.out_bps));
  document.getElementById('nifs').innerHTML = ifs.length ? `<div class="tablewrap"><table><thead><tr><th>Adapter</th><th class="num">Turun</th><th class="num">Naik</th><th class="num">Total</th></tr></thead><tbody>${ifs.map(i => `<tr><td>${esc(i.description || i.name)} ${i.up ? '<span class="badge ok">up</span>' : '<span class="badge bad">down</span>'}</td><td class="num">${fmtBps(i.in_bps)}</td><td class="num">${fmtBps(i.out_bps)}</td><td class="num">${fmtBytes(i.in_bytes + i.out_bytes)}</td></tr>`).join('')}</tbody></table></div>` : '<div class="empty">Tidak ada adapter.</div>';
  const p = n.process;
  document.getElementById('nproc').innerHTML = p && p.found ? `<div class="grid g2">
      <div class="stat"><span class="k">Baca</span><span class="v">${fmtBps(p.read_bps)}</span><span class="s">${fmtBytes(p.read_bytes)}</span></div>
      <div class="stat"><span class="k">Tulis</span><span class="v">${fmtBps(p.write_bps)}</span><span class="s">${fmtBytes(p.write_bytes)}</span></div>
      <div class="stat"><span class="k">CPU</span><span class="v">${(p.cpu_percent || 0).toFixed(1)}%</span></div>
      <div class="stat"><span class="k">PID</span><span class="v smallv">${(p.pids || []).join(', ') || '-'}</span><span class="s">${esc(p.name || '')}</span></div>
    </div><p class="small muted u-calib">Kalibrasi ${(p.calibration || 1).toFixed(2)}×</p>`
    : '<div class="empty">Proses ZCode tidak ditemukan.</div>';
}

/* ---------------- claim ---------------- */
function pageClaim(view) {
  view.innerHTML = skeleton('Klaim plan', 'Cek plan weekend/trial yang tersedia lalu klaim.') + `
  <div class="toolbar"><button class="btn" id="cscan" type="button">Cek plan</button><button class="btn primary" id="cnow" type="button">Klaim sekarang</button><span class="small muted" id="cstat"></span></div>
  <div class="grid g3">
    <div class="card stat"><span class="k">Plan terdeteksi</span><span class="v smallv" id="csum-plans">—</span><span class="s">dari pemindaian terakhir</span></div>
    <div class="card stat"><span class="k">Pemindaian terakhir</span><span class="v smallv" id="csum-scan">—</span><span class="s">waktu lokal</span></div>
    <div class="card stat"><span class="k">Riwayat klaim</span><span class="v smallv" id="csum-hist">—</span><span class="s">percobaan tersimpan</span></div>
  </div>
  <div class="card mt14" id="cbody"><div class="empty"><b>Belum ada plan aktif</b><span class="empty-hint">Tekan “Cek plan” untuk memindai penawaran weekend/trial.</span></div></div>
  <div class="card mt14"><h2>Riwayat klaim</h2><div id="chist"><div class="empty">Memuat…</div></div></div>`;
  $('#cscan').onclick = () => scanClaim();
  $('#cnow').onclick = () => claimNow();
  scanClaim();
}

const CLAIM_CAPTCHA_SDK = 'https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js';
let claimCaptchaScript = null;

function loadClaimCaptchaSdk() {
  if (claimCaptchaScript) return claimCaptchaScript;
  claimCaptchaScript = new Promise((resolve, reject) => {
    if (window.initAliyunCaptcha) { resolve(); return; }
    const s = document.createElement('script');
    s.src = CLAIM_CAPTCHA_SDK;
    s.async = true;
    s.onload = () => resolve();
    s.onerror = () => {
      claimCaptchaScript = null;
      reject(new Error(ZB.lang === 'en' ? 'Captcha SDK failed to load' : 'Gagal memuat SDK captcha'));
    };
    document.head.appendChild(s);
  });
  return claimCaptchaScript;
}

function solveClaimCaptcha() {
  return api('/v1/claim/verification-config').then(cfgRes => {
    const cfg = cfgRes.config || {};
    if (!cfg.enabled || !cfg.region || !cfg.prefix || !cfg.scene_id) {
      throw new Error(ZB.lang === 'en' ? 'Claim needs captcha: open the ZCode app and press Claim' : 'Klaim butuh captcha: buka app ZCode lalu tekan Claim');
    }
    return loadClaimCaptchaSdk().then(() => new Promise((resolve, reject) => {
      const wrap = document.createElement('div');
      wrap.style.cssText = 'position:fixed;inset:0;z-index:9999;display:flex;align-items:center;justify-content:center;background:rgba(0,0,0,.55)';
      wrap.innerHTML = '<div style="background:var(--panel,#12161c);color:var(--fg,#e6edf3);border:1px solid var(--line,#2a3038);border-radius:14px;padding:20px 22px;max-width:360px;text-align:center">' +
        '<div style="font-weight:600;margin-bottom:6px">' + (ZB.lang === 'en' ? 'Security check' : 'Verifikasi keamanan') + '</div>' +
        '<p class="small muted" style="margin:0 0 14px">' + (ZB.lang === 'en' ? 'Solve the captcha to claim this plan.' : 'Selesaikan captcha untuk mengklaim plan ini.') + '</p>' +
        '<div id="zbCaptcha"></div>' +
        '<button id="zbCaptchaBtn" class="btn primary" type="button">' + (ZB.lang === 'en' ? 'Verify' : 'Verifikasi') + '</button> ' +
        '<button id="zbCaptchaCancel" class="btn ghost" type="button">' + (ZB.lang === 'en' ? 'Cancel' : 'Batal') + '</button>' +
        '</div>';
      document.body.appendChild(wrap);
      const done = v => { wrap.remove(); resolve({ param: v, region: cfg.region }); };
      const fail = e => { wrap.remove(); reject(new Error(typeof e === 'string' && e ? e : (ZB.lang === 'en' ? 'Captcha failed' : 'Captcha gagal'))); };
      wrap.querySelector('#zbCaptchaCancel').onclick = () => fail('');
      try {
        window.AliyunCaptchaConfig = { region: cfg.region, prefix: cfg.prefix };
        window.initAliyunCaptcha({
          SceneId: cfg.scene_id,
          region: cfg.region,
          prefix: cfg.prefix,
          mode: 'popup',
          language: ZB.lang === 'en' ? 'en' : 'cn',
          showErrorTip: true,
          element: '#zbCaptcha',
          button: '#zbCaptchaBtn',
          getInstance: inst => { try { if (inst && typeof inst.show === 'function') inst.show(); } catch (_) {} },
          success: param => { const p = typeof param === 'string' ? param.trim() : ''; p ? done(p) : fail(''); },
          fail: fail,
        });
      } catch (e) {
        fail(e && e.message);
      }
    }));
  });
}

async function claimNow() {
  const btn = $('#cnow');
  if (btn) btn.disabled = true;
  try {
    try {
      const r = await api('/v1/claim', { method: 'POST', body: '{}' });
      toast(r.message || (ZB.lang === 'en' ? 'Claim request sent' : 'Permintaan klaim dikirim'), 'ok');
      scanClaim();
      return;
    } catch (e) {
      if (!/captcha|3007/i.test(e.message)) throw e;
    }
    const solved = await solveClaimCaptcha();
    const r = await api('/v1/claim', { method: 'POST', body: JSON.stringify({ captcha_param: solved.param, captcha_region: solved.region }) });
    toast(r.message || (ZB.lang === 'en' ? 'Plan claimed' : 'Plan berhasil diklaim'), 'ok');
    scanClaim();
  } catch (e) {
    toast(e.message, 'bad');
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function scanClaim() {
  const body = $('#cbody'), st = $('#cstat');
  if (!body) return;
  body.innerHTML = '<div class="empty">Memindai…</div>';
  try {
    const c = await api('/v1/claim');
    const plans = c.plans || c.candidates || [];
    if (st) st.textContent = c.message || '';
    const cnt = $('#csum-plans'), scan = $('#csum-scan');
    if (cnt) cnt.textContent = plans.length;
    if (scan) scan.textContent = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    body.innerHTML = plans.length ? `<div class="tablewrap"><table><thead><tr><th>Plan</th><th>Status</th><th>Mulai</th><th>Selesai</th><th>Unit</th></tr></thead><tbody>${plans.map(p => `<tr><td>${esc(p.name || p.plan_name || p.plan || p.id || '-')}</td><td>${esc(p.status || '-')}</td><td>${when(p.starts_at || p.start)}</td><td>${when(p.ends_at || p.end)}</td><td>${esc(p.units || p.total_units || '-')}</td></tr>`).join('')}</tbody></table></div>` : `<div class="empty"><b>Tidak ada plan yang bisa diklaim</b><span class="empty-hint">${esc(c.message || 'Coba pindai lagi nanti; penawaran biasanya muncul akhir pekan.')}</span></div>`;
    if (!plans.length) renderClaimPoolHint(body, c.message);
  } catch (e) { body.innerHTML = `<div class="empty">${esc(e.message)}</div>`; }
  try {
    const h = await api('/v1/claim/history');
    const list = h.claims || h.entries || [];
    const hc = $('#csum-hist'); if (hc) hc.textContent = list.length;
    const box = $('#chist');
    if (box) box.innerHTML = list.length ? `<div class="tablewrap"><table><thead><tr><th>Waktu</th><th>Plan</th><th>Status</th></tr></thead><tbody>${list.map(x => `<tr><td>${when(x.at || x.created_at)}</td><td>${esc(x.plan || x.plan_id || '-')}</td><td>${esc(x.status || x.result || '-')}${x.message ? ' · ' + esc(x.message) : ''}</td></tr>`).join('')}</tbody></table></div>` : '<div class="empty">Belum ada riwayat.</div>';
  } catch (e) {
    const box = $('#chist'); if (box) box.innerHTML = `<div class="empty">${esc(e.message)}</div>`;
  }
}

async function renderClaimPoolHint(body, msg) {
  try {
    const p = await api('/v1/accounts/pool');
    const others = (p.accounts || []).filter(a => !a.default && (a.rows || []).length && !a.error);
    if (!others.length || !body) return;
    const fmt = n => Number(n || 0).toLocaleString('id-ID');
    const rows = others.map(a => `<tr><td><code>${esc(a.id)}</code></td><td>${(a.rows || []).map(r => `${esc(r.model)} · ${esc(r.plan_name || '')} · ${fmt(r.remaining)} / ${fmt(r.total)}`).join('<br>')}</td><td><button class="btn sm" type="button" data-acct="${esc(a.id)}">${ZB.lang === 'en' ? 'Use account' : 'Pakai akun'}</button></td></tr>`).join('');
    body.innerHTML = `<div class="empty"><b>Akun aktif tidak punya plan</b><span class="empty-hint">${esc(msg || 'Tapi akun lain punya plan aktif:')}</span></div>
      <div class="tablewrap"><table><thead><tr><th>Akun</th><th>Plan</th><th></th></tr></thead><tbody>${rows}</tbody></table></div>`;
    body.querySelectorAll('button[data-acct]').forEach(btn => {
      btn.onclick = async () => {
        try {
          await api('/v1/accounts/switch', { method: 'POST', body: JSON.stringify({ id: btn.dataset.acct }) });
          toast(ZB.lang === 'en' ? 'Account switched' : 'Akun diganti', 'ok');
          scanClaim();
        } catch (e) { toast(e.message, 'bad'); }
      };
    });
  } catch (_) { /* pool is best-effort */ }
}

/* ---------------- sharing ---------------- */
const shareState = { sort: 'new', posts: [], image: '' };
const shareOpen = new Set();
const SHARE_ICON_LIKE = '<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><path d="M5 7.2 7.6 2c.5 0 .9.4.9.9v2.6h3.3c.9 0 1.5.8 1.3 1.6l-1 4.4c-.1.6-.7 1-1.3 1H5M5 7.2H2.5v5.5H5z"/></svg>';
const SHARE_ICON_COMMENT = '<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><path d="M2.5 3.2h11v7.1h-6L4.3 13v-2.7H2.5z"/></svg>';

function shareClient() {
  let id = localStorage.getItem('zbShareClient');
  if (!id) {
    id = (window.crypto && crypto.randomUUID) ? crypto.randomUUID() : 'c' + Date.now().toString(36) + Math.random().toString(36).slice(2, 10);
    localStorage.setItem('zbShareClient', id);
  }
  return id;
}

function shareName() {
  return (localStorage.getItem('zbShareName') || '').trim() || 'anon';
}

function shareAgo(ms) {
  const s = Math.max(0, Math.round((Date.now() - (Number(ms) || 0)) / 1000));
  if (s < 60) return ZB.lang === 'en' ? 'just now' : 'baru saja';
  const units = ZB.lang === 'en'
    ? [[86400, 'd'], [3600, 'h'], [60, 'm']]
    : [[86400, 'hari'], [3600, 'jam'], [60, 'menit']];
  for (const [size, label] of units) {
    if (s >= size) {
      const n = Math.floor(s / size);
      return ZB.lang === 'en' ? n + label + ' ago' : n + ' ' + label + ' lalu';
    }
  }
  return ZB.lang === 'en' ? 'just now' : 'baru saja';
}

function shareCount(n) {
  if (ZB.lang === 'en') return n + ' post' + (n === 1 ? '' : 's');
  return n + ' post';
}

function sharePostHTML(p) {
  const open = shareOpen.has(p.id);
  const image = p.image ? `<img class="share-img" src="${esc(p.image)}" alt="${esc(tr('Lampiran'))}: ${esc(p.title)}" width="960" height="540" loading="lazy">` : '';
  const link = p.link ? `<a class="share-link" href="${esc(p.link)}" target="_blank" rel="noopener noreferrer">${esc(p.link)}</a>` : '';
  const comments = p.comments || [];
  const body = comments.map(c => `<div class="share-comment">
      <div class="share-comment-meta"><b class="share-comment-author">${esc(c.author)}</b><span aria-hidden="true">·</span><time>${esc(shareAgo(c.createdAt))}</time></div>
      <p class="share-comment-body">${esc(c.body)}</p>
    </div>`).join('') || `<p class="small muted">${esc(tr('Belum ada komentar.'))}</p>`;
  return `<article class="share-post" data-id="${esc(p.id)}">
    <div class="share-post-head">
      <h3 class="share-title">${esc(p.title)}</h3>
      ${p.mine ? `<button class="btn ghost sm danger" data-del="${esc(p.id)}" type="button">${esc(tr('Hapus'))}</button>` : ''}
    </div>
    <div class="share-meta"><span class="share-author">${esc(p.author)}</span><span aria-hidden="true">·</span><time datetime="${new Date(p.createdAt).toISOString()}">${esc(shareAgo(p.createdAt))}</time></div>
    ${p.body ? `<p class="share-body">${esc(p.body)}</p>` : ''}
    ${link}
    ${image}
    <div class="share-actions">
      <button class="share-action ${p.liked ? 'on' : ''}" data-like="${esc(p.id)}" aria-pressed="${p.liked}" type="button">${SHARE_ICON_LIKE}<span>${p.likes}</span><span class="share-action-label">${esc(tr('Suka'))}</span></button>
      <button class="share-action" data-comments="${esc(p.id)}" aria-expanded="${open}" type="button">${SHARE_ICON_COMMENT}<span>${comments.length}</span><span class="share-action-label">${esc(tr('Komentar'))}</span></button>
    </div>
    <section class="share-comments" ${open ? '' : 'hidden'} data-comments-body="${esc(p.id)}">
      ${body}
      <form class="share-comment-form" data-cform="${esc(p.id)}">
        <input maxlength="1000" placeholder="${esc(tr('Tulis komentar...'))}" aria-label="${esc(tr('Komentar'))}" autocomplete="off">
        <button class="btn sm" type="submit">${esc(tr('Kirim'))}</button>
      </form>
    </section>
  </article>`;
}

function renderShareFeed() {
  const box = $('#share-feed');
  if (!box) return;
  const count = $('#share-count');
  if (count) count.textContent = shareCount(shareState.posts.length);
  const list = shareState.sort === 'top'
    ? shareState.posts.slice().sort((a, b) => (b.likes - a.likes) || (b.createdAt - a.createdAt))
    : shareState.posts;
  box.innerHTML = list.length
    ? list.map(sharePostHTML).join('')
    : `<div class="empty share-empty"><b>${esc(tr('Belum ada post'))}</b><span class="empty-hint">${esc(tr('Mulai dari form di atas.'))}</span></div>`;
}

async function loadSharing() {
  const box = $('#share-feed');
  if (box && !shareState.posts.length) box.innerHTML = `<div class="empty">${esc(tr('Memuat feed...'))}</div>`;
  try {
    const q = '/v1/sharing?sort=' + encodeURIComponent(shareState.sort) + '&client=' + encodeURIComponent(shareClient());
    const d = await api(q);
    shareState.posts = d.posts || [];
    renderShareFeed();
  } catch (e) {
    if (box) box.innerHTML = `<div class="empty">${esc(e.message)}</div>`;
  }
}

function replaceSharePost(post) {
  const i = shareState.posts.findIndex(p => p.id === post.id);
  if (i >= 0) shareState.posts[i] = post;
  renderShareFeed();
}

function pageSharing(view) {
  shareState.sort = 'new';
  shareState.posts = [];
  shareState.image = '';
  shareOpen.clear();
  const name = shareName();
  view.innerHTML = skeleton('Sharing', 'Papan sharing lokal: post, link, gambar, dan komentar tersimpan di bridge ini.') + `
  <form class="card share-composer" id="share-form">
    <h2>Post baru <span class="muted">tersimpan lokal</span></h2>
    <input id="share-title" maxlength="120" placeholder="Judul post" aria-label="Judul" autocomplete="off">
    <textarea id="share-body" rows="3" maxlength="4000" placeholder="Tulis pesan..." aria-label="Tulis pesan..."></textarea>
    <div id="share-preview" class="share-preview hidden"><img id="share-preview-img" alt="Pratinjau gambar"><button class="btn ghost sm" id="share-drop" type="button">Hapus gambar</button></div>
    <div class="share-tools">
      <label class="btn ghost sm share-attach" for="share-image">
        <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><path d="M2.5 4.5h11v7h-11z"/><path d="m3.5 10 3-3 2.5 2.5 1.5-1.5 2 2"/><circle cx="10.7" cy="6.3" r=".9"/></svg>
        <span>Gambar</span>
        <input id="share-image" type="file" accept="image/png,image/jpeg,image/gif,image/webp" hidden>
      </label>
      <input id="share-link" class="grow" maxlength="600" placeholder="Link (opsional)" aria-label="Link (opsional)" autocomplete="off">
      <label class="share-name-wrap"><span class="small muted">Nama</span><input id="share-name" maxlength="40" value="${esc(name === 'anon' ? '' : name)}" placeholder="Nama" aria-label="Nama tampilan" autocomplete="off"></label>
      <span class="spacer"></span>
      <button class="btn primary" id="share-submit" type="submit">Post</button>
    </div>
    <p class="small muted share-warn">Jangan posting token privat. Siapa pun yang membuka bridge ini bisa membaca papan.</p>
  </form>
  <div class="share-feed-head">
    <h2>Feed</h2><span class="small muted" id="share-count"></span><span class="spacer"></span>
    <select id="share-sort" aria-label="Urutkan post"><option value="new">Terbaru</option><option value="top">Terpopuler</option></select>
    <button class="btn ghost sm" id="share-refresh" type="button">Muat ulang</button>
  </div>
  <div id="share-feed" class="share-feed" aria-live="polite"><div class="empty">${esc(tr('Memuat feed...'))}</div></div>`;

  const form = $('#share-form');
  const nameInput = $('#share-name');
  nameInput.oninput = () => localStorage.setItem('zbShareName', nameInput.value.trim());
  $('#share-image').onchange = ev => {
    const file = ev.target.files && ev.target.files[0];
    if (!file) return;
    if (!/^image\/(png|jpeg|gif|webp)$/.test(file.type)) { toast(tr('Hanya file gambar yang didukung'), 'bad'); ev.target.value = ''; return; }
    if (file.size > 120 * 1024) { toast(tr('Gambar maksimal 120 KB'), 'bad'); ev.target.value = ''; return; }
    const reader = new FileReader();
    reader.onload = () => {
      shareState.image = String(reader.result || '');
      $('#share-preview-img').src = shareState.image;
      $('#share-preview').classList.remove('hidden');
    };
    reader.onerror = () => toast(tr('Gambar gagal dibaca'), 'bad');
    reader.readAsDataURL(file);
    ev.target.value = '';
  };
  $('#share-drop').onclick = () => {
    shareState.image = '';
    $('#share-preview').classList.add('hidden');
    $('#share-preview-img').removeAttribute('src');
  };
  $('#share-sort').onchange = ev => { shareState.sort = ev.target.value === 'top' ? 'top' : 'new'; loadSharing(); };
  $('#share-refresh').onclick = () => loadSharing();

  form.onsubmit = async ev => {
    ev.preventDefault();
    const title = $('#share-title').value.trim();
    const body = $('#share-body').value.trim();
    const link = $('#share-link').value.trim();
    const author = shareName();
    if (!title) { toast(tr('Judul wajib diisi'), 'bad'); $('#share-title').focus(); return; }
    if (!body && !link && !shareState.image) { toast(tr('Isi, link, atau gambar wajib ada'), 'bad'); $('#share-body').focus(); return; }
    const btn = $('#share-submit');
    btn.disabled = true;
    try {
      const r = await api('/v1/sharing', { method: 'POST', body: JSON.stringify({ title, body, link, image: shareState.image, author, client: shareClient() }) });
      shareState.posts.unshift(r.post);
      shareState.image = '';
      form.reset();
      nameInput.value = author === 'anon' ? '' : author;
      $('#share-preview').classList.add('hidden');
      $('#share-preview-img').removeAttribute('src');
      renderShareFeed();
      toast(tr('Post terkirim'), 'ok');
      $('#share-title').focus();
    } catch (e) { toast(e.message, 'bad'); }
    finally { btn.disabled = false; }
  };

  const feed = $('#share-feed');
  feed.addEventListener('click', async ev => {
    const likeBtn = ev.target.closest('[data-like]');
    if (likeBtn) {
      likeBtn.disabled = true;
      try {
        const r = await api('/v1/sharing/like', { method: 'POST', body: JSON.stringify({ id: likeBtn.dataset.like, client: shareClient() }) });
        replaceSharePost(r.post);
      } catch (e) { toast(e.message, 'bad'); likeBtn.disabled = false; }
      return;
    }
    const commentBtn = ev.target.closest('[data-comments]');
    if (commentBtn) {
      const id = commentBtn.dataset.comments;
      if (shareOpen.has(id)) shareOpen.delete(id); else shareOpen.add(id);
      renderShareFeed();
      if (shareOpen.has(id)) {
        const input = feed.querySelector(`[data-comments-body="${CSS.escape(id)}"] input`);
        if (input) input.focus();
      }
      return;
    }
    const delBtn = ev.target.closest('[data-del]');
    if (delBtn) {
      const post = shareState.posts.find(p => p.id === delBtn.dataset.del);
      if (!confirm(tr('Hapus post ini?'))) return;
      delBtn.disabled = true;
      try {
        await api('/v1/sharing?id=' + encodeURIComponent(delBtn.dataset.del) + '&client=' + encodeURIComponent(shareClient()), { method: 'DELETE' });
        shareState.posts = shareState.posts.filter(p => p.id !== delBtn.dataset.del);
        renderShareFeed();
        toast(tr('Post dihapus'), 'ok');
      } catch (e) { toast(e.message, 'bad'); delBtn.disabled = false; }
    }
  });
  feed.addEventListener('submit', async ev => {
    const cform = ev.target.closest('[data-cform]');
    if (!cform) return;
    ev.preventDefault();
    const input = cform.querySelector('input');
    const body = input.value.trim();
    if (!body) return;
    const btn = cform.querySelector('button');
    btn.disabled = true;
    try {
      const r = await api('/v1/sharing/comment', { method: 'POST', body: JSON.stringify({ id: cform.dataset.cform, author: shareName(), body, client: shareClient() }) });
      shareOpen.add(cform.dataset.cform);
      replaceSharePost(r.post);
      const next = feed.querySelector(`[data-comments-body="${CSS.escape(cform.dataset.cform)}"] input`);
      if (next) next.focus();
    } catch (e) { toast(e.message, 'bad'); btn.disabled = false; }
  });

  loadSharing();
}

/* ---------------- settings ---------------- */
async function pageSettings(view) {
  view.innerHTML = skeleton('Pengaturan', 'Nilai efektif proses ini. Perubahan env butuh restart.') + `
  <div class="grid g2 set-grid">
    <div class="card"><h2>Bridge</h2><div id="setbridge"><div class="empty">Memuat…</div></div></div>
    <div class="set-side">
      <div class="card"><h2>Token & tema</h2>
        <label class="field"><span>Bearer token (sesi tab)</span><input id="settoken" type="password" placeholder="belum di-set" autocomplete="off"></label>
        <div class="row mt10"><button class="btn" id="settokensave" type="button">Simpan token</button><button class="btn ghost" id="settokenclear" type="button">Hapus</button></div>
        <hr class="hr-line">
        <div class="row"><span class="small muted">Tema</span><span class="spacer"></span><button class="btn ghost sm" id="settheme" type="button">Ganti tema</button></div>
      </div>
      <div class="card"><h2>Data & alat</h2>
        <div class="row"><a class="btn ghost sm" href="/classic" target="_blank" rel="noopener">Dashboard klasik</a><a class="btn ghost sm" href="/overlay" target="_blank" rel="noopener">Overlay</a><a class="btn ghost sm" href="/health" target="_blank" rel="noopener">/health</a><a class="btn ghost sm" href="/v1/models" target="_blank" rel="noopener">/v1/models</a></div>
        <p class="small muted mt10">Capture dan flag lain dibaca saat start; ubah env lalu restart bridge.</p>
      </div>
    </div>
  </div>
  <div class="card mt14"><h2>Integrasi agen <span class="muted">OpenAI compatible</span></h2>
    <div id="intbody"><div class="empty">Memuat...</div></div>
  </div>
  <div class="card mt14"><h2>Environment</h2><div id="setenv"></div></div>`;
  $('#settheme').onclick = toggleTheme;
  $('#settokensave').onclick = () => { ZB.token = $('#settoken').value.trim(); sessionStorage.setItem('zbToken', ZB.token); toast('Token disimpan', 'ok'); connectSSE(); refreshHealth(); };
  $('#settokenclear').onclick = () => { ZB.token = ''; sessionStorage.removeItem('zbToken'); $('#settoken').value = ''; toast('Token dihapus', 'ok'); connectSSE(); };
  try {
    const s = await api('/v1/settings');
    const rows = [
      ['Lane', s.lane], ['Upstream', s.upstream], ['Host', s.host], ['Port', s.port],
      ['Auth', s.auth], ['Capture', s.capture && s.capture.enabled ? 'aktif' : 'nonaktif'],
      ['Capture kapasitas', s.capture && s.capture.capacity], ['Token plan', s.plan_jwt ? 'ada' : 'tidak'],
      ['Device MID', s.device_mid], ['Async', s.async ? 'aktif' : 'nonaktif'], ['Klaim', s.claim ? 'aktif' : 'nonaktif'],
      ['Update check', s.update_check ? 'aktif' : 'nonaktif'], ['Versi', s.version]
    ];
    $('#setbridge').innerHTML = '<dl class="kv">' + rows.map(([k, v]) => `<dt>${esc(k)}</dt><dd>${esc(v == null || v === '' ? '-' : v)}</dd>`).join('') + '</dl>';
    $('#setenv').innerHTML = s.env && Object.keys(s.env).length ? '<div class="tablewrap"><table><thead><tr><th>Variabel</th><th>Nilai</th></tr></thead><tbody>' + Object.entries(s.env).map(([k, v]) => `<tr><td class="mono">${esc(k)}</td><td class="mono wrap">${esc(v)}</td></tr>`).join('') + '</tbody></table></div>' : '<div class="empty">Tidak ada override env yang terbaca.</div>';
  } catch (e) {
    $('#setbridge').innerHTML = `<div class="empty">${esc(e.message)}</div>`;
    $('#setenv').innerHTML = `<div class="empty">${esc(e.message)}</div>`;
  }
  loadIntegration();
}

function integrationCurl(s) {
  const key = s.key || '<kunci-api>';
  const body = JSON.stringify({ model: s.default_model, messages: [{ role: 'user', content: 'Halo' }], stream: true });
  return `curl ${s.endpoints.chat} \\\n  -H "Authorization: Bearer ${key}" \\\n  -H "Content-Type: application/json" \\\n  -d '${body}'`;
}

async function loadIntegration() {
  const box = $('#intbody');
  if (!box) return;
  box.innerHTML = '<div class="empty">Memuat...</div>';
  try {
    const s = await api('/v1/integration');
    const curl = integrationCurl(s);
    const source = s.key_source === 'env' ? 'dari env ZCODE_BRIDGE_TOKEN (hanya baca)' : (s.key_source === 'file' ? 'tersimpan di data/agent-key.json' : 'belum di-set, bridge terbuka di loopback');
    box.innerHTML = `
      <label class="field"><span>Kunci API (Bearer hilir)</span>
        <div class="row"><input id="intkey" class="grow mono" readonly value="${esc(s.key || '')}" placeholder="belum di-set"><button class="btn ghost sm" id="intcopy" type="button">Salin</button></div>
      </label>
      <p class="small muted int-note">${esc(s.masked_key ? 'Tampilan: ' + s.masked_key + ' - ' : '')}${esc(source)}</p>
      <label class="field"><span>Base URL</span>
        <div class="row"><input id="intbase" class="grow mono" readonly value="${esc(s.base_url)}"><button class="btn ghost sm" id="intcopybase" type="button">Salin</button></div>
      </label>
      <div class="row mt10"><span class="small muted">Model default ${esc(s.default_model)}</span><span class="spacer"></span>
        <button class="btn ghost sm" id="introtate" type="button">${s.key_present ? 'Ganti kunci' : 'Buat kunci'}</button>
        <button class="btn ghost sm danger" id="intdelete" type="button" ${s.key_present && s.editable ? '' : 'disabled'}>Hapus</button>
      </div>
      <div class="row int-head"><span class="small muted">Contoh permintaan</span><span class="spacer"></span><button class="btn ghost sm" id="intcurlcopy" type="button">Salin contoh</button></div>
      <pre class="snippet" id="intcurl">${esc(curl)}</pre>
      <p class="small muted int-tail">Klien lain memakai kunci ini sebagai <span class="mono">Authorization: Bearer</span>. Kunci env tidak bisa diubah dari dashboard.</p>`;
    $('#intcopy').onclick = () => copyText(s.key || '');
    $('#intcopybase').onclick = () => copyText(s.base_url);
    $('#intcurlcopy').onclick = () => copyText(curl);
    const rotate = $('#introtate');
    rotate.disabled = !s.editable;
    rotate.onclick = async () => {
      if (s.key_present && !confirm('Ganti kunci? Klien yang memakai kunci lama langsung ditolak.')) return;
      rotate.disabled = true;
      try {
        const r = await api('/v1/integration/key', { method: 'POST' });
        ZB.token = r.key;
        sessionStorage.setItem('zbToken', r.key);
        toast('Kunci baru dibuat dan dipakai tab ini', 'ok');
        connectSSE(); refreshHealth(); loadIntegration();
      } catch (e) { toast(e.message, 'bad'); rotate.disabled = false; }
    };
    const del = $('#intdelete');
    if (del) del.onclick = async () => {
      if (!confirm('Hapus kunci? Setelah ini bridge terbuka untuk siapa pun di loopback.')) return;
      del.disabled = true;
      try {
        await api('/v1/integration/key', { method: 'DELETE' });
        ZB.token = '';
        sessionStorage.removeItem('zbToken');
        toast('Kunci dihapus', 'ok');
        connectSSE(); refreshHealth(); loadIntegration();
      } catch (e) { toast(e.message, 'bad'); del.disabled = false; }
    };
  } catch (e) {
    box.innerHTML = `<div class="empty">${esc(e.message)}</div>`;
  }
}

/* ---------------- live ---------------- */
async function refreshHealth() {
  try { ZB.health = await api('/health'); } catch (e) { return; }
  const h = ZB.health;
  const dot = $('#livedot'); if (dot) dot.classList.add('live');
  const lane = $('#toplane'); if (lane) lane.textContent = h.lane || 'lane';
  const sub = $('#brandsub'); if (sub) sub.textContent = h.auth || '';
  const ver = $('#sidever'); if (ver && h.version) ver.textContent = 'v' + h.version;
  if (ZB.page === 'overview' || ZB.page === 'accounts') { patchOverview(); renderAccountCard(); }
}

function connectSSE() {
  if (ZB.esAbort) { try { ZB.esAbort.abort(); } catch (e) {} }
  const ctrl = new AbortController();
  ZB.esAbort = ctrl;
  const headers = { Accept: 'text/event-stream' };
  if (ZB.token) headers.Authorization = 'Bearer ' + ZB.token;
  fetch('/v1/events', { headers, signal: ctrl.signal, cache: 'no-store' }).then(async res => {
    if (res.status === 401) { openToken(); return; }
    if (!res.ok || !res.body) throw new Error('SSE ' + res.status);
    $('#livedot').classList.add('live');
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      let idx;
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const frame = buf.slice(0, idx); buf = buf.slice(idx + 2);
        handleEventFrame(frame);
      }
    }
  }).catch(err => {
    if (err.name === 'AbortError') return;
    $('#livedot').classList.remove('live');
    setTimeout(() => { if (ZB.esAbort === ctrl) connectSSE(); }, 2500);
  });
}

function handleEventFrame(frame) {
  let data = '';
  for (const line of frame.split('\n')) if (line.startsWith('data:')) data += line.slice(5).trim();
  if (!data) return;
  let msg; try { msg = JSON.parse(data); } catch (e) { return; }
  if (msg.health) ZB.health = msg.health;
  if (msg.usage) ZB.usage = msg.usage;
  if (msg.net) ZB.net = msg.net;
  patchSideStats();
  const h = ZB.health || {};
  const lane = $('#toplane'); if (lane) lane.textContent = h.lane || 'lane';
  if (document.getElementById('ov-tps') && ZB.page === 'overview') patchOverview();
  if (document.getElementById('ur-last') && ZB.page === 'usage') renderUsage();
  if (document.getElementById('ntiles') && ZB.page === 'network') renderNetwork();
}

/* ---------------- boot ---------------- */
function toggleTheme() {
  const cur = document.documentElement.dataset.theme === 'light' ? 'dark' : 'light';
  document.documentElement.dataset.theme = cur;
  localStorage.setItem('zbTheme', cur);
}

function boot() {
  document.documentElement.dataset.theme = localStorage.getItem('zbTheme') || 'dark';
  $('#themebtn').onclick = toggleTheme;
  $('#tokenbtn').onclick = openToken;
  $('#navtoggle').onclick = () => document.body.classList.toggle('navopen');
  const scrim = document.getElementById('scrim');
  if (scrim) scrim.onclick = () => document.body.classList.remove('navopen');
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') document.body.classList.remove('navopen');
  });
  $('#tokenform').addEventListener('submit', e => {
    e.preventDefault();
    ZB.token = $('#tokeninput').value.trim();
    sessionStorage.setItem('zbToken', ZB.token);
    const d = $('#tokendlg'); if (d.close) d.close(); else d.removeAttribute('open');
    toast('Token disimpan', 'ok');
    connectSSE(); route(); refreshHealth();
  });
  $('#tokencancel').onclick = () => { const d = $('#tokendlg'); if (d.close) d.close(); else d.removeAttribute('open'); };
  route();
  applyLanguage();
  refreshHealth();
  api('/v1/usage').then(u => { ZB.usage = u; patchSideStats(); if (ZB.page === 'overview') patchOverview(); if (ZB.page === 'usage') renderUsage(); }).catch(() => {});
  api('/v1/net').then(n => { ZB.net = n; patchSideStats(); if (ZB.page === 'overview') patchOverview(); if (ZB.page === 'network') renderNetwork(); }).catch(() => {});
  connectSSE();
  setInterval(refreshHealth, 15000);
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape' && ZB.page === 'chat' && window.ZChat) window.ZChat.escape();
  });
}
if (document.readyState === 'complete') boot(); else window.addEventListener('load', boot, { once: true });
