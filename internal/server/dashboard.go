package server

import (
	"io"
	"net/http"
)

// handleDashboard serves the built-in status page at GET /dashboard.
// The page reads /health and /v1/quota on the same origin, so it needs
// no CORS setup, no token, and no external assets. It is observability
// only: everything here is also available as JSON from those endpoints.
func handleDashboard(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, dashboardHTML)
}

// dashboardHTML is a single-file page. Design intent: a local developer
// tool that lives next to a terminal, so it is dark, flat, and calm:
// one accent color (green = healthy lane), status colors only where
// they carry state, no animation beyond the data refresh itself.
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
.wrap{max-width:860px;margin:0 auto}
header p{margin:2px 0 0;color:var(--dim);font-size:13px}
h1{margin:0;font-size:20px;letter-spacing:-.01em}
main{margin-top:18px}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:18px 20px;margin-bottom:14px}
.card h2{margin:0 0 12px;font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.1em;color:var(--dim)}
.kv{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}
.kv span{display:block;font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--dim)}
.kv b{font-weight:600;font-size:14px;overflow-wrap:anywhere}
.big{font-size:32px;font-weight:800;letter-spacing:-.02em;margin:10px 0 2px;font-variant-numeric:tabular-nums}
.big small{font-size:14px;font-weight:600;color:var(--dim)}
.bar{height:10px;border-radius:6px;background:#1d2330;overflow:hidden;margin:12px 0 8px}
.fill{height:100%;border-radius:6px;background:var(--ok);transition:width .3s ease}
.fill.warn{background:var(--warn)}.fill.bad{background:var(--bad)}
.bar.meta{height:4px;margin:0}
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
.note code{background:#1d2330;border:1px solid var(--line);border-radius:6px;padding:1px 7px;font-size:13px}
.errbox{border-color:#5c2323}
.errbox .msg{color:var(--bad);margin:4px 0 10px;overflow-wrap:anywhere}
footer{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;color:var(--dim);font-size:12px;margin-top:6px}
.controls{display:flex;align-items:center;gap:14px;flex-wrap:wrap}
button{background:transparent;border:1px solid var(--line);border-radius:8px;color:var(--fg);font:inherit;font-size:13px;padding:7px 14px;cursor:pointer}
button:hover{border-color:var(--ok)}
button:active{background:#1d2330}
label{cursor:pointer;display:inline-flex;align-items:center;gap:6px}
:focus-visible{outline:2px solid var(--ok);outline-offset:2px}
.dim{color:var(--dim)}
@media (max-width:560px){.big{font-size:26px}.card{padding:16px}th,td{white-space:normal;overflow-wrap:anywhere}}
</style>
</head>
<body>
<div class="wrap">
<header>
<h1>zcode-bridge</h1>
<p>Status lane, kredensial, dan kuota plan. Data langsung dari bridge ini, tanpa token.</p>
</header>
<main>
<section class="card" id="health" aria-label="Kredensial dan lane"><h2>Kredensial dan lane</h2><p class="note">memuat...</p></section>
<section class="card" id="quota" aria-label="Kuota plan"><h2>Kuota plan</h2><p class="note">memuat...</p></section>
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
var UNIT = {sec:'dtk',min:'mnt',hour:'jam',day:'hari'};
function byId(id){return document.getElementById(id);}
function esc(s){return String(s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];});}
function fmt(n){n=Number(n)||0;if(n>=1e9)return (n/1e9).toFixed(2)+' B';if(n>=1e6)return (n/1e6).toFixed(1)+' M';if(n>=1e3)return (n/1e3).toFixed(0)+' K';return String(n);}
function when(t){if(!t)return '-';return new Date(t*1000).toLocaleString('id-ID',{day:'2-digit',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit'});}
function left(t){if(!t)return '-';var s=Math.max(0,t-Date.now()/1000);var d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60);if(d>0)return d+' hari '+h+' jam';if(h>0)return h+' jam '+m+' mnt';return m+' mnt';}
function now(){return new Date().toLocaleTimeString('id-ID');}
function errCard(title,msg){return '<h2>'+title+'</h2><div class="msg">'+esc(msg)+'</div><p class="note">Periksa apakah bridge masih jalan, lalu klik Muat ulang.</p>';}

function tag(text,cls){return '<span class="tag'+(cls?' '+cls:'')+'">'+esc(text)+'</span>';}

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
  var head = '<div class="row" style="display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap">'
    +'<span class="dim">Akun: '+esc(q.account||'default')+' · sumber: '+esc(q.source||'-')+'</span>'
    +'<span class="dim">'+rows.length+' plan</span></div>';
  var html = head;
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
