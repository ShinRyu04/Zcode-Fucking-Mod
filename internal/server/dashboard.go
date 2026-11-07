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
.cols{display:grid;grid-template-columns:1fr 1fr;gap:20px}
pre{background:#0f131b;border:1px solid var(--line);border-radius:8px;padding:12px;margin:0;font-size:12.5px;line-height:1.5;overflow:auto;max-height:340px;white-space:pre-wrap;overflow-wrap:anywhere}
.events{display:grid;gap:6px}
.ev{display:flex;gap:8px;align-items:baseline;flex-wrap:wrap;font-size:13px;border:1px solid var(--line);border-radius:8px;padding:6px 10px}
.emptystate{border:1px dashed var(--line);border-radius:10px;padding:22px;text-align:center;color:var(--dim)}
.qnote{color:var(--dim);font-size:13px;margin:0 0 8px}
@media (prefers-reduced-motion:reduce){.fill{transition:none}}
@media (max-width:760px){.cols{grid-template-columns:1fr}.toolbtns{margin-left:0;width:100%}.toolbtns .btn,.toolbtns button{flex:1 1 auto;text-align:center}}
@media (max-width:560px){.big{font-size:26px}.card{padding:16px}th,td{white-space:normal;overflow-wrap:anywhere}td.path{max-width:none}}
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
</div>

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
</div>
</div>
<p class="qnote" id="istate">memuat...</p>
<div id="ilist" class="tablewrap" aria-live="polite"></div>
<div id="idetail"></div>
</div>
</section>
</main>

<footer>
<div class="controls">
<button type="button" id="refresh">Muat ulang</button>
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
  byId('ver').textContent = h.version ? ' · v'+h.version : '';
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

// ---- tabs ----
var activeTab = 'panel-status';
var inspectorTimer = null;
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
    fetch('/quota',{cache:'no-store'}).then(function(r){return r.json();})
  ]).then(function(out){
    renderHealth(out[0].status==='fulfilled'?out[0].value:null);
    renderQuota(out[1]);
    byId('stamp').textContent = 'diperbarui '+now();
  });
}

var timer = setInterval(tick, 15000);
byId('auto').addEventListener('change', function(e){
  if(e.target.checked){ timer = setInterval(tick, 15000); tick(); }
  else { clearInterval(timer); }
});
byId('refresh').addEventListener('click', tick);
tick();
})();
</script>
</body>
</html>
`
