# Day04 + Day05：认证闭环

## 目标与范围

本阶段交付一个可实际使用的基础会话流程：注册、登录、刷新页面恢复身份、读取当前账户和退出。对应 Issue 为 `[PW-004] 完成前后端认证闭环`，开发分支为 `feat/PW-004-auth`。

本阶段不实现监控 CRUD、邮箱验证、找回密码、OAuth、设备管理、用户删除、SSRF 防护或 Worker 任务。监控接口虽然已在 OpenAPI 中预留，但在 Day06 前保持未注册状态。

## 会话设计

浏览器收到两种用途不同的凭据：

1. API 响应中的 Access Token 是 HS256 JWT，有效期 15 分钟，只放在 Web 模块内存中。它用于 `Authorization: Bearer <token>` 访问受保护接口。
2. `refresh_token` 是随机生成的 Cookie，有效期 7 天。它设置为 `HttpOnly`，所以 JavaScript 不能读取；浏览器只会在认证路径请求中自动携带它。

页面刷新后，内存中的 Access Token 会自然消失。Web 先请求 `POST /api/v1/auth/refresh`，浏览器自动附带 HttpOnly Cookie；成功后取得一个新的 Access Token，并调用 `GET /api/v1/auth/me` 确认用户。恢复完成前只显示固定的加载界面，因此不会短暂显示受保护的 Account 内容。

Access Token 不进入 Cookie、`localStorage`、`sessionStorage` 或 URL。退出时浏览器清空内存，服务端撤销 Refresh Token 并清 Cookie。已经签发的 Access Token 最多还有 15 分钟有效期；这里不维护黑名单，以短令牌有效期换取较简单、可扩展的服务端状态。

## 后端边界

依赖方向为：

```text
HTTP Handler -> Auth Service -> Repository/sqlc -> PostgreSQL
```

- Handler 只解析 HTTP、设置或清理 Cookie、输出响应。
- Service 处理邮箱规范化、密码规则、会话生成和业务错误。
- Repository 使用 PostgreSQL 事务；不会读取 Cookie、JWT 或 HTTP 请求。

注册会把邮箱去空格并转小写，密码按 UTF-8 字节限制为 8 到 72 字节。密码只保存 bcrypt 摘要。创建用户与创建 Refresh Token 摘要处于同一个事务中，任何一步失败都不会留下半个账户。

Refresh Token 用 `crypto/rand` 生成 32 字节随机值，浏览器获得 Base64URL 原值，数据库只保存该原值的 SHA-256 摘要。刷新时事务以 `FOR UPDATE` 锁定旧记录，检查未撤销和未过期后撤销旧记录并创建新记录。同一旧 Token 并发刷新时最多一个请求成功。

Cookie 名称为 `refresh_token`，属性固定为 `HttpOnly`、`SameSite=Lax`、`Path=/api/v1/auth`、7 天 `Max-Age/Expires`。开发环境的 `Secure=false` 便于本机 HTTP 调试；其他环境强制 `Secure=true`。

## 接口与错误

实际注册的接口只有：

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

认证错误使用统一结构，始终有 `request_id`：

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "请检查输入",
    "field_errors": {"email": ["请输入有效邮箱"]},
    "request_id": "..."
  }
}
```

稳定错误码为 `VALIDATION_ERROR`、`EMAIL_ALREADY_REGISTERED`、`INVALID_CREDENTIALS` 和 `UNAUTHORIZED`。错误和日志都不得包含密码、JWT、Cookie、Refresh Token、Authorization 或数据库连接串。

## 配置

API 需要在根目录 `.env` 中配置：

```dotenv
JWT_ACCESS_SECRET=at-least-32-byte-development-secret-value
JWT_ISSUER=pulsewatch
```

`JWT_ACCESS_SECRET` 缺失或少于 32 字节时，API 拒绝启动且只报告变量名，不打印值。Worker 不校验 JWT 配置。Web 使用 `web/.env` 中的 `VITE_API_BASE_URL`；开发默认值为 `http://localhost:8080`。

## 本机演练

先启动 Day02 的 PostgreSQL、Redis、Mailpit，再应用 Day03 的数据库迁移，并分别启动 API 与 Web：

```bash
docker compose -f deploy/compose.dev.yml up -d
cd server
go run ./cmd/api

# 另一个终端
cd web
npm install
npm run dev
```

浏览器依次完成：注册一个新邮箱、退出、登录、刷新页面、再次退出。刷新页面后仍能进入 Account，说明 Refresh Cookie 正在恢复内存中的 Access Token；退出后访问 `/` 应被送回 `/login`。

也可用 curl 观察 Cookie 轮换。`cookies.txt` 只用于本地演练，不能提交：

```bash
curl -i -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"correct-horse-battery-staple"}' \
  http://localhost:8080/api/v1/auth/register

curl -i -b cookies.txt -c cookies-next.txt -X POST \
  http://localhost:8080/api/v1/auth/refresh

curl -i -b cookies.txt -X POST http://localhost:8080/api/v1/auth/refresh
curl -i -b cookies-next.txt -X POST http://localhost:8080/api/v1/auth/logout
```

第三个请求使用已经轮换掉的旧 Cookie，应返回 `401`；退出后的新 Cookie 再刷新也应返回 `401`。

## 验收命令

```bash
npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml
/tmp/pulsewatch-tools/oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
(cd server && /tmp/pulsewatch-tools/sqlc generate && /tmp/pulsewatch-tools/sqlc compile)
(cd web && npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts)

(cd server && go test ./... && go test -race ./... && go vet ./...)
(cd web && npm run lint && npm run test -- --run && npm run build)
git diff --check
```

重复执行代码生成命令后，生成文件不应产生 Git 差异。数据库认证集成测试使用临时 schema，完成后应删除该 schema，避免污染日常开发数据。

## 后续衔接

Day06 的监控 Handler 直接复用 `RequireUser` 中间件和用户 ID context。Day08 的调度与 Day10 的通知继续使用 PostgreSQL 作为事实来源；Redis 和 Worker 不保存认证状态。
