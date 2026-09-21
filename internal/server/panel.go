package server

// panelHTML is the embedded management panel: a single file with no external
// dependencies. It only talks JSON to /v0/management/* with the management key
// kept in sessionStorage.
const panelHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>atria2api 控制面板</title>
<style>
  :root { --bg:#0d1117; --panel:#161b22; --border:#2a313b; --text:#dbe2ea; --dim:#8b96a5;
    --accent:#4f8cff; --green:#2ea043; --red:#f85149; --amber:#d29922; --code:#1e242e; }
  * { box-sizing:border-box; }
  body { margin:0; background:var(--bg); color:var(--text);
    font:14px/1.55 -apple-system,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif; }
  header { display:flex; align-items:center; gap:14px; padding:14px 22px;
    border-bottom:1px solid var(--border); position:sticky; top:0; background:var(--bg); z-index:10; }
  header h1 { font-size:17px; margin:0; }
  .dot { width:9px;height:9px;border-radius:50%;background:var(--green);display:inline-block; }
  .dot.off { background:var(--red); }
  header .sp { flex:1; }
  .btn { background:var(--panel); color:var(--text); border:1px solid var(--border);
    border-radius:6px; padding:5px 12px; cursor:pointer; font-size:13px; }
  .btn:hover { border-color:var(--accent); }
  .btn.primary { background:var(--accent); border-color:var(--accent); color:#fff; }
  .btn.danger { border-color:var(--red); color:var(--red); }
  .btn.sm { padding:2px 8px; font-size:12px; }
  main { max-width:1180px; margin:0 auto; padding:20px 22px 60px; }
  section { background:var(--panel); border:1px solid var(--border); border-radius:10px;
    padding:16px 18px; margin-bottom:18px; }
  section h2 { font-size:14px; margin:0 0 12px; color:var(--dim); text-transform:none;
    letter-spacing:.4px; display:flex; align-items:center; gap:8px; }
  table { width:100%; border-collapse:collapse; font-size:13px; }
  th, td { padding:6px 8px; text-align:left; border-bottom:1px solid var(--border); }
  th { color:var(--dim); font-weight:500; white-space:nowrap; }
  td { white-space:nowrap; overflow:hidden; text-overflow:ellipsis; max-width:340px; }
  tr:last-child td { border-bottom:none; }
  code, .mono { font:12px/1.4 ui-monospace,Menlo,Consolas,monospace; background:var(--code);
    padding:1px 5px; border-radius:4px; }
  .cards { display:grid; grid-template-columns:repeat(auto-fill,minmax(180px,1fr)); gap:12px; }
  .card { background:var(--panel); border:1px solid var(--border); border-radius:10px; padding:12px 14px; }
  .card .k { color:var(--dim); font-size:12px; margin-bottom:4px; }
  .card .v { font-size:16px; font-weight:600; }
  .pill { display:inline-block; padding:1px 8px; border-radius:99px; font-size:12px; border:1px solid var(--border); }
  .pill.ok { color:var(--green); border-color:var(--green); }
  .pill.bad { color:var(--red); border-color:var(--red); }
  .pill.warn { color:var(--amber); border-color:var(--amber); }
  .pill.dim { color:var(--dim); }
  form.inline { display:flex; gap:8px; flex-wrap:wrap; margin-top:10px; align-items:center; }
  input, select { background:var(--bg); color:var(--text); border:1px solid var(--border);
    border-radius:6px; padding:5px 9px; font-size:13px; outline:none; }
  input:focus { border-color:var(--accent); }
  input.wide { min-width:220px; }
  .muted { color:var(--dim); }
  .err { color:var(--red); }
  .right { text-align:right; }
  #toast { position:fixed; bottom:22px; left:50%; transform:translateX(-50%);
    background:var(--panel); border:1px solid var(--accent); color:var(--text);
    padding:9px 18px; border-radius:8px; opacity:0; transition:opacity .25s; pointer-events:none; z-index:99; }
  #toast.show { opacity:1; }
  #toast.err { border-color:var(--red); }
  #login { position:fixed; inset:0; background:var(--bg); display:flex;
    align-items:center; justify-content:center; z-index:50; }
  #login .box { width:360px; background:var(--panel); border:1px solid var(--border);
    border-radius:12px; padding:24px; }
  #login h2 { margin:0 0 6px; font-size:16px; }
  #login p { color:var(--dim); font-size:12px; margin:0 0 14px; }
  #login input { width:100%; margin-bottom:12px; }
  .foot { color:var(--dim); font-size:12px; text-align:center; }
