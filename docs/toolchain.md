# 工具链版本

Day03 使用固定版本生成和校验工具。生成文件只能由 OpenAPI、SQL 查询和配置文件重建，不能手工修改。

| 工具 | 版本 | 用途 |
| --- | --- | --- |
| Go | 1.26.5 | Go 编译和测试 |
| goose | 3.24.3 | PostgreSQL 迁移 |
| sqlc | 1.28.0 | SQL 到 Go 数据访问代码 |
| oapi-codegen | 2.4.1 | OpenAPI 到 Go 类型和 Gin 接口 |
| openapi-typescript | 7.13.0 | OpenAPI 到 TypeScript 类型 |
| Node.js | 22.22.3 | Web 构建 |
| npm | 10.9.8 | Web 依赖管理 |
| Redocly CLI | 1.34.5 | OpenAPI 校验（通过 npx 固定版本执行） |

## 生成命令

Redocly 会自动读取仓库根目录的 `.redocly.yaml`；健康探针只定义
`200/503`，因此关闭了不适用的通用 `4XX` 响应提醒。

在仓库根目录执行：

```bash
npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml
/tmp/pulsewatch-tools/oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
(cd server && /tmp/pulsewatch-tools/sqlc generate && /tmp/pulsewatch-tools/sqlc compile)
(cd web && npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts)
```

CI 或个人环境应将这些工具安装到固定版本，而不是长期依赖 `@latest`。
