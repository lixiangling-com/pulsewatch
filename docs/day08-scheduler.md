# Day08：调度与队列

## 1. 目标和边界

Day08 在现有 Redis 平台客户端之上接入 Asynq，实现监控的定时调度与 Worker 消费。PostgreSQL 以 `run_id` + 状态为权威，Redis 只作为可恢复的异步任务传输层，不保存业务状态。

Issue：`[PW-007] 完成调度与队列`。建议分支：`feat/PW-007-scheduler`。

本阶段只做「到点入队 + 消费占位」，真实 HTTP 检查与 up/down 状态机留 Day09，故障与告警留 Day10。

## 2. 调度与队列

- 接入 Asynq：Worker 内 Scheduler 创建 `check_runs`，Dispatcher 投递 Redis，Consumer 消费，复用 Day02 的 Redis 客户端。
- 按监控间隔（1/5/10 分钟）扫描到期监控并入队。
- 暂停（`paused`）与已删除（`deleted_at IS NOT NULL`）监控不再入队。
- Worker 消费时校验 `config_version`，过期任务丢弃或跳过，避免改配置后旧任务生效。

## 3. check_runs 生命周期

- 每次计划检查写入 `check_runs`，字段包含 `scheduled_at`、`enqueued_at`、`run_id`、`config_version` 与状态。
- `run_id` 由应用生成 UUID；PostgreSQL 以 `run_id` + 状态为权威。
- 状态占位：`pending` → 到期调度；实际 `up`/`confirming_down`/`down` 判定留 Day09。

## 4. 验收证据

必须通过：

1. `cd server && sqlc generate && sqlc compile`；
2. `cd server && go test -race ./...` 与 `go vet ./...`；
3. 设置 `DATABASE_URL` 后运行 `go test -race ./internal/check/...`：并发 Scheduler 只创建一个 run，暂停/删除/未到期监控跳过，`scheduled_at`/`config_version`/`next_check_at` 正确，Consumer 对重复、暂停、删除和旧版本任务实现幂等取消；
4. Redis 恢复演练：停止 Redis，确认 Dispatcher 投递失败时 `enqueued_at` 仍为空；恢复 Redis 后等待补偿周期，确认同一 `run_id` 投递并只标记一次；
5. Worker 生命周期测试确认停机顺序为停止 Scheduler/Dispatcher、等待 heartbeat、停止 Consumer、关闭 HTTP 探针。

## 5. 明确不包含

实际 HTTP 探测、DNS/私网/重定向 SSRF 校验（Day09）、连续失败开故障与告警邮件（Day10）、SSE 提示（Day11）、重启后的精确补偿调度策略。

## 6. 后续衔接

- Day09 在消费任务里做真实检查 + 连接期 SSRF 防护；
- Day10 用 `check_runs` 的连续失败状态触发故障。