</style>
</head>
<body>

<div id="login">
  <div class="box">
    <h2>atria2api 控制面板</h2>
    <p>请输入管理密钥（remote-management.secret-key）。密钥仅保存在当前浏览器会话。</p>
    <input id="mgmtKey" type="password" placeholder="管理密钥" onkeydown="if(event.key==='Enter')doLogin()">
    <button class="btn primary" style="width:100%" onclick="doLogin()">登录</button>
    <div id="loginErr" class="err" style="margin-top:8px;font-size:12px"></div>
  </div>
</div>

<header>
  <span class="dot" id="healthDot"></span>
  <h1>atria2api 控制面板</h1>
  <span class="mono muted" id="upstreamLbl"></span>
  <span class="sp"></span>
  <span class="muted" id="refreshLbl"></span>
  <button class="btn sm" id="autoBtn" onclick="toggleAuto()">自动刷新:开</button>
  <button class="btn sm" onclick="refresh()">刷新</button>
  <button class="btn sm" onclick="logout()">登出</button>
</header>

<main>
  <div class="cards" id="overview"></div>

  <section>
    <h2>上游 Atria Key 池 <span class="pill dim" id="keysPill"></span></h2>
    <table>
      <thead><tr>
        <th>Key (掩码)</th><th>权重</th><th>状态</th><th>原因</th><th>到期</th>
        <th>RPM 余量</th><th>进行中</th><th>成功/失败</th><th class="right">操作</th>
      </tr></thead>
      <tbody id="keysBody"></tbody>
    </table>
    <form class="inline" onsubmit="return addKey(event)">
      <input id="newKey" class="wide" placeholder="atr_ 开头的 key">
      <input id="newWeight" type="number" value="1" min="1" style="width:70px" title="权重">
      <input id="newProxy" placeholder="代理覆盖（可空，如 socks5://host:port 或 none）" style="min-width:260px">
      <button class="btn primary" type="submit">添加上游 Key</button>
    </form>
  </section>

  <section>
    <h2>下游调用 Key（客户端访问本网关用）</h2>
    <table>
      <thead><tr><th>Key (掩码)</th><th class="right">操作</th></tr></thead>
      <tbody id="apiKeysBody"></tbody>
    </table>
    <form class="inline" onsubmit="return addAPIKey(event)">
      <input id="newAPIKey" class="wide" placeholder="新的下游 api key">
      <button class="btn primary" type="submit">添加下游 Key</button>
    </form>
  </section>

  <section>
    <h2>接口调用统计</h2>
    <table>
      <thead><tr>
        <th>路径</th><th>请求数</th><th>错误数</th><th>进行中</th><th>最近状态</th>
        <th>最近调用</th><th>最近错误</th>
      </tr></thead>
      <tbody id="statsBody"></tbody>
    </table>
  </section>

  <section>
    <h2>SOCKS5 代理池 <span class="pill dim" id="proxiesPill"></span></h2>
    <table>
      <thead><tr><th>代理（掩码）</th><th>权重</th><th>健康</th><th>失败次数</th><th>成功次数</th><th>恢复时间</th></tr></thead>
      <tbody id="proxiesBody"></tbody>
    </table>
    <div class="muted" style="margin-top:8px;font-size:12px">代理列表在 config.yaml 的 proxy.socks5 中维护，修改后重启生效。</div>
  </section>

  <section>
    <h2>Token 用量</h2>
    <h2 style="font-size:13px">按 Key</h2>
    <table><thead><tr><th>Key</th><th>请求数</th><th>输入 tokens</th><th>输出 tokens</th><th>错误</th></tr></thead>
      <tbody id="usageKeysBody"></tbody></table>
    <h2 style="font-size:13px;margin-top:14px">按模型</h2>
    <table><thead><tr><th>模型</th><th>请求数</th><th>输入 tokens</th><th>输出 tokens</th></tr></thead>
      <tbody id="usageModelsBody"></tbody></table>
  </section>

  <div class="foot">atria2api · 状态与配置仅缓存于当前会话 · <a class="muted" href="/healthz">/healthz</a></div>
