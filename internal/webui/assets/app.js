'use strict';

const ZB = window.ZB = {
  token: sessionStorage.getItem('zbToken') || '',
  health: null,
  usage: null,
  quota: null,
  models: null,
  page: 'overview',
  range: '24h',
  live: false,
  esAbort: null,
  pipeTimer: null,
  pipeMsgs: [],
  pipeIdx: 0,
  started: false,
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
  'Akun, pemakaian, dan request terakhir.': 'Accounts, usage, and recent requests.',
  'Total akun': 'Total accounts',
  'Akun aktif': 'Active account',
  'Sisa kuota': 'Quota left',
  'unit plan aktif': 'plan units',
  'dipakai lane plan': 'used by the plan lane',
  'sejak proses hidup': 'since the process started',
  'keluar + masuk': 'output + input',
  'token per detik': 'tokens per second',
  'Tekanan kuota': 'Quota pressure',
  'Lihat penggunaan': 'View usage',
  'Semua request': 'All requests',
  'Belum ada akun tersimpan': 'No stored accounts',
  'Status tiap akun dan sisa kuota per model.': 'Per-account status and remaining quota by model.',
  'Akun di pool': 'Account pool',
  'Billing dibaca per akun, cache 45 detik.': 'Billing is read per account, cached for 45 seconds.',
  'Hubungkan akun': 'Connect account',
  'Login Z.ai lewat tautan persetujuan.': 'Sign in to Z.ai through an approval link.',
  'Membaca kuota semua akun…': 'Reading quota for every account…',
  'puncak jendela': 'window peak',
  'offline': 'offline',
  'Kelola': 'Manage',
  'Akun': 'Accounts',
  'Penggunaan': 'Usage',
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
  'Request': 'Requests',
  'Kuota': 'Quota',
  'Lane': 'Lane',
  'Data': 'Data',
  'Akun dan angka utama bridge.': 'Accounts and the bridge key numbers.',
  'Riwayat request di jendela proses ini.': 'Request history for this process window.',
  'Sisa grant per akun dan model.': 'Remaining grants per account and model.',
  'Identitas yang dipakai tiap request upstream.': 'Identity used for every upstream request.',
  'Request terakhir': 'Recent requests',
  'Tekanan kuota': 'Quota pressure',
  'Info lane': 'Lane info',
  'Akun siap': 'Ready accounts',
  'Token plan': 'Plan token',
  'Versi app': 'App version',
  'Terpakai': 'Used',
  'Sisa': 'Left',
  'Kelola akun': 'Manage accounts',
  'Ganti akun': 'Switch account',
  'Buka →': 'Open →',
  'Riwayat request': 'Request history',
  'Grafik kecepatan': 'Speed chart',
  'Belum ada grant': 'No grants yet',
  'Base URL, endpoint, model, dan contoh request untuk klien.': 'Base URLs, endpoints, models, and request examples for clients.',
  'Endpoint LLM memakai kunci bridge; endpoint admin memakai kunci yang sama.': 'LLM endpoints use the bridge key; admin endpoints use the same key.',
  'Grant per model per akun.': 'Grants per model per account.',
  'Katalog model aktif dan sisa grant.': 'Live model catalog and remaining grants.',
  'Chat completion, streaming opsional.': 'Chat completion, optional streaming.',
  'Messages API, streaming opsional.': 'Messages API, optional streaming.',
  'Antrean async, balas 202 + id.': 'Async queue, replies 202 + id.',
  'Status antrean async.': 'Async queue status.',
  'JSON-RPC lewat streamable HTTP.': 'JSON-RPC over streamable HTTP.',
  'Statistik pemakaian dan kecepatan.': 'Usage and speed statistics.',
  'SSE snapshot dan patch.': 'SSE snapshot and patches.',
  'Konfigurasi efektif bridge.': 'Effective bridge settings.',
  'Kunci dan URL integrasi.': 'Integration key and URLs.',
  'Konsol ini.': 'This console.',
  'Status lane, akun, dan JWT.': 'Lane, account, and JWT status.',
  'chat completions dan responses': 'chat completions and responses',
  'messages API': 'messages API',
  'OpenAI chat completion, streaming.': 'OpenAI chat completion, streaming.',
  'Anthropic messages, non-stream.': 'Anthropic messages, non-stream.',
  'Simpan kunci': 'Save key',
  'Acak kunci': 'Randomize key',
  'Kunci tidak boleh kosong': 'Key cannot be empty',
  'Kunci disimpan dan dipakai tab ini': 'Key saved and used by this tab',
  'Alur live': 'Live pipeline',
  'Jalur request dari klien sampai upstream.': 'Request path from client to upstream.',
  'Menunggu request lewat bridge…': 'Waiting for requests through the bridge…',
  'KLIEN': 'CLIENT',
  'GATE LOKAL': 'LOCAL GATE',
  '3-blok identitas': '3-block identity',
  'RATA-RATA': 'AVERAGE',
  'REQUEST': 'REQUESTS',
  'Token hari ini': 'Tokens today',
  'volume sejak proses hidup': 'volume since process start',
  'akumulasi harian': 'daily accumulation',
  'porsi request hari ini': 'request share today',
  'Sesi operator': 'Operator session',
  'Kecepatan lane, volume hari ini, dan identitas upstream.': 'Lane speed, today\'s volume, and upstream identity.',
  'Lane & upstream': 'Lane & upstream',
  'Identitas yang dipakai tiap request.': 'Identity used for every request.',
  'Akun di lane': 'Accounts in lane',
  'Urutan pemakaian saat satu akun sibuk.': 'Usage order when an account is busy.',
  'Request hari ini': 'Requests today',
  'Akun aktif': 'Active account',
  'token per detik': 'tokens per second',
  'sejak proses hidup': 'since process start',
  'dipakai lane plan': 'used by the plan lane',
  'Device mid': 'Device mid',
  'Peran': 'Role',
  'siaga': 'standby',
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
  'Klaim semua akun': 'Claim all accounts',
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

const I18N_SKIP = '.bw,.msg-body,.snippet,script,style,textarea';

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
  $$('.lang-switch button').forEach(btn => {
    const on = btn.dataset.lang === ZB.lang;
    btn.setAttribute('aria-pressed', on ? 'true' : 'false');
    btn.onclick = () => {
      if (btn.dataset.lang === ZB.lang) return;
      localStorage.setItem('zbLang', btn.dataset.lang);
      location.reload();
    };
  });
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
  if (!['overview', 'accounts', 'chat', 'inspector', 'docs', 'usage', 'claim', 'requests', 'quota', 'lane', 'session', 'settings'].includes(hash)) hash = 'overview';
  ZB.page = hash;
  $$('#nav a').forEach(a => a.classList.toggle('active', a.dataset.page === hash));
  const crumb = $('#crumb');
  if (crumb) {
    const active = $$('#nav a').find(a => a.dataset.page === hash);
    crumb.textContent = active ? active.textContent.trim() : 'Ringkasan';
  }
  document.body.classList.toggle('chat-mode', hash === 'chat');
  const view = $('#view');
  view.innerHTML = '';
  if (hash === 'chat' && window.ZChat) window.ZChat.mount(view);
  else if (hash === 'chat') view.innerHTML = '<div class="empty">Memuat chat…</div>';
  else ({ overview: pageOverview, accounts: pageAccounts, inspector: pageInspector, docs: pageDocs, usage: pageUsage, claim: pageClaim, requests: pageRequests, quota: pageQuota, lane: pageLane, session: pageSession, settings: pageSettings })[hash](view);
  view.scrollTop = 0;
  blurInTitle(view);
}
window.addEventListener('hashchange', route);

function blurInTitle(root) {
  const h = root && root.querySelector('h1');
  if (!h || h.dataset.blur) return;
  const text = tr((h.textContent || '').trim());
  if (!text) return;
  h.dataset.blur = '1';
  h.setAttribute('aria-label', text);
  h.innerHTML = text.split(' ').map((w, i) =>
    `<span class="bw" style="animation-delay:${i * 28}ms" aria-hidden="true">${esc(w)}</span>`).join(' ');
}

function skeleton(title, sub, actions) {
  return `<header class="page-head"><div><h1>${esc(title)}</h1><p class="page-desc">${esc(sub)}</p></div>${actions ? `<div class="actions">${actions}</div>` : ''}</header>`;
}

