# Day11：SSE 变化提示

## 1. 目标和边界

Day11 增加 Redis Pub/Sub、认证 SSE 与 Nginx 代理，向 Web 推送监控与通知的变化提示。SSE 只描述「变化发生了」，最终数据仍由 REST 查询 PostgreSQL。API server 有意不设置长连接 `WriteTimeout`。

Issue：`[PW-010] 完成 SSE 变化提示`。建议分支：`feat/PW-010-sse`。

## 2. 通道

- 监控状态变化、通知创建时发布到 Redis Pub/Sub。
- 认证 SSE 端点只订阅当前用户的变化提示。
- Nginx 代理 SSE 长连接。

## 3. 语义

- SSE 只发「某资源变化」提示，客户端据此 REST 拉取最新数据。
- 断线重连与心跳保活。

## 4. 验收证据

必须通过：

1. `cd server && go test -race ./...` 与 `cd web && npm run build`；
2. 手动验证：改监控状态后页面收到提示并刷新；
3. Nginx 配置下 SSE 长连接不中断。

## 5. 明确不包含

WebSocket、Redis Streams、通用 Outbox、微服务拆分。

## 6. 后续衔接

- Day14 再把迁移、生成和服务放入完整容器交付流程。