</main>

<div id="toast"></div>

<script>
var KEY = sessionStorage.getItem('atria2api_mgmt') || '';
var AUTO = true, TIMER = null;

function doLogin() {
  KEY = document.getElementById('mgmtKey').value.trim();
  fetch('/v0/management/stats', {headers: hdr()})
    .then(function(r){
      if (!r.ok) throw 0;
      sessionStorage.setItem('atria2api_mgmt', KEY);
      document.getElementById('login').style.display='none';
      start();
    })
    .catch(function(){
      document.getElementById('loginErr').textContent = '密钥无效或被拒绝（401/403）';
    });
}
function logout() { sessionStorage.removeItem('atria2api_mgmt'); location.reload(); }
function hdr() { return {'X-Management-Key': KEY, 'Content-Type':'application/json'}; }

function api(path, method, body) {
  var opts = {method: method||'GET', headers: hdr()};
  if (body) opts.body = JSON.stringify(body);
  return fetch(path, opts).then(function(r){
    return r.json().then(function(j){
      if (!r.ok) throw (j && j.error && j.error.message) || ('HTTP '+r.status);
      return j;
    });
  });
}

function toast(msg, isErr) {
  var t = document.getElementById('toast');
  t.textContent = msg; t.className = 'show' + (isErr ? ' err' : '');
  setTimeout(function(){ t.className = ''; }, 2600);
}

function esc(s) { return String(s==null?'':s).replace(/[&<>"]/g, function(c){
  return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]; }); }

function fmtTime(t) {
  if (!t || t.indexOf('0001-')===0) return '-';
  var d = new Date(t);
  return d.toLocaleTimeString('zh-CN', {hour12:false});
}
function pill(state, text) {
  var cls = 'dim';
  if (state==='healthy'||state==='ok'||state==='enabled') cls='ok';
  else if (state==='disabled') cls='bad';
  else if (state==='cooldown'||state==='manual_off'||state==='rpm_reserve') cls='warn';
  return '<span class="pill '+cls+'">'+esc(text||state)+'</span>';
}

function refresh() {
  Promise.all([
    api('/v0/management/stats'),
    api('/v0/management/keys'),
    api('/v0/management/api-keys'),
    api('/v0/management/usage').catch(function(){ return {enabled:false}; }),
    api('/v0/management/proxies')
  ]).then(function(all){
    renderOverview(all[0]); renderKeys(all[0], all[1]); renderAPIKeys(all[2]);
    renderStats(all[0]); renderProxies(all[0], all[4]); renderUsage(all[3]);
    document.getElementById('healthDot').className =
      'dot' + (all[0].keys_healthy>0 ? '' : ' off');
  }).catch(function(e){
    toast('刷新失败: '+e, true);
  });
}

function start() {
  refresh();
  if (TIMER) clearInterval(TIMER);
  TIMER = setInterval(function(){ if (AUTO) refresh(); }, 2500);
}
function toggleAuto() {
  AUTO = !AUTO;
  document.getElementById('autoBtn').textContent = '自动刷新:' + (AUTO?'开':'关');
}
function refreshLbl(msg) { document.getElementById('refreshLbl').textContent = msg; }