/* ---------------- overview ---------------- */
const ICON = {
  users: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>',
  check: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="m8.5 12 2.5 2.5 4.5-5"/></svg>',
  coins: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="6" rx="7" ry="3"/><path d="M5 6v6c0 1.7 3.1 3 7 3s7-1.3 7-3V6"/><path d="M5 12v6c0 1.7 3.1 3 7 3s7-1.3 7-3v-6"/></svg>',
  pulse: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12h4l2-7 4 14 2-7h6"/></svg>',
  hash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 9h16M4 15h16M10 3 8 21M16 3l-2 18"/></svg>',
  gauge: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21a9 9 0 1 1 9-9"/><path d="m12 12 4-4"/><circle cx="12" cy="12" r="1.6"/></svg>',
  clock: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/></svg>',
  list: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M8 6h13M8 12h13M8 18h13"/><path d="M3 6h.01M3 12h.01M3 18h.01"/></svg>'
};

function pageOverview(view) {
  view.innerHTML = skeleton('Ringkasan', 'Akun dan angka utama bridge.', '<button class="btn" id="ovrefresh" type="button">Muat ulang</button>') + `
  <div class="stat-grid" aria-label="Ringkasan angka">
    <a class="stat-card" href="#/accounts"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.users}</span><span class="stat-label">Total akun</span></div><p class="stat-value" id="st-accounts">–</p><p class="stat-note">tersimpan lokal</p><div class="stat-more">Kelola akun <span>Buka →</span></div></a>
    <a class="stat-card" href="#/accounts"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.check}</span><span class="stat-label">Akun aktif</span></div><p class="stat-value" id="st-active">–</p><p class="stat-note">dipakai lane plan</p><div class="stat-more">Ganti akun <span>Buka →</span></div></a>
    <a class="stat-card" href="#/quota"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.coins}</span><span class="stat-label">Sisa kuota</span></div><p class="stat-value" id="st-quota">–</p><p class="stat-note" id="st-quota-note">unit plan aktif</p><div class="stat-more">Tekanan kuota <span>Buka →</span></div></a>
    <a class="stat-card" href="#/requests"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.pulse}</span><span class="stat-label">Request hari ini</span></div><p class="stat-value" id="st-req">–</p><p class="stat-note">sejak proses hidup</p><div class="stat-more">Riwayat request <span>Buka →</span></div></a>
    <a class="stat-card" href="#/usage"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.hash}</span><span class="stat-label">Token hari ini</span></div><p class="stat-value" id="st-tok">–</p><p class="stat-note">keluar + masuk</p><div class="stat-more">Penggunaan <span>Buka →</span></div></a>
    <a class="stat-card" href="#/usage"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.gauge}</span><span class="stat-label">Kecepatan terakhir</span></div><p class="stat-value" id="st-tps">–</p><p class="stat-note">token per detik</p><div class="stat-more">Grafik kecepatan <span>Buka →</span></div></a>
  </div>

  <article class="card pipe-card">
    <div class="card-head">
      <div><h2>Alur live</h2><p class="section-note" id="pipe-note">Jalur request dari klien sampai upstream.</p></div>
      <span class="pipe-live"><i aria-hidden="true"></i>LIVE</span>
    </div>
    <div class="pipe-body">
      <svg viewBox="0 0 580 172" width="100%" role="img" aria-label="Diagram alur request live">
        <defs>
          <marker id="pipeArrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto">
            <path d="M2 1.5L7.5 5L2 8.5" fill="none" stroke="rgba(236,92,19,0.5)" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
          </marker>
        </defs>
        <path class="pipe-link" d="M116,88 L158,88" marker-end="url(#pipeArrow)"/>
        <path class="pipe-link" d="M268,88 L306,88" marker-end="url(#pipeArrow)"/>
        <path class="pipe-link dim" d="M411,88 C425,88 435,50 448,50"/>
        <path class="pipe-link dim" d="M411,88 L448,88"/>
        <path class="pipe-link dim" d="M411,88 C425,88 435,126 448,126"/>
        <g class="pipe-flow">
          <circle r="2.5" fill="var(--accent)"><animateMotion dur="1.05s" repeatCount="indefinite" begin="0s" path="M116,88 L158,88"/></circle>
          <circle r="1.8" fill="var(--accent)" opacity=".6"><animateMotion dur="1.05s" repeatCount="indefinite" begin="0.35s" path="M116,88 L158,88"/></circle>
          <circle r="2.5" fill="var(--accent)"><animateMotion dur="0.88s" repeatCount="indefinite" begin="0.18s" path="M268,88 L306,88"/></circle>
          <circle r="1.8" fill="var(--accent)" opacity=".6"><animateMotion dur="0.88s" repeatCount="indefinite" begin="0.62s" path="M268,88 L306,88"/></circle>
          <circle r="2.2" fill="var(--accent)" opacity=".9"><animateMotion dur="1.3s" repeatCount="indefinite" begin="0.08s" path="M411,88 C425,88 435,50 448,50"/></circle>
          <circle r="2.2" fill="var(--accent)" opacity=".9"><animateMotion dur="1.15s" repeatCount="indefinite" begin="0.28s" path="M411,88 L448,88"/></circle>
          <circle r="2.2" fill="var(--accent)" opacity=".9"><animateMotion dur="1.4s" repeatCount="indefinite" begin="0.45s" path="M411,88 C425,88 435,126 448,126"/></circle>
        </g>
        <g>
          <rect x="16" y="66" width="100" height="44" rx="8" class="pipe-node"/>
          <text x="66" y="83" text-anchor="middle" class="pipe-k">KLIEN</text>
          <text x="66" y="100" text-anchor="middle" class="pipe-v">User query</text>
          <text x="66" y="122" text-anchor="middle" class="pipe-mono">node-01</text>
        </g>
        <g>
          <rect x="158" y="66" width="110" height="44" rx="8" class="pipe-node"/>
          <text x="213" y="83" text-anchor="middle" class="pipe-k">GATE LOKAL</text>
          <text x="213" y="100" text-anchor="middle" class="pipe-v">3-blok identitas</text>
          <text x="213" y="122" text-anchor="middle" class="pipe-mono">bridge</text>
        </g>
        <g>
          <rect x="306" y="53" width="105" height="70" rx="10" class="pipe-core"/>
          <text x="358" y="78" text-anchor="middle" class="pipe-k hot">LANE PLAN</text>
          <text x="358" y="97" text-anchor="middle" class="pipe-v hi">Upstream</text>
          <circle cx="346" cy="113" r="2.8" fill="var(--accent)" class="pipe-dot d1"/>
          <circle cx="358" cy="113" r="2.8" fill="var(--accent)" class="pipe-dot d2"/>
          <circle cx="370" cy="113" r="2.8" fill="var(--accent)" class="pipe-dot d3"/>
          <text x="358" y="139" text-anchor="middle" class="pipe-mono hot">z.ai · plan</text>
        </g>
        <g>
          <rect x="448" y="35" width="116" height="30" rx="7" class="pipe-out"/>
          <text x="486" y="53.5" text-anchor="middle" class="pipe-out-v">Chat stream</text>
          <circle cx="550" cy="43" r="3" fill="var(--fg)"/>
        </g>
        <g>
          <rect x="448" y="73" width="116" height="30" rx="7" class="pipe-out"/>
          <text x="486" y="91.5" text-anchor="middle" class="pipe-out-v">Tool call</text>
          <circle cx="550" cy="81" r="3" fill="var(--accent)" class="pipe-dot d2"/>
        </g>
        <g>
          <rect x="448" y="111" width="116" height="30" rx="7" class="pipe-out"/>
          <text x="486" y="129.5" text-anchor="middle" class="pipe-out-v">Penggunaan</text>
          <circle cx="550" cy="119" r="3" fill="var(--accent)" class="pipe-dot d3"/>
        </g>
      </svg>
    </div>
    <div class="pipe-tick">
      <span class="pipe-caret" aria-hidden="true">›</span>
      <p class="pipe-msg" id="pipe-msg">Menunggu request lewat bridge…</p>
    </div>
    <div class="pipe-foot">
      <div><span>REQUEST</span><b id="pipe-req">–</b></div>
      <div><span>TOKEN</span><b id="pipe-tok">–</b></div>
      <div><span>RATA-RATA</span><b id="pipe-avg">–</b></div>
      <div class="pipe-stack"><span>STACK</span><b id="pipe-stack">z.ai · plan</b></div>
    </div>
  </article>

  <article class="card overview-accounts">
    <div class="card-head">
      <div><h2>Akun</h2><p class="section-note" id="ov-accounts-note">Memuat akun.</p></div>
      <div class="actions"><a class="btn small" href="#/accounts">Kelola</a></div>
    </div>
    <div id="ov-accounts"><div class="empty">Memuat akun…</div></div>
  </article>`;
  $('#ovrefresh').onclick = () => { refreshHealth(); loadUsage(true); loadOverviewExtras(); };
  patchOverview();
  loadOverviewExtras();
}

