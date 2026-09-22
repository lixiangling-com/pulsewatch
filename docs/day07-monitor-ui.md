# Day07：监控管理 UI

## 1. 目标和边界

Day07 在 `web/` 用 React 实现监控管理的桌面端界面，复用 Day04/05 的认证闭环与 Day06 的监控 API。列表、新建、编辑、暂停/恢复、删除全部走 `/api/v1/monitors`，类型来自 OpenAPI 生成物，不手写接口类型。

Issue：`[PW-006] 完成监控管理 UI`。建议分支：`feat/PW-006-monitor-ui`。

本阶段不接 SSE，不展示检查历史与故障，不做移动端布局。

## 2. 页面与交互

- 监控列表：分页（`page`/`page_size`）、空态、加载态与错误态。
- 新建监控：名称、URL、间隔、期望状态码；前端校验与后端 `field_errors` 回显。
- 编辑：部分更新（`PATCH`），只发送变更字段。
- 暂停/恢复：走 `POST /:id/pause` / `POST /:id/resume`，界面反映 `paused` 状态。
- 删除：软删除，二次确认；删除后列表移除。
- 状态展示：把确认中状态合并展示（`confirming_down` 归并到 `down`、`confirming_up` 归并到 `up`）。

## 3. 数据与状态

- 访问 JWT 在内存持有（15 分钟），刷新页面用 refresh cookie 恢复会话，复用 Day04/05 逻辑。
- 类型来自 `src/generated/api-types.ts`（openapi-typescript 生成）。
- 错误处理：统一错误对象 `code` / `message` / `field_errors` / `request_id`，表单错误逐字段展示；20 个上限与非本人/已删除 404 给出友好提示。
- 视觉遵循既有 shadcn/Tailwind 约定。

## 4. 验收证据

必须通过：

1. `cd web && npm run build`；
2. 本地跑通：登录 → 列表空态 → 新建（含非法 URL / 超上限报错）→ 改名（不触发 pending）→ 改间隔（回到 pending）→ 暂停/恢复 → 删除后消失；
3. 刷新页面会话保持。

## 5. 明确不包含

SSE 实时状态提示、移动端布局、检查历史与故障展示、监控批量操作。

## 6. 后续衔接

- Day08 的调度产生真实 `monitors.status` 后，列表状态徽章直接消费该字段；
- Day11 接入 SSE 后，列表/详情可增量更新。