function renderOverview(s) {
  var p = s.ports || {};
  var cards = [
    ['监听端口', p.http + (p.tls ? ' (TLS)' : '')],
    ['调试端口', p.pprof],
    ['上游地址', p.upstream],
    ['默认模型', p.default_model],
    ['上游 Key', s.keys_healthy + ' 可用 / ' + s.keys_total + ' 总计'],
    ['代理节点', (s.proxies_total||0) + ' 个'],
    ['客户端鉴权', p.auth_required ? '已开启' : '开放'],
    ['远程管理', p.mgmt_remote ? '允许' : '仅本机']
  ];
  document.getElementById('overview').innerHTML = cards.map(function(c){
    return '<div class="card"><div class="k">'+esc(c[0])+'</div><div class="v mono">'+esc(c[1])+'</div></div>';
  }).join('');
  document.getElementById('upstreamLbl').textContent = p.upstream || '';
  refreshLbl('每 2.5 秒自动刷新');
}

function renderKeys(s, keys) {
  document.getElementById('keysPill').textContent = s.keys_healthy + '/' + s.keys_total;
  var rows = (keys.keys||[]).map(function(k){
    var until = (k.until && k.until.indexOf('0001-')!==0) ? fmtTime(k.until) : '-';
    var act = k.state==='disabled' || k.state==='manual_off'
      ? '<button class="btn sm" onclick="keyAct(\''+k.id+'\',\'enable\')">启用</button>'
      : '<button class="btn sm" onclick="keyAct(\''+k.id+'\',\'disable\')">禁用</button>';
    act += ' <button class="btn sm danger" onclick="keyAct(\''+k.id+'\',\'del\')">删除</button>';
    return '<tr><td class="mono">'+esc(k.id)+'</td><td>'+k.weight+'</td>'
      + '<td>'+pill(k.state)+'</td><td class="muted">'+esc(k.reason||'-')+'</td>'
      + '<td>'+until+'</td>'
      + '<td>'+(k.rpm_limit ? (k.rpm_remaining+' / '+k.rpm_limit) : '-')+'</td>'
      + '<td>'+k.inflight+'</td><td>'+k.success_count+' / <span class="'+(k.error_count?'err':'')+'">'+k.error_count+'</span></td>'
      + '<td class="right">'+act+'</td></tr>';
  }).join('') || '<tr><td colspan="9" class="muted">暂无上游 key，在下方添加</td></tr>';
  document.getElementById('keysBody').innerHTML = rows;
}

function keyAct(id, action) {
  var path = '/v0/management/keys/' + id + '/' + action;
  if (action === 'del') {
    if (!confirm('确认删除该上游 key？')) return;
    path = '/v0/management/keys/' + id;
    api(path, 'DELETE').then(function(){ toast('已删除'); refresh(); })
      .catch(function(e){ toast('删除失败: '+e, true); });
    return;
  }
  api(path, 'POST').then(function(){ toast(action==='disable'?'已禁用':'已启用'); refresh(); })
    .catch(function(e){ toast('操作失败: '+e, true); });
}

function addKey(e) {
  e.preventDefault();
  var key = document.getElementById('newKey').value.trim();
  if (!key) { toast('请填写 key', true); return false; }
  api('/v0/management/keys', 'POST', {
    key: key,
    weight: parseInt(document.getElementById('newWeight').value) || 1,
    proxy: document.getElementById('newProxy').value.trim()
  }).then(function(){
    document.getElementById('newKey').value='';
    document.getElementById('newProxy').value='';
    toast('上游 key 已添加并生效'); refresh();
  }).catch(function(err){ toast('添加失败: '+err, true); });
  return false;
}