function patchOverview() {
  const h = ZB.health || {}, u = ZB.usage;
  if (!$('#st-accounts')) return;
  const accounts = h.accounts || [];
  $('#st-accounts').textContent = fmtN(accounts.length);
  $('#st-active').textContent = fmtN(accounts.filter(a => a.active || a.default).length);
  const q = ZB.quota;
  if (q && q.rows && q.rows.length) {
    const rem = q.rows.reduce((s, r) => {
      const total = Number(r.total_units) || 0, used = Number(r.used_units) || 0;
      const left = r.remaining_units != null ? Number(r.remaining_units) : (r.used_units != null ? total - used : total);
      return s + (Number.isFinite(left) ? left : 0);
    }, 0);
    $('#st-quota').textContent = fmtN(rem);
    $('#st-quota-note').textContent = q.rows.length + ' model plan';
  } else if (q) {
    $('#st-quota').textContent = '–';
    $('#st-quota-note').textContent = 'tidak ada grant';
  }
  if (u) {
    $('#st-req').textContent = fmtN(u.today && u.today.requests);
    $('#st-tok').textContent = fmtN((u.today && u.today.output_tokens || 0) + (u.today && u.today.input_tokens || 0));
    $('#st-tps').textContent = fmtTps(u.rings && u.rings.last_tps);
  }
  const note = $('#ov-accounts-note');
  if (note) note.textContent = accounts.length ? accounts.length + ' akun siap' : 'Belum ada akun tersimpan';
  renderPipeline();
}

function pipelineMessages() {
  const rows = ((ZB.usage && ZB.usage.recent) || []).slice(-8).reverse();
  if (!rows.length) return ['Menunggu request lewat bridge…'];
  return rows.map(r => `${prettyModel(r.model || 'request')} · ${fmtN(r.output_tokens)} token · ${fmtTps(r.tps)} t/s · ${when(r.at)}`);
}

function showPipeMsg() {
  const el = $('#pipe-msg');
  if (!el || !ZB.pipeMsgs.length) return;
  el.textContent = ZB.pipeMsgs[ZB.pipeIdx % ZB.pipeMsgs.length];
  el.classList.remove('in');
  void el.offsetWidth;
  el.classList.add('in');
}

