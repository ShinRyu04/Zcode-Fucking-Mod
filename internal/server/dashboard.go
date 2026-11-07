package server

import (
	"io"
	"net/http"
)

// handleDashboard serves the built-in status page at GET /dashboard.
// The page reads /health, /quota, /inspect and (when metering lands) /v1/usage
// on the same origin, so it needs no CORS setup and no external assets. It is
// observability only: everything here is also available as JSON.
func handleDashboard(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, dashboardHTML)
}

// dashboardHTML is a single-file page. Design intent: a local developer tool
// that lives next to a terminal, so it is dark, flat, and calm: one accent
// color (green = healthy lane), status colors only where they carry state, no
// animation beyond the data refresh itself. Tabs separate three reads of the
// same lane: live status, captured traffic, and metered throughput.
const dashboardHTML = `<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>zcode-bridge - dashboard</title>
<style>
:root{--bg:#0c0e13;--card:#151922;--line:#242b38;--fg:#e8ecf4;--dim:#9aa4b5;--ok:#34d399;--warn:#fbbf24;--bad:#f87171}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.55 ui-sans-serif,system-ui,"Segoe UI",sans-serif;padding:28px 16px 40px}
.wrap{max-width:960px;margin:0 auto}
header p{margin:2px 0 0;color:var(--dim);font-size:13px}
h1{margin:0;font-size:20px;letter-spacing:-.01em}
main{margin-top:14px}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:18px 20px;margin-bottom:14px}
.card h2{margin:0 0 12px;font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.1em;color:var(--dim)}
.card h3{margin:18px 0 8px;font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.08em;color:var(--dim)}
.kv{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}
.kv span{display:block;font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--dim)}
.kv b{font-weight:600;font-size:14px;overflow-wrap:anywhere}
.big{font-size:32px;font-weight:800;letter-spacing:-.02em;margin:10px 0 2px;font-variant-numeric:tabular-nums}
.big small{font-size:14px;font-weight:600;color:var(--dim)}
.bar{height:10px;border-radius:6px;background:#1d2330;overflow:hidden;margin:12px 0 8px}
.fill{height:100%;border-radius:6px;background:var(--ok);transition:width .3s ease}
.fill.warn{background:var(--warn)}.fill.bad{background:var(--bad)}
.meta{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin-top:14px}
.meta span{display:block;font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--dim)}
.meta b{font-weight:600;font-size:14px;font-variant-numeric:tabular-nums}
.tag{display:inline-block;font-size:12px;padding:2px 9px;border-radius:6px;border:1px solid var(--line);color:var(--dim);vertical-align:middle;white-space:nowrap}
.tag.ok{color:var(--ok);border-color:#1d4a38}
.tag.warn{color:var(--warn);border-color:#54401a}
.tag.bad{color:var(--bad);border-color:#5c2323}
.tablewrap{overflow-x:auto}
table{width:100%;border-collapse:collapse;font-size:14px}
th,td{text-align:left;padding:9px 10px;border-bottom:1px solid var(--line);white-space:nowrap}
th{font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--dim);font-weight:600}
td.num,th.num{text-align:right;font-variant-numeric:tabular-nums}
tr:last-child td{border-bottom:none}
.note{color:var(--dim);font-size:14px;margin:4px 0 0}
.note code,.mono,pre{font-family:ui-monospace,SFMono-Regular,Consolas,monospace}
.note code{background:#1d2330;border:1px solid var(--line);border-radius:6px;padding:1px 7px;font-size:13px}
.errbox{border-color:#5c2323}
.errbox .msg{color:var(--bad);margin:4px 0 10px;overflow-wrap:anywhere}
footer{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;color:var(--dim);font-size:12px;margin-top:6px}
.controls{display:flex;align-items:center;gap:14px;flex-wrap:wrap}
button,.btn{background:transparent;border:1px solid var(--line);border-radius:8px;color:var(--fg);font:inherit;font-size:13px;padding:7px 14px;cursor:pointer;text-decoration:none;display:inline-block}
button:hover,.btn:hover{border-color:var(--ok)}
button:active{background:#1d2330}
button[aria-pressed=true]{border-color:var(--ok);color:var(--ok);background:rgba(52,211,153,.08)}
button.danger:hover{border-color:var(--bad);color:var(--bad)}
label{cursor:pointer;display:inline-flex;align-items:center;gap:6px}
:focus-visible{outline:2px solid var(--ok);outline-offset:2px}
.dim{color:var(--dim)}
.tabs{display:flex;gap:4px;border-bottom:1px solid var(--line);margin-top:18px}
.tabs [role=tab]{border:0;border-bottom:2px solid transparent;border-radius:0;padding:9px 14px;color:var(--dim);background:none}
.tabs [role=tab][aria-selected=true]{color:var(--fg);border-bottom-color:var(--ok)}
.toolbar{display:flex;gap:12px;align-items:flex-end;flex-wrap:wrap;margin-bottom:10px}
.field{display:flex;flex-direction:column;gap:4px;cursor:default}
.field span{font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--dim)}
.field.grow{flex:1 1 220px}
select,input[type=search],input[type=password]{background:#0f131b;border:1px solid var(--line);border-radius:8px;color:var(--fg);font:inherit;font-size:13px;padding:8px 10px;min-width:0}
select:hover,input:hover{border-color:#35405550}
.toolbtns{display:flex;gap:8px;margin-left:auto;flex-wrap:wrap}
.mth{display:inline-block;font-size:11px;font-weight:700;letter-spacing:.06em;padding:1px 6px;border-radius:4px;border:1px solid var(--line);color:var(--dim);font-family:ui-monospace,SFMono-Regular,Consolas,monospace}
tbody tr[role=button]{cursor:pointer}
tbody tr[role=button]:hover td{background:#1a1f2a}
tbody tr.sel td{background:#1d2534}
tbody tr.sel td:first-child{box-shadow:inset 2px 0 0 var(--ok)}
td.path{max-width:340px;overflow:hidden;text-overflow:ellipsis;font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:13px}
.detail{border-top:1px solid var(--line);margin-top:14px;padding-top:6px}
.dhead{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin:10px 0 4px;overflow-wrap:anywhere}
.dhead code{font-size:13px}
.actions{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin:4px 0 10px;font-size:12px}
.actions .dim{font-size:11px}
.cols{display:grid;grid-template-columns:1fr 1fr;gap:20px}
pre{background:#0f131b;border:1px solid var(--line);border-radius:8px;padding:12px;margin:0;font-size:12.5px;line-height:1.5;overflow:auto;max-height:340px;white-space:pre-wrap;overflow-wrap:anywhere}
.events{display:grid;gap:6px}
.ev{display:flex;gap:8px;align-items:baseline;flex-wrap:wrap;font-size:13px;border:1px solid var(--line);border-radius:8px;padding:6px 10px}
.emptystate{border:1px dashed var(--line);border-radius:10px;padding:22px;text-align:center;color:var(--dim)}
.qnote{color:var(--dim);font-size:13px;margin:0 0 8px}
.chartwrap{background:#0f131b;border:1px solid var(--line);border-radius:8px;padding:10px 12px}
.chartwrap svg{display:block;width:100%;height:120px}
.legend{display:flex;gap:14px;flex-wrap:wrap;font-size:12px;color:var(--dim);margin-top:10px}
.legend span{display:inline-flex;align-items:center;gap:5px}
.swatch{display:inline-block;width:8px;height:8px;border-radius:2px}
.split{display:grid;grid-template-columns:1.1fr .9fr;gap:20px;align-items:start}
.gauges{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin-bottom:6px}
.gauge{background:#0f131b;border:1px solid var(--line);border-radius:10px;padding:12px 10px 10px}
.gauge svg{display:block;width:100%;max-width:180px;margin:0 auto}
.gauge .gval{fill:var(--fg);font-size:24px;font-weight:800;font-variant-numeric:tabular-nums}
.gauge .gsub{fill:var(--dim);font-size:8px;letter-spacing:.06em;text-transform:uppercase}
.gcap{text-align:center;margin-top:4px}
.gcap b{display:block;font-size:12px;font-weight:600}
.gcap span{display:block;font-size:11px;color:var(--dim);overflow-wrap:anywhere}
@media (max-width:760px){.split{grid-template-columns:1fr}}
@media (max-width:640px){.gauges{grid-template-columns:1fr 1fr}}
@media (prefers-reduced-motion:reduce){.fill{transition:none}}
@media (max-width:760px){.cols{grid-template-columns:1fr}.toolbtns{margin-left:0;width:100%}.toolbtns .btn,.toolbtns button{flex:1 1 auto;text-align:center}}
@media (max-width:560px){.big{font-size:26px}.card{padding:16px}th,td{white-space:normal;overflow-wrap:anywhere}td.path{max-width:none}}
.modules{border:1px solid var(--line);border-radius:10px;padding:10px 14px;margin:0 0 18px;background:#0d1119}
.modules summary{cursor:pointer;font-size:13px;color:var(--dim);font-weight:600}
.modgrid{display:flex;flex-wrap:wrap;gap:10px 22px;margin-top:10px;font-size:13px}
.modgrid label{display:inline-flex;align-items:center;gap:6px;cursor:pointer}
.modgrid input{accent-color:#34d399}
.statusbar{position:sticky;bottom:0;display:flex;flex-wrap:wrap;gap:6px 18px;align-items:center;font-size:12px;color:var(--dim);background:rgba(9,12,17,.96);border:1px solid var(--line);border-radius:10px;padding:8px 14px;margin:18px 0 6px;font-variant-numeric:tabular-nums;z-index:5}
.statusbar b{color:var(--fg);font-weight:600}
.statusbar .sep{color:#2a3242}
.netgrid{display:grid;grid-template-columns:repeat(auto-fit,minmax(130px,1fr));gap:10px;margin:10px 0}
.netstat{background:#0f131b;border:1px solid var(--line);border-radius:10px;padding:10px 12px}
.netstat span{display:block;font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:var(--dim)}
.netstat b{display:block;font-size:20px;font-weight:800;margin-top:3px;font-variant-numeric:tabular-nums}
.netstat small{display:block;color:var(--dim);font-size:11px;margin-top:2px}
@media (max-width:560px){.statusbar{font-size:11px}}
</style>
</head>
<body>
<div class="wrap">
<header>
<h1>zcode-bridge</h1>
<p>Status lane, kredensial, kuota, dan lalu lintas yang lewat bridge ini. Data langsung dari proses lokal.</p>
</header>

<div class="tabs" role="tablist" aria-label="Panel dashboard">
<button role="tab" id="tab-status" aria-controls="panel-status" aria-selected="true" data-panel="panel-status">Status</button>
<button role="tab" id="tab-inspector" aria-controls="panel-inspector" aria-selected="false" data-panel="panel-inspector">Inspector</button>
<button role="tab" id="tab-usage" aria-controls="panel-usage" aria-selected="false" data-panel="panel-usage">Penggunaan</button>
<button role="tab" id="tab-playground" aria-controls="panel-playground" aria-selected="false" data-panel="panel-playground">Playground</button>
</div>

<details class="modules" id="modules">
<summary>Modul tampilan</summary>
<div class="modgrid" id="modgrid" aria-label="Pilih modul dashboard"></div>
</details>

<main>
<section class="card" id="tokenask" hidden aria-label="Token bridge">
<h2>Token bridge diperlukan</h2>
<p class="msg" id="tokenmsg">Endpoint ini dijaga ZCODE_BRIDGE_TOKEN.</p>
<div class="toolbar">
<label class="field grow"><span>Token</span><input type="password" id="tokeninput" autocomplete="off" placeholder="nilai ZCODE_BRIDGE_TOKEN"></label>
<div class="toolbtns"><button type="button" id="tokensave">Simpan</button></div>
</div>
<p class="note">Token disimpan di sesi tab ini saja, tidak dikirim ke mana pun selain bridge lokal.</p>
</section>

<section id="panel-status" role="tabpanel" aria-labelledby="tab-status">
<section class="card" id="health" aria-label="Kredensial dan lane"><h2>Kredensial dan lane</h2><p class="note">memuat...</p></section>
<section class="card" id="quota" aria-label="Kuota plan"><h2>Kuota plan</h2><p class="note">memuat...</p></section>
<section class="card" id="claimcard" aria-label="Klaim plan">
<h2>Klaim plan <span class="dim" style="text-transform:none;letter-spacing:0;font-weight:400">weekend / trial</span></h2>
<div class="actions"><button type="button" id="claimlist">Cek plan</button><span class="dim" id="claimstate"></span></div>
<div id="claimbox"><p class="note">Tekan Cek plan untuk melihat kampanye yang bisa diklaim.</p></div>
</section>
<section class="card" id="netcard" aria-label="Monitor jaringan">
<h2>Monitor jaringan <span class="dim" id="netstate" style="text-transform:none;letter-spacing:0;font-weight:400"></span></h2>
<div class="netgrid" id="netsum"><div class="netstat"><span>Turun</span><b>-</b></div></div>
<div id="netif" class="tablewrap"></div>
<div id="netproc"></div>
</section>
</section>

<section id="panel-inspector" role="tabpanel" aria-labelledby="tab-inspector" hidden>
<div class="card">
<h2>Inspector <span class="dim" id="icount" style="text-transform:none;letter-spacing:0;font-weight:400"></span></h2>
<div class="toolbar">
<label class="field"><span>Metode</span>
<select id="imethod"><option value="">semua</option><option>POST</option><option>GET</option><option>DELETE</option></select>
</label>
<label class="field"><span>Status</span>
<select id="istatus">
<option value="">semua</option>
<option value="2xx">2xx</option>
<option value="3xx">3xx</option>
<option value="4xx">4xx</option>
<option value="5xx">5xx</option>
<option value="err">punya kode error</option>
<option value="3012">kode 3012</option>
<option value="3008">kode 3008</option>
<option value="3009">kode 3009</option>
<option value="1113">kode 1113</option>
</select>
</label>
<label class="field grow"><span>Cari path</span>
<input type="search" id="iq" placeholder="mis. chat/completions" autocomplete="off">
</label>
<div class="toolbtns">
<button type="button" id="irefresh">Muat ulang</button>
<button type="button" id="iclear" class="danger">Bersihkan</button>
<a class="btn" id="iexport" href="/inspect?limit=200&amp;full=1" download="zcode-inspect.json">Ekspor JSON</a>
<a class="btn" id="ihar" href="/inspect?format=har&amp;limit=50" download="zcode-session.har">Ekspor HAR</a>
</div>
</div>
<p class="qnote" id="istate">memuat...</p>
<div id="ilist" class="tablewrap" aria-live="polite"></div>
<div id="idetail"></div>
</div>
</section>

<section id="panel-usage" role="tabpanel" aria-labelledby="tab-usage" hidden>
<div class="card" aria-label="Penggunaan token dan kecepatan">
<h2>Penggunaan <span class="dim" id="ustate" style="text-transform:none;letter-spacing:0;font-weight:400"></span></h2>
<div class="gauges" id="uga"></div>
<h3>Hari ini</h3>
<div class="meta" id="utotals"></div>
<h3>Semua waktu</h3>
<div class="meta" id="ualltime"></div>
<h3>Kecepatan <span class="dim" style="text-transform:none;letter-spacing:0;font-weight:400">· pilih rentang</span></h3>
<div class="toolbtns" id="urange" role="group" aria-label="Rentang kecepatan"></div>
<div id="uroll"></div>
<h3>Tren per model</h3>
<div id="umtrend"></div>
<h3>Kecepatan per permintaan terakhir</h3>
<div id="uchart"></div>
<h3>Per model</h3>
<div id="utable" class="tablewrap" aria-live="polite"></div>
<h3>Aplikasi ZCode <span class="dim" style="text-transform:none;letter-spacing:0;font-weight:400">· dibaca dari db.sqlite, terpisah dari trafik bridge</span></h3>
<div class="meta" id="udsum"></div>
<div class="toolbar">
<div class="toolbtns" id="udrange" role="group" aria-label="Rentang grafik aplikasi"></div>
<div class="toolbtns" id="udview" role="group" aria-label="Tampilan grafik aplikasi"></div>
</div>
<div id="udchart"></div>
<div id="udmodels" class="tablewrap" aria-live="polite"></div>
</div>
</section>

<section id="panel-playground" role="tabpanel" aria-labelledby="tab-playground" hidden>
<div class="card" aria-label="Playground chat">
<h2>Playground <span class="dim" id="pstate" style="text-transform:none;letter-spacing:0;font-weight:400">uji model langsung dari dashboard</span></h2>
<div class="toolbar">
<label class="field"><span>Model</span><input type="search" id="pmodel" value="glm-5.3-flash" autocomplete="off" spellcheck="false"></label>
<label class="field grow"><span>Pesan</span><input type="search" id="pmsg" placeholder="tulis prompt, Enter untuk kirim" autocomplete="off"></label>
<label><input type="checkbox" id="pstream" checked> stream</label>
<div class="toolbtns"><button type="button" id="psend">Kirim</button></div>
</div>
<div id="pout" class="card" style="margin:0;background:#0f131b"><p class="note">Balasan muncul di sini; kalau model mengirim thinking, ia tampil di panel reasoning.</p></div>
<div class="meta" id="pmeta"></div>
<p class="note">Lewat jalur yang sama dengan klien lain: plan lane, 3 blok identitas, id model huruf kecil. Token tidak disimpan.</p>
</div>
</section>
</main>

<div class="statusbar" id="statusbar" role="status" aria-live="polite">
<span>lane <b id="sb-lane">-</b></span><span class="sep">·</span>
<span>akun <b id="sb-acc">-</b></span><span class="sep">·</span>
<span>permintaan <b id="sb-req">-</b></span><span class="sep">·</span>
<span>kecepatan <b id="sb-tps">-</b></span><span class="sep">·</span>
<span>jaringan <b id="sb-net">-</b></span><span class="sep">·</span>
<span id="sb-state">memuat...</span>
</div>

<footer>
<div class="controls">
<button type="button" id="refresh">Muat ulang</button>
<a class="btn" href="/overlay" target="_blank" rel="noopener">Overlay t/s</a>
<label><input type="checkbox" id="auto" checked> auto 15 dtk</label>
</div>
<div><span id="stamp">-</span><span id="ver" class="dim"></span></div>
</footer>
</div>
<script>
(function(){
'use strict';
function byId(id){return document.getElementById(id);}
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];});}
function fmt(n){n=Number(n)||0;if(n>=1e9)return (n/1e9).toFixed(2)+' B';if(n>=1e6)return (n/1e6).toFixed(1)+' M';if(n>=1e3)return (n/1e3).toFixed(0)+' K';return String(n);}
function fmtBytes(n){n=Number(n)||0;if(n>=1024*1024)return (n/1024/1024).toFixed(1)+' MB';if(n>=1024)return (n/1024).toFixed(1)+' KB';return n+' B';}
function when(t){if(!t)return '-';return new Date(t*1000).toLocaleString('id-ID',{day:'2-digit',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit'});}
function whenISO(t){if(!t)return '-';var d=new Date(t);return isNaN(d.getTime())?'-':d.toLocaleString('id-ID',{day:'2-digit',month:'short',hour:'2-digit',minute:'2-digit',second:'2-digit'});}
function clock(t){if(!t)return '-';var d=new Date(t);if(isNaN(d.getTime()))return '-';return d.toLocaleTimeString('id-ID',{hour12:false});}
function left(t){if(!t)return '-';var s=Math.max(0,t-Date.now()/1000);var d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60);if(d>0)return d+' hari '+h+' jam';if(h>0)return h+' jam '+m+' mnt';return m+' mnt';}
function now(){return new Date().toLocaleTimeString('id-ID');}
function errCard(title,msg){return '<h2>'+title+'</h2><div class="msg">'+esc(msg)+'</div><p class="note">Periksa apakah bridge masih jalan, lalu klik Muat ulang.</p>';}
function tag(text,cls){return '<span class="tag'+(cls?' '+cls:'')+'">'+esc(text)+'</span>';}

var TOKEN = sessionStorage.getItem('zbToken') || '';
function api(url,opts){
  opts = opts || {};
  var headers = opts.headers || {};
  if(TOKEN) headers['Authorization'] = 'Bearer ' + TOKEN;
  opts.headers = headers;
  opts.cache = 'no-store';
  return fetch(url,opts).then(function(r){
    if(r.status === 401){ showTokenAsk(); throw new Error('token diperlukan'); }
    return r.json();
  });
}
function showTokenAsk(){
  byId('tokenask').hidden = false;
  byId('tokenask').classList.add('errbox');
}
byId('tokensave').addEventListener('click', function(){
  var v = byId('tokeninput').value.trim();
  if(!v) return;
  TOKEN = v;
  sessionStorage.setItem('zbToken', v);
  byId('tokenask').hidden = true;
  byId('tokeninput').value = '';
  loadInspector();
});
byId('claimlist').addEventListener('click', loadClaimPlans);

// ---- status tab ----
function renderHealth(h){
  var el = byId('health');
  if(!h || h.ok === false){el.className='card errbox';el.innerHTML=errCard('Kredensial dan lane','/health tidak merespons dengan benar.');return;}
  var jwt = h.planJwt || {};
  var jwtTxt = jwt.present ? (jwt.count ? 'ada ('+jwt.count+' akun)' : 'ada') : 'tidak ada';
  var html = '<div class="kv">'
    +'<div><span>Lane</span><b>'+esc(h.lane||'-')+'</b></div>'
    +'<div><span>Upstream</span><b>'+esc(h.upstream||'-')+'</b></div>'
    +'<div><span>Versi</span><b>'+esc(h.version||h.clientVersion||'-')+'</b></div>'
    +'<div><span>Device mid</span><b>'+esc(h.deviceMid||'-')+'</b></div>'
    +'<div><span>Auth</span><b>'+esc(h.auth||'-')+'</b></div>'
    +'<div><span>Token plan</span><b>'+esc(jwtTxt)+'</b></div>'
    +'</div>';
  var accs = h.accounts || [];
  if(accs.length){
    var opts = '';
    for(var j=0;j<accs.length;j++){
      var aj = accs[j];
      opts += '<option value="'+esc(aj.id)+'"'+(aj.default?' selected':'')+'>'+esc(aj.label||aj.id)+(aj.needsReauth?' (perlu login)':'')+'</option>';
    }
    html += '<div class="actions"><label class="field" style="flex-direction:row;align-items:center;gap:8px"><span>Akun aktif</span><select id="accsel">'+opts+'</select></label>'
      +'<button type="button" id="accapply">Terapkan</button><span class="dim" id="accstate"></span></div>';
    html += '<div class="tablewrap"><table><tr><th>Akun</th><th>Email</th><th>Status</th><th>Refresh token</th><th class="num">Kedaluwarsa</th></tr>';
    for(var i=0;i<accs.length;i++){
      var a = accs[i];
      var st,cls;
      if(a.needsReauth){st='perlu login ulang';cls='bad';}
      else if(a.cooldownUntil){st='cooldown s.d. '+esc(String(a.cooldownUntil).replace('T',' ').slice(0,16));cls='warn';}
      else if(a.expired){st='kedaluwarsa';cls='warn';}
      else{st=(a.default?'utama':'siap');cls=a.default?'ok':'';}
      var exp = a.expiresAt ? when(a.expiresAt/1000) : 'tidak pernah';
      html += '<tr><td>'+esc((a.default?'[utama] ':'')+(a.label||a.id||'-'))+'</td>'
        +'<td class="dim">'+esc(a.email||'-')+'</td>'
        +'<td>'+tag(st,cls)+'</td>'
        +'<td>'+(a.hasRefresh?'ada':'tidak')+'</td>'
        +'<td class="num dim">'+esc(exp)+'</td></tr>';
    }
    html += '</table></div>';
  } else {
    html += '<p class="note">Belum ada akun terdaftar. Jalankan <code>zcode-bridge login</code> untuk menambahkan.</p>';
  }
  el.className='card';el.innerHTML=html;
  var accApply = byId('accapply');
  if(accApply) accApply.addEventListener('click', switchAccount);
  byId('ver').textContent = h.version ? ' · v'+h.version : '';
  sb('lane', h.lane || '-');
  var primary = (accs.filter(function(a){ return a.default; })[0]) || accs[0];
  sb('acc', primary ? (primary.label || primary.id || '-') : 'belum ada');
}

function switchAccount(){
  var sel = byId('accsel');
  var note = byId('accstate');
  if(!sel) return;
  var id = sel.value;
  if(note) note.textContent = 'menerapkan...';
  api('/v1/accounts/switch', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({id:id})}).then(function(res){
    if(!res || res.ok === false){
      if(note) note.textContent = 'gagal: '+((res&&res.error)||'tidak diketahui');
      return;
    }
    if(note) note.textContent = 'akun aktif: '+id;
    tick();
  }).catch(function(e){
    if(note) note.textContent = 'gagal: '+e.message;
  });
}

function loadClaimPlans(){
  var box = byId('claimbox');
  var note = byId('claimstate');
  if(note) note.textContent = 'memuat...';
  api('/v1/claim').then(function(res){
    if(!res || res.ok === false){
      box.innerHTML = '<p class="note">Gagal: '+esc((res&&res.error)||'tidak diketahui')+'</p>';
      if(note) note.textContent = '';
      return;
    }
    if(note) note.textContent = (res.enabled?'auto '+(res.auto?'aktif':'nonaktif'):'nonaktif');
    var plans = res.plans || [];
    if(!plans.length){
      box.innerHTML = '<p class="note">Tidak ada kampanye aktif untuk akun ini.</p>';
      return;
    }
    var html = '<div class="tablewrap"><table><tr><th>Plan</th><th>Sumber</th><th class="num">Prioritas</th><th></th></tr>';
    for(var i=0;i<plans.length;i++){
      var p = plans[i];
      html += '<tr><td>'+esc(p.name||p.plan_id)+'</td><td class="dim">'+esc(p.source||'-')+'</td>'
        +'<td class="num">'+(Number(p.priority)||0)+'</td>'
        +'<td><button type="button" class="claimbtn" data-plan="'+esc(p.plan_id)+'">Klaim</button></td></tr>';
    }
    box.innerHTML = html+'</table></div>';
    var btns = box.querySelectorAll('.claimbtn');
    for(var j=0;j<btns.length;j++) btns[j].addEventListener('click', function(){ claimPlan(this.getAttribute('data-plan')); });
  }).catch(function(e){
    box.innerHTML = '<p class="note">Gagal: '+esc(e.message)+'</p>';
    if(note) note.textContent = '';
  });
}
function claimPlan(planId){
  if(!window.confirm('Klaim plan '+planId+' sekarang?')) return;
  var note = byId('claimstate');
  if(note) note.textContent = 'mengklaim '+planId+'...';
  api('/v1/claim', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({plan_id:planId})}).then(function(res){
    if(!res || res.ok === false){
      if(note) note.textContent = 'gagal: '+((res&&res.error)||'tidak diketahui');
      return;
    }
    if(note) note.textContent = 'plan '+planId+' diklaim';
    tick();
  }).catch(function(e){
    if(note) note.textContent = 'gagal: '+e.message;
  });
}

function renderQuota(res){
  var el = byId('quota');
  if(!res || res.status !== 'fulfilled'){
    var why = res && res.reason ? String(res.reason.message || res.reason) : 'tidak diketahui';
    el.className='card errbox';el.innerHTML=errCard('Kuota plan','Gagal memuat /quota: '+why);return;
  }
  var q = res.value;
  if(!q || q.ok === false){
    var msg = (q && (q.error || (q.errors && q.errors.join('; ')))) || 'respons tidak dikenal';
    el.className='card errbox';el.innerHTML=errCard('Kuota plan',msg);return;
  }
  var rows = q.rows || [];
  if(!rows.length){
    el.className='card';
    el.innerHTML='<h2>Kuota plan</h2><p class="note">Belum ada plan aktif untuk akun ini. Cek dari app ZCode atau jalankan <code>zcode-bridge login</code>.</p>';
    return;
  }
  var html = '<div style="display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap">'
    +'<span class="dim">Akun: '+esc(q.account||'default')+' · sumber: '+esc(q.source||'-')+'</span>'
    +'<span class="dim">'+rows.length+' plan</span></div>';
  for(var i=0;i<rows.length;i++){
    var r = rows[i];
    var total = Number(r.total_units)||0, used = Number(r.used_units)||0;
    var rem = (r.available_units !== undefined && r.available_units !== null) ? Number(r.available_units) : (Number(r.remaining_units)||0);
    var pct = total ? Math.min(100, used/total*100) : 0;
    var cls = pct>=95?' bad':(pct>=80?' warn':'');
    var end = r.expires_at || r.ends_at;
    html += '<div style="margin-top:16px">'
      +'<div style="display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;align-items:baseline">'
      +'<b>'+esc(r.model||'-')+'</b>'+tag(r.plan_name||r.plan||'-', r.plan_status==='active'?'ok':'')
      +'</div>'
      +'<div class="big">'+fmt(used)+' <small>/ '+fmt(total)+' '+esc(r.unit_type||'token')+' terpakai</small></div>'
      +'<div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="'+pct.toFixed(1)+'" aria-label="Terpakai '+pct.toFixed(1)+' persen">'
      +'<div class="fill'+cls+'" style="width:'+pct.toFixed(1)+'%"></div></div>'
      +'<div class="meta">'
      +'<div><span>Sisa</span><b style="color:var(--ok)">'+fmt(rem)+'</b></div>'
      +'<div><span>Terpakai</span><b>'+pct.toFixed(1)+'%</b></div>'
      +'<div><span>Mulai</span><b>'+when(r.period_start)+'</b></div>'
      +'<div><span>Berakhir</span><b>'+when(end)+'</b></div>'
      +'<div><span>Sisa waktu</span><b>'+left(end)+'</b></div>'
      +'</div></div>';
  }
  el.className='card';el.innerHTML=html;
}

// ---- inspector tab ----
var INS = {entries:[], selected:0, detail:null, loaded:false};

function statusTag(e){
  var s = Number(e.status)||0;
  var cls = s>=500?'bad':(s>=400?'warn':(s>=200&&s<300?'ok':''));
  var out = tag(String(s), cls);
  if(e.error_code !== undefined && e.error_code !== null){
    out += ' '+tag('kode '+e.error_code, s>=500?'bad':'warn');
  }
  return out;
}
function filterEntries(){
  var method = byId('imethod').value;
  var st = byId('istatus').value;
  var q = (byId('iq').value||'').toLowerCase();
  return INS.entries.filter(function(e){
    if(method && e.method !== method) return false;
    if(q && String(e.path||'').toLowerCase().indexOf(q) < 0) return false;
    var code = Number(e.status)||0;
    if(st === 'err') return e.error_code !== undefined && e.error_code !== null;
    if(st === '2xx') return code>=200 && code<300;
    if(st === '3xx') return code>=300 && code<400;
    if(st === '4xx') return code>=400 && code<500;
    if(st === '5xx') return code>=500;
    if(/^[0-9]+$/.test(st)) return String(e.error_code) === st;
    return true;
  });
}
function renderList(){
  var rows = filterEntries();
  byId('icount').textContent = rows.length + ' dari ' + INS.entries.length + ' entri';
  if(!rows.length){
    byId('ilist').innerHTML = '<div class="emptystate">Tidak ada entri yang cocok. '
      + (INS.entries.length ? 'Longgarkan filter atau ubah kata kunci.' : 'Lakukan satu panggilan ke /v1/messages atau /v1/chat/completions, lalu muat ulang.') + '</div>';
    return;
  }
  var html = '<table><thead><tr><th>Waktu</th><th>Rute</th><th>Path</th><th class="num">Status</th><th class="num">Durasi</th><th class="num">TTFT</th><th>Akun</th></tr></thead><tbody>';
  for(var i=0;i<rows.length;i++){
    var e = rows[i];
    var sel = e.id === INS.selected ? ' class="sel"' : '';
    html += '<tr tabindex="0" role="button" data-id="'+esc(e.id)+'"'+sel+' aria-label="Buka entri '+esc(e.route)+' '+esc(e.path)+'">'
      +'<td class="mono dim">'+esc(clock(e.started_at))+'</td>'
      +'<td><span class="mth">'+esc(e.method)+'</span> '+esc(e.route||'-')+'</td>'
      +'<td class="path" title="'+esc(e.path)+'">'+esc(e.path)+'</td>'
      +'<td class="num">'+statusTag(e)+'</td>'
      +'<td class="num">'+(Number(e.duration_ms)||0)+' ms</td>'
      +'<td class="num'+(e.first_token_ms?'':' dim')+'">'+(e.first_token_ms?e.first_token_ms+' ms':'&ndash;')+'</td>'
      +'<td class="dim">'+esc(e.account||'-')+'</td>'
      +'</tr>';
  }
  html += '</tbody></table>';
  byId('ilist').innerHTML = html;
}
function headerTable(h){
  var keys = Object.keys(h||{});
  if(!keys.length) return '<p class="note">tidak ada header</p>';
  var out = '<div class="tablewrap"><table><tbody>';
  keys.sort();
  for(var i=0;i<keys.length;i++){
    out += '<tr><td class="mono dim">'+esc(keys[i])+'</td><td class="mono">'+esc(h[keys[i]])+'</td></tr>';
  }
  return out+'</tbody></table></div>';
}
function pretty(body){
  if(!body) return '(kosong)';
  try{ return JSON.stringify(JSON.parse(body), null, 2); }catch(e){ return body; }
}
function renderDetail(){
  var el = byId('idetail');
  var ex = INS.detail;
  if(!ex){ el.innerHTML = '<p class="note">Pilih satu entri untuk melihat header, body, dan waktu.</p>'; return; }
  var html = '<div class="detail">'
    +'<div class="dhead"><span class="mth">'+esc(ex.method)+'</span><code>'+esc(ex.path)+'</code>'+statusTag(ex)+'</div>'
    +'<div class="actions"><button type="button" id="ireplay">Putar ulang</button>'
    +'<a class="btn" href="/inspect?format=har&amp;limit=50" download="zcode-session.har">Ekspor HAR</a>'
    +'<span class="dim" id="irstate">replay memanggil lane lagi (memakai kuota).</span></div>'
    +'<div class="meta">'
    +'<div><span>Rute</span><b>'+esc(ex.route||'-')+'</b></div>'
    +'<div><span>Model</span><b>'+esc(ex.model||'-')+'</b></div>'
    +'<div><span>Akun</span><b>'+esc(ex.account||'-')+'</b></div>'
    +'<div><span>Aliran</span><b>'+(ex.stream?'SSE':'JSON')+'</b></div>'
    +'<div><span>Mulai</span><b>'+esc(whenISO(ex.started_at))+'</b></div>'
    +'<div><span>Durasi</span><b>'+(Number(ex.duration_ms)||0)+' ms</b></div>'
    +'<div><span>TTFT</span><b>'+(ex.first_token_ms?ex.first_token_ms+' ms':'&ndash;')+'</b></div>'
    +'<div><span>Ukuran</span><b>'+fmtBytes(ex.request_bytes)+' &rarr; '+fmtBytes(ex.response_bytes)+'</b></div>'
    +(ex.truncated?'<div><span>Body</span><b style="color:var(--warn)">dipotong di batas simpan</b></div>':'')
    +'</div>';
  var events = ex.events || [];
  html += '<h3>Percobaan upstream</h3>';
  if(!events.length){
    html += '<p class="note">Tidak ada catatan percobaan (permintaan gagal sebelum upstream, atau capture baru menyala).</p>';
  } else {
    html += '<div class="events">';
    for(var i=0;i<events.length;i++){
      var ev = events[i];
      var cls = Number(ev.status) >= 400 ? 'bad' : 'ok';
      html += '<div class="ev">'+tag(String(ev.status||'-'), cls)
        +'<span>akun <b>'+esc(ev.account||'-')+'</b></span>'
        +'<span class="dim">percobaan '+(Number(ev.attempt)||1)+'</span>'
        +(ev.code!==undefined?'<span class="dim">kode '+esc(ev.code)+'</span>':'')
        +(ev.switchable?'<span class="dim">rotasi akun</span>':'')
        +(ev.duration_ms?'<span class="dim">'+ev.duration_ms+' ms</span>':'')
        +(ev.detail?'<span class="dim" style="flex-basis:100%;overflow-wrap:anywhere">'+esc(ev.detail)+'</span>':'')
      +'</div>';
    }
    html += '</div>';
  }
  html += '<div class="cols">'
    +'<div><h3>Request headers</h3>'+headerTable(ex.request_headers)
    +'<h3>Request body</h3><pre>'+esc(pretty(ex.request_body))+'</pre></div>'
    +'<div><h3>Response headers</h3>'+headerTable(ex.response_headers)
    +'<h3>Response body</h3><pre>'+esc(pretty(ex.response_body))+'</pre></div>'
    +'</div></div>';
  el.innerHTML = html;
  var rp = byId('ireplay');
  if(rp) rp.addEventListener('click', function(){ replayDetail(ex.id); });
}
function replayDetail(id){
  if(!window.confirm('Putar ulang permintaan ini melalui lane? Ini memakai kuota plan dan menambah entri capture.')) return;
  var btn = byId('ireplay');
  var note = byId('irstate');
  if(btn){ btn.disabled = true; btn.textContent = 'memutar...'; }
  api('/inspect/replay?id='+encodeURIComponent(id), {method:'POST'}).then(function(res){
    if(!res || res.ok === false){ if(note) note.textContent = 'replay gagal: '+((res&&res.error)||'tidak diketahui'); return; }
    if(note) note.textContent = 'replay '+res.status+' · '+(res.truncated?'body dipotong':'selesai');
    loadInspector();
  }).catch(function(e){
    if(note) note.textContent = 'replay gagal: '+e.message;
  }).then(function(){
    if(btn){ btn.disabled = false; btn.textContent = 'Putar ulang'; }
  });
}
function loadDetail(id){
  INS.selected = id;
  renderList();
  api('/inspect?id='+encodeURIComponent(id)).then(function(ex){
    if(!ex || ex.ok === false){ INS.detail = null; renderDetail(); return; }
    INS.detail = ex;
    renderDetail();
  }).catch(function(){});
}
function loadInspector(){
  byId('istate').textContent = 'memuat...';
  api('/inspect?limit=200').then(function(res){
    if(!res || res.enabled === false){
      INS.entries = []; INS.detail = null; INS.selected = 0;
      byId('icount').textContent = '';
      byId('istate').textContent = (res && res.hint) ? res.hint : 'Capture mati. Set ZCODE_CAPTURE=1 lalu jalankan ulang bridge.';
      byId('ilist').innerHTML = '<div class="emptystate">Inspector tidak merekam apa pun saat ini.</div>';
      byId('idetail').innerHTML = '';
      return;
    }
    INS.entries = res.entries || [];
    INS.loaded = true;
    byId('istate').textContent = 'ring ' + res.count + ' / ' + res.capacity + ' entri · body disimpan sampai ' + fmtBytes(res.body_max);
    renderList();
    if(!INS.selected){ byId('idetail').innerHTML = '<p class="note">Pilih satu entri untuk melihat header, body, dan waktu.</p>'; }
    else {
      var still = INS.entries.some(function(e){ return e.id === INS.selected; });
      if(still) loadDetail(INS.selected);
      else { INS.selected = 0; INS.detail = null; renderDetail(); }
    }
  }).catch(function(){
    if(!INS.loaded) byId('istate').textContent = 'Gagal memuat /inspect.';
  });
}
byId('ilist').addEventListener('click', function(ev){
  var row = ev.target.closest('tr[data-id]');
  if(row) loadDetail(Number(row.getAttribute('data-id')));
});
byId('ilist').addEventListener('keydown', function(ev){
  if(ev.key !== 'Enter' && ev.key !== ' ') return;
  var row = ev.target.closest('tr[data-id]');
  if(!row) return;
  ev.preventDefault();
  loadDetail(Number(row.getAttribute('data-id')));
});
byId('imethod').addEventListener('change', renderList);
byId('istatus').addEventListener('change', renderList);
byId('iq').addEventListener('input', renderList);
byId('irefresh').addEventListener('click', loadInspector);
bindDesktopControls();
bindUsageControls();
byId('iclear').addEventListener('click', function(){
  if(!window.confirm('Hapus semua entri yang terekam?')) return;
  api('/inspect',{method:'DELETE'}).then(function(){
    INS.entries = []; INS.detail = null; INS.selected = 0;
    byId('idetail').innerHTML = '';
    loadInspector();
  }).catch(function(){});
});
byId('iexport').addEventListener('click', function(ev){
  ev.preventDefault();
  api('/inspect?limit=200&full=1').then(function(data){
    var blob = new Blob([JSON.stringify(data, null, 2)], {type:'application/json'});
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'zcode-inspect.json';
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(function(){ URL.revokeObjectURL(a.href); }, 1000);
  }).catch(function(){});
});

// ---- usage tab ----
var USAGE = { loaded:false, recent:[] };
function tpsColor(v){
  if(v < 40) return '#34d399';
  if(v < 80) return '#a3e635';
  if(v < 160) return '#fbbf24';
  if(v < 240) return '#fb923c';
  if(v < 320) return '#f87171';
  return '#e879f9';
}
function fmtTps(v){ return (Number(v)||0).toFixed(1); }
function fmtN(n){ n = Number(n) || 0; return n < 10000 ? n.toLocaleString('id-ID') : fmt(n); }
function renderUsageChart(recent){
  var rows = (recent || []).filter(function(r){ return Number(r.tps) > 0; }).slice(-60);
  if(!rows.length){
    return '<div class="emptystate">Belum ada permintaan streaming yang terukur.<br>Bar muncul setelah satu permintaan dengan <code>stream: true</code> selesai.</div>';
  }
  var maxTps = 0;
  for(var i=0;i<rows.length;i++){ if(rows[i].tps > maxTps) maxTps = rows[i].tps; }
  // Scale to what actually happened; the 320 tok/dtk tier line only appears
  // once the data reaches it, otherwise the chart is all empty space.
  var yMax = Math.max(80, Math.ceil(maxTps / 80) * 80);
  var W = 600, H = 120, gap = 2;
  var barW = Math.max(1, (W - (rows.length - 1) * gap) / rows.length);
  var bars = rows.map(function(r, i){
    var h = Math.max(2, Number(r.tps) / yMax * H);
    var x = i * (barW + gap);
    return '<rect x="' + x.toFixed(1) + '" y="' + (H - h).toFixed(1) + '" width="' + barW.toFixed(1) + '" height="' + h.toFixed(1) + '" fill="' + tpsColor(r.tps) + '">'
      + '<title>' + esc(r.model) + ' \u00b7 ' + fmtTps(r.tps) + ' tok/dtk \u00b7 ' + esc(whenISO(r.at)) + '</title></rect>';
  }).join('');
  var refY = yMax >= 320 ? '<line x1="0" y1="' + (H - 320 / yMax * H).toFixed(1) + '" x2="' + W + '" y2="' + (H - 320 / yMax * H).toFixed(1) + '" stroke="#334155" stroke-dasharray="4 4" vector-effect="non-scaling-stroke"/>' : '';
  var refNote = yMax >= 320 ? '<span class="dim">garis putus-putus: 320 tok/dtk</span>' : '';
  var legend = [['#34d399','&lt;40'],['#a3e635','40-80'],['#fbbf24','80-160'],['#fb923c','160-240'],['#f87171','240-320'],['#e879f9','320+']]
    .map(function(t){ return '<span><span class="swatch" style="background:' + t[0] + '"></span>' + t[1] + '</span>'; }).join('');
  return '<div class="chartwrap"><svg viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Kecepatan generasi ' + rows.length + ' permintaan terakhir" preserveAspectRatio="none">'
    + refY + bars + '</svg></div><div class="legend">' + legend + refNote + '</div>';
}
function gaugeCard(value, max, color, label, display, unit, sub){
  var r = 52, c = 2 * Math.PI * r, arc = c * 2 / 3;
  var frac = max > 0 ? Math.min(1, Math.max(0, value / max)) : 0;
  return '<div class="gauge"><svg viewBox="0 0 120 120" role="img" aria-label="' + esc(label + ' ' + display + ' ' + unit) + '">'
    + '<circle cx="60" cy="60" r="' + r + '" fill="none" stroke="#1c222e" stroke-width="10" stroke-linecap="round" stroke-dasharray="' + arc.toFixed(1) + ' ' + c.toFixed(1) + '" transform="rotate(150 60 60)"/>'
    + '<circle cx="60" cy="60" r="' + r + '" fill="none" stroke="' + color + '" stroke-width="10" stroke-linecap="round" stroke-dasharray="' + (frac * arc).toFixed(1) + ' ' + c.toFixed(1) + '" transform="rotate(150 60 60)"/>'
    + '<text x="60" y="61" text-anchor="middle" class="gval">' + esc(display) + '</text>'
    + '<text x="60" y="77" text-anchor="middle" class="gsub">' + esc(unit) + '</text></svg>'
    + '<div class="gcap"><b>' + esc(label) + '</b><span>' + esc(sub) + '</span></div></div>';
}
function cacheRate(today){
  var cached = Number(today.cache_read_tokens) || 0;
  var fresh = Number(today.input_tokens) || 0;
  var total = cached + fresh;
  return total ? cached / total * 100 : 0;
}
// ---- bridge usage ranges and per-model trend ----
var URANGES = [['1h', '1 jam'], ['6h', '6 jam'], ['24h', '24 jam'], ['7d', '7 hari']];
var URANGE_MS = { '1h': 3600e3, '6h': 21600e3, '24h': 86400e3, '7d': 604800e3 };
var URANGE = localStorage.getItem('zcode.usageRange') || '1h';
var MODEL_COLORS = ['#34d399', '#60a5fa', '#fbbf24', '#f472b6', '#a3e635'];
function rangeSpan(key){ return URANGE_MS[key] || URANGE_MS['1h']; }
function rangeLabel(key){
  for(var i = 0; i < URANGES.length; i++){ if(URANGES[i][0] === key) return URANGES[i][1]; }
  return key;
}
// bucketRows splits measured rows into n equal time buckets over the
// selected span. Each bucket keeps summed output tokens and generation time,
// so the plotted value is a weighted tok/dtk, not an average of averages.
function bucketRows(recent, key, n){
  var span = rangeSpan(key), nowMs = Date.now(), start = nowMs - span;
  var buckets = [], i;
  for(i = 0; i < n; i++) buckets.push({out: 0, ms: 0, count: 0});
  (recent || []).forEach(function(row){
    if(Number(row.tps) <= 0) return;
    var t = new Date(row.at).getTime();
    if(isNaN(t) || t < start || t > nowMs) return;
    var idx = Math.floor((t - start) / span * n);
    if(idx < 0) idx = 0;
    if(idx >= n) idx = n - 1;
    buckets[idx].out += Number(row.output_tokens) || 0;
    buckets[idx].ms += Number(row.gen_ms) || 0;
    buckets[idx].count++;
  });
  return buckets;
}
function bucketTps(b){ return b.ms > 0 ? b.out / (b.ms / 1000) : 0; }
function linePath(vals, W, H, yMax){
  return vals.map(function(v, i){
    return (i ? 'L' : 'M') + (i / (vals.length - 1) * W).toFixed(1) + ' ' + (H - v / yMax * H).toFixed(1);
  }).join(' ');
}
function renderRangeChart(recent){
  var n = 60, buckets = bucketRows(recent, URANGE, n);
  var vals = buckets.map(bucketTps);
  var peak = vals.reduce(function(a, b){ return b > a ? b : a; }, 0);
  if(peak <= 0) return '<div class="emptystate">Belum ada permintaan streaming pada rentang ' + esc(rangeLabel(URANGE)) + '.</div>';
  var yMax = Math.max(60, Math.ceil(peak / 60) * 60), W = 600, H = 130;
  var grid = [0.25, 0.5, 0.75].map(function(f){
    var y = (H - f * H).toFixed(1);
    return '<line x1="0" y1="' + y + '" x2="' + W + '" y2="' + y + '" stroke="#1c222e" vector-effect="non-scaling-stroke"/>';
  }).join('');
  var line = linePath(vals, W, H, yMax);
  return '<div class="chartwrap"><svg viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Kecepatan generasi ' + esc(rangeLabel(URANGE)) + '" preserveAspectRatio="none">'
    + grid + '<path d="' + line + ' L' + W + ' ' + H + ' L0 ' + H + ' Z" fill="rgba(52,211,153,.12)"/>'
    + '<path d="' + line + '" fill="none" stroke="#34d399" stroke-width="1.6" vector-effect="non-scaling-stroke"/></svg></div>'
    + '<div class="legend"><span>puncak bucket ' + fmtTps(peak) + ' tok/dtk</span><span class="dim">' + esc(rangeLabel(URANGE)) + ' · skala 0-' + yMax + ' tok/dtk</span></div>';
}
function renderModelTrend(recent){
  var n = 60, span = rangeSpan(URANGE), nowMs = Date.now(), start = nowMs - span;
  var perModel = {}, totals = {};
  (recent || []).forEach(function(row){
    if(Number(row.tps) <= 0) return;
    var t = new Date(row.at).getTime();
    if(isNaN(t) || t < start || t > nowMs) return;
    var name = row.model || 'unknown';
    if(!perModel[name]){
      perModel[name] = [];
      for(var i = 0; i < n; i++) perModel[name].push({out: 0, ms: 0, count: 0});
      totals[name] = 0;
    }
    var idx = Math.floor((t - start) / span * n);
    if(idx < 0) idx = 0;
    if(idx >= n) idx = n - 1;
    perModel[name][idx].out += Number(row.output_tokens) || 0;
    perModel[name][idx].ms += Number(row.gen_ms) || 0;
    perModel[name][idx].count++;
    totals[name] += Number(row.output_tokens) || 0;
  });
  var names = Object.keys(perModel).sort(function(a, b){ return totals[b] - totals[a]; });
  if(!names.length) return '<div class="emptystate">Belum ada model terukur pada rentang ' + esc(rangeLabel(URANGE)) + '.</div>';
  if(names.length > 5) names = names.slice(0, 5);
  var series = names.map(function(name){
    return {name: name, vals: perModel[name].map(bucketTps)};
  });
  var peak = 0;
  series.forEach(function(s){ s.vals.forEach(function(v){ if(v > peak) peak = v; }); });
  if(peak <= 0) return '<div class="emptystate">Belum ada model terukur pada rentang ' + esc(rangeLabel(URANGE)) + '.</div>';
  var yMax = Math.max(60, Math.ceil(peak / 60) * 60), W = 600, H = 130;
  var grid = [0.25, 0.5, 0.75].map(function(f){
    var y = (H - f * H).toFixed(1);
    return '<line x1="0" y1="' + y + '" x2="' + W + '" y2="' + y + '" stroke="#1c222e" vector-effect="non-scaling-stroke"/>';
  }).join('');
  var paths = series.map(function(s, i){
    var color = MODEL_COLORS[i % MODEL_COLORS.length];
    return '<path d="' + linePath(s.vals, W, H, yMax) + '" fill="none" stroke="' + color + '" stroke-width="1.6" vector-effect="non-scaling-stroke"/>';
  }).join('');
  var legend = series.map(function(s, i){
    return '<span><span class="swatch" style="background:' + MODEL_COLORS[i % MODEL_COLORS.length] + '"></span>' + esc(s.name) + '</span>';
  }).join('');
  return '<div class="chartwrap"><svg viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="Tren kecepatan per model" preserveAspectRatio="none">'
    + grid + paths + '</svg></div><div class="legend">' + legend + '<span class="dim">skala 0-' + yMax + ' tok/dtk</span></div>';
}
function renderRangeButtons(){
  byId('urange').innerHTML = URANGES.map(function(r){
    return '<button type="button" data-ur="' + r[0] + '" aria-pressed="' + (URANGE === r[0]) + '">' + r[1] + '</button>';
  }).join('');
}
function bindUsageControls(){
  byId('urange').addEventListener('click', function(e){
    var b = e.target.closest('button[data-ur]');
    if(!b) return;
    URANGE = b.getAttribute('data-ur');
    localStorage.setItem('zcode.usageRange', URANGE);
    renderRangeButtons();
    var recent = USAGE.recent || [];
    byId('uroll').innerHTML = renderRangeChart(recent);
    byId('umtrend').innerHTML = renderModelTrend(recent);
  });
}
function renderUsage(res){
  renderDesktop(res.desktop);
  var today = res.today || {}, allTime = res.all_time || {};
  var rings = res.rings || {};
  byId('ustate').textContent = today.requests ? today.requests + ' permintaan hari ini' : '';
  var last = null;
  (res.recent || []).forEach(function(row){ if(Number(row.tps) > 0) last = row; });
  var lastTps = Number(rings.last_tps) || (last ? Number(last.tps) : 0);
  var highTps = Number(rings.high_tps) || Number(allTime.high_tps) || lastTps;
  var weekAvg = Number(rings.seven_day_avg_tps) || 0;
  var hit = cacheRate(today);
  renderRangeButtons();
  byId('uga').innerHTML =
    gaugeCard(lastTps, Math.max(120, Math.ceil(lastTps / 40) * 40), tpsColor(lastTps),
      'Kecepatan terakhir', last ? fmtTps(lastTps) : '-', 'tok/dtk', last ? last.model + ' · ' + whenISO(last.at) : 'belum ada stream')
    + gaugeCard(highTps, Math.max(120, Math.ceil(highTps / 40) * 40), tpsColor(highTps),
      'Tertinggi', fmtTps(highTps), 'tok/dtk', 'puncak satu permintaan')
    + gaugeCard(weekAvg, Math.max(80, Math.ceil(weekAvg / 40) * 40), tpsColor(weekAvg),
      'Rata-rata 7 hari', fmtTps(weekAvg), 'tok/dtk', 'dari baris tersimpan');
  var totals = [
    ['Permintaan', fmtN(today.requests)],
    ['Token masuk', fmtN(today.input_tokens)],
    ['Token keluar', fmtN(today.output_tokens)],
    ['Cache dibaca', fmtN(today.cache_read_tokens)],
    ['Cache hit', hit.toFixed(1) + '%']
  ].map(function(t){ return '<div><span>' + t[0] + '</span><b>' + t[1] + '</b></div>'; }).join('');
  if(today.requests) totals += '<div><span>Rata-rata tok/dtk</span><b>' + fmtTps(today.avg_tps) + '</b></div>';
  byId('utotals').innerHTML = totals;
  byId('ualltime').innerHTML = [
    ['Total permintaan', fmtN(allTime.requests || 0)],
    ['Token masuk', fmtN(allTime.input_tokens)],
    ['Token keluar', fmtN(allTime.output_tokens)],
    ['Cache dibaca', fmtN(allTime.cache_read_tokens)],
    ['Rata-rata tok/dtk', fmtTps(allTime.avg_tps)],
    ['Tertinggi', fmtTps(allTime.high_tps)]
  ].map(function(t){ return '<div><span>' + t[0] + '</span><b>' + t[1] + '</b></div>'; }).join('');
  USAGE.recent = res.recent || [];
  sb('req', fmtN(today.requests || 0));
  sb('tps', fmtTps(lastTps) + ' tok/dtk');
  byId('uroll').innerHTML = renderRangeChart(USAGE.recent);
  byId('umtrend').innerHTML = renderModelTrend(USAGE.recent);
  byId('uchart').innerHTML = renderUsageChart(USAGE.recent);
  var models = today.per_model || {};
  var names = Object.keys(models).sort(function(a, b){ return models[b].requests - models[a].requests; });
  if(!names.length){
    byId('utable').innerHTML = '<div class="emptystate">Belum ada data model. Baris tulis ke <code>data/usage-YYYYMMDD.jsonl</code> setelah satu permintaan sukses.</div>';
    return;
  }
  var head = '<table><thead><tr><th>Model</th><th class="num">Permintaan</th><th class="num">Masuk</th><th class="num">Keluar</th><th class="num">Cache</th><th class="num">tok/dtk</th></tr></thead><tbody>';
  var rows = names.map(function(name){
    var m = models[name];
    return '<tr><td class="mono">' + esc(name) + '</td>'
      + '<td class="num">' + fmtN(m.requests) + '</td>'
      + '<td class="num">' + fmtN(m.input_tokens) + '</td>'
      + '<td class="num">' + fmtN(m.output_tokens) + '</td>'
      + '<td class="num">' + fmtN(m.cache_read_tokens) + '</td>'
      + '<td class="num">' + fmtTps(m.avg_tps) + '</td></tr>';
  }).join('');
  byId('utable').innerHTML = head + rows + '</tbody></table>';
}
// ---- desktop application database (ZCode CLI db.sqlite) ----
var DUSAGE = {
  range: localStorage.getItem('zcode.deskRange') || '15m',
  view: localStorage.getItem('zcode.deskView') || 'all',
  snap: null
};
var DRANGES = [['15m', '15 mnt'], ['1h', '1 jam'], ['6h', '6 jam'], ['24h', '24 jam']];
function hhmm(ms){
  var d = new Date(Number(ms) || 0);
  return ('0' + d.getHours()).slice(-2) + ':' + ('0' + d.getMinutes()).slice(-2);
}
function desktopChart(snap){
  var key = DUSAGE.range;
  var range = (snap.ranges || []).filter(function(r){ return r.key === key; })[0];
  if(!range) return '<div class="emptystate">Rentang ' + esc(key) + ' tidak tersedia.</div>';
  var buckets = range.buckets || [], n = buckets.length;
  var peak = 0, calls = 0;
  buckets.forEach(function(b){ if(Number(b.tps) > peak) peak = Number(b.tps); calls += Number(b.calls) || 0; });
  if(n === 0) return '<div class="emptystate">Rentang kosong.</div>';
  if(peak <= 0 && calls === 0) return '<div class="emptystate">Belum ada pemanggilan selesai pada rentang ini.</div>';
  var yMax = Math.max(60, Math.ceil(peak / 60) * 60), W = 600, H = 130;
  var line = buckets.map(function(b, i){
    return (i ? 'L' : 'M') + (i / (n - 1) * W).toFixed(1) + ' ' + (H - (Number(b.tps) || 0) / yMax * H).toFixed(1);
  }).join(' ');
  var grid = [0.25, 0.5, 0.75].map(function(f){
    var y = (H - f * H).toFixed(1);
    return '<line x1="0" y1="' + y + '" x2="' + W + '" y2="' + y + '" stroke="#1c222e" vector-effect="non-scaling-stroke"/>';
  }).join('');
  var anchors = [0, Math.floor(n / 2), n - 1];
  var ticks = anchors.map(function(i){
    var x = i / (n - 1) * W;
    var anchor = i === 0 ? 'start' : (i === n - 1 ? 'end' : 'middle');
    return '<text x="' + x.toFixed(1) + '" y="' + (H + 13) + '" text-anchor="' + anchor + '" style="fill:#64748b;font-size:9px">' + hhmm(buckets[i].at) + '</text>';
  }).join('');
  return '<div class="chartwrap"><svg viewBox="0 0 ' + W + ' ' + (H + 18) + '" role="img" aria-label="Kecepatan aplikasi ZCode ' + esc(key) + '" preserveAspectRatio="none">'
    + grid + '<path d="' + line + '" fill="none" stroke="#38bdf8" stroke-width="1.6" vector-effect="non-scaling-stroke"/>' + ticks + '</svg></div>'
    + '<div class="legend"><span>puncak ' + fmtTps(peak) + ' tok/dtk</span>'
    + '<span class="dim">' + calls + ' pemanggilan · ' + Math.round((range.bucket_ms || 0) / 1000) + ' detik per titik</span></div>';
}
function desktopModels(snap){
  var rows = snap.models || [];
  if(!rows.length) return '<div class="emptystate">Belum ada model yang tercatat di database aplikasi.</div>';
  var head = '<table><thead><tr><th>Model</th><th class="num">Panggilan</th><th class="num">Token</th><th class="num">Porsi</th><th class="num">Rata-rata</th><th class="num">Puncak</th></tr></thead><tbody>';
  var body = rows.map(function(m){
    return '<tr><td class="mono">' + esc(m.model) + '</td>'
      + '<td class="num">' + fmtN(m.requests) + '</td>'
      + '<td class="num">' + fmtN(m.tokens) + '</td>'
      + '<td class="num">' + (Number(m.share) * 100).toFixed(1) + '%</td>'
      + '<td class="num">' + fmtTps(m.avg_tps) + '</td>'
      + '<td class="num">' + fmtTps(m.peak_tps) + '</td></tr>';
  }).join('');
  return head + body + '</tbody></table>';
}
function desktopButtons(){
  byId('udrange').innerHTML = DRANGES.map(function(r){
    return '<button type="button" data-dr="' + r[0] + '" aria-pressed="' + (DUSAGE.range === r[0]) + '">' + r[1] + '</button>';
  }).join('');
  byId('udview').innerHTML = [['all', 'Keseluruhan'], ['model', 'Per model']].map(function(v){
    return '<button type="button" data-dv="' + v[0] + '" aria-pressed="' + (DUSAGE.view === v[0]) + '">' + v[1] + '</button>';
  }).join('');
}
function desktopPaint(){
  desktopButtons();
  var snap = DUSAGE.snap;
  if(!snap) return;
  if(DUSAGE.view === 'model'){
    byId('udchart').innerHTML = desktopModels(snap);
    byId('udmodels').innerHTML = '';
  } else {
    byId('udchart').innerHTML = desktopChart(snap);
    byId('udmodels').innerHTML = desktopModels(snap);
  }
}
function renderDesktop(block){
  block = block || {};
  if(!block.available || !block.snapshot){
    byId('udsum').innerHTML = '<div><span>Status</span><b>tidak terbaca</b></div>'
      + '<div><span>Path</span><b class="mono">' + esc(block.path || '-') + '</b></div>'
      + '<div><span>Catatan</span><b>' + esc(block.error || 'database aplikasi belum ada') + '</b></div>';
    byId('udrange').innerHTML = '';
    byId('udview').innerHTML = '';
    byId('udchart').innerHTML = '<div class="emptystate">Angka aplikasi ZCode hanya muncul kalau <code>db.sqlite</code> bisa dibaca. Trafik lewat bridge tetap terukur di atas.</div>';
    byId('udmodels').innerHTML = '';
    DUSAGE.snap = null;
    return;
  }
  var snap = block.snapshot, t = snap.today || {};
  DUSAGE.snap = snap;
  var tokens = (Number(t.input_tokens) || 0) + (Number(t.output_tokens) || 0) + (Number(t.reasoning_tokens) || 0) + (Number(t.cache_creation_tokens) || 0);
  var cards = [
    ['Panggilan hari ini', fmtN(t.requests)],
    ['Token hari ini', fmtN(tokens)],
    ['Rata-rata hari ini', fmtTps(t.avg_tps) + ' tok/dtk'],
    ['Tertinggi 7 hari', fmtTps(snap.high_7d) + ' tok/dtk'],
    ['Rata-rata 7 hari', fmtTps(snap.avg_7d) + ' tok/dtk'],
    ['Sedang berjalan', fmtN(snap.in_flight)],
    ['Sesi', fmtN(snap.sessions)],
    ['Aktivitas terakhir', snap.last_activity ? whenISO(new Date(Number(snap.last_activity)).toISOString()) : '-']
  ];
  byId('udsum').innerHTML = cards.map(function(c){
    return '<div><span>' + esc(c[0]) + '</span><b>' + esc(c[1]) + '</b></div>';
  }).join('');
  desktopPaint();
}
function bindDesktopControls(){
  byId('udrange').addEventListener('click', function(e){
    var b = e.target.closest('button[data-dr]');
    if(!b) return;
    DUSAGE.range = b.getAttribute('data-dr');
    localStorage.setItem('zcode.deskRange', DUSAGE.range);
    desktopPaint();
  });
  byId('udview').addEventListener('click', function(e){
    var b = e.target.closest('button[data-dv]');
    if(!b) return;
    DUSAGE.view = b.getAttribute('data-dv');
    localStorage.setItem('zcode.deskView', DUSAGE.view);
    desktopPaint();
  });
}
function loadUsage(){
  api('/v1/usage').then(function(res){
    USAGE.loaded = true;
    renderUsage(res || {});
  }).catch(function(){
    if(!USAGE.loaded) byId('utable').innerHTML = '<div class="emptystate">Gagal memuat /v1/usage.</div>';
  });
}

// ---- playground ----
var PM = { busy:false };
function pRender(text, reasoning){
  var html = '<pre style="max-height:360px">' + (text ? esc(text) : '<span class="dim">(kosong)</span>') + '</pre>';
  if(reasoning) html += '<details class="dim" style="margin-top:8px"><summary>reasoning</summary><pre style="max-height:220px">' + esc(reasoning) + '</pre></details>';
  byId('pout').innerHTML = html;
}
function pMeta(pairs){
  byId('pmeta').innerHTML = pairs.map(function(p){ return '<div><span>' + esc(p[0]) + '</span><b>' + esc(p[1]) + '</b></div>'; }).join('');
}
function pSend(){
  if(PM.busy) return;
  var model = (byId('pmodel').value || '').trim();
  var msg = (byId('pmsg').value || '').trim();
  if(!model || !msg) return;
  var stream = byId('pstream').checked;
  PM.busy = true; byId('psend').disabled = true;
  byId('pout').innerHTML = '<p class="note">menunggu upstream…</p>';
  byId('pmeta').innerHTML = '';
  var started = performance.now(), text = '', reasoning = '';
  var headers = {'content-type':'application/json'};
  if(TOKEN) headers['Authorization'] = 'Bearer ' + TOKEN;
  fetch('/v1/chat/completions', {
    method: 'POST', headers: headers, cache: 'no-store',
    body: JSON.stringify({model: model, stream: stream, messages: [{role: 'user', content: msg}]})
  }).then(function(res){
    if(res.status === 401){ showTokenAsk(); throw new Error('token diperlukan'); }
    if(!res.ok){ return res.text().then(function(t){ throw new Error('HTTP ' + res.status + ' · ' + t.slice(0, 200)); }); }
    var type = res.headers.get('content-type') || '';
    if(stream && type.indexOf('text/event-stream') >= 0){
      var reader = res.body.getReader(), dec = new TextDecoder(), buf = '';
      var pump = function(){
        return reader.read().then(function(chunk){
          if(chunk.done){ pRender(text, reasoning); return; }
          buf += dec.decode(chunk.value, {stream: true});
          var idx;
          while((idx = buf.indexOf('\n\n')) >= 0){
            var frame = buf.slice(0, idx); buf = buf.slice(idx + 2);
            frame.split('\n').forEach(function(line){
              if(line.indexOf('data:') !== 0) return;
              var payload = line.slice(5).trim();
              if(!payload || payload === '[DONE]') return;
              var event;
              try { event = JSON.parse(payload); } catch(e) { return; }
              var delta = event.choices && event.choices[0] && event.choices[0].delta;
              if(!delta) return;
              if(delta.reasoning_content) reasoning += delta.reasoning_content;
              if(delta.content) text += delta.content;
              pRender(text, reasoning);
            });
          }
          return pump();
        });
      };
      return pump().then(function(){
        pMeta([['Model', model], ['Karakter', fmt(text.length)], ['Latensi', Math.round(performance.now() - started) + ' ms'], ['Mode', 'stream']]);
      });
    }
    return res.json().then(function(body){
      var choice = (body.choices && body.choices[0]) || {};
      var m = choice.message || {};
      pRender(m.content || '', m.reasoning_content || '');
      var u = body.usage || {};
      pMeta([['Model', body.model || model], ['Masuk', fmt(u.prompt_tokens)], ['Keluar', fmt(u.completion_tokens)],
        ['Latensi', Math.round(performance.now() - started) + ' ms'], ['Selesai', choice.finish_reason || '-']]);
    });
  }).catch(function(err){
    byId('pout').innerHTML = '<p class="note" style="color:var(--bad)">' + esc(err.message || err) + '</p>';
  }).then(function(){
    PM.busy = false; byId('psend').disabled = false;
  });
}
byId('psend').addEventListener('click', pSend);
byId('pmsg').addEventListener('keydown', function(ev){ if(ev.key === 'Enter'){ ev.preventDefault(); pSend(); } });

// ---- module settings ----
var MODULE_DEFS = [
  {key:'status', label:'Status', tab:'tab-status', panel:'panel-status'},
  {key:'inspector', label:'Inspector', tab:'tab-inspector', panel:'panel-inspector'},
  {key:'usage', label:'Penggunaan', tab:'tab-usage', panel:'panel-usage'},
  {key:'playground', label:'Playground', tab:'tab-playground', panel:'panel-playground'},
  {key:'net', label:'Monitor jaringan', card:'netcard'}
];
var MODULES = (function(){
  var stored = null;
  try { stored = JSON.parse(localStorage.getItem('zcode.modules') || 'null'); } catch(e){ stored = null; }
  var on = {};
  MODULE_DEFS.forEach(function(m){ on[m.key] = stored ? stored[m.key] !== false : true; });
  return on;
})();
function renderModules(){
  byId('modgrid').innerHTML = MODULE_DEFS.map(function(m){
    return '<label><input type="checkbox" data-mod="' + m.key + '"' + (MODULES[m.key] ? ' checked' : '') + '> ' + esc(m.label) + '</label>';
  }).join('');
}
function selectFirstVisibleTab(){
  for(var i = 0; i < MODULE_DEFS.length; i++){
    var m = MODULE_DEFS[i];
    if(m.panel && MODULES[m.key]){ selectTab(m.panel); return; }
  }
}
function applyModules(){
  MODULE_DEFS.forEach(function(m){
    if(m.tab){ byId(m.tab).hidden = !MODULES[m.key]; }
    if(m.card){ byId(m.card).hidden = !MODULES[m.key] || !MODULES.status; }
    if(m.panel && !MODULES[m.key]){
      byId(m.panel).hidden = true;
      if(activeTab === m.panel) selectFirstVisibleTab();
    }
  });
  if(!MODULES.status && activeTab === 'panel-status') selectFirstVisibleTab();
}

// ---- network monitor ----
var SB = {};
function sb(key, value){
  SB[key] = value;
  var el = byId('sb-' + key);
  if(el) el.textContent = value;
}
function fmtRate(bps){
  bps = Number(bps) || 0;
  if(bps >= 1048576) return (bps / 1048576).toFixed(1) + ' MB/s';
  if(bps >= 1024) return (bps / 1024).toFixed(1) + ' KB/s';
  return Math.round(bps) + ' B/s';
}
function netStat(label, value, sub){
  return '<div class="netstat"><span>' + esc(label) + '</span><b>' + esc(value) + '</b><small>' + esc(sub) + '</small></div>';
}
function renderNet(net){
  if(!net || net.supported === false){
    byId('netstate').textContent = net && net.error ? '· ' + net.error : '· tidak tersedia di host ini';
    byId('netsum').innerHTML = netStat('Turun', '-', '') + netStat('Naik', '-', '');
    byId('netif').innerHTML = '';
    byId('netproc').innerHTML = '';
    sb('net', '-');
    return;
  }
  byId('netstate').textContent = '· ' + (net.tcp_connections || 0) + ' koneksi TCP';
  byId('netsum').innerHTML =
    netStat('Turun', fmtRate(net.total_in_bps), 'semua antarmuka') +
    netStat('Naik', fmtRate(net.total_out_bps), 'semua antarmuka') +
    netStat('TCP', String(net.tcp_connections || 0), (net.tcp_established || 0) + ' established') +
    netStat('Antarmuka', String((net.interfaces || []).length), 'terdeteksi');
  var rows = (net.interfaces || []).filter(function(i){
    return (Number(i.in_bytes) || 0) > 0 || (Number(i.out_bytes) || 0) > 0;
  }).slice(0, 6);
  if(rows.length){
    byId('netif').innerHTML = '<table><tr><th>Antarmuka</th><th class="num">Turun</th><th class="num">Naik</th><th>Status</th></tr>'
      + rows.map(function(i){
        return '<tr><td>' + esc(i.name || i.description)
          + (i.description && i.description !== i.name ? '<div class="dim" style="font-size:11px">' + esc(i.description) + '</div>' : '')
          + '</td><td class="num">' + fmtRate(i.in_bps) + '</td><td class="num">' + fmtRate(i.out_bps) + '</td>'
          + '<td>' + tag(i.up ? 'aktif' : 'nonaktif', i.up ? 'ok' : '') + '</td></tr>';
      }).join('') + '</table>';
  } else {
    byId('netif').innerHTML = '<p class="note">Belum ada lalu lintas antarmuka.</p>';
  }
  var p = net.process;
  if(p && p.found){
    byId('netproc').innerHTML = '<h3>Proses ZCode <span class="dim" style="text-transform:none;letter-spacing:0">· '
      + esc(p.name || '') + ' (' + (p.pids || []).length + ' proses)</span></h3><div class="netgrid">'
      + netStat('Baca', fmtRate(p.read_bps), fmtBytes(p.read_bytes) + ' total')
      + netStat('Tulis', fmtRate(p.write_bps), fmtBytes(p.write_bytes) + ' total')
      + netStat('CPU', (Number(p.cpu_percent) || 0).toFixed(1) + '%', 'satu inti')
      + netStat('Kalibrasi', 'x' + (Number(p.calibration) || 1), 'ZCODE_PROCESS_CALIBRATION')
      + '</div>';
  } else {
    byId('netproc').innerHTML = '<p class="note">Proses ZCode tidak berjalan; baris proses disembunyikan.</p>';
  }
  sb('net', fmtRate(net.total_in_bps) + ' turun / ' + fmtRate(net.total_out_bps) + ' naik');
}

// ---- tabs ----
var activeTab = 'panel-status';
var inspectorTimer = null;
var usageTimer = null;
function selectTab(panel, address){
  activeTab = panel;
  if(address !== false && window.history && window.history.replaceState){
    window.history.replaceState(null, '', '#' + panel.replace('panel-',''));
  }
  var tabs = document.querySelectorAll('[role=tab]');
  for(var i=0;i<tabs.length;i++){
    var on = tabs[i].getAttribute('data-panel') === panel;
    tabs[i].setAttribute('aria-selected', on ? 'true' : 'false');
    var target = byId(tabs[i].getAttribute('data-panel'));
    if(target) target.hidden = !on;
  }
  if(panel === 'panel-inspector'){
    loadInspector();
    if(!inspectorTimer) inspectorTimer = setInterval(function(){ if(activeTab==='panel-inspector' && byId('auto').checked) loadInspector(); }, 5000);
  } else if(inspectorTimer){ clearInterval(inspectorTimer); inspectorTimer = null; }
  if(panel === 'panel-usage'){
    loadUsage();
    if(!usageTimer) usageTimer = setInterval(function(){ if(activeTab==='panel-usage' && byId('auto').checked) loadUsage(); }, 5000);
  } else if(usageTimer){ clearInterval(usageTimer); usageTimer = null; }
  if(panel === 'panel-playground' && byId('pmsg')) byId('pmsg').focus();
}
var initialHash = (window.location.hash || '').replace('#','');
if(initialHash && byId('panel-' + initialHash)) selectTab('panel-' + initialHash, false);
var tabButtons = document.querySelectorAll('[role=tab]');
for(var t=0;t<tabButtons.length;t++){
  tabButtons[t].addEventListener('click', function(ev){ selectTab(ev.currentTarget.getAttribute('data-panel')); });
}

function tick(){
  byId('stamp').textContent = 'memuat...';
  Promise.allSettled([
    fetch('/health',{cache:'no-store'}).then(function(r){return r.json();}),
    fetch('/quota',{cache:'no-store'}).then(function(r){return r.json();}),
    fetch('/v1/net',{cache:'no-store'}).then(function(r){return r.json();})
  ]).then(function(out){
    renderHealth(out[0].status==='fulfilled'?out[0].value:null);
    renderQuota(out[1]);
    renderNet(out[2].status==='fulfilled'?out[2].value:null);
    byId('stamp').textContent = 'diperbarui '+now();
    sb('state', 'diperbarui ' + now());
  });
}

function tickNet(){
  fetch('/v1/net',{cache:'no-store'}).then(function(r){
    if(r.status === 401){ showTokenAsk(); return null; }
    return r.json();
  }).then(function(net){ if(net) renderNet(net); }).catch(function(){});
}

var timer = setInterval(tick, 15000);
var netTimer = setInterval(tickNet, 3000);
byId('auto').addEventListener('change', function(e){
  if(e.target.checked){ timer = setInterval(tick, 15000); netTimer = setInterval(tickNet, 3000); tick(); tickNet(); }
  else { clearInterval(timer); clearInterval(netTimer); }
});
byId('refresh').addEventListener('click', tick);
renderModules();
byId('modgrid').addEventListener('change', function(e){
  var key = e.target.getAttribute('data-mod');
  if(!key) return;
  MODULES[key] = e.target.checked;
  localStorage.setItem('zcode.modules', JSON.stringify(MODULES));
  applyModules();
});
applyModules();
tickNet();
tick();
})();
</script>
</body>
</html>
`
