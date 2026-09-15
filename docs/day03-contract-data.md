# Day03：OpenAPI 契约与可迁移数据库

## 1. 目标和边界

Day03 建立两份基础合同：OpenAPI 是 Web 与 API 的 HTTP 合同，迁移 SQL 是程序与 PostgreSQL 的数据合同。今天只建立结构、查询和生成链路，不实现注册、登录、监控 CRUD、调度、检测、邮件或 SSE。

Issue：`[PW-003] 建立 OpenAPI 与数据库契约`。建议分支：`feat/PW-003-contract-data`。

## 2. OpenAPI

唯一源文件是 `api/openapi.yaml`，业务接口使用 `/api/v1` 前缀。Day02 已存在的 `/health/live` 和 `/health/ready` 保留在根路径。Worker 的 `127.0.0.1:8081` 探针属于内部运维接口，不放入公开契约。

契约预先覆盖认证、监控、检查历史、故障、通知和事件接口，但 Day03 不注册这些业务路由。统一错误对象包含 `code`、`message`、`field_errors` 和 `request_id`；字段错误是字符串数组。分页统一使用 `page`、`page_size` 和 `meta.total`。

Bearer JWT 和 `refresh_token` HttpOnly Cookie 的安全方案提前写入契约，实际认证实现留到 Day04。SSE 只描述变化提示，最终数据仍由 REST 查询 PostgreSQL。

## 3. 六张表

- `users`：账号邮箱和密码摘要，邮箱使用 `lower(email)` 唯一索引。
- `refresh_tokens`：Refresh Token 摘要、过期和撤销状态。
- `monitors`：用户拥有的网址、间隔、期望状态码、状态、配置版本和软删除时间。
- `check_runs`：每次计划检查的生命周期和脱敏结果。
- `incidents`：连续失败形成的故障时间线，一个监控最多一个未恢复故障。
- `notifications`：故障/恢复的站内与邮件投递记录，使用去重键。

UUID 统一由应用生成。所有外键使用 `ON DELETE CASCADE`；用户删除接口不在 Day03，监控业务删除使用 `deleted_at` 软删除。监控状态保留 `pending`、`up`、`confirming_down`、`down`、`confirming_up`、`paused` 六个技术状态，后续 Web 可以把确认中状态合并展示。

URL 只在结构层限制为非空、2048 字以内的 HTTP/HTTPS 地址；Day09 才执行 DNS、私网和重定向安全检查。

## 4. sqlc 查询边界

`server/db/queries/` 提供用户、令牌、监控、检查、故障和通知的基础读写查询。资源查询带 `user_id` 所有权条件，不先按 ID 查询再在 Go 中比较用户。调度、检测、状态机和告警事务查询留给后续日期。

## 5. 验收证据

必须通过：

1. OpenAPI lint；
2. oapi-codegen、openapi-typescript 和 sqlc 生成；
3. `sqlc compile`；
4. 生成两次后 `git diff --exit-code` 无差异；
5. PostgreSQL 隔离 schema 的 `up/down/up`、约束和事务回滚测试；
6. `go test ./...` 和 `npm run build`。

真实数据库测试使用随机临时 schema，不触碰开发表。测试覆盖大小写邮箱唯一、非法状态和间隔、无效外键、开放故障唯一、通知去重、级联删除和事务回滚。

## 6. 后续衔接

- Day04 使用用户和 Refresh Token 表实现认证；
- Day06 使用监控表和所有权查询实现 CRUD；
- Day08 使用 `check_runs`、`scheduled_at`、`enqueued_at` 和 `config_version` 实现调度与队列；
- Day10 使用故障和通知约束实现告警事务；
- Day11 使用通知/监控变化提示实现 SSE；
- Day14 再把迁移、生成和服务放入完整容器交付流程。
