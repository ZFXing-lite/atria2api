<p align="center">
  <img src="https://img.shields.io/badge/atria2api-API%20Gateway-1a73e8?style=flat" alt="atria2api">
</p>

<h1 align="center">🚪 Atria Dawn Preview API 网关 (atria2api)</h1>

<p align="center"><strong>Version 1.0.0</strong></p>

<p align="center">
  <em>单二进制 + 一个 YAML 配置 · 无数据库 · 零运行时依赖 · 把任意 OpenAI / Anthropic / Responses 兼容客户端指向它，填入 atr_ 账号，剩下的轮询、限速、重试和 SOCKS5 出口全交给网关。Please star ⭐</em>
</p>

<p align="center">
  <a href="https://github.com/ZFXing-lite/atria2api/stargazers"><img src="https://img.shields.io/github/stars/ZFXing-lite/atria2api?logo=github&label=Stars" alt="GitHub stars"></a>
  <a href="https://github.com/ZFXing-lite/atria2api/blob/master/LICENSE"><img src="https://img.shields.io/badge/license-MIT-65a30d?style=flat" alt="MIT license"></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=fff" alt="Go"></a>
  <br>
  <img src="https://img.shields.io/badge/platform-Linux%20%7C%20Windows%20%7C%20macOS-lightgrey" alt="Platform">
  <img src="https://img.shields.io/badge/version-1.0.0-blue" alt="Version">
</p>

<p align="center">
  <a href="README.md">简体中文</a> | <a href="README_EN.md">English</a>
</p>

> 部署时只需要面板密码。上游账号、下游密钥和代理都留空，启动后在面板里填。

## Contents

