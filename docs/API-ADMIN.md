# 管理类 HTTP / WebSocket 接口

面向运维与自动化脚本，**与对话 API 共用同一监听端口**。若配置了 `api-keys`，除 `/health` 与根路径 `GET /` 外，下列接口均需在请求头携带：

```http
Authorization: Bearer <与 api-keys 中某一项完全一致>
```

`api-keys` 为空时不校验（仅限可信网络；**公网暴露务必配置密钥**）。

---

## 接口一览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/stats` | 账号统计（状态、请求数、额度等） |
| POST | `/refresh` | 强制刷新全部账号 Token（SSE 进度流） |
| POST | `/check-quota` | 查询全部账号额度（SSE 进度流） |
| POST | `/recover-auth` | 401 恢复：按邮箱/路径/全部同步刷新；失败可能将凭据重命名为 `*.json.disabled` |
| POST | `/admin/accounts/ingest` | **导入账号**：HTTP 上传 JSON 填充号池（见下文） |
| GET 或 POST | `/admin/accounts/ingest` | 同上路径，带 **`Upgrade: websocket`** 时使用 **WebSocket** 导入（见下文） |
| POST | `/admin/accounts/export` | **导出账号**：将选中账号导出为 sub2api JSON（见下文） |
| POST | `/admin/accounts/delete` | **硬删除账号**：从本地号池与持久化存储移除单个账号，不撤销上游 OAuth Token |
| GET | `/admin/qmsg/config` | 读取 qmsg 私聊通知配置 |
| PUT | `/admin/qmsg/config` | 保存 qmsg 私聊通知配置，立即生效并写入服务端本地文件 |
| POST | `/admin/qmsg/test` | 发送 qmsg 测试消息，验证推送通道 |

对话相关接口（`/v1/*`）的鉴权规则相同：配置了 `api-keys` 则必须带 Bearer。

---

## 账号导入：`/admin/accounts/ingest`

用于在不重启服务的情况下向号池追加或更新账号，等价于放入 `auth-dir` 下的 `*.json`（磁盘模式）或写入数据库 `codex_accounts`（`db-enabled: true`）。

### 前置条件

| 模式 | 要求 |
|------|------|
| **仅磁盘**（`db-enabled: false`） | 必须配置 **`auth-dir`**；导入文件会写入该目录。 |
| **数据库**（`db-enabled: true`） | 凭据 upsert 到库；`auth-dir` 可为空。若库中既无 `account_id` 也无 `email`，服务会为该条生成稳定的 `upload_<hash>` 作为 `account_id`。 |

### HTTP：`POST /admin/accounts/ingest`

**Content-Type:** `application/json`（或 `text/plain` 传 NDJSON 亦可，只要 body 字节可被解析）

**请求体**支持下列形式（标准账号字段至少包含 `refresh_token` / `rk` / `access_token` / `id_token` 之一；推荐提供 `refresh_token` 或 `rk`）：

1. **单个对象** — 与一个 `*.json` 文件内容相同。  
2. **JSON 数组** — `[{...}, {...}]`，一次提交多个账号。  
3. **NDJSON** — 每行一个 JSON 对象；空行与 `#` 开头行忽略。  
4. **sub2api 多账号 JSON** — 支持从账号对象的 `credentials` 嵌套对象读取凭据，并用 `name` 作为 `email` 的回退身份；也支持 sub2api 导出的顶层包装对象 `{ "exported_at": "...", "proxies": [], "accounts": [...] }`。`platform`、`type`、`group_ids` 等 sub2api 元数据会被忽略，不会写入账号池。

**响应** `200` 且 body 为 JSON，例如：

```json
{
  "added": 2,
  "updated": 1,
  "failed": 0,
  "pool_total": 100,
  "errors": []
}
```

| 字段 | 含义 |
|------|------|
| `added` | 新加入号池的账号数 |
| `updated` | 已存在同一逻辑键（磁盘为文件路径，库为 `db:<account_id>` 或 `db:<email>`）时覆盖 Token 的数量 |
| `failed` | 解析或校验失败的对象数 |
| `pool_total` | 导入完成后号池内账号总数 |
| `errors` | 失败条目说明（条数有上限，避免响应过大） |

**400** 时 body 含 `error.message`，多为整段 body 无法解析（例如空 body、JSON 语法错误）。

持久化通过现有 **异步写回队列**完成；与后台刷新相同，极端高并发时若队列已满可能短暂延迟落盘/入库。

**curl 示例（单账号）：**

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/accounts/ingest" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d @./auths/example.json
```

**curl 示例（数组）：**

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/accounts/ingest" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '[{"refresh_token":"...","email":"a@b.com"}]'
```

