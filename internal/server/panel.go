package server

// panelHTML is the embedded management panel: a single file with no external
// dependencies. It only talks JSON to /v0/management/* with the management key
// kept in sessionStorage. Layout reflows for phone, tablet and desktop.
const panelHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="color-scheme" content="dark">
<title>atria2api 控制面板</title>
<style>
  :root {
    --bg:#0b0e14; --panel:#141922; --panel-2:#1b2230; --border:#2a3344;
    --text:#e7edf5; --dim:#93a0b4; --accent:#6ea8ff; --accent-2:#3d7dff;
    --green:#3fb950; --red:#ff7b72; --amber:#e3b341; --code:#10151d;
    --shadow:0 10px 30px rgba(0,0,0,.28);
    --pad:16px; --radius:14px;
  }
  * { box-sizing:border-box; }
  html, body { margin:0; background:var(--bg); color:var(--text); }
  body {
    font:15px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;
    padding:env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);
  }
  button, input, select, textarea { font:inherit; color:inherit; }
  header {
    position:sticky; top:0; z-index:20;
    display:flex; flex-wrap:wrap; align-items:center; gap:10px 14px;
    padding:12px var(--pad);
    background:rgba(11,14,20,.92); backdrop-filter:saturate(140%) blur(10px);
    border-bottom:1px solid var(--border);
  }
  .brand { display:flex; align-items:center; gap:10px; min-width:0; }
  .brand h1 { font-size:16px; margin:0; font-weight:650; letter-spacing:.2px; }
  .brand .sub { color:var(--dim); font-size:12px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; max-width:42vw; }
  .dot { width:9px; height:9px; border-radius:50%; background:var(--green); box-shadow:0 0 0 4px rgba(63,185,80,.15); flex:none; }
  .dot.off { background:var(--red); box-shadow:0 0 0 4px rgba(255,123,114,.15); }
  .sp { flex:1 1 8px; }
  .tools { display:flex; flex-wrap:wrap; gap:8px; align-items:center; }
  .btn {
    background:var(--panel-2); color:var(--text); border:1px solid var(--border);
    border-radius:10px; padding:8px 12px; cursor:pointer; min-height:36px;
  }
  .btn:hover { border-color:var(--accent); }
  .btn.primary { background:var(--accent-2); border-color:var(--accent-2); color:#fff; }
  .btn.danger { border-color:transparent; background:rgba(255,123,114,.12); color:var(--red); }
  .btn.sm { padding:6px 10px; min-height:32px; font-size:13px; }
  main { width:min(1180px, 100%); margin:0 auto; padding:18px var(--pad) 72px; }
  section {
    background:var(--panel); border:1px solid var(--border); border-radius:var(--radius);
    padding:16px; margin-bottom:16px; box-shadow:var(--shadow);
  }
  section h2 {
    font-size:15px; margin:0 0 4px; font-weight:650; display:flex; align-items:center; gap:8px; flex-wrap:wrap;
  }
  .hint { color:var(--dim); font-size:12.5px; margin:0 0 12px; }
  .cards { display:grid; grid-template-columns:repeat(auto-fit,minmax(150px,1fr)); gap:10px; margin-bottom:16px; }
  .card { background:var(--panel); border:1px solid var(--border); border-radius:var(--radius); padding:12px 14px; min-width:0; }
  .card .k { color:var(--dim); font-size:12px; margin-bottom:4px; }
  .card .v { font-size:15px; font-weight:650; overflow-wrap:anywhere; }
  .pill { display:inline-flex; align-items:center; padding:1px 8px; border-radius:99px; font-size:12px; border:1px solid var(--border); white-space:nowrap; }
  .pill.ok { color:var(--green); border-color:rgba(63,185,80,.55); background:rgba(63,185,80,.08); }
  .pill.bad { color:var(--red); border-color:rgba(255,123,114,.55); background:rgba(255,123,114,.08); }
  .pill.warn { color:var(--amber); border-color:rgba(227,179,65,.55); background:rgba(227,179,65,.08); }
  .pill.dim { color:var(--dim); }
  .scroll { overflow-x:auto; -webkit-overflow-scrolling:touch; margin:0 -4px; padding:0 4px; }
  table { width:100%; border-collapse:collapse; font-size:13px; min-width:640px; }
  table.slim { min-width:0; }
  th, td { padding:8px 8px; text-align:left; border-bottom:1px solid var(--border); vertical-align:middle; }
  th { color:var(--dim); font-weight:550; white-space:nowrap; }
  td { overflow-wrap:anywhere; }
  tr:last-child td { border-bottom:none; }
  .mono, code { font:12.5px/1.45 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; }
  .mono { background:var(--code); padding:2px 6px; border-radius:6px; }
  form.row, .row { display:flex; gap:8px; flex-wrap:wrap; align-items:center; margin-top:12px; }
  input, select, textarea {
    background:var(--bg); border:1px solid var(--border); border-radius:10px;
    padding:9px 11px; outline:none; min-height:40px; width:100%;
  }
  input:focus, select:focus, textarea:focus { border-color:var(--accent); }
  .grow { flex:1 1 220px; min-width:0; }
  .narrow { flex:0 1 110px; }
  textarea { min-height:120px; resize:vertical; }
  label.chk { display:flex; align-items:center; gap:8px; color:var(--dim); min-height:40px; }
  .muted { color:var(--dim); }
  .err { color:var(--red); }
  .right { text-align:right; }
  .num { font-variant-numeric:tabular-nums; white-space:nowrap; }
  .actions { display:flex; gap:6px; justify-content:flex-end; flex-wrap:wrap; }
  .subhead { margin:16px 0 8px; font-size:13px; color:var(--dim); font-weight:650; }
  #toast {
    position:fixed; left:50%; bottom:max(18px, env(safe-area-inset-bottom));
    transform:translateX(-50%); max-width:min(92vw, 480px);
    background:var(--panel-2); border:1px solid var(--accent); color:var(--text);
    padding:10px 16px; border-radius:12px; opacity:0; transition:opacity .2s;
    pointer-events:none; z-index:99; text-align:center;
  }
  #toast.show { opacity:1; }
  #toast.err { border-color:var(--red); }
  #login {
    position:fixed; inset:0; z-index:50; display:flex; align-items:center; justify-content:center;
    padding:20px; background:
      radial-gradient(900px 420px at 20% -10%, rgba(61,125,255,.22), transparent 60%),
      var(--bg);
  }
  #login .box { width:min(420px, 100%); background:var(--panel); border:1px solid var(--border); border-radius:18px; padding:24px; box-shadow:var(--shadow); }
  #login h2 { margin:0 0 6px; font-size:20px; }
  #login p { color:var(--dim); font-size:13px; margin:0 0 16px; }
  #login input { margin-bottom:12px; }
  .foot { color:var(--dim); font-size:12px; text-align:center; }
  .foot a { color:var(--dim); }
  details { margin-top:12px; }
  summary { cursor:pointer; color:var(--dim); }
  @media (max-width: 720px) {
    :root { --pad:12px; }
    .brand .sub { display:none; }
    .tools { width:100%; }
    .tools .btn { flex:1 1 auto; }
    table.hide-phone { display:none; }
    .cards { grid-template-columns:1fr 1fr; }
  }
  @media (min-width: 721px) {
    .phone-list { display:none; }
  }
  .kv { display:grid; gap:8px; }
  .kv .item {
    display:grid; grid-template-columns:88px 1fr; gap:8px; align-items:start;
    padding:10px 0; border-bottom:1px solid var(--border); font-size:13px;
  }
  .kv .item:last-child { border-bottom:none; }
  .kv .item b { color:var(--dim); font-weight:550; }