- [安装](#安装)
- [首次配置](#首次配置)
- [客户端接入](#客户端接入)
- [配置文件](#配置文件)
- [管理面板](#管理面板)
- [账号池行为](#账号池行为)
- [接口一览](#接口一览)
- [流式](#流式)
- [目录结构](#目录结构)
- [开发](#开发)
- [注意事项](#注意事项)

---

## 安装

### 方式一：Docker Compose（推荐）

```bash
git clone https://github.com/ZFXing-lite/atria2api.git
cd atria2api
ATRIA2API_MGMT_KEY=你的面板密码 docker compose up -d --build
```

浏览器打开 `http://服务器IP:8318/v0/management/panel`，用密码登录，在面板里添加上游账号。

### 方式二：Docker Run

```bash
docker run -d -p 8318:8318 \
  -e ATRIA2API_MGMT_KEY=你的面板密码 \
  -e ATRIA2API_ALLOW_REMOTE=1 \
  -v "$PWD/config.yaml:/app/config.yaml" \
  --name atria2api atria2api
```

### 方式三：本机编译运行

```bash
git clone https://github.com/ZFXing-lite/atria2api.git
cd atria2api
go build -ldflags="-s -w" -o atria2api ./cmd/server
ATRIA2API_MGMT_KEY=你的面板密码 ./atria2api -c config.yaml
```

Windows：

```powershell
go build -ldflags="-s -w" -o atria2api.exe ./cmd/server
$env:ATRIA2API_MGMT_KEY = "你的面板密码"
.\atria2api.exe -c config.yaml
```

### 方式四：go run 直接跑

```bash
ATRIA2API_MGMT_KEY=你的面板密码 go run ./cmd/server
```

---

## 首次配置

1. 启动后浏览器访问 `http://127.0.0.1:8318/v0/management/panel`
2. 输入面板密码登录
3. 在「上游账号」页面添加 `atr_` 开头的账号
4. 在「系统设置」页面填写默认模型（如 `Atria-Dawn-Preview`）
5. 客户端指向网关地址即可开始使用

没填上游账号时 `/healthz` 返回 `ready: false`，填上后变为 `ready: true`。

---

## 客户端接入

三种接口共用同一个地址：

| 客户端 | Base URL | 接口 |
| --- | --- | --- |
| OpenAI SDK / curl | `http://host:8318/v1` | `/v1/chat/completions` |
| Anthropic SDK / Claude Code | `http://host:8318` | `/v1/messages`（`x-api-key`） |
| Codex / Qoder | `http://host:8318/v1` | `/v1/responses` |

curl 示例：

```bash
export OPENAI_API_KEY=你的下游密钥
curl -X POST http://127.0.0.1:8318/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"Atria-Dawn-Preview","messages":[{"role":"user","content":"hi"}]}'
```

`GET /v1/models` 返回固定的 `Atria-Dawn-Preview` 条目。

---

## 配置文件

`config.yaml` 不存在会自动生成，必须可写（面板修改会回写）。环境变量覆盖同名配置项。配置文件变更会热重载，无需重启。

```yaml
port: 8318
api-keys: []                        # 留空=不校验；或填客户端访问本网关用的密钥

upstream:
  base-url: "https://api.atria-asi.ai"
  default-model: "Atria-Dawn-Preview"
  force-model: true                  # 把请求体的 model 改写为 default-model
  keys:
    - key: "atr_xxx"
      weight: 1                      # 加权轮询的权重
      proxy: ""                      # 可选：该 key 专用 socks5:// 代理，或 "none" 直连
    - key: "atr_yyy"

proxy:
  policy: "round-robin"              # round-robin | random | sticky-key
  health-every: 30s
  fail-cooldown: 30s
  socks5:
    - url: "socks5://user:pass@1.2.3.4:1080"

rate-limit:
  respect-header: true               # 解析 x-rpm-* / Retry-After
  min-rpm-reserve: 1                 # 剩余次数低于此值的 key 跳过
  cooldown-429: 60s                  # 上游没给 Retry-After 时用
  cooldown-5xx: 30s
  err-threshold: 3                   # 连续错误达到阈值后冷却
  err-cooldown: 10m
  disable-on-401: true               # 无效/已吊销的 key 直接永久停用
  max-retries: 3                     # 单请求跨 key 重试次数
  retry-on: [429, 500, 502, 503, 504]
  backoff: 1s                        # 退避基数，指数增长 ±25% 抖动

remote-management:
  allow-remote: true                 # 允许远程访问面板
  secret-key: "你的面板密码"

metrics:
  enabled: true
  state-file: state/state.json
  flush-every: 5s
```

---

## 管理面板

浏览器打开：

```
http://127.0.0.1:8318/v0/management/panel
```

零依赖单文件页面，每 5 秒自动刷新，支持：

- **概览**：状态、总请求、错误率、上游账号数、代理数、默认模型
- **上游账号池**：实时查看每个账号的状态/冷却原因/到期时间/RPM 余量/进行中/成功失败计数，一键禁用、启用、删除，表单实时添加新账号（权重 + 代理覆盖）；**批量导入**支持粘贴多行或上传 .txt
- **下游调用密钥**：实时增删客户端访问本网关用的密钥，删除即刻失效
- **接口调用统计**：每个 `/v1/` 路径的请求数、错误数、进行中、最近状态码与最近错误
- **网关设置**：默认模型、代理选择策略、上游 Base URL，保存后立即生效
- **代理池**：每个 SOCKS5 节点的健康/失败/成功计数；按行粘贴 `socks5://` 地址即可整池替换，留空保存则改为直连
- **Token 用量**：按账号与按模型汇总

所有写操作立即生效，并原子回写 `config.yaml`。

### 公网部署

```bash
docker run -p 8318:8318 \
  -e ATRIA2API_MGMT_KEY=你的面板密码 \
  -e ATRIA2API_ALLOW_REMOTE=1 \
  -v "$PWD/config.yaml:/app/config.yaml" \
  atria2api
```

浏览器访问 `http://你的服务器IP:8318/v0/management/panel`，输入密码即可。

**面板密码与下游调用密钥完全独立**：面板密码只用于登录面板和管理 API，下游密钥只用于客户端调用 `/v1/*`，两者互不影响。

---

## 账号池行为

- **选号**：先洗牌，按有效权重排序，截取 top 5 后加权随机，权重相同时用 LRU 打破平局。全部冷却中时兜底选"最快恢复"的那个
- **冷却**用或门汇总到一个到期时间戳：冷却期间再次撞 429 不会叠加惩罚。429 认 `Retry-After`（封顶 10m），5xx 用 `cooldown-5xx`，连续错误触发熔断，有界指数退避（10m、20m、40m……封顶 6h）
- **401** 永久停用该账号（按文档，401 表示账号无效或已吊销），可通过管理 API 或重启重新启用
- **传输层失败**（代理挂了、超时）只把账号临时降级出池 `err-cooldown`，不记在账号上
- **持久化**：池状态与用量计数原子写入 `state.json`（tmp + rename，0600），每隔几秒合并落盘，重启自动恢复

---

## 接口一览

| 路径 | 说明 |
| --- | --- |
| `POST /v1/chat/completions` | OpenAI Chat Completions |
| `POST /v1/messages` | Anthropic Messages（`x-api-key`） |
| `POST /v1/responses` | OpenAI Responses |
| `GET /v1/models` | 固定模型目录 |
| `GET /healthz` | 进程存活探针，始终 200；`ready` 表示是否已有可用上游账号 |
| `GET /status` | 脱敏的池状态与用量快照 |
| `*/v0/management/panel` | Web 管理面板 |
| `*/v0/management/*` | 运维 API（需 `remote-management.secret-key`） |

---

## 流式

SSE 响应逐块透传并按块 flush（带 `X-Accel-Buffering: no`，nginx 不会缓冲），等首 token 期间发送 SSE 注释心跳。响应头只在收到上游第一个块之后才提交，所以"首字节前"的上游失败仍能换账号重试；流式过程中的失败降级为 SSE error 事件，而不是掐断连接。

---

## 目录结构

```
atria2api/
├── cmd/server/main.go          # 入口
├── internal/
│   ├── config/                 # YAML 配置加载与热重载
│   ├── keypool/                # 上游账号池（轮询/冷却/重试）
│   ├── proxypool/              # SOCKS5 代理池
│   ├── relay/                  # 上游请求转发
│   ├── metrics/                # Token 用量记录
│   └── server/                 # HTTP 路由与管理面板
├── config.yaml                 # 配置文件
├── docker-compose.yml
└── Dockerfile
```

---

## 开发

```bash
go test ./...                              # 单元测试 + 端到端测试
go build -ldflags="-s -w" -o atria2api ./cmd/server
```

---

## 注意事项

- Atria 的限速是**账户级**的，名下所有账号共享，所以只有当你的账号来自不同账户时轮询才能真正分散限速
- `Atria-Dawn-Preview` 是纯文本模型（256K 上下文）；网关原样转发请求体，不做多模态转码
- 输出长度上限（`max_completion_tokens` / `max_tokens` / `max_output_tokens`，1–65536）由上游强制执行

---

MIT