function renderPipeline() {
  if (!$('#pipe-msg')) return;
  const h = ZB.health || {}, u = ZB.usage;
  const accounts = h.accounts || [];
  const today = (u && u.today) || {};
  const note = $('#pipe-note');
  if (note) note.textContent = `${accounts.length} akun · ${fmtN(today.requests || 0)} request hari ini`;
  $('#pipe-req').textContent = u ? fmtN(today.requests || 0) : '–';
  $('#pipe-tok').textContent = u ? fmtN((today.output_tokens || 0) + (today.input_tokens || 0)) : '–';
  $('#pipe-avg').textContent = u ? fmtTps(today.avg_tps) + ' t/s' : '–';
  $('#pipe-stack').textContent = h.upstream ? String(h.upstream).replace(/^https?:\/\//, '').split('/')[0] : 'z.ai · plan';
  ZB.pipeMsgs = pipelineMessages();
  if (!ZB.pipeTimer) {
    ZB.pipeIdx = 0;
    ZB.pipeTimer = setInterval(() => {
      if (ZB.page !== 'overview' || !$('#pipe-msg')) {
        clearInterval(ZB.pipeTimer);
        ZB.pipeTimer = null;
        return;
      }
      ZB.pipeIdx = (ZB.pipeIdx + 1) % Math.max(1, ZB.pipeMsgs.length);
      showPipeMsg();
    }, 2700);
  }
  showPipeMsg();
}

function patchSideStats() { renderSession(); }

function renderSession() {
  const h = ZB.health || {}, u = ZB.usage;
  const set = (id, html) => { const el = document.getElementById(id); if (el) el.innerHTML = html; };
  if (!document.getElementById('ses-tps')) return;
  set('ses-tps', u && u.rings ? fmtTps(u.rings.last_tps) + ' <small>t/s</small>' : '-');
  set('ses-req', u && u.today ? fmtN(u.today.requests) : '-');
  const accounts = h.accounts || [];
  const active = accounts.find(a => a.active || a.default) || accounts[0];
  set('ses-acct', active ? esc(active.label || active.id) : '–');
  set('ses-lane', esc(h.lane || '-'));
  set('ses-up', esc(h.upstream || '-'));
  set('ses-ver', esc(h.version || '-'));
  set('ses-mid', esc(h.deviceMid || '-'));
  set('ses-auth', esc(h.auth || '-'));
  set('ses-jwt', h.planJwt && h.planJwt.present ? `ada (${h.planJwt.count || 1} akun)` : 'tidak ada');
  const state = document.getElementById('ses-state');
  if (state) {
    const off = h.live === false;
    state.className = 'badge ' + (off ? 'bad' : 'ok');
    state.textContent = off ? 'offline' : 'aktif';
  }
  set('ses-accounts', accounts.length
    ? '<ul class="lane-accounts">' + accounts.map(a => `<li class="${a.active || a.default ? 'is-active' : ''}">
        <span class="account-avatar" aria-hidden="true">${esc(accountInitials(a.label || a.id))}</span>
        <span class="lane-account-main"><b>${esc(a.label || a.id)}</b><span class="account-sub">${esc(a.email || a.id)}</span></span>
        <span class="acct-badges">${a.active || a.default ? '<span class="badge ok">aktif</span>' : '<span class="badge">siaga</span>'}${a.needsReauth ? '<span class="badge bad">perlu login</span>' : ''}</span>
      </li>`).join('') + '</ul>'
    : '<div class="empty"><b>Belum ada akun</b><span class="empty-hint">Hubungkan akun lewat halaman Akun.</span></div>');
}

function pageSession(view) {
  view.innerHTML = skeleton('Sesi operator', 'Kecepatan lane, volume hari ini, dan identitas upstream.', '<button class="btn" id="ses-refresh" type="button">Segarkan</button>') + `
  <div class="stat-grid">
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.gauge}</span><span class="stat-label">Kecepatan</span></div><div class="stat-value" id="ses-tps">-</div><div class="stat-note">token per detik</div></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.pulse}</span><span class="stat-label">Request hari ini</span></div><div class="stat-value" id="ses-req">-</div><div class="stat-note">sejak proses hidup</div></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.users}</span><span class="stat-label">Akun aktif</span></div><div class="stat-value" id="ses-acct">-</div><div class="stat-note">dipakai lane plan</div></article>
  </div>
  <div class="split">
    <article class="card"><div class="card-head"><div><h2>Lane &amp; upstream</h2><p class="section-note">Identitas yang dipakai tiap request.</p></div></div>
      <dl class="lane-facts">
        <div><dt>Lane</dt><dd id="ses-lane">-</dd></div>
        <div><dt>Upstream</dt><dd id="ses-up">-</dd></div>
        <div><dt>Status</dt><dd><span class="badge" id="ses-state">-</span></dd></div>
        <div><dt>Versi</dt><dd id="ses-ver">-</dd></div>
        <div><dt>Device mid</dt><dd id="ses-mid">-</dd></div>
        <div><dt>Autentikasi</dt><dd id="ses-auth">-</dd></div>
        <div><dt>Plan JWT</dt><dd id="ses-jwt">-</dd></div>
      </dl>
    </article>
    <article class="card"><div class="card-head"><div><h2>Akun di lane</h2><p class="section-note">Urutan pemakaian saat satu akun sibuk.</p></div></div>
      <div id="ses-accounts" aria-live="polite"><div class="empty">Membaca akun…</div></div>
    </article>
  </div>`;
  $('#ses-refresh').onclick = async () => {
    await refreshHealth();
    try { ZB.usage = await api('/v1/usage'); } catch (e) { /* keep stale */ }
    renderSession();
    toast('Sesi diperbarui', 'ok');
  };
  renderSession();
}

let accountFlow = null, accountTimer = null;

function accountBadges(a) {
  const out = [];
  if (a.expired) out.push('<span class="badge bad">kedaluwarsa</span>');
  if (a.needsReauth) out.push('<span class="badge bad">perlu login</span>');
  if (a.cooldownUntil) out.push('<span class="badge warn">jeda</span>');
  if (a.hasRefresh) out.push('<span class="badge">refresh</span>');
  return out.join('') || '<span class="badge">siap</span>';
}

function accountInitials(s) {
  const parts = String(s || '?').trim().split(/\s+/);
  return ((parts[0] || '')[0] || '?').toUpperCase() + ((parts[1] || '')[0] || '').toUpperCase();
}

function renderAccountCard() {
  const accounts = (ZB.health && ZB.health.accounts) || [];
  const box = $('#ov-accounts');
  if (!box) return;
  const note = $('#ov-accounts-note');
  if (note) note.textContent = accounts.length ? accounts.length + ' akun siap' : 'Belum ada akun tersimpan';
  if (!accounts.length) {
    box.innerHTML = '<div class="empty"><b>Belum ada akun</b><span class="empty-hint">Hubungkan akun Z.ai lewat halaman Akun.</span></div>';
    return;
  }
  const pool = (ZB.pool && ZB.pool.accounts) || [];
  box.innerHTML = '<ul class="overview-account-list">' + accounts.map(a => {
    const pr = pool.find(x => x.id === a.id);
    const rows = (pr && pr.rows) || [];
    const rem = rows.reduce((s, r) => s + (Number(r.remaining) || 0), 0);
    const active = a.active || a.default;
    const right = rows.length
      ? `<b>${poolUnits(rem)}</b>unit · ${rows.length} model`
      : '<b>–</b> tanpa grant';
    return `<li class="overview-account ${active ? 'is-active' : ''}">
      <div class="account-main"><span class="account-avatar" aria-hidden="true">${esc(accountInitials(a.label || a.id))}</span>
        <div style="min-width:0"><h3>${esc(a.label || a.id)}${a.default ? '<span class="badge ok">aktif</span>' : ''}${a.needsReauth ? '<span class="badge bad">perlu login</span>' : ''}</h3>
        <p class="account-sub">${esc(a.email || a.id)}</p></div>
      </div>
      <div class="overview-account-points">${right}</div>
    </li>`;
  }).join('') + '</ul>';
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
  const accs = p.accounts || [];
  if (!accs.length) {
    box.innerHTML = `<div class="empty"><b>Belum ada akun</b><span class="empty-hint">Hubungkan akun Z.ai pertama lewat form di bawah.</span></div>`;
    return;
  }
  box.innerHTML = '<div class="tablewrap acct-table-wrap"><table class="acct-table">' +
    '<thead><tr><th>Akun</th><th>Status</th><th>Kuota per model</th><th class="num">Sisa</th><th></th></tr></thead><tbody>' +
    accs.map(a => {
      const rows = a.rows || [];
      const rem = rows.reduce((s, r) => s + Number(r.remaining || 0), 0);
      const fit = rows.map(r => {
        const used = Number(r.used || 0), total = Number(r.total || 0);
        return `<div class="acct-fact">
          <span class="acct-model">${esc(prettyModel(r.model))}</span>
          <b>${poolUnits(r.remaining)} / ${poolUnits(r.total)}</b>
          <div class="pool-bar"><span style="width:${pct(used, total).toFixed(1)}%"></span></div>
        </div>`;
      }).join('');
      return `<tr class="${a.default ? 'is-active' : ''}">
        <td><div class="acct-who"><span class="account-avatar" aria-hidden="true">${esc(accountInitials(a.label || a.id))}</span>
          <div style="min-width:0"><b>${esc(a.label || a.id)}</b><span class="account-sub">${esc(a.email || a.id)}</span></div>
        </div></td>
        <td><span class="acct-badges">${a.default ? '<span class="badge ok">aktif</span>' : ''}${accountBadges(a)}</span>${a.error ? `<div class="acct-error">${esc(poolErrText(a.error))}</div>` : ''}</td>
        <td><div class="acct-facts">${fit || (a.error ? '<span class="acct-model">–</span>' : '<span class="acct-model">tanpa grant</span>')}</div></td>
        <td class="num">${rows.length ? `<b>${poolUnits(rem)}</b><span class="account-sub">${rows.length} model</span>` : '–'}</td>
        <td class="acct-actions">
          ${a.default ? '' : `<button class="btn ghost sm" data-acc-use="${esc(a.id)}" type="button">Aktifkan</button>`}
          <button class="btn ghost sm danger" data-acc-del="${esc(a.id)}" data-acc-label="${esc(a.label || a.id)}" type="button">Hapus</button>
        </td></tr>`;
    }).join('') + '</tbody></table></div>' +
    '<p class="small muted pool-note">Kuota dibaca langsung dari billing tiap akun; cache 45 detik. Kolom model muncul otomatis dari plan yang terdeteksi.</p>';
  bindAccountActions(box);
}

function pageAccounts(view) {
  view.innerHTML = skeleton('Akun', 'Status tiap akun dan sisa kuota per model.', '<button class="btn" id="pool-refresh" type="button">Segarkan</button>') + `
  <article class="card">
    <div class="card-head"><div><h2>Akun di pool</h2><p class="section-note">Billing dibaca per akun, cache 45 detik.</p></div></div>
    <div id="acc-pool" aria-live="polite"><div class="empty">Membaca kuota semua akun…</div></div>
  </article>
  <article class="card mt14">
    <div class="card-head"><div><h2>Hubungkan akun</h2><p class="section-note">Login Z.ai lewat tautan persetujuan.</p></div></div>
    <form id="acc-add" class="row"><input id="acc-label" class="grow" maxlength="40" placeholder="Label akun baru (mis. kerja)" autocomplete="off"><button class="btn primary" id="acc-addbtn" type="submit">Hubungkan akun</button></form>
    <div id="acc-flow"></div>
  </article>`;
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
  api('/v1/quota').then(q => { ZB.quota = q; patchOverview(); }).catch(() => {});
  api('/v1/accounts/pool').then(p => { ZB.pool = p; renderAccountCard(); }).catch(() => {});
}

/* ---------------- requests ---------------- */
function pageRequests(view) {
  view.innerHTML = skeleton('Request', 'Riwayat request di jendela proses ini.', '<button class="btn" id="req-refresh" type="button">Muat ulang</button>') + `
  <article class="card">
    <div class="card-head"><div><h2>Request terakhir</h2><p class="section-note" id="req-note">Memuat riwayat.</p></div><a class="link" href="#/usage">Penggunaan</a></div>
    <div id="req-list"><div class="empty">Memuat request…</div></div>
  </article>`;
  $('#req-refresh').onclick = async () => {
    await loadUsage(true);
    renderRequests();
    toast('Request diperbarui', 'ok');
  };
  renderRequests();
}

function renderRequests() {
  const box = $('#req-list'); if (!box) return;
  const rows = ((ZB.usage && ZB.usage.recent) || []).slice().reverse();
  const note = $('#req-note'); if (note) note.textContent = rows.length ? rows.length + ' request tercatat' : 'Kosong';
  if (!rows.length) {
    box.innerHTML = '<div class="empty"><b>Belum ada request</b><span class="empty-hint">Riwayat muncul setelah chat lewat bridge.</span></div>';
    return;
  }
  box.innerHTML = '<div class="tablewrap"><table><thead><tr><th>Waktu</th><th>Model</th><th class="num">Token</th><th class="num">t/s</th><th class="num">Status</th></tr></thead><tbody>' +
    rows.map(r => `<tr><td>${when(r.at)}</td><td>${esc(prettyModel(r.model))}</td><td class="num">${fmtN(r.output_tokens)}</td><td class="num">${fmtTps(r.tps)}</td><td class="num"><span class="badge ok">ok</span></td></tr>`).join('') +
    '</tbody></table></div>';
}

/* ---------------- quota ---------------- */
function pageQuota(view) {
  view.innerHTML = skeleton('Kuota', 'Sisa grant per akun dan model.', '<button class="btn" id="qu-refresh" type="button">Segarkan</button>') + `
  <article class="card">
    <div class="card-head">
      <div><h2>Tekanan kuota</h2><p class="section-note" id="qu-note">Membaca kuota…</p></div>
      <div class="tabs" id="qu-tabs" role="tablist" aria-label="Akun"></div>
    </div>
    <div id="qu-list"><div class="empty">Membaca kuota…</div></div>
  </article>`;
  $('#qu-refresh').onclick = () => { loadQuota(true); toast('Kuota disegarkan', 'ok'); };
  renderQuota();
  loadQuota();
}

async function loadQuota(force) {
  const jobs = [api('/v1/quota')];
  if (force || !ZB.pool) jobs.push(api('/v1/accounts/pool'));
  try {
    const [q, p] = await Promise.all(jobs);
    ZB.quota = q;
    if (p) ZB.pool = p;
  } catch (e) {
    const box = $('#qu-list');
    if (box) box.innerHTML = `<div class="empty">${esc(e.message)}</div>`;
    return;
  }
  renderQuota();
}

function quotaGroups() {
  const q = ZB.quota || {};
  const accounts = (ZB.pool && ZB.pool.accounts) || [];
  if (accounts.length) {
    return accounts.map(a => ({
      id: a.id,
      label: a.label || a.id,
      active: !!(a.active || a.default),
      error: a.error,
      rows: (a.rows || []).map(r => ({ ...r, source: 'pool' })),
    }));
  }
  return [{ id: q.account || 'akun', label: q.account || 'akun', active: true, rows: q.rows || [], error: q.errors && q.errors[0] }];
}

function renderQuota() {
  const box = $('#qu-list'); if (!box) return;
  const q = ZB.quota;
  if (!q) { box.innerHTML = '<div class="empty">Membaca kuota…</div>'; return; }
  const groups = quotaGroups();
  const tabs = $('#qu-tabs');
  if (ZB.quotaTab == null || ZB.quotaTab >= groups.length) ZB.quotaTab = Math.max(0, groups.findIndex(g => g.active));
  if (ZB.quotaTab < 0) ZB.quotaTab = 0;
  if (tabs) {
    tabs.innerHTML = groups.map((g, i) => `<button type="button" role="tab" aria-selected="${i === ZB.quotaTab}" class="${i === ZB.quotaTab ? 'on' : ''}" data-qi="${i}">${esc(g.label)}</button>`).join('');
    tabs.querySelectorAll('button[data-qi]').forEach(b => b.onclick = () => { ZB.quotaTab = Number(b.dataset.qi); renderQuota(); });
  }
  const g = groups[ZB.quotaTab];
  const note = $('#qu-note');
  if (note) note.textContent = `${g.label} · ${q.source || 'tidak diketahui'}${g.error ? ' · ' + g.error : ''}`;
  if (!g.rows.length) {
    const why = g.error || (q.errors && q.errors[0]) || 'Tidak ada grant untuk akun ini.';
    box.innerHTML = `<div class="empty"><b>Belum ada grant</b><span class="empty-hint">${esc(why)}</span></div>`;
    return;
  }
  box.innerHTML = '<div class="ring-grid">' + g.rows.map(r => {
    const total = Number(r.total_units ?? r.total) || 0, used = Number(r.used_units ?? r.used) || 0;
    const rem = r.remaining_units != null ? Number(r.remaining_units) : (r.remaining != null ? Number(r.remaining) : total - used);
    const p = total > 0 ? Math.min(100, Math.max(0, used / total * 100)) : 0;
    return `<article class="ring-card">
      <div class="ring-wrap" aria-hidden="true">
        ${ringSvg(p)}
        <span class="ring-pct">${Math.round(p)}%</span>
      </div>
      <div class="ring-main">
        <h3>${esc(prettyModel(r.model || r.plan_name || 'plan'))}</h3>
        <p><b>${fmtN(rem)}</b> sisa / ${fmtN(total)}</p>
        <span class="section-note">${fmtN(used)} terpakai</span>
      </div>
    </article>`;
  }).join('') + '</div>';
}

function ringSvg(p) {
  const r = 32, c = 2 * Math.PI * r;
  const off = c * (1 - p / 100);
  return `<svg viewBox="0 0 80 80"><circle class="ring-track" cx="40" cy="40" r="${r}"/><circle class="ring-fill" cx="40" cy="40" r="${r}" stroke-dasharray="${c.toFixed(1)}" stroke-dashoffset="${off.toFixed(1)}" transform="rotate(-90 40 40)"/></svg>`;
}

/* ---------------- lane ---------------- */
function pageLane(view) {
  view.innerHTML = skeleton('Lane', 'Identitas yang dipakai tiap request upstream.', '<button class="btn" id="ln-refresh" type="button">Segarkan</button>') + `
  <div class="stat-grid">
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.pulse}</span><span class="stat-label">Status</span></div><div class="stat-value" id="ln-state">-</div><div class="stat-note">koneksi upstream</div></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.check}</span><span class="stat-label">Akun siap</span></div><div class="stat-value" id="ln-accounts">-</div><div class="stat-note">di ring akun</div></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.hash}</span><span class="stat-label">Token plan</span></div><div class="stat-value" id="ln-jwt">-</div><div class="stat-note">JWT tersedia</div></article>
  </div>
  <article class="card">
    <div class="card-head"><h2>Info lane</h2></div>
    <dl class="lane-facts">
      <div><dt>Lane</dt><dd id="ln-lane">-</dd></div>
      <div><dt>Upstream</dt><dd id="ln-up" class="mono">-</dd></div>
      <div><dt>Versi app</dt><dd id="ln-ver">-</dd></div>
      <div><dt>Device MID</dt><dd id="ln-mid" class="mono">-</dd></div>
      <div><dt>Autentikasi</dt><dd id="ln-auth">-</dd></div>
      <div><dt>Mode</dt><dd id="ln-mode">-</dd></div>
    </dl>
  </article>`;
  $('#ln-refresh').onclick = async () => { await refreshHealth(); renderLane(); toast('Lane disegarkan', 'ok'); };
  renderLane();
}

function renderLane() {
  const h = ZB.health || {};
  const set = (id, text) => { const el = document.getElementById(id); if (el) el.textContent = text; };
  if (!document.getElementById('ln-lane')) return;
  set('ln-lane', h.lane || '-');
  set('ln-up', h.upstream || '-');
  set('ln-ver', h.version || '-');
  set('ln-mid', h.deviceMid || '-');
  set('ln-auth', h.auth || '-');
  set('ln-mode', h.live === false ? 'offline' : 'aktif');
  set('ln-accounts', fmtN((h.accounts || []).length));
  set('ln-jwt', h.planJwt && h.planJwt.present ? 'ada' : 'tidak');
  const st = document.getElementById('ln-state');
  if (st) {
    st.textContent = h.live === false ? 'offline' : 'aktif';
    st.classList.toggle('bad', h.live === false);
  }
}

/* ---------------- usage ---------------- */
function pageUsage(view) {
  view.innerHTML = skeleton('Penggunaan', 'Token, kecepatan, dan tren per model.', '<button class="btn" id="urefresh" type="button">Muat ulang</button>') + `
  <div class="stat-grid">
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.gauge}</span><span class="stat-label">Terakhir</span></div><p class="stat-value" id="ur-last">-</p><p class="stat-note">token per detik</p></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.pulse}</span><span class="stat-label">Tertinggi</span></div><p class="stat-value" id="ur-high">-</p><p class="stat-note">puncak jendela</p></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.clock}</span><span class="stat-label">Rata-rata 7 hari</span></div><p class="stat-value" id="ur-avg">-</p><p class="stat-note">token per detik</p></article>
  </div>
  <div class="grid g2 mt14">
    <article class="card"><div class="card-head"><div><h2>Hari ini</h2><p class="section-note">volume sejak proses hidup</p></div></div><div class="grid g2" id="u-today"></div></article>
    <article class="card"><div class="card-head"><div><h2>Sepanjang waktu</h2><p class="section-note">akumulasi harian</p></div></div><div class="grid g2" id="u-all"></div></article>
  </div>
  <article class="card mt14">
    <div class="card-head">
      <div><h2>Kecepatan</h2><p class="section-note" id="u-range-label">1h</p></div>
      <div class="tabs" id="urange" role="tablist" aria-label="Rentang waktu">
        ${['1h', '6h', '24h', '7d'].map(r => `<button type="button" role="tab" data-r="${r}" class="${ZB.range === r ? 'on' : ''}">${r === '7d' ? '7 hari' : r}</button>`).join('')}
      </div>
    </div>
    <div class="chartbox"><svg class="chart" id="uchart" preserveAspectRatio="none"></svg><div class="chart-tip" id="uchart-tip" hidden></div></div>
    <div class="legend"><span><i class="tps"></i>t/s</span><span><i class="out"></i>token keluar</span></div>
  </article>
  <div class="grid g2 mt14">
    <article class="card"><div class="card-head"><div><h2>Per model</h2><p class="section-note">porsi request hari ini</p></div></div><div id="umodels"></div></article>
    <article class="card"><div class="card-head"><div><h2>Aplikasi ZCode</h2><p class="section-note" id="udnote"></p></div></div><div id="udesktop"></div></article>
  </div>`;
  $('#urange').onclick = e => {
    const b = e.target.closest('button[data-r]'); if (!b) return;
    ZB.range = b.dataset.r;
    $$('#urange button').forEach(x => x.classList.toggle('on', x === b));
    drawUsageChart();
  };
  $('#urefresh').onclick = () => loadUsage(true);
  bindUsageChart();
  renderUsage();
}

function bindUsageChart() {
  const svg = $('#uchart'), tip = $('#uchart-tip');
  if (!svg || svg.dataset.hover) return;
  svg.dataset.hover = '1';
  const hide = () => {
    if (tip) tip.hidden = true;
    const g = svg.querySelector('#uchart-hover');
    if (g) g.setAttribute('hidden', '');
  };
  svg.addEventListener('mousemove', e => {
    const pts = ZB.chartPts || [];
    if (!pts.length || !tip) return;
    const W = 640, H = 180;
    const rect = svg.getBoundingClientRect();
    const vx = (e.clientX - rect.left) / rect.width * W;
    let best = null, bd = Infinity;
    pts.forEach(p => { const d = Math.abs(p.x - vx); if (d < bd) { bd = d; best = p; } });
    if (!best || bd > 36) { hide(); return; }
    const g = svg.querySelector('#uchart-hover');
    if (g) {
      g.removeAttribute('hidden');
      g.innerHTML = `<line class="chart-hover-line" x1="${best.x.toFixed(1)}" y1="12" x2="${best.x.toFixed(1)}" y2="${H - 14}"/><circle cx="${best.x.toFixed(1)}" cy="${best.y.toFixed(1)}" r="5" fill="none" stroke="var(--accent)" stroke-width="2"/>`;
    }
    tip.hidden = false;
    tip.innerHTML = `<b>${fmtTps(best.tps)} t/s</b><span>${esc(prettyModel(best.model || 'request'))} · ${fmtN(best.tokens)} token</span><span>${when(best.at)}</span>`;
    const px = best.x / W * rect.width;
    const py = best.y / H * rect.height;
    tip.style.left = Math.max(76, Math.min(rect.width - 76, px)) + 'px';
    tip.style.top = Math.max(64, py - 10) + 'px';
  });
  svg.addEventListener('mouseleave', hide);
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
  set('ur-high', fmtN(u.rings.high_tps) + ' <small>t/s</small>');
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
  const totalReq = models.reduce((s, [, x]) => s + (x.requests || 0), 0) || 1;
  set('umodels', models.length ? '<div class="ring-grid">' + models.map(([m, s]) => {
    const p = Math.min(100, Math.max(0, (s.requests || 0) / totalReq * 100));
    return `<article class="ring-card">
      <div class="ring-wrap" aria-hidden="true">${ringSvg(p)}<span class="ring-pct">${p < 10 ? p.toFixed(1) : Math.round(p)}%</span></div>
      <div class="ring-main">
        <h3>${esc(prettyModel(m))}</h3>
        <p><b>${fmtN(s.requests)}</b> request</p>
        <span class="section-note">${fmtN(s.output_tokens)} token · ${fmtTps(s.avg_tps)} t/s</span>
      </div>
    </article>`;
  }).join('') + '</div>' : '<div class="empty">Belum ada request hari ini.</div>');
  drawUsageChart();
  renderDesktop(u.desktop);
}

function drawUsageChart() {
  const u = ZB.usage; if (!u) return;
  const svg = document.getElementById('uchart'); if (!svg) return;
  const label = document.getElementById('u-range-label');
  if (label) label.textContent = ZB.range;
  const tip = document.getElementById('uchart-tip');
  const now = Date.now();
  const span = { '1h': 36e5, '6h': 216e5, '24h': 864e5, '7d': 6048e5 }[ZB.range] || 864e5;
  const rows = (u.recent || []).filter(r => now - new Date(r.at).getTime() <= span);
  const W = 640, H = 180, padL = 42, padR = 10, padT = 12, padB = 14;
  ZB.chartPts = [];
  if (tip) tip.hidden = true;
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  if (!rows.length) {
    svg.innerHTML = `<text x="${W / 2}" y="${H / 2}" text-anchor="middle" fill="var(--muted)" font-size="12">Belum ada request pada rentang ini</text>`;
    return;
  }
  let maxTps = 1, maxTok = 1;
  rows.forEach(r => { maxTps = Math.max(maxTps, r.tps || 0); maxTok = Math.max(maxTok, r.output_tokens || 0); });
  const t0 = Math.min(...rows.map(r => new Date(r.at).getTime()));
  const t1 = Math.max(...rows.map(r => new Date(r.at).getTime()), t0 + 1);
  const x = t => padL + (W - padL - padR) * ((t - t0) / (t1 - t0 || 1));
  const yTps = v => H - padB - (H - padB - padT) * ((v || 0) / maxTps);
  const yTok = v => H - padB - (H - padB - padT) * ((v || 0) / maxTok);
  const tpsPts = rows.map(r => `${x(new Date(r.at).getTime()).toFixed(1)},${yTps(r.tps).toFixed(1)}`).join(' ');
  const tokPts = rows.map(r => `${x(new Date(r.at).getTime()).toFixed(1)},${yTok(r.output_tokens).toFixed(1)}`).join(' ');
  const areaPts = `${x(t0).toFixed(1)},${H - padB} ${tpsPts} ${x(t1).toFixed(1)},${H - padB}`;
  let grid = '';
  for (let i = 0; i <= 4; i++) {
    const val = maxTps * i / 4;
    const yy = yTps(val);
    grid += `<line class="chart-grid" x1="${padL}" y1="${yy.toFixed(1)}" x2="${W - padR}" y2="${yy.toFixed(1)}"/>` +
      `<text class="chart-axis" x="${padL - 6}" y="${yy.toFixed(1)}" text-anchor="end" dominant-baseline="middle">${Math.round(val)}</text>`;
  }
  const last = rows[rows.length - 1];
  const lx = x(new Date(last.at).getTime()), ly = yTps(last.tps);
  svg.innerHTML = `<defs><linearGradient id="uarea" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="var(--accent)" stop-opacity="0.22"/>
      <stop offset="100%" stop-color="var(--accent)" stop-opacity="0"/>
    </linearGradient></defs>` +
    grid +
    `<polygon points="${areaPts}" fill="url(#uarea)"/>` +
    `<polyline points="${tokPts}" fill="none" stroke="var(--muted)" stroke-width="1.4" stroke-dasharray="3 3" opacity=".7"/>` +
    `<polyline points="${tpsPts}" fill="none" stroke="var(--accent)" stroke-width="2.2" stroke-linejoin="round"/>` +
    `<circle class="chart-live" cx="${lx.toFixed(1)}" cy="${ly.toFixed(1)}" r="4" fill="var(--accent)"/>` +
    `<g id="uchart-hover" hidden></g>`;
  ZB.chartPts = rows.map(r => ({
    at: r.at, model: r.model, tps: r.tps, tokens: r.output_tokens,
    x: x(new Date(r.at).getTime()), y: yTps(r.tps),
  }));
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

/* ---------------- claim ---------------- */
function pageClaim(view) {
  view.innerHTML = skeleton('Klaim plan', 'Cek plan weekend/trial yang tersedia lalu klaim.') + `
  <div class="toolbar"><button class="btn" id="cscan" type="button">Cek plan</button><button class="btn primary" id="cnow" type="button">Klaim sekarang</button><button class="btn" id="call" type="button">Klaim semua akun</button><span class="small muted" id="cstat"></span></div>
  <div class="stat-grid">
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.list}</span><span class="stat-label">Plan terdeteksi</span></div><p class="stat-value" id="csum-plans">–</p><p class="stat-note">dari pemindaian terakhir</p></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.clock}</span><span class="stat-label">Pemindaian terakhir</span></div><p class="stat-value" id="csum-scan">–</p><p class="stat-note">waktu lokal</p></article>
    <article class="stat-card"><div class="stat-top"><span class="stat-icon" aria-hidden="true">${ICON.check}</span><span class="stat-label">Riwayat klaim</span></div><p class="stat-value" id="csum-hist">–</p><p class="stat-note">percobaan tersimpan</p></article>
  </div>
  <div class="card mt14" id="cbody"><div class="empty"><b>Belum ada plan aktif</b><span class="empty-hint">Tekan “Cek plan” untuk memindai penawaran weekend/trial.</span></div></div>
  <div class="card mt14"><h2>Riwayat klaim</h2><div id="chist"><div class="empty">Memuat…</div></div></div>`;
  $('#cscan').onclick = () => scanClaim();
  $('#cnow').onclick = () => claimNow();
  $('#call').onclick = () => claimAll();
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

async function claimAll() {
  const btn = $('#call');
  if (btn) btn.disabled = true;
  try {
    const r = await api('/v1/claim/all', { method: 'POST', body: '{}' });
    const rows = r.results || [];
    const ok = rows.filter(x => x.ok).length;
    const need = rows.filter(x => x.status === 'perlu-app').length;
    const msg = (ZB.lang === 'en' ? `${ok}/${rows.length} claimed` : `${ok}/${rows.length} diklaim`) + (need ? (ZB.lang === 'en' ? ` · ${need} need the app` : ` · ${need} perlu app`) : '');
    toast(msg, ok ? 'ok' : 'bad');
    scanClaim();
  } catch (e) {
    toast(e.message, 'bad');
  } finally {
    if (btn) btn.disabled = false;
  }
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

/* ---------------- docs ---------------- */
function pageDocs(view) {
  const base = location.origin;
  const token = ZB.token || '<kunci-bridge>';
  const model = 'glm-5.3-flash';
  const chatURL = base + '/v1/chat/completions';
  const curl = `curl ${chatURL} \\\n  -H "Authorization: Bearer ${token}" \\\n  -H "Content-Type: application/json" \\\n  -d '{"model":"${model}","messages":[{"role":"user","content":"Halo"}],"stream":true}'`;
  const anthCurl = `curl ${base}/v1/messages \\\n  -H "x-api-key: ${token}" \\\n  -H "anthropic-version: 2023-06-01" \\\n  -H "Content-Type: application/json" \\\n  -d '{"model":"${model}","max_tokens":256,"messages":[{"role":"user","content":"Halo"}]}'`;
  const endpoints = [
    ['GET', '/health', 'Publik', 'Status lane, akun, dan JWT.'],
    ['GET', '/v1/models', 'Publik', 'Katalog model aktif dan sisa grant.'],
    ['GET', '/v1/quota', 'Publik', 'Grant per model per akun.'],
    ['POST', '/v1/chat/completions', 'OpenAI', 'Chat completion, streaming opsional.'],
    ['POST', '/v1/responses', 'OpenAI', 'Responses API.'],
    ['POST', '/v1/messages', 'Anthropic', 'Messages API, streaming opsional.'],
    ['POST', '/async/v1/chat/completions', 'OpenAI', 'Antrean async, balas 202 + id.'],
    ['POST', '/async/v1/messages', 'Anthropic', 'Antrean async, balas 202 + id.'],
    ['GET', '/async/v1/health', 'Publik', 'Status antrean async.'],
    ['POST', '/mcp', 'MCP', 'JSON-RPC lewat streamable HTTP.'],
    ['GET', '/v1/usage', 'Admin', 'Statistik pemakaian dan kecepatan.'],
    ['GET', '/v1/events', 'Admin', 'SSE snapshot dan patch.'],
    ['GET', '/v1/settings', 'Admin', 'Konfigurasi efektif bridge.'],
    ['GET', '/v1/integration', 'Admin', 'Kunci dan URL integrasi.'],
    ['GET', '/dashboard', 'Publik', 'Konsol ini.'],
  ];
  view.innerHTML = skeleton('API Docs', 'Base URL, endpoint, model, dan contoh request untuk klien.') + `
  <div class="grid g2">
    <article class="card">
      <div class="card-head"><div><h2>OpenAI compatible</h2><p class="section-note">chat completions dan responses</p></div><span class="badge">/v1</span></div>
      <p class="docs-label">Base URL</p>
      <div class="row"><code class="docs-code grow">${esc(base)}/v1</code><button class="btn ghost sm" data-copy="${esc(base)}/v1" type="button">Salin</button></div>
      <p class="docs-label">Header autentikasi</p>
      <div class="row"><code class="docs-code grow">Authorization: Bearer ${esc(token)}</code><button class="btn ghost sm" data-copy="Authorization: Bearer ${esc(token)}" type="button">Salin</button></div>
    </article>
    <article class="card">
      <div class="card-head"><div><h2>Anthropic</h2><p class="section-note">messages API</p></div><span class="badge">/v1</span></div>
      <p class="docs-label">Base URL</p>
      <div class="row"><code class="docs-code grow">${esc(base)}/v1</code><button class="btn ghost sm" data-copy="${esc(base)}/v1" type="button">Salin</button></div>
      <p class="docs-label">Header autentikasi</p>
      <div class="row"><code class="docs-code grow">x-api-key: ${esc(token)}</code><button class="btn ghost sm" data-copy="x-api-key: ${esc(token)}" type="button">Salin</button></div>
    </article>
  </div>

  <div class="docs-sec"><h2>Endpoint</h2><p class="section-note">Endpoint LLM memakai kunci bridge; endpoint admin memakai kunci yang sama.</p></div>
  <div class="tablewrap"><table>
    <thead><tr><th>Metode</th><th>Jalur</th><th>Lane</th><th>Keterangan</th></tr></thead>
    <tbody>${endpoints.map(([m, p, lane, note]) => `<tr><td><span class="method ${m.toLowerCase()}">${m}</span></td><td class="mono">${esc(p)}</td><td>${esc(lane)}</td><td class="muted">${esc(note)}</td></tr>`).join('')}</tbody>
  </table></div>

  <div class="docs-sec"><h2>Model</h2><p class="section-note" id="docs-model-note">Memuat dari /v1/models…</p></div>
  <div class="tablewrap"><table>
    <thead><tr><th>ID model</th><th>Nama</th><th>Pemilik</th><th class="num">Sisa grant</th></tr></thead>
    <tbody id="docs-models"><tr><td colspan="4" class="muted">Memuat…</td></tr></tbody>
  </table></div>

  <article class="card mt14">
    <div class="card-head">
      <div><h2>Contoh request</h2><p class="section-note" id="docs-example-note">OpenAI chat completion, streaming.</p></div>
      <div class="row"><button class="btn ghost sm" id="docs-mode" type="button">Anthropic</button><button class="btn ghost sm" id="docs-copy" type="button">Salin</button></div>
    </div>
    <pre class="snippet" id="docs-curl">${esc(curl)}</pre>
  </article>`;

  const copy = view.querySelectorAll('[data-copy]');
  copy.forEach(b => b.onclick = () => copyText(b.dataset.copy));
  let anthropic = false;
  $('#docs-mode').onclick = () => {
    anthropic = !anthropic;
    $('#docs-mode').textContent = anthropic ? 'OpenAI' : 'Anthropic';
    $('#docs-example-note').textContent = anthropic ? 'Anthropic messages, non-stream.' : 'OpenAI chat completion, streaming.';
    $('#docs-curl').textContent = anthropic ? anthCurl : curl;
  };
  $('#docs-copy').onclick = () => copyText($('#docs-curl').textContent);

  api('/v1/models').then(d => {
    const rows = d.data || [];
    const note = $('#docs-model-note');
    if (note) note.textContent = rows.length ? rows.length + ' model aktif' : 'Katalog kosong.';
    const body = $('#docs-models');
    if (!body) return;
    body.innerHTML = rows.length ? rows.map(m => `<tr>
      <td class="mono">${esc(m.id)}</td>
      <td>${esc(prettyModel(m.id))}</td>
      <td class="muted">${esc(m.owned_by || 'zcode-plan')}</td>
      <td class="num">${m.plan_remaining_tokens != null ? fmtN(m.plan_remaining_tokens) : '–'}</td>
    </tr>`).join('') : '<tr><td colspan="4" class="muted">Belum ada model.</td></tr>';
  }).catch(e => {
    const note = $('#docs-model-note');
    if (note) note.textContent = e.message;
  });
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
        <div class="row"><a class="btn ghost sm" href="/classic" target="_blank" rel="noopener">Dashboard klasik</a><a class="btn ghost sm" href="/health" target="_blank" rel="noopener">/health</a><a class="btn ghost sm" href="/v1/models" target="_blank" rel="noopener">/v1/models</a></div>
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
        <div class="row"><input id="intkey" class="grow mono" ${s.editable ? '' : 'readonly'} value="${esc(s.key || '')}" placeholder="belum di-set"><button class="btn ghost sm" id="intcopy" type="button">Salin</button></div>
      </label>
      <p class="small muted int-note">${esc(s.masked_key ? 'Tampilan: ' + s.masked_key + ' - ' : '')}${esc(source)}</p>
      <label class="field"><span>Base URL</span>
        <div class="row"><input id="intbase" class="grow mono" readonly value="${esc(s.base_url)}"><button class="btn ghost sm" id="intcopybase" type="button">Salin</button></div>
      </label>
      <div class="row mt10"><span class="small muted">Model default ${esc(s.default_model)}</span><span class="spacer"></span>
        ${s.editable ? '<button class="btn ghost sm" id="intsave" type="button">Simpan kunci</button>' : ''}
        <button class="btn ghost sm" id="introtate" type="button">${s.key_present ? 'Acak kunci' : 'Buat kunci'}</button>
        <button class="btn ghost sm danger" id="intdelete" type="button" ${s.key_present && s.editable ? '' : 'disabled'}>Hapus</button>
      </div>
      <div class="row int-head"><span class="small muted">Contoh permintaan</span><span class="spacer"></span><button class="btn ghost sm" id="intcurlcopy" type="button">Salin contoh</button></div>
      <pre class="snippet" id="intcurl">${esc(curl)}</pre>
      <p class="small muted int-tail">Klien lain memakai kunci ini sebagai <span class="mono">Authorization: Bearer</span>. Ketik kunci sendiri lalu Simpan, atau Acak untuk kunci acak. Kunci env tidak bisa diubah dari dashboard.</p>`;
    $('#intcopy').onclick = () => copyText(s.key || '');
    $('#intcopybase').onclick = () => copyText(s.base_url);
    $('#intcurlcopy').onclick = () => copyText(curl);
    const save = $('#intsave');
    if (save) save.onclick = async () => {
      const key = ($('#intkey').value || '').trim();
      if (!key) { toast('Kunci tidak boleh kosong', 'bad'); $('#intkey').focus(); return; }
      if (s.key_present && !confirm('Simpan kunci baru? Klien yang memakai kunci lama langsung ditolak.')) return;
      save.disabled = true;
      try {
        const r = await api('/v1/integration/key', { method: 'POST', body: JSON.stringify({ key }) });
        ZB.token = r.key;
        sessionStorage.setItem('zbToken', r.key);
        toast('Kunci disimpan dan dipakai tab ini', 'ok');
        connectSSE(); refreshHealth(); loadIntegration();
      } catch (e) { toast(e.message, 'bad'); save.disabled = false; }
    };
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
  const dot = $('#livedot'); if (dot) { dot.classList.remove('bad'); dot.classList.add('on'); }
  const sub = $('#brandsub'); if (sub) sub.textContent = h.auth || 'lane lokal';
  const ver = $('#sidever'); if (ver && h.version) ver.textContent = 'v' + h.version;
  const accounts = h.accounts || [];
  const acct = accounts.find(a => a.active || a.default) || accounts[0];
  const init = $('#sideacct-init'); if (init) init.textContent = acct ? accountInitials(acct.label || acct.id) : '–';
  const name = $('#sideacct-name'); if (name) name.textContent = acct ? (acct.label || acct.id) : 'Belum ada akun';
  const asub = $('#sideacct-sub'); if (asub) asub.textContent = acct ? (acct.email || acct.id) : 'hubungkan lewat halaman Akun';
  if (ZB.page === 'overview') { patchOverview(); renderAccountCard(); }
  if (ZB.page === 'session') renderSession();
  if (ZB.page === 'lane') renderLane();
  if (ZB.page === 'quota') renderQuota();
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
    const lw = $('#liveword'); if (lw) lw.textContent = 'terhubung';
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
    const lw = $('#liveword'); if (lw) lw.textContent = 'menyambung';
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
  patchSideStats();
  const h = ZB.health || {};
  if (document.getElementById('st-accounts') && ZB.page === 'overview') patchOverview();
  if (document.getElementById('ur-last') && ZB.page === 'usage') renderUsage();
  if (document.getElementById('req-list') && ZB.page === 'requests') renderRequests();
  if (document.getElementById('ln-lane') && ZB.page === 'lane') renderLane();
  if (document.getElementById('qu-list') && ZB.page === 'quota') renderQuota();
  if (document.getElementById('ses-tps') && ZB.page === 'session') renderSession();
}

/* ---------------- boot ---------------- */
function toggleTheme() { cycleTheme(); }

const THEME_KEY = 'zbTheme';
function themeMode() {
  const v = localStorage.getItem(THEME_KEY);
  return v === 'light' || v === 'dark' || v === 'system' ? v : 'system';
}
function systemDark() {
  return !!(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
}
function applyTheme(mode) {
  document.documentElement.dataset.theme = mode === 'system' ? (systemDark() ? 'dark' : 'light') : mode;
  localStorage.setItem(THEME_KEY, mode);
  $$('input[name="theme"]').forEach(i => { i.checked = i.value === mode; });
}
function cycleTheme() {
  const order = ['system', 'dark', 'light'];
  applyTheme(order[(order.indexOf(themeMode()) + 1) % order.length]);
}

function boot() {
  applyTheme(themeMode());
  $$('input[name="theme"]').forEach(i => i.addEventListener('change', () => { if (i.checked) applyTheme(i.value); }));
  if (window.matchMedia) {
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if (themeMode() === 'system') applyTheme('system'); });
  }
  const logout = $('#logoutbtn');
  if (logout) logout.onclick = () => {
    if (ZB.esAbort) ZB.esAbort.abort();
    sessionStorage.removeItem('zbToken');
    ZB.token = '';
    location.reload();
  };
  $('#tokenform').addEventListener('submit', e => {
    e.preventDefault();
    ZB.token = $('#tokeninput').value.trim();
    sessionStorage.setItem('zbToken', ZB.token);
    const d = $('#tokendlg'); if (d.close) d.close(); else d.removeAttribute('open');
    toast('Token disimpan', 'ok');
    connectSSE(); route(); refreshHealth();
  });
  $('#tokencancel').onclick = () => { const d = $('#tokendlg'); if (d.close) d.close(); else d.removeAttribute('open'); };
  $('#gateform').addEventListener('submit', async e => {
    e.preventDefault();
    const err = $('#gateerr');
    const btn = $('#gateform button[type="submit"]');
    const pass = ($('#gatepass').value || '').trim();
    if (!pass) { showGateError('Kata sandi tidak boleh kosong.'); $('#gatepass').focus(); return; }
    btn.disabled = true;
    if (err) err.hidden = true;
    const ok = await tokenOK(pass);
    btn.disabled = false;
    if (!ok) { showGateError('Kata sandi salah.'); $('#gatepass').select(); return; }
    ZB.token = pass;
    sessionStorage.setItem('zbToken', pass);
    $('#gatepass').value = '';
    startApp();
  });
  authenticate();
}

async function tokenOK(token) {
  try {
    const res = await fetch('/v1/settings', { cache: 'no-store', headers: token ? { Authorization: 'Bearer ' + token } : {} });
    return res.status === 200;
  } catch (e) {
    return false;
  }
}

async function authenticate() {
  const saved = (sessionStorage.getItem('zbToken') || '').trim();
  if (saved && await tokenOK(saved)) {
    ZB.token = saved;
    startApp();
    return;
  }
  sessionStorage.removeItem('zbToken');
  try {
    const res = await fetch('/v1/integration', { cache: 'no-store' });
    if (res.ok) {
      const s = await res.json();
      if (!s.key_present) { startApp(); return; }
    }
  } catch (e) {}
  const app = $('#app');
  if (app) app.hidden = true;
  const gate = $('#gate');
  if (gate) gate.hidden = false;
  const pass = $('#gatepass');
  if (pass) setTimeout(() => pass.focus(), 50);
}

function showGateError(msg) {
  const err = $('#gateerr');
  if (!err) return;
  err.textContent = msg;
  err.hidden = false;
}

function startApp() {
  if (ZB.started) return;
  ZB.started = true;
  const app = $('#app');
  if (app) app.hidden = false;
  const gate = $('#gate');
  if (gate) gate.hidden = true;
  route();
  applyLanguage();
  refreshHealth();
  api('/v1/usage').then(u => { ZB.usage = u; patchSideStats(); if (ZB.page === 'overview') patchOverview(); if (ZB.page === 'usage') renderUsage(); if (ZB.page === 'requests') renderRequests(); if (ZB.page === 'session') renderSession(); }).catch(() => {});
  connectSSE();
  setInterval(refreshHealth, 15000);
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape' && ZB.page === 'chat' && window.ZChat) window.ZChat.escape();
  });
}
if (document.readyState === 'complete') boot(); else window.addEventListener('load', boot, { once: true });
