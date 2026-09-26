# Day09：检查执行与 SSRF 防护

## 1. 目标和边界

Day09 在 Day08 的队列消费之上实现真实 HTTP/HTTPS 探测与连接期 SSRF 防护。每次检查在连接前解析并固定公网地址，拒绝私网/loopback/link-local，重定向同样重新校验，防止 DNS 重绑定与重定向型 SSRF。同时把 `monitors.status` 从 `pending` 推进到 `up`/`down`，并把脱敏结果写入 `check_runs`。

Issue：`[PW-008] 完成检查执行与 SSRF 防护`。分支：`feat/PW-008-checker-ssrf`。

## 2. 探测

- 对到期任务发起 GET HTTP/HTTPS 请求，总超时 10 秒、最多跟随 5 次跳转、最多读取 4 KiB。
- Worker 并发维持 10。
- 按 `expected_status` 判定 up/down。
- 结果（状态码、耗时、脱敏错误）写入 `check_runs`，不保存响应体。
- 网络请求在数据库事务之外执行；领取任务与最终保存结果使用短事务。
- 30 天历史清理留到 Day12。

## 3. SSRF 防护

- 连接前解析 DNS 并固定（pin）公网地址，拒绝 private/loopback/link-local/multicast 地址。
- 每次重定向重新执行解析与校验，拒绝跳转到私网地址。
- 禁用 URL 凭据，限制重定向次数。
- 仅 `APP_ENV=development` 精确放行 Compose 主机名 `mocktarget`；其他私网目标都拦截。
- 安全拦截记为 `failed + blocked_address`，不推进 up/down 状态、不暂停监控。

## 4. 状态机

- `pending` → 首次成功进入 `up`，首次失败进入 `confirming_down`。
- 状态翻转经确认态缓冲：连续失败才转 `down`，恢复时经 `confirming_up` 回到 `up`（与 Day10 的故障判定联动，状态本身在 Day09 推进）。

## 5. 验收证据

必须通过：

1. `cd server && go test -race ./internal/check/...` 和 `go test -race ./...`；
2. 单元/集成测试覆盖状态机、私网拦截、DNS 重绑定、重定向限制、脱敏和数据库状态；
3. `deploy/compose.dev.yml` 中的 `mocktarget` 提供 `/target?status=500`、`/target?delay_ms=...`、`/target?redirect=...` 场景；
4. 本地 mocktarget 与一个公网 HTTPS 目标各跑通一次，确认 `check_runs` 落库且不含响应正文。

## 6. 明确不包含

故障与告警邮件（Day10）、SSE 提示（Day11）、自定义请求头/请求体、Webhook。

## 7. 后续衔接

- Day10 用连续失败状态打开故障、发送告警邮件。