**curl 示例（sub2api 多账号 JSON）：**

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/accounts/ingest" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '[{"name":"user@example.com","platform":"openai","type":"oauth","group_ids":[1,2],"credentials":{"refresh_token":"rt_xxx"}}]'
```

**curl 示例（sub2api 导出文件）：**

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/accounts/ingest" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '{"exported_at":"2026-05-10T07:11:15.576Z","proxies":[],"accounts":[{"name":"user@example.com","platform":"openai","type":"oauth","credentials":{"refresh_token":"rt_xxx"}}]}'
```

### WebSocket：同一 URL

与 `POST /v1/responses` 类似，可使用 **GET 或 POST** 发起握手，并携带：

- `Upgrade: websocket`
- `Connection: Upgrade`  
等标准 WebSocket 头。

连接建立后，**每个文本帧**的 payload 与 **HTTP POST 的 body** 语义相同（单对象、数组或 NDJSON 整段放在一个帧里）。

- 服务端对每一帧返回 **一条** JSON：成功时为与 HTTP 相同的 `IngestResult`；失败时为 `{"ok":false,"error":"..."}`。  
- 发送文本 **`ping`** 可收到 `{"type":"pong"}`（便于探活）。

适合脚本分批推送、或避免单次 HTTP body 过大时拆成多帧（每帧仍应是完整可解析的 JSON 片段，而不是把一个 JSON 对象切成多帧）。

---

## 账号导出：`/admin/accounts/export`

将号池中指定账号导出为 **sub2api 兼容 JSON**。响应体包含完整 Token 凭据，**务必与管理 API 同等鉴权**，勿在不可信环境调用。

### HTTP：`POST /admin/accounts/export`

**Content-Type:** `application/json`

**请求体：**

```json
{
  "emails": ["user@example.com", "other@example.com"],
  "format": "sub2api-export"
}
```

| 字段 | 含义 |
|------|------|
| `emails` | 必填。要导出的账号邮箱列表；重复邮箱会自动去重 |
| `format` | 可选。`sub2api-export`（默认，完整导出文件）或 `sub2api-array`（仅账号数组） |

**响应** `200` 且 body 为 JSON，例如：

```json
{
  "format": "sub2api-export",
  "exported": 2,
  "not_found": ["missing@example.com"],
  "failed": [{"email": "bad@example.com", "error": "缺少可导出的凭据"}],
  "data": {
    "exported_at": "2026-05-26T08:00:00.123456789Z",
    "proxies": [],
    "accounts": [
      {
        "name": "user@example.com",
        "platform": "openai",
        "type": "oauth",
        "credentials": {
          "refresh_token": "rt_xxx",
          "access_token": "at_xxx",
          "id_token": "id_xxx",
          "account_id": "acct_xxx",
          "chatgpt_account_id": "acct_xxx",
          "email": "user@example.com",
          "expired": "2026-01-01T00:00:00Z",
          "expires_at": 1767225600
        }
      }
    ]
  }
}
```

| 字段 | 含义 |
|------|------|
| `exported` | 成功导出的账号数 |
| `not_found` | 号池中未找到的邮箱 |
| `failed` | 找到账号但导出失败（如缺少凭据） |
| `data` | 导出内容；`sub2api-export` 时为完整包装对象，`sub2api-array` 时为账号数组 |

**404** 表示没有任何账号可导出（全部未找到或失败）。

**curl 示例（sub2api 导出文件）：**

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/accounts/export" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '{"emails":["user@example.com"],"format":"sub2api-export"}'
```

**curl 示例（sub2api 账号数组）：**

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/accounts/export" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '{"emails":["user@example.com"],"format":"sub2api-array"}'
```

导出的 JSON 可直接用于 `/admin/accounts/ingest` 导入，实现 sub2api 格式 round-trip。

---

## 账号删除：`POST /admin/accounts/delete`

用于从当前本地号池中硬删除单个账号。删除会移出内存账号池，并删除本地持久化凭据：数据库模式删除 `codex_accounts` 记录，文件模式删除对应 `*.json` 凭据文件。

注意：该接口不会调用上游 OAuth revoke，不等同于远端登出；如只是临时下线账号，应优先在管理界面使用停用开关。

**请求体：**

```json
{
  "email": "user@example.com"
}
```

也可使用 `file_path` 定位账号：

```json
{
  "file_path": "auths/user.json"
}
```

**成功响应：**

```json
{
  "email": "user@example.com",
  "file_path": "db:acct_xxx",
  "deleted": true,
  "pool_total": 99
}
```

