package server

import (
	"net/http"
)

// handleOverlay serves a compact always-on-top floating readout: current and
// average generation speed plus today's token total. It is the bridge-native
// counterpart of the speed panel's floating widget — same data the Usage tab
// renders, one small page, refreshed once per second.
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
:root{--bg:#0c0e13;--card:#151922;--line:#242b38;--fg:#e8ecf4;--dim:#9aa4b5;--ok:#34d399}
*{box-sizing:border-box}
html,body{margin:0;height:100%}
body{background:var(--bg);color:var(--fg);font:13px/1.4 ui-sans-serif,system-ui,"Segoe UI",sans-serif;padding:10px}
.card{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:10px 12px}
.head{display:flex;justify-content:space-between;align-items:baseline;color:var(--dim);font-size:11px;text-transform:uppercase;letter-spacing:.08em}
.live{display:flex;align-items:baseline;gap:6px;margin:2px 0 6px}
.live b{font-size:30px;font-weight:800;font-variant-numeric:tabular-nums;letter-spacing:-.02em}
.live small{color:var(--dim);font-size:11px}
.row{display:flex;justify-content:space-between;font-size:12px;color:var(--dim);font-variant-numeric:tabular-nums}
.row b{color:var(--fg);font-weight:600}
svg{display:block;width:100%;height:44px;margin-top:8px}
.dot{width:7px;height:7px;border-radius:50%;background:var(--ok);display:inline-block;margin-right:5px}
</style>
</head>
<body>
<div class="card">
<div class="head"><span><i class="dot"></i>zcode-bridge</span><span id="stamp">-</span></div>
<div class="live"><b id="cur">-</b><small>tok/dtk</small></div>
<div class="row"><span>rata-rata hari ini</span><b id="avg">-</b></div>
<div class="row"><span>token keluar hari ini</span><b id="tok">-</b></div>
<div class="row"><span>permintaan hari ini</span><b id="req">-</b></div>
<svg id="spark" viewBox="0 0 300 44" preserveAspectRatio="none" role="img" aria-label="kecepatan terakhir"></svg>
</div>
<script>
(function(){
function fmt(n){n=Number(n)||0;if(n<10000)return n.toLocaleString('id-ID');if(n>=1e9)return (n/1e9).toFixed(2)+' B';if(n>=1e6)return (n/1e6).toFixed(1)+' M';return (n/1e3).toFixed(0)+' K';}
function color(v){if(v<40)return '#34d399';if(v<80)return '#a3e635';if(v<160)return '#fbbf24';if(v<240)return '#fb923c';if(v<320)return '#f87171';return '#e879f9';}
function token(){return sessionStorage.getItem('zbToken')||'';}
var scale=120;
function draw(rows){
  var vals=rows.filter(function(r){return Number(r.tps)>0;}).slice(-60).map(function(r){return Number(r.tps);});
  var peak=vals.reduce(function(a,b){return b>a?b:a;},0);
  scale=Math.max(60,Math.ceil(peak/60)*60);
  if(!vals.length){document.getElementById('spark').innerHTML='';return;}
  var W=300,H=44,n=vals.length;
  var line=vals.map(function(v,i){return (i?'L':'M')+(i/(Math.max(1,n-1))*W).toFixed(1)+' '+(H-v/scale*H).toFixed(1);}).join(' ');
  document.getElementById('spark').innerHTML='<path d="'+line+'" fill="none" stroke="'+color(vals[vals.length-1])+'" stroke-width="1.6" vector-effect="non-scaling-stroke"/>';
}
function load(){
  var h={'cache':'no-store'}; if(token()) h['Authorization']='Bearer '+token();
  fetch('/v1/usage',{headers:h,cache:'no-store'}).then(function(r){return r.json();}).then(function(res){
    var recent=res.recent||[], last=null;
    recent.forEach(function(r){if(Number(r.tps)>0)last=r;});
    var today=res.today||{};
    document.getElementById('cur').textContent=last?(Number(last.tps)).toFixed(1):'-';
    document.getElementById('cur').style.color=last?color(Number(last.tps)):'var(--dim)';
    document.getElementById('avg').textContent=(Number(today.avg_tps)||0).toFixed(1);
    document.getElementById('tok').textContent=fmt(today.output_tokens);
    document.getElementById('req').textContent=fmt(today.requests);
    document.getElementById('stamp').textContent=new Date().toLocaleTimeString('id-ID',{hour12:false});
    draw(recent);
  }).catch(function(){document.getElementById('stamp').textContent='offline';});
}
load(); setInterval(load,1000);
})();
</script>
</body>
</html>
`
