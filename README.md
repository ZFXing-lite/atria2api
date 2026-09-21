# atria2api

[English](README_EN.md) | 简体中文

[Atria Dawn Preview API](https://api.atria-asi.ai/docs) 的专属网关。把任意
OpenAI / Anthropic / Responses 兼容客户端指向它，填入一个或多个 `atr_` key，
剩下的 key 轮询、限速处理、重试和 SOCKS5 出口都交给网关。

沿用 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)、
[autoclaw2api](https://github.com/ZFXing-lite/autoclaw2api)、
[workbuddy2api](https://github.com/Sliverkiss/workbuddy2api) 的成熟模式：
单二进制 + 一个 YAML 配置，无数据库，零运行时依赖。

## 为什么需要

Atria API 按账户限速（响应头 `x-rpm-limit` / `x-rpm-remaining`，429 时带
`Retry-After`），三种接口共用同一个 base URL。本网关：

- 在多个 key 之间轮询，一个账户的限额用尽不会让你的客户端停摆；
- 读取文档里的限速头，让 key 在本分钟窗口内主动休息，而不是撞 429；
- 可重试的失败（429 / 5xx / 传输错误）自动换 key 重试，但 400 类客户端错误
  原样回传、流式开始后绝不换号；
- 可让全部上游流量走 SOCKS5 代理池。

## 快速开始

```bash
git clone https://github.com/ZFXing-lite/atria2api
cd atria2api
cp config.example.yaml config.yaml
# 编辑 config.yaml，在 upstream.keys 填入你的 atr_ key
go run ./cmd/server -c config.yaml
```

或用 Docker：

```bash
docker build -t atria2api .
docker run -p 8318:8318 -v "$PWD/config.yaml:/app/config.yaml:ro" atria2api
```

然后把客户端指向网关（三种接口同一个地址）：

```bash
export OPENAI_API_KEY=gw-change-me-1
curl -X POST http://127.0.0.1:8318/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"Atria-Dawn-Preview","messages":[{"role":"user","content":"hi"}]}'
```

| 客户端 | Base URL | 接口 |
| --- | --- | --- |
| OpenAI SDK / curl | `http://host:8318/v1` | `/v1/chat/completions` |
| Anthropic SDK / Claude Code | `http://host:8318` | `/v1/messages`（`x-api-key`） |
| Codex / Qoder | `http://host:8318/v1` | `/v1/responses` |

`GET /v1/models` 返回固定的 `Atria-Dawn-Preview` 条目。

## 配置

全部配置在 `config.yaml` 里（环境变量覆盖：`ATRIA2API_KEYS`、`ATRIA2API_API_KEYS`、
`ATRIA2API_PROXIES`、`ATRIA2API_BASE_URL`、`ATRIA2API_PORT`……）。配置文件变更会热重载。

```yaml
port: 8318
api-keys: ["gw-change-me-1"]        # 客户端访问本网关用的 key

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
```

## key 池行为

- **选号**：先洗牌，按有效权重（配置权重按观测到的剩余 RPM 衰减）排序，截取 top 5
  后加权随机，权重相同时用 LRU 打破平局。全部冷却中时兜底选"最快恢复"的那个。
- **冷却**用或门汇总到一个到期时间戳：冷却期间再次撞 429 不会叠加惩罚。429 认
  `Retry-After`（封顶 10m），5xx 用 `cooldown-5xx`，连续错误触发熔断，有界指数退避
  （10m、20m、40m……封顶 6h）。
- **401** 永久停用该 key（按文档，401 表示 key 无效或已吊销），可通过管理 API 或
  重启重新启用。
- **传输层失败**（代理挂了、超时）只把 key 临时降级出池 `err-cooldown`，不记在
  key 账上。
- **持久化**：池状态与用量计数原子写入 `state.json`（tmp + rename，0600），每隔
  几秒合并落盘，重启自动恢复。

## 接口

| 路径 | 说明 |
| --- | --- |
| `POST /v1/chat/completions` | OpenAI Chat Completions |
| `POST /v1/messages` | Anthropic Messages（`x-api-key`） |
| `POST /v1/responses` | OpenAI Responses |
| `GET /v1/models` | 固定模型目录 |
| `GET /healthz` | 存活探针（无可用 key 时 503） |
| `GET /status` | 脱敏的池状态与用量快照 |
| `*/v0/management/panel` | Web 管理面板 |
| `*/v0/management/*` | 运维 API（未设 `remote-management.secret-key` 时返回 404） |

## 管理面板

设置 `remote-management.secret-key` 后，浏览器打开：

```
http://127.0.0.1:8318/v0/management/panel
```

面板是零依赖单文件页面，每 2.5 秒自动刷新，支持：

- **概览**：监听端口、TLS、pprof 端口、上游地址、默认模型、可用 key 数、代理数、鉴权状态
- **上游 key 池**：实时查看每个 key 的状态/冷却原因/到期时间/RPM 余量/进行中/成功失败计数，
  一键禁用、启用、删除，表单实时添加新 key（权重 + 代理覆盖）；
  **批量导入**支持粘贴多行或上传 .txt（每行一条 key，空行和 `#` 注释自动忽略、自动去重）
- **下游调用 key**：实时增删客户端访问本网关用的 key，**删除即刻失效**
- **接口调用统计**：每个路径的请求数、错误数、进行中、最近状态码与最近错误
- **代理池**：每个 SOCKS5 节点的健康/失败/成功计数（节点列表仍在 config.yaml 维护）
- **Token 用量**：按 key 与按模型汇总

所有写操作立即生效，并原子回写 config.yaml（注意：回写会丢失文件里的注释）。
管理 API 默认只允许本机访问，需要远程访问设 `remote-management.allow-remote: true`。
管理密钥错误 4 次后该来源 IP 被锁定 15 分钟；密钥用常量时间比较，且 `GET /keys/x/disable`
这类变更端点只接受 `POST`（GET 链接无法误触发）。
面板返回的配置信息只含掩码（`atr_****ey`、代理 `socks5://***@host:port`、`secret-key-set` 布尔值），不含任何明文密钥。

管理 API（默认仅允许回环访问）：

```bash
curl -H "Authorization: Bearer $MGMT_KEY" http://127.0.0.1:8318/v0/management/keys
curl -X POST -H "Authorization: Bearer $MGMT_KEY" \
  http://127.0.0.1:8318/v0/management/keys/<id>/disable
curl -X POST -H "Authorization: Bearer $MGMT_KEY" \
  http://127.0.0.1:8318/v0/management/keys/<id>/enable
```

key 永远不会出现在日志或管理 API 里，只有一个稳定的掩码哈希 id。

## 流式

SSE 响应逐块透传并按块 flush（带 `X-Accel-Buffering: no`，nginx 不会缓冲），
等首 token 期间发送 SSE 注释心跳。响应头只在收到上游第一个块之后才提交，
所以"首字节前"的上游失败仍能换 key 重试；流式过程中的失败降级为 SSE error
事件，而不是掐断连接。

## 开发

```bash
go test ./...          # 单元测试 + 对 mock 上游的端到端测试
go build -ldflags="-s -w" -o atria2api ./cmd/server
```

## 注意事项与限制

- Atria 的限速是**账户级**的、名下所有 key 共享，所以只有当你的 key 来自不同账户
  时轮询才能真正分散限速；它抬不动单个账户的 RPM。
- `Atria-Dawn-Preview` 是纯文本模型（256K 上下文）；网关原样转发请求体，不做
  多模态转码。
- 输出长度上限（`max_completion_tokens` / `max_tokens` / `max_output_tokens`，
  1–65536）由上游强制执行。

## 许可

MIT。