- `400`：请求体为空、JSON 无效，或未提供 `email`/`file_path`。
- `404`：当前号池中未找到匹配账号。

**curl 示例：**

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/accounts/delete" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

---

## qmsg 私聊通知：`/admin/qmsg/*`

用于配置 qmsg 私聊推送通道。配置保存到服务端本地文件 `auth-dir/qmsg-config.json`，保存后立即生效，无需重启。启用后，账号发生自动删除或自动停用事件时会异步推送通知；推送失败只记录日志，不会阻断账号处置流程。

qmsg 接口使用 JSON 私聊推送：`POST https://qmsg.zendee.cn/jsend/{KEY}`，请求体为 `{"msg":"...","qq":"可选","bot":"可选"}`。其中 `bot` 是可选的 Qmsg 酱机器人 QQ；不填时由 qmsg 自动随机选择在线机器人。qmsg 的成功判断以响应体中的 `success` 为准，详见 [Qmsg酱 API 文档](https://qmsg.zendee.cn/docs/api/)。

### `GET /admin/qmsg/config`

读取当前配置。出于安全考虑，响应不会返回明文 key，只返回掩码。

```json
{
  "object": "qmsg_config",
  "config": {
    "enabled": true,
    "key_masked": "abcd********mnop",
    "qq": "12345",
    "bot": "67890",
    "timeout_sec": 10,
    "message_template": "账号自动{{action}}通知\n邮箱：{{email}}...",
    "endpoint_template": "https://qmsg.zendee.cn/jsend/{key}",
    "configured": true
  }
}
```

### `PUT /admin/qmsg/config`

保存配置。`enabled=true` 时必须已有 key 或在本次请求中提供 `key`；如果请求中的 `key` 为空且服务端已有 key，则保留原 key。`bot` 为可选项，用于指定发送消息的 Qmsg 酱机器人；如果测试返回“您选择的Qmsg不在线，请选择其他Qmsg酱”，可以清空 `bot` 让 qmsg 随机选择在线机器人，或换成其他在线机器人 QQ。

```bash
curl -sS -X PUT "http://127.0.0.1:8080/admin/qmsg/config" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "key": "qmsg-key",
    "qq": "12345",
    "bot": "67890",
    "timeout_sec": 10,
    "message_template": "账号自动{{action}}通知\n邮箱：{{email}}\n原因：{{reason_code}}\n详情：{{detail}}\n时间：{{timestamp}}"
  }'
```

模板变量包括：

| 变量 | 含义 |
|------|------|
| `{{action}}` | 中文动作：删除 / 停用 |
| `{{action_code}}` | 原始动作编码：`remove` / `disable` |
| `{{email}}` | 账号邮箱 |
| `{{reason_code}}` | 删除/停用原因编码 |
| `{{detail}}` | 处置详情 |
| `{{storage_mode}}` | 持久化模式：`file` / `db` |
| `{{timestamp}}` | 本地时间 |
| `{{timestamp_utc}}` | UTC RFC3339 时间 |

### `POST /admin/qmsg/test`

发送测试消息。请求体可为空；为空时服务端生成默认测试消息。

```bash
curl -sS -X POST "http://127.0.0.1:8080/admin/qmsg/test" \
  -H "Authorization: Bearer sk-your-custom-key" \
  -H "Content-Type: application/json" \
  -d '{"message":"这是一条 qmsg 测试消息"}'
```

成功响应示例：

```json
{
  "object": "qmsg_test_result",
  "success": true,
  "result": {
    "success": true,
    "reason": "操作成功",
    "code": 0,
    "info": { "msgId": 5866868 },
    "msg_id": 5866868
  }
}
```

---

## 其他管理接口摘要

### `POST /recover-auth`

请求体示例：`{"email":"a@b.com"}`、`{"file_path":"相对或绝对路径.json"}`、`{"all":true}`。具体行为见 `config.example.yaml` 顶部管理接口注释。

### `POST /refresh`、`POST /check-quota`

响应为 **SSE**（`text/event-stream`），用于进度展示；脚本可按行解析 `data: {...}`。

### `GET /stats`

分页与筛选参数若存在，以当前实现为准（参见 `internal/handler/stats.go`）。

---

## 安全建议

- 导入接口会写入**完整 OAuth 凭据**，权限与修改号池等价；务必 **配置 `api-keys`**、限制来源 IP，或通过反向代理仅对内网开放 `/admin/`。  
- qmsg 配置文件包含明文 KEY，默认保存为 `auth-dir/qmsg-config.json` 且文件权限为 `0600`；请保护 `auth-dir` 目录访问权限。
- 勿在日志、工单中粘贴真实 `refresh_token` 或 qmsg KEY。
