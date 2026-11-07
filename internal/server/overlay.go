package server

import (
	"net/http"
)

// handleOverlay serves the compact always-on-top readout: current and average
// generation speed plus today's totals. One page carries three presentation
// modes — card, mini gauge and capsule — so the floating window can be
// resized without another build. `/overlay/{mode}` pre-selects a mode, while
// the browser remembers the last choice in localStorage.
func handleOverlay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(overlayPage))
}

const overlayPage = `<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>zcode-bridge overlay</title>
<style>
:root{--bg:#0c0e13;--card:#151922;--line:#242b38;--fg:#e8ecf4;--dim:#9aa4b5;--ok:#34d399;--accent:#34d399}
*{box-sizing:border-box}
html,body{margin:0;height:100%}
body{background:var(--bg);color:var(--fg);font:13px/1.4 ui-sans-serif,system-ui,"Segoe UI",sans-serif;padding:10px;overflow:hidden}
.card{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:10px 12px;position:relative}
.head{display:flex;justify-content:space-between;align-items:baseline;color:var(--dim);font-size:11px;text-transform:uppercase;letter-spacing:.08em}
.live{display:flex;align-items:baseline;gap:6px;margin:2px 0 6px}
.live b{font-size:30px;font-weight:800;font-variant-numeric:tabular-nums;letter-spacing:-.02em}
.live small{color:var(--dim);font-size:11px}
.row{display:flex;justify-content:space-between;font-size:12px;color:var(--dim);font-variant-numeric:tabular-nums}
.row b{color:var(--fg);font-weight:600}
svg.spark{display:block;width:100%;height:44px;margin-top:8px}
.dot{width:7px;height:7px;border-radius:50%;background:var(--ok);display:inline-block;margin-right:5px}
.modes{position:absolute;top:6px;right:8px;display:flex;gap:4px;opacity:.35;transition:opacity .15s}
.card:hover .modes,.modes:focus-within{opacity:1}
.modes button{font:11px/1 ui-sans-serif,system-ui;color:var(--dim);background:transparent;border:1px solid var(--line);border-radius:6px;padding:4px 6px;cursor:pointer}
.modes button[aria-pressed=true]{color:var(--fg);border-color:var(--accent)}
.mode{display:none}
body[data-mode=card] .mode.card{display:block}
body[data-mode=gauge] .mode.gauge{display:flex}
body[data-mode=capsule] .mode.capsule{display:flex}
.gauge .dial{position:relative;width:132px;height:132px;flex:none}
.gauge .dial svg{display:block;width:132px;height:132px;transform:rotate(-90deg)}
.gauge .dial .mid{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center}
.gauge .dial .mid b{font-size:26px;font-weight:800;font-variant-numeric:tabular-nums}
.gauge .dial .mid small{color:var(--dim);font-size:10px;text-transform:uppercase;letter-spacing:.08em}
.gauge .side{display:flex;flex-direction:column;justify-content:center;gap:6px;min-width:0}
.gauge{align-items:center;gap:14px}
.capsule{align-items:center;gap:8px;padding:7px 12px;line-height:1}
.capsule .lamp{width:8px;height:8px;border-radius:50%;background:var(--ok);flex:none;box-shadow:0 0 0 0 rgba(52,211,153,.5);animation:pulse 2s infinite}
@keyframes pulse{0%{box-shadow:0 0 0 0 rgba(52,211,153,.45)}70%{box-shadow:0 0 0 7px rgba(52,211,153,0)}100%{box-shadow:0 0 0 0 rgba(52,211,153,0)}}
.capsule b{font-size:20px;font-weight:800;font-variant-numeric:tabular-nums}
.capsule small{color:var(--dim);font-size:10px}
.capsule .sep{width:1px;height:16px;background:var(--line)}
[hidden]{display:none!important}
</style>
</head>
<body>
<div class="modes" role="group" aria-label="Mode overlay">
<button type="button" data-mode="capsule" title="Kapsul ringkas">kapsul</button>
<button type="button" data-mode="gauge" title="Gauge mini">gauge</button>
<button type="button" data-mode="card" title="Kartu lengkap">kartu</button>
</div>

<div class="mode card">
<div class="card">
<div class="head"><span><i class="dot"></i>zcode-bridge</span><span id="stamp">-</span></div>
<div class="live"><b id="cur">-</b><small>tok/dtk</small></div>
<div class="row"><span>rata-rata hari ini</span><b id="avg">-</b></div>
<div class="row"><span>token keluar hari ini</span><b id="tok">-</b></div>
<div class="row"><span>permintaan hari ini</span><b id="req">-</b></div>
<svg id="spark" class="spark" viewBox="0 0 300 44" preserveAspectRatio="none" role="img" aria-label="kecepatan terakhir"></svg>
</div>
</div>

<div class="mode gauge">
<div class="card gauge">
<div class="dial">
<svg viewBox="0 0 120 120" aria-hidden="true">
<circle cx="60" cy="60" r="52" fill="none" stroke="#242b38" stroke-width="9"/>
<circle id="arc" cx="60" cy="60" r="52" fill="none" stroke="var(--accent)" stroke-width="9" stroke-linecap="round" stroke-dasharray="326.7" stroke-dashoffset="326.7"/>
</svg>
<div class="mid"><b id="gcur">-</b><small>tok/dtk</small></div>
</div>
<div class="side">
<div class="row"><span>rata-rata</span><b id="gavg">-</b></div>
<div class="row"><span>token hari ini</span><b id="gtok">-</b></div>
<div class="row"><span>permintaan</span><b id="greq">-</b></div>
<div class="row"><span id="gstamp" class="dim">-</span></div>
</div>
</div>
</div>

<div class="mode capsule">
<div class="card capsule">
<i class="lamp" id="plamp"></i>
<b id="ccur">-</b><small>tok/dtk</small>
<i class="sep"></i>
<b id="ctok">-</b><small>token</small>
<i class="sep"></i>
<b id="creq">-</b><small>req</small>
</div>
</div>

<script>
(function(){
function fmt(n){n=Number(n)||0;if(n<10000)return n.toLocaleString('id-ID');if(n>=1e9)return (n/1e9).toFixed(2)+' B';if(n>=1e6)return (n/1e6).toFixed(1)+' M';return (n/1e3).toFixed(0)+' K';}
function color(v){if(v<40)return '#34d399';if(v<80)return '#a3e635';if(v<160)return '#fbbf24';if(v<240)return '#fb923c';if(v<320)return '#f87171';return '#e879f9';}
function byId(id){return document.getElementById(id);}
function token(){return sessionStorage.getItem('zbToken')||'';}
function initialMode(){
  var m=(window.location.pathname.split('/')[2]||'').toLowerCase();
  if(m==='mini')m='gauge';
  if(m&&['card','gauge','capsule'].indexOf(m)>=0)return m;
  var q=new URLSearchParams(window.location.search).get('mode');
  if(q&&['card','gauge','capsule'].indexOf(q)>=0)return q;
  try{var s=localStorage.getItem('zbOverlayMode');if(s&&['card','gauge','capsule'].indexOf(s)>=0)return s;}catch(e){}
  return 'card';
}
function setMode(mode,save){
  document.body.setAttribute('data-mode',mode);
  var btns=document.querySelectorAll('.modes button');
  for(var i=0;i<btns.length;i++)btns[i].setAttribute('aria-pressed',String(btns[i].getAttribute('data-mode')===mode));
  if(save){try{localStorage.setItem('zbOverlayMode',mode);}catch(e){}}
}
var scale=120;
function draw(rows){
  var vals=rows.filter(function(r){return Number(r.tps)>0;}).slice(-60).map(function(r){return Number(r.tps);});
  var peak=vals.reduce(function(a,b){return b>a?b:a;},0);
  scale=Math.max(60,Math.ceil(peak/60)*60);
  var spark=byId('spark');
  if(!vals.length){spark.innerHTML='';return;}
  var W=300,H=44,n=vals.length;
  var line=vals.map(function(v,i){return (i?'L':'M')+(i/(Math.max(1,n-1))*W).toFixed(1)+' '+(H-v/scale*H).toFixed(1);}).join(' ');
  spark.innerHTML='<path d="'+line+'" fill="none" stroke="'+color(vals[vals.length-1])+'" stroke-width="1.6" vector-effect="non-scaling-stroke"/>';
}
function render(res){
  var recent=res.recent||[],last=null,peaks=0;
  recent.forEach(function(r){var v=Number(r.tps)||0;if(v>0){last=r;if(v>peaks)peaks=v;}});
  var today=res.today||{};
  var cur=last?Number(last.tps):0, col=last?color(cur):'#9aa4b5';
  var stamp=new Date().toLocaleTimeString('id-ID',{hour12:false});

  byId('cur').textContent=last?cur.toFixed(1):'-';
  byId('cur').style.color=col;
  byId('avg').textContent=(Number(today.avg_tps)||0).toFixed(1);
  byId('tok').textContent=fmt(today.output_tokens);
  byId('req').textContent=fmt(today.requests);
  byId('stamp').textContent=stamp;
  draw(recent);

  var cap=Math.max(60,Math.ceil(peaks/60)*60);
  var frac=Math.max(0,Math.min(1,cur/cap));
  var arc=byId('arc');
  arc.setAttribute('stroke-dashoffset',String((326.7*(1-frac)).toFixed(1)));
  arc.setAttribute('stroke',col);
  byId('gcur').textContent=last?cur.toFixed(1):'-';
  byId('gavg').textContent=(Number(today.avg_tps)||0).toFixed(1);
  byId('gtok').textContent=fmt(today.output_tokens);
  byId('greq').textContent=fmt(today.requests);
  byId('gstamp').textContent=stamp;

  byId('ccur').textContent=last?cur.toFixed(0):'-';
  byId('ccur').style.color=col;
  byId('ctok').textContent=fmt(today.output_tokens);
  byId('creq').textContent=fmt(today.requests);
  byId('plamp').style.background=col;

  document.documentElement.style.setProperty('--accent',col);
}
function load(){
  var h={}; if(token())h['Authorization']='Bearer '+token();
  fetch('/v1/usage',{headers:h,cache:'no-store'}).then(function(r){return r.json();}).then(function(res){
    render(res);
    try{document.title='zcode-bridge '+((res.today&&res.today.requests)||0)+' req';}catch(e){}
  }).catch(function(){
    byId('stamp').textContent='offline';
    var gs=byId('gstamp'); if(gs)gs.textContent='offline';
    byId('pmood').textContent='offline';
  });
}
setMode(initialMode(),false);
var btns=document.querySelectorAll('.modes button');
for(var i=0;i<btns.length;i++)btns[i].addEventListener('click',function(ev){setMode(ev.currentTarget.getAttribute('data-mode'),true);});
load(); setInterval(load,1000);
})();
</script>
</body>
</html>
`
