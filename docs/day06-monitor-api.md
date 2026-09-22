# Day06：监控生命周期 API

## 1. 目标和边界

Day06 在认证之上实现 `/api/v1/monitors` 的完整生命周期：列表、创建、详情、部分更新、暂停、恢复、软删除。所有对象查询带 `user_id` 归属与 `deleted_at IS NULL` 过滤；缺失、已删除、非本人 ID 统一返回 `MONITOR_NOT_FOUND`，不泄露资源是否存在。

Issue：`[PW-005] 完成监控生命周期 API`。建议分支：`feat/PW-005-monitor-api`。

本阶段只做存储边界与字段规则，不实现调度、检查历史、故障与连接期 SSRF 防护。

## 2. 路由

认证后的接口都在 `/api/v1/monitors` 下：

- `GET /`：当前用户活跃记录的分页列表（`page`/`page_size`）。
- `POST /`：创建 `pending` 状态监控。
- `GET /:id` 与 `PATCH /:id`：读取或更新本人监控。
- `POST /:id/pause` 与 `POST /:id/resume`：变更调度状态。
- `DELETE /:id`：软删除本人监控。

## 3. 字段规则

- 名称 trim 后 1–80 字符。
- URL 为 HTTP/HTTPS、≤2048 字符、不含凭据或 fragment，且不能使用显式 local/private/loopback/link-local 地址。
- 间隔 1/5/10 分钟；期望状态码 100–599。
- 每人最多 20 个未删除监控：创建时先锁用户行再计数 + 插入，并发请求无法超限。
- `config_version` 语义：URL/间隔/期望状态码变更 +1，活跃监控变 `pending` 且立即到期，暂停监控保持暂停；仅改名不失效排队任务。
- 暂停/恢复仅在真实状态转换时 +1；删除 +1 且不可重复，二次调用返回 404。

本校验只是 Day06 的存储边界。checker 仍须在每次连接与重定向前解析并固定公网地址，防止 DNS 重绑定与重定向型 SSRF。

## 4. 验收证据

必须通过：

```bash
npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml
/tmp/pulsewatch-tools/oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
(cd server && /tmp/pulsewatch-tools/sqlc generate && /tmp/pulsewatch-tools/sqlc compile)
(cd web && npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts)

cd server
go test -race ./internal/monitor/...
go test -race ./...
```

设 `DATABASE_URL` 跑 PostgreSQL 所有权与并发上限测试；测试使用隔离临时 schema，测后清理。

## 5. 明确不包含

调度、检查历史（`check_runs` 写入）、故障/通知、连接期 SSRF（DNS 重绑定/重定向）。

## 6. 后续衔接

- Day07 用这套 API 做监控管理 UI；
- Day08 用 `check_runs`、`scheduled_at`、`enqueued_at` 与 `config_version` 接入 Asynq 调度；
- Day09 才执行 DNS、私网和重定向安全检查。