</style>
</head>
<body>

<div id="login">
  <div class="box">
    <h2>atria2api 控制面板</h2>
    <p>输入部署时设置的面板密码。它只存在于当前浏览器会话，不能当作下游调用密钥，下游密钥也登不了这个面板。</p>
    <input id="mgmtKey" type="password" placeholder="面板密码" autocomplete="current-password" onkeydown="if(event.key==='Enter')doLogin()">
    <button class="btn primary" style="width:100%" onclick="doLogin()">登录</button>
    <div id="loginErr" class="err" style="margin-top:10px;font-size:13px"></div>
  </div>
</div>

<header>
  <div class="brand">
    <span class="dot" id="healthDot"></span>
    <div>
      <h1>atria2api 控制面板</h1>
      <div class="sub mono" id="upstreamLbl"></div>
    </div>
  </div>
  <span class="sp"></span>
  <div class="tools">
    <span class="muted" id="refreshLbl" style="font-size:12px"></span>
    <button class="btn sm" id="autoBtn" onclick="toggleAuto()">自动刷新：开</button>
    <button class="btn sm" onclick="refresh()">刷新</button>
    <button class="btn sm" onclick="logout()">退出</button>
  </div>
</header>

<main>
  <div class="cards" id="overview"></div>

  <section>
    <h2>上游 Atria 密钥 <span class="pill dim" id="keysPill"></span></h2>
    <p class="hint">这些密钥用来调用 Atria。列表里只显示哈希编号，不显示原文。同一账户下的密钥共享每分钟额度。</p>
    <div class="scroll">
      <table>
        <thead><tr>
          <th>编号</th><th>权重</th><th>状态</th><th>原因</th><th>恢复</th>
          <th>每分钟余量</th><th>进行中</th><th>成功 / 失败</th>
          <th>请求</th><th>输入</th><th>输出</th><th>合计</th><th class="right">操作</th>
        </tr></thead>
        <tbody id="keysBody"></tbody>
      </table>
    </div>
    <form class="row" onsubmit="return addKey(event)">
      <input id="newKey" class="grow" placeholder="atr_ 开头的上游密钥" autocomplete="off">
      <input id="newWeight" class="narrow" type="number" value="1" min="1" title="权重" inputmode="numeric">
      <input id="newProxy" class="grow" placeholder="单独代理，可留空。填 socks5://地址:端口，或 none 表示直连">
      <button class="btn primary" type="submit">添加上游密钥</button>
    </form>
    <details id="bulkBox">
      <summary id="bulkBtn">批量导入</summary>
      <p class="hint">每行一条密钥。空行和以 # 开头的行会忽略，重复的会去掉。不以 atr_ 开头的行会跳过。</p>
      <textarea id="bulkText" placeholder="atr_xxx&#10;atr_yyy&#10;# 注释会被忽略"></textarea>
      <div class="row">
        <input type="file" id="bulkFile" accept=".txt,text/plain" class="grow">
        <button class="btn sm" type="button" onclick="loadBulkFile()">读取文本文件</button>
        <button class="btn sm" type="button" onclick="previewBulk()">预览</button>
        <span id="bulkPreview" class="muted" style="font-size:12px"></span>
      </div>
      <div class="row">
        <span class="muted" style="font-size:13px">统一权重</span>
        <input id="bulkWeight" class="narrow" type="number" value="1" min="1" inputmode="numeric">
        <input id="bulkProxy" class="grow" placeholder="统一代理，可留空">
        <button class="btn primary" type="button" onclick="importBulk()">开始导入</button>
      </div>
    </details>
  </section>

  <section>
    <h2>下游调用密钥</h2>
    <p class="hint">客户端访问这个网关时使用。和面板密码无关，删掉后对应客户端会立刻无法调用。</p>
    <div class="scroll">
      <table class="slim">
        <thead><tr><th>密钥（已掩码）</th><th class="right">操作</th></tr></thead>
        <tbody id="apiKeysBody"></tbody>
      </table>
    </div>
    <form class="row" onsubmit="return addAPIKey(event)">
      <input id="newAPIKey" class="grow" placeholder="新的下游调用密钥" autocomplete="off">
      <button class="btn primary" type="submit">添加下游密钥</button>
    </form>
  </section>

  <section>
    <h2>接口调用</h2>
    <p class="hint">本网关收到的请求，不是上游 Atria 的账单。</p>
    <div class="scroll">
      <table>
        <thead><tr>
          <th>路径</th><th>请求</th><th>错误</th><th>进行中</th><th>最近状态</th><th>最近时间</th><th>最近错误</th>
        </tr></thead>
        <tbody id="statsBody"></tbody>
      </table>
    </div>
  </section>

  <section>
    <h2>网关设置</h2>
    <p class="hint">保存后立即生效，并写回配置文件。改上游地址仍需编辑配置文件后重载。</p>
    <form onsubmit="return saveSettings(event)">
      <div class="row">
        <input id="setModel" class="grow" placeholder="默认模型，例如 Atria-Dawn-Preview">
        <select id="setPolicy" class="grow" title="代理策略">
          <option value="round-robin">轮询</option>
          <option value="random">随机</option>
          <option value="sticky-key">按密钥固定</option>
        </select>
        <button class="btn primary" type="submit">保存设置</button>
      </div>
      <label class="chk"><input id="setForce" type="checkbox"> 把客户端传来的模型名改写成上面的默认模型</label>
    </form>
  </section>

  <section>
    <h2>SOCKS5 代理 <span class="pill dim" id="proxiesPill"></span></h2>
    <p class="hint">每行一个 socks5:// 地址。空行和 # 注释会忽略。保存会替换整个代理池；留空保存则改为直连。</p>
    <div class="scroll">
      <table class="slim">
        <thead><tr><th>地址（已掩码）</th><th>权重</th><th>状态</th><th>失败</th><th>成功</th><th>恢复</th></tr></thead>
        <tbody id="proxiesBody"></tbody>
      </table>
    </div>
    <textarea id="proxyText" rows="4" placeholder="socks5://用户:密码@1.2.3.4:1080"></textarea>
    <div class="row">
      <button class="btn primary" type="button" onclick="saveProxies()">保存代理池</button>
      <span id="proxyHint" class="muted" style="font-size:12px"></span>
    </div>
  </section>

  <section>
    <h2>Token 用量</h2>
    <p class="hint">成功请求里解析到的用量。流式响应要等结束才会记入。</p>
    <div class="subhead">按密钥</div>
    <div class="scroll">
      <table class="slim"><thead><tr><th>编号</th><th>请求</th><th>输入</th><th>输出</th><th>错误</th></tr></thead>
        <tbody id="usageKeysBody"></tbody></table>
    </div>
    <div class="subhead">按模型</div>
    <div class="scroll">
      <table class="slim"><thead><tr><th>模型</th><th>请求</th><th>输入</th><th>输出</th></tr></thead>
        <tbody id="usageModelsBody"></tbody></table>
    </div>
  </section>

  <div class="foot">atria2api · 面板密码只留在当前会话 · <a href="/healthz">健康检查</a></div>
