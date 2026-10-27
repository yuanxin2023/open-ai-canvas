# Agent 官方默认形象恢复

## 范围与来源

用户在 v1.7.1 整合完成后单独要求恢复官方同款卡通形象。本次取代原 CAN-07 中关于内置默认动画的排除决定；自定义动画 ZIP 上传、状态映射编辑、远程推荐目录和皮肤入口仍不恢复。

- 官方基线：`v1.7.1` / `89fcb53fd65969cb58ae1eba5d18381db51819d1`。
- 资源：原样恢复 `web/public/doll/avatar.json` 与 12 个 MP4，全部 Git blob 哈希与官方相同。
- 播放：沿用官方绿幕透明处理及八种状态映射，新增减弱动态即时暂停、隐藏入口暂停、错误回退；为下方标签保留窗口内空间。
- 配置：前后端接受 `bundle`、`orb`、`live2d`。新安装默认 `bundle`，既有明确选择不被读取或升级自动重写；管理员可保存切换。本机新版预览通过现有外观管理 API 单独启用 `bundle`。
- 保留：自有品牌、助手名称、文案、Glow、权限、账号边界、现有 Live2D 导入与预览。切换形象不清除既有模型引用。
- 后端没有新增上传或公开媒体 API；内置资源由前端静态服务提供，不占用用户资源配额。不变更数据库 Schema、迁移编号或历史账本。

## 实际验证

- `bun test test/agent-avatar.test.tsx test/agent-live2d-lifecycle.test.ts test/live2d-avatar-loading.test.tsx test/agent-launcher-position.test.mjs test/appearance-refresh.test.ts test/site-appearance.test.ts`：28 项通过，148 个断言。覆盖资源完整性、八种状态、未知状态与片段缺失、非法清单、后台预览位置及既有 Live2D 控件、位置与外观刷新。
- `bun run build`：通过 TypeScript 检查与 Vite 构建。仍有既有大分块和动态/静态导入混用提示。
- `bun run lint`：通过。
- Linux Go 1.25.14、CGO/GCC 环境运行 `go test ./internal/app ./internal/handler -run 'Test(CanvasAppearance|Appearance|Live2D)' -count=1`：两个包通过，覆盖内置形象读写、旧选择保留、品牌/文案/Glow 保留、外观权限及 Live2D 约束。
- OpenAPI YAML 解析通过；`git diff --check` 通过。
- 真实应用 Chrome 验收通过：透明像素、持续播放、标签未越界、收起展开、鼠标拖动、打开面板后暂停、即时减弱动态、暗色模式、手机、后台选项和预览、清单 404 和视频 404 各自回退。无运行异常、非预期失败请求或模型写请求；两次预期 404 仅用于故障注入。
- 所有 13 个静态资源哈希与官方一致；MP4 的 MIME 为 `video/mp4`，字节范围请求返回 206。
- 本机新版预览保持健康，在已有隔离副本上先备份再启用形象。核对保存前后只改变形象类型，其他公开外观字段一致；临时验证会话及凭据文件已删除，原业务数据和旧版预览保持原状。

## 仍待验收

- 用户确认实际账号下的视觉尺寸、入口位置和交互偏好。
- 真实 Agent 任务的审批和结束动画：本轮验证状态映射及页面播放，没有调用付费模型。
- 部署环境的缓存/CDN/代理媒体访问及实际 Live2D 模型播放；本机验收不替代部署验收。

交付分支为自有仓库的 `codex/merge-upstream-v1.7.1`，按用户要求提交并推送本次修改；不创建 PR、推送标签、发布或生产部署。