function renderAPIKeys(res) {
  var rows = (res.keys||[]).map(function(k){
    return '<tr><td class="mono">'+esc(k.key)+'</td>'
      + '<td class="right"><button class="btn sm danger" onclick="delAPIKey(\''+k.id+'\')">删除</button></td></tr>';
  }).join('') || '<tr><td colspan="2" class="muted">未配置下游 key（网关当前为开放模式）</td></tr>';
  document.getElementById('apiKeysBody').innerHTML = rows;
}
function addAPIKey(e) {
  e.preventDefault();
  var v = document.getElementById('newAPIKey').value.trim();
  if (!v) { toast('请填写 key', true); return false; }
  api('/v0/management/api-keys', 'POST', {key: v}).then(function(){
    document.getElementById('newAPIKey').value='';
    toast('下游 key 已添加'); refresh();
  }).catch(function(err){ toast('添加失败: '+err, true); });
  return false;
}
function delAPIKey(id) {
  if (!confirm('确认删除该下游 key？使用它的客户端将立即失效。')) return;
  api('/v0/management/api-keys/'+id, 'DELETE').then(function(){ toast('已删除'); refresh(); })
    .catch(function(e){ toast('删除失败: '+e, true); });
}

function renderStats(s) {
  var eps = s.endpoints || {};
  var keys = Object.keys(eps).sort();
  var rows = keys.map(function(p){
    var c = eps[p];
    return '<tr><td class="mono">'+esc(p)+'</td><td>'+c.requests+'</td>'
      + '<td class="'+(c.errors?'err':'')+'">'+c.errors+'</td><td>'+c.inflight+'</td>'
      + '<td>'+(c.last_status||'-')+'</td><td class="muted">'+fmtTime(c.last_call)+'</td>'
      + '<td class="err" title="'+esc(c.last_error||'')+'">'+esc((c.last_error||'').slice(0,60))+'</td></tr>';
  }).join('') || '<tr><td colspan="7" class="muted">尚无调用记录</td></tr>';
  document.getElementById('statsBody').innerHTML = rows;
}

function renderProxies(s, res) {
  var list = res.proxies || [];
  document.getElementById('proxiesPill').textContent = list.length + ' 个';
  var rows = list.map(function(px){
    return '<tr><td class="mono">'+esc(px.url)+'</td><td>'+px.weight+'</td>'
      + '<td>'+(px.healthy ? pill('ok','正常') : pill('bad','故障'))+'</td>'
      + '<td class="'+(px.fail_count?'err':'')+'">'+px.fail_count+'</td>'
      + '<td>'+px.success_count+'</td><td>'+fmtTime(px.fail_until)+'</td></tr>';
  }).join('') || '<tr><td colspan="6" class="muted">未配置代理（直连）</td></tr>';
  document.getElementById('proxiesBody').innerHTML = rows;
}

function renderUsage(u) {
  if (!u || !u.keys) {
    document.getElementById('usageKeysBody').innerHTML =
      '<tr><td colspan="5" class="muted">用量统计未启用</td></tr>';
    document.getElementById('usageModelsBody').innerHTML =
      '<tr><td colspan="4" class="muted">-</td></tr>';
    return;
  }
  var rows = Object.keys(u.keys).map(function(id){
    var k = u.keys[id];
    return '<tr><td class="mono">'+esc(id)+'</td><td>'+k.requests+'</td>'
      + '<td>'+k.prompt_tokens+'</td><td>'+k.completion_tokens+'</td>'
      + '<td class="'+(k.errors?'err':'')+'">'+k.errors+'</td></tr>';
  }).join('') || '<tr><td colspan="5" class="muted">暂无用量</td></tr>';
  document.getElementById('usageKeysBody').innerHTML = rows;
  var mrows = Object.keys(u.models||{}).map(function(m){
    var v = u.models[m];
    return '<tr><td class="mono">'+esc(m)+'</td><td>'+v.requests+'</td>'
      + '<td>'+v.prompt_tokens+'</td><td>'+v.completion_tokens+'</td></tr>';
  }).join('') || '<tr><td colspan="4" class="muted">暂无用量</td></tr>';
  document.getElementById('usageModelsBody').innerHTML = mrows;
}

if (KEY) {
  document.getElementById('login').style.display='none';
  start();
} else {
  document.getElementById('mgmtKey').focus();
}
</script>
</body>
</html>`