</main>

<div id="toast"></div>

<script>
var KEY = sessionStorage.getItem('atria2api_mgmt') || '';
var AUTO = true, TIMER = null;

function doLogin() {
  KEY = document.getElementById('mgmtKey').value.trim();
  fetch('/v0/management/stats', {headers: hdr()})
    .then(function(r){
      if (!r.ok) {
        var msg = '面板密码不正确';
        if (r.status === 403) msg = '当前部署没有对公网开放面板。请设置 allow-remote: true，或环境变量 ATRIA2API_ALLOW_REMOTE=1';
        else if (r.status === 404) msg = '还没有设置面板密码。请配置 secret-key 或 ATRIA2API_MGMT_KEY';
        else if (r.status === 429) msg = '尝试次数过多，这个 IP 已锁定 15 分钟';
        document.getElementById('loginErr').textContent = msg;
        throw 0;
      }
      sessionStorage.setItem('atria2api_mgmt', KEY);
      document.getElementById('login').style.display = 'none';
      start();
    })
    .catch(function(){});
}
function logout() { sessionStorage.removeItem('atria2api_mgmt'); location.reload(); }
function hdr() { return {'X-Management-Key': KEY, 'Content-Type':'application/json'}; }

function api(path, method, body) {
  var opts = {method: method || 'GET', headers: hdr()};
  if (body) opts.body = JSON.stringify(body);
  return fetch(path, opts).then(function(r){
    return r.text().then(function(text){
      var j = {};
      if (text) { try { j = JSON.parse(text); } catch (e) { j = {}; } }
      if (!r.ok) throw (j && j.error && j.error.message) || ('请求失败 ' + r.status);
      return j;
    });
  });
}

