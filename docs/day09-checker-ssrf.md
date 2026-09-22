# Day09：检查执行与 SSRF 防护

## 1. 目标和边界

Day09 在 Day08 的队列消费之上实现真实 HTTP/HTTPS 探测与连接期 SSRF 防护。每次检查在连接前解析并固定公网地址，拒绝私网/loopback/link-local，重定向同样重新校验，防止 DNS 重绑定与重定向型 SSRF。同时把 `monitors.status` 从 `pending` 推进到 `up`/`down`，并把脱敏结果写入 `check_runs`。

Issue：`[PW-008] 完成检查执行与 SSRF 防护`。建议分支：`feat/PW-008-checker-ssrf`。

## 2. 探测

- 对到期任务发起 HTTP/HTTPS 请求，设置连接与总超时。
- 按 `expected_status` 判定 up/down。
- 结果（状态码、耗时、脱敏错误）写入 `check_runs`，不保存响应体。
- 检查历史保留 30 天，过期清理。

## 3. SSRF 防护

- 连接前解析 DNS 并固定（pin）公网地址，拒绝 private/loopback/link-local/multicast 地址。
- 每次重定向重新执行解析与校验，拒绝跳转到私网地址。
- 禁用 URL 凭据，限制重定向次数。

## 4. 状态机

- `pending` → 首次检查后进入 `up` 或 `confirming_down`。
- 状态翻转经确认态缓冲：连续失败才转 `down`，恢复时经 `confirming_up` 回到 `up`（与 Day10 的故障判定联动，状态本身在 Day09 推进）。

## 5. 验收证据

必须通过：

1. `cd server && go test -race ./...`，含 SSRF 单元测试（私网地址、重定向跳私网、DNS 重绑定用例）；
2. 本地对公网地址跑通一次检查，`check_runs` 落库且结果脱敏；
3. 30 天历史清理策略生效。

## 6. 明确不包含

故障与告警邮件（Day10）、SSE 提示（Day11）、自定义请求头/请求体、Webhook。

## 7. 后续衔接

- Day10 用连续失败状态打开故障、发送告警邮件。