function toast(msg, isErr) {
  var t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'show' + (isErr ? ' err' : '');
  clearTimeout(toast._t);
  toast._t = setTimeout(function(){ t.className = ''; }, 2600);
}

function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"]/g, function(c){
    return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c];
  });
}
function attr(s) { return esc(s).replace(/'/g, '&#39;'); }

function fmtTime(t) {
  if (!t || String(t).indexOf('0001-') === 0) return '—';
  var d = new Date(t);
  if (isNaN(d.getTime())) return '—';
  return d.toLocaleString('zh-CN', {hour12:false, month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit', second:'2-digit'});
}
var STATE_TEXT = {
  healthy:'可用', ok:'正常', enabled:'已启用', disabled:'已停用',
  cooldown:'冷却中', manual_off:'已禁用', rpm_reserve:'额度不足'
};
function stateText(state) { return STATE_TEXT[state] || state || '未知'; }
function pill(state, text) {
  var cls = 'dim';
  if (state === 'healthy' || state === 'ok' || state === 'enabled') cls = 'ok';
  else if (state === 'disabled') cls = 'bad';
  else if (state === 'cooldown' || state === 'manual_off' || state === 'rpm_reserve') cls = 'warn';
  return '<span class="pill ' + cls + '">' + esc(text || stateText(state)) + '</span>';
}
function fmtTokens(n) { return String(n || 0).replace(/\B(?=(\d{3})+(?!\d))/g, ','); }
function emptyRow(cols, text) {
  return '<tr><td colspan="' + cols + '" class="muted">' + esc(text) + '</td></tr>';
}

function refresh() {
  Promise.all([
    api('/v0/management/stats'),
    api('/v0/management/keys'),
    api('/v0/management/api-keys'),
    api('/v0/management/usage').catch(function(){ return {enabled:false}; }),
    api('/v0/management/proxies'),
    api('/v0/management/settings').catch(function(){ return null; })
  ]).then(function(all){
    renderOverview(all[0]);
    renderKeys(all[0], all[1], all[3]);
    renderAPIKeys(all[2]);
    renderStats(all[0]);
    renderProxies(all[4]);
    renderUsage(all[3]);
    renderSettings(all[5]);
    document.getElementById('healthDot').className = 'dot' + (all[0].keys_healthy > 0 ? '' : ' off');
  }).catch(function(e){ toast('刷新失败：' + e, true); });
}

function start() {
  refresh();
  if (TIMER) clearInterval(TIMER);
  TIMER = setInterval(function(){ if (AUTO) refresh(); }, 2500);
}
function toggleAuto() {
  AUTO = !AUTO;
  document.getElementById('autoBtn').textContent = '自动刷新：' + (AUTO ? '开' : '关');
}
function refreshLbl(msg) { document.getElementById('refreshLbl').textContent = msg; }

function renderOverview(s) {
  var p = s.ports || {};
  var cards = [
    ['监听', (p.http || '—') + (p.tls ? ' · 已启用 TLS' : '')],
    ['上游', p.upstream || '—'],
    ['默认模型', p.default_model || '—'],
    ['上游密钥', (s.keys_healthy || 0) + ' 可用 / ' + (s.keys_total || 0) + ' 个'],
    ['代理', (s.proxies_total || 0) ? ((s.proxies_total || 0) + ' 个节点') : '直连'],
    ['客户端鉴权', p.auth_required ? '需要下游密钥' : '未设置，任何人可调用'],
    ['面板访问', p.mgmt_remote ? '允许远程' : '仅本机'],
    ['调试端口', p.pprof || '未开启']
  ];
  document.getElementById('overview').innerHTML = cards.map(function(c){
    return '<div class="card"><div class="k">' + esc(c[0]) + '</div><div class="v">' + esc(c[1]) + '</div></div>';
  }).join('');
  document.getElementById('upstreamLbl').textContent = p.upstream || '';
  refreshLbl(AUTO ? '每 2.5 秒刷新' : '已暂停刷新');
}

function renderKeys(s, keys, usage) {
  document.getElementById('keysPill').textContent = (s.keys_healthy || 0) + ' / ' + (s.keys_total || 0);
  var byID = (usage && usage.keys) || {};
  var rows = (keys.keys || []).map(function(k){
    var until = (k.until && String(k.until).indexOf('0001-') !== 0) ? fmtTime(k.until) : '—';
    var on = k.state === 'disabled' || k.state === 'manual_off';
    var act = on
      ? '<button class="btn sm" onclick="keyAct(\'' + attr(k.id) + '\',\'enable\')">启用</button>'
      : '<button class="btn sm" onclick="keyAct(\'' + attr(k.id) + '\',\'disable\')">禁用</button>';
    act += '<button class="btn sm danger" onclick="keyAct(\'' + attr(k.id) + '\',\'del\')">删除</button>';
    var u = byID[k.id] || {};
    var total = (u.prompt_tokens || 0) + (u.completion_tokens || 0);
    return '<tr><td class="mono">' + esc(k.id) + '</td><td class="num">' + esc(k.weight) + '</td>'
      + '<td>' + pill(k.state) + '</td><td class="muted">' + esc(k.reason || '—') + '</td><td>' + until + '</td>'
      + '<td class="num">' + (k.rpm_limit ? (k.rpm_remaining + ' / ' + k.rpm_limit) : '—') + '</td>'
      + '<td class="num">' + esc(k.inflight) + '</td>'
      + '<td class="num">' + esc(k.success_count) + ' / <span class="' + (k.error_count ? 'err' : '') + '">' + esc(k.error_count) + '</span></td>'
      + '<td class="num">' + (u.requests || 0) + '</td>'
      + '<td class="num">' + fmtTokens(u.prompt_tokens) + '</td>'
      + '<td class="num">' + fmtTokens(u.completion_tokens) + '</td>'
      + '<td class="num">' + fmtTokens(total) + '</td>'
      + '<td class="right"><div class="actions">' + act + '</div></td></tr>';
  }).join('');
  document.getElementById('keysBody').innerHTML = rows || emptyRow(13, '还没有上游密钥，在下面添加');
}

function keyAct(id, action) {
  if (action === 'del') {
    if (!confirm('删除这条上游密钥？正在使用它的请求结束后不会再选到它。')) return;
    api('/v0/management/keys/' + encodeURIComponent(id), 'DELETE')
      .then(function(){ toast('已删除'); refresh(); })
      .catch(function(e){ toast('删除失败：' + e, true); });
    return;
  }
  api('/v0/management/keys/' + encodeURIComponent(id) + '/' + action, 'POST')
    .then(function(){ toast(action === 'disable' ? '已禁用' : '已启用'); refresh(); })
    .catch(function(e){ toast('操作失败：' + e, true); });
}

function addKey(e) {
  e.preventDefault();
  var key = document.getElementById('newKey').value.trim();
  if (!key) { toast('请填写上游密钥', true); return false; }
  api('/v0/management/keys', 'POST', {
    key: key,
    weight: parseInt(document.getElementById('newWeight').value, 10) || 1,
    proxy: document.getElementById('newProxy').value.trim()
  }).then(function(){
    document.getElementById('newKey').value = '';
    document.getElementById('newProxy').value = '';
    toast('上游密钥已添加');
    refresh();
  }).catch(function(err){ toast('添加失败：' + err, true); });
  return false;
}

function parseBulkText(text) {
  var seen = {}, out = [];
  (text || '').split(/\r?\n/).forEach(function(line){
    var k = line.trim();
    if (!k || k.charAt(0) === '#' || seen[k]) return;
    seen[k] = true;
    out.push(k);
  });
  return out;
}
function loadBulkFile() {
  var f = document.getElementById('bulkFile').files[0];
  if (!f) { toast('请先选择文本文件', true); return; }
  var rd = new FileReader();
  rd.onload = function(){ document.getElementById('bulkText').value = rd.result; previewBulk(); toast('已读取 ' + f.name); };
  rd.onerror = function(){ toast('文件读取失败', true); };
  rd.readAsText(f, 'UTF-8');
}
function previewBulk() {
  var keys = parseBulkText(document.getElementById('bulkText').value);
  document.getElementById('bulkPreview').textContent = keys.length
    ? ('可导入 ' + keys.length + ' 条，前 3 条：' + keys.slice(0, 3).join('，') + (keys.length > 3 ? '…' : ''))
    : '没有可导入的密钥';
}
function importBulk() {
  var keys = parseBulkText(document.getElementById('bulkText').value);
  if (!keys.length) { toast('没有可导入的密钥', true); return; }
  api('/v0/management/keys/bulk', 'POST', {
    keys: keys,
    weight: parseInt(document.getElementById('bulkWeight').value, 10) || 1,
    proxy: document.getElementById('bulkProxy').value.trim()
  }).then(function(res){
    toast('导入完成：新增 ' + res.added + '，更新 ' + res.updated + '，跳过 ' + res.skipped);
    document.getElementById('bulkText').value = '';
    document.getElementById('bulkPreview').textContent = '';
    refresh();
  }).catch(function(err){ toast('导入失败：' + err, true); });
}

function renderAPIKeys(res) {
  var rows = (res.keys || []).map(function(k){
    return '<tr><td class="mono">' + esc(k.key) + '</td><td class="right"><div class="actions">'
      + '<button class="btn sm danger" onclick="delAPIKey(\'' + attr(k.id) + '\')">删除</button></div></td></tr>';
  }).join('');
  document.getElementById('apiKeysBody').innerHTML = rows || emptyRow(2, '还没有下游密钥，网关目前不校验调用方');
}
function addAPIKey(e) {
  e.preventDefault();
  var v = document.getElementById('newAPIKey').value.trim();
  if (!v) { toast('请填写下游密钥', true); return false; }
  api('/v0/management/api-keys', 'POST', {key: v}).then(function(){
    document.getElementById('newAPIKey').value = '';
    toast('下游密钥已添加');
    refresh();
  }).catch(function(err){ toast('添加失败：' + err, true); });
  return false;
}
function delAPIKey(id) {
  if (!confirm('删除这条下游密钥？正在使用它的客户端会立刻无法调用。')) return;
  api('/v0/management/api-keys/' + encodeURIComponent(id), 'DELETE')
    .then(function(){ toast('已删除'); refresh(); })
    .catch(function(e){ toast('删除失败：' + e, true); });
}

function renderStats(s) {
  var eps = s.endpoints || {};
  var names = Object.keys(eps).sort();
  var rows = names.map(function(p){
    var c = eps[p];
    return '<tr><td class="mono">' + esc(p) + '</td><td class="num">' + c.requests + '</td>'
      + '<td class="num ' + (c.errors ? 'err' : '') + '">' + c.errors + '</td><td class="num">' + c.inflight + '</td>'
      + '<td class="num">' + (c.last_status || '—') + '</td><td class="muted">' + fmtTime(c.last_call) + '</td>'
      + '<td class="err" title="' + esc(c.last_error || '') + '">' + esc(c.last_error || '—') + '</td></tr>';
  }).join('');
  document.getElementById('statsBody').innerHTML = rows || emptyRow(7, '还没有调用记录');
}

function renderSettings(st) {
  if (!st) return;
  var model = document.getElementById('setModel');
  var force = document.getElementById('setForce');
  var policy = document.getElementById('setPolicy');
  if (document.activeElement !== model) model.value = st.default_model || '';
  if (document.activeElement !== force) force.checked = !!st.force_model;
  if (document.activeElement !== policy && st.proxy_policy) policy.value = st.proxy_policy;
}
function saveSettings(e) {
  e.preventDefault();
  var model = document.getElementById('setModel').value.trim();
  if (!model) { toast('默认模型不能为空', true); return false; }
  api('/v0/management/settings', 'PUT', {
    default_model: model,
    force_model: document.getElementById('setForce').checked,
    proxy_policy: document.getElementById('setPolicy').value
  }).then(function(){ toast('设置已保存'); refresh(); })
    .catch(function(err){ toast('保存失败：' + err, true); });
  return false;
}

function parseProxyText(text) {
  var out = [];
  (text || '').split(/\r?\n/).forEach(function(line){
    var u = line.trim();
    if (!u || u.charAt(0) === '#') return;
    out.push({url: u, weight: 1});
  });
  return out;
}
function saveProxies() {
  var list = parseProxyText(document.getElementById('proxyText').value);
  var msg = list.length ? ('用这 ' + list.length + ' 个节点替换当前代理池？') : '代理列表是空的，改为直连？';
  if (!confirm(msg)) return;
  api('/v0/management/proxies', 'PUT', {proxies: list}).then(function(res){
    toast((res.count || 0) ? ('代理池已更新，共 ' + res.count + ' 个节点') : '已改为直连');
    document.getElementById('proxyText').value = '';
    refresh();
  }).catch(function(err){ toast('代理保存失败：' + err, true); });
}
function renderProxies(res) {
  var list = (res && res.proxies) || [];
  document.getElementById('proxiesPill').textContent = list.length ? (list.length + ' 个') : '直连';
  var rows = list.map(function(px){
    return '<tr><td class="mono">' + esc(px.url) + '</td><td class="num">' + esc(px.weight) + '</td>'
      + '<td>' + (px.healthy ? pill('ok', '正常') : pill('bad', '不可用')) + '</td>'
      + '<td class="num ' + (px.fail_count ? 'err' : '') + '">' + esc(px.fail_count) + '</td>'
      + '<td class="num">' + esc(px.success_count) + '</td><td>' + fmtTime(px.fail_until) + '</td></tr>';
  }).join('');
  document.getElementById('proxiesBody').innerHTML = rows || emptyRow(6, '没有配置代理，当前直连');
  var box = document.getElementById('proxyText');
  if (document.activeElement !== box && !box.value) {
    document.getElementById('proxyHint').textContent = list.length
      ? '上面是掩码地址。要修改请重新粘贴完整地址再保存。'
      : '当前直连';
  }
}

function renderUsage(u) {
  if (!u || !u.keys) {
    document.getElementById('usageKeysBody').innerHTML = emptyRow(5, '用量统计未开启');
    document.getElementById('usageModelsBody').innerHTML = emptyRow(4, '—');
    return;
  }
  var rows = Object.keys(u.keys).map(function(id){
    var k = u.keys[id];
    return '<tr><td class="mono">' + esc(id) + '</td><td class="num">' + k.requests + '</td>'
      + '<td class="num">' + fmtTokens(k.prompt_tokens) + '</td><td class="num">' + fmtTokens(k.completion_tokens) + '</td>'
      + '<td class="num ' + (k.errors ? 'err' : '') + '">' + k.errors + '</td></tr>';
  }).join('');
  document.getElementById('usageKeysBody').innerHTML = rows || emptyRow(5, '还没有用量');
  var models = Object.keys(u.models || {}).map(function(m){
    var v = u.models[m];
    return '<tr><td class="mono">' + esc(m) + '</td><td class="num">' + v.requests + '</td>'
      + '<td class="num">' + fmtTokens(v.prompt_tokens) + '</td><td class="num">' + fmtTokens(v.completion_tokens) + '</td></tr>';
  }).join('');
  document.getElementById('usageModelsBody').innerHTML = models || emptyRow(4, '还没有用量');
}

if (KEY) {
  document.getElementById('login').style.display = 'none';
  start();
} else {
  document.getElementById('mgmtKey').focus();
}
</script>
</body>
</html>`
