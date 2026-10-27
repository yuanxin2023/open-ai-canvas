# 官方 v1.6.9.2 实际冲突清单

下表保留隔离预合并最初的索引快照，表中“待讨论”是当时状态，不能用作当前进度。已确认的模块决策及验证见文末和[审查报告](upstream-v1.6.9.2-review.md)；最终验收仍须核对真实数据库副本与浏览器。

- HEAD：`15861487c12780d08043ed7d7eca864deea73c53`。
- MERGE_HEAD：`19eec9bc4e71cbb5941b9dc4139fd6487fc91015`。
- 基线：`5ea9466bb41e2664b4ffd3de0b2387c78856dd3a`。
- 191 个冲突路径，550 条未合并索引记录。
- 初始未合并索引 SHA-256：`962dae1e265ddb3ea189d7d572b2f3be61860c2a513308903ca4d3297c3cf774`。
- UU：双方修改；DU：本地删除/官方修改；UD：本地修改/官方删除。
- 文本标记采用 diff3，包含本地、共同祖先和官方三个版本。禁止将共同祖先块误当作当前实现。

## 文本与删除冲突

| 编号 | 状态 | 主要责任 | 路径 | 结论 |
| --- | --- | --- | --- | --- |
| C-001 | UU | 构建与运行 | `.github/workflows/quality.yml` | 待讨论 |
| C-002 | UU | 文档与质量 | `README.md` | OPS-01a/b 自有源码/发布入口已整合；其他品牌、署名及重复文档冲突待完成 |
| C-003 | UU | 数据库迁移 | `backend/cmd/migrate-sqlite-postgres/main.go` | DB-04 已确认，83 只读副本/原账本复制及隔离验证完成 |
| C-004 | UU | 构建与运行 | `backend/cmd/server/main.go` | 待讨论 |
| C-005 | UU | 构建与运行 | `backend/cmd/server/main_test.go` | 待讨论 |
| C-006 | UU | 账号与权限 | `backend/internal/app/admin.go` | AUTH/ACC 候选整合：本地用户/管理员入口及权限迁入独立文件，app 运行待验收 |
| C-007 | UU | 插件与模型 | `backend/internal/app/admin_channel_test.go` | 渠道用例整合，使用真实公网 IP 避免 DNS；app 运行待验收 |
| C-008 | UU | 支付积分与经营 | `backend/internal/app/analytics.go` | ANA-01 已确认并整合：已结算订单去重、全局可用余额；app 运行待验收 |
| C-009 | UU | UI 与产品功能 | `backend/internal/app/appearance.go` | 待讨论 |
| C-010 | DU | UI 与产品功能 | `backend/internal/app/appearance_button_fill_test.go` | 待讨论 |
| C-011 | UU | UI 与产品功能 | `backend/internal/app/appearance_canvas.go` | CAN-07：保留本地动态球/Live2D 合同，候选文本已整合，运行待验收 |
| C-012 | DU | UI 与产品功能 | `backend/internal/app/appearance_skins.go` | 待讨论 |
| C-013 | UU | UI 与产品功能 | `backend/internal/app/appearance_test.go` | 待讨论 |
| C-014 | UU | 账号与权限 | `backend/internal/app/auth_bridge.go` | 待讨论 |
| C-015 | UU | 插件与模型 | `backend/internal/app/channel_models.go` | 已迁出声明核对并去重，保留 PLG/PAY 协议计价合同，app 运行待验收 |
| C-016 | UU | 画布 Agent 与媒体 | `backend/internal/app/cloud_agent_media.go` | PAY/CAN 已确认：整合结算与官方事务保存，保留本地逐用户额度；文本已整合，完整后端待运行 |
| C-017 | UU | 支付积分与经营 | `backend/internal/app/credit_cost.go` | 待讨论 |
| C-018 | UU | 跨模块基础设施 | `backend/internal/app/error_codes.go` | 待讨论 |
| C-019 | UU | 跨模块基础设施 | `backend/internal/app/feature_availability_test.go` | 已按 AUTH/UI-02/F-03/F-09 合并默认开关断言；数据库测试待公共依赖解决 |
| C-020 | UU | 插件与模型 | `backend/internal/app/model_capability.go` | 待讨论 |
| C-021 | UU | 插件与模型 | `backend/internal/app/model_router.go` | 已迁出声明核对并去重，保留备用线路授权与新参数，app 运行待验收 |
| C-022 | UU | 支付积分与经营 | `backend/internal/app/payment.go` | 待讨论 |
| C-023 | UU | 插件与模型 | `backend/internal/app/protocol_plugins.go` | 待讨论 |
| C-024 | UU | 画布 Agent 与媒体 | `backend/internal/app/provider.go` | 已迁出声明核对并去重，保留参考资源归属及逐用户限额，app 运行待验收 |
| C-025 | UU | 画布 Agent 与媒体 | `backend/internal/app/provider_task_recovery.go` | 保留请求日志权限并接官方恢复参数/审计，app 运行待验收 |
| C-026 | UU | 画布 Agent 与媒体 | `backend/internal/app/resource.go` | 按拆分资源合同整合，恢复逐用户配额，app 运行待验收 |
| C-027 | UU | 画布 Agent 与媒体 | `backend/internal/app/resource_deletion_worker.go` | ACC-05 定时清理已接入，回收站仍按逐用户策略；领域专项通过，app 待验收 |
| C-028 | UU | 画布 Agent 与媒体 | `backend/internal/app/resource_test.go` | 同时补齐逐用户配额与上传预留夹具，app 运行待验收 |
| C-029 | UU | 跨模块基础设施 | `backend/internal/app/service.go` | 兑换码/Agent 恢复及 PAY-15 可选退款工作进程整合，尚未启动 |
| C-030 | UU | 画布 Agent 与媒体 | `backend/internal/app/skills_bridge.go` | 待讨论 |
| C-031 | UU | 画布 Agent 与媒体 | `backend/internal/app/storage_location.go` | 保留本地存储测试对象前缀，接运行时访问策略；app 运行待验收 |
| C-032 | UU | 跨模块基础设施 | `backend/internal/app/task_creation.go` | 按 PAY/PLG/UI-02 整合：保留本地准入、参数/计价，处理任务提示词及工作台开关；事务待验收 |
| C-033 | UU | 跨模块基础设施 | `backend/internal/app/task_lifecycle.go` | 保留本地 GORM/module 导入，补齐已确认的工作台关闭校验；UI-02b 已确认，资格检查共享、历史手动生成及来源事务复核已整合，完整运行待验收 |
| C-034 | UU | 画布 Agent 与媒体 | `backend/internal/app/upload_quota.go` | 保留逐用户配额并接数据库分片预留/资源发布，app 运行待验收 |
| C-035 | UU | 账号与权限 | `backend/internal/auth/auth.go` | 待讨论 |
| C-036 | UU | 账号与权限 | `backend/internal/auth/email_domain_test.go` | 待讨论 |
| C-037 | UU | 账号与权限 | `backend/internal/auth/linuxdo.go` | 待讨论 |
| C-038 | UU | 账号与权限 | `backend/internal/auth/password_reset_test.go` | 待讨论 |
| C-039 | UU | 数据库迁移 | `backend/internal/database/migrations.go` | DB-02/03 已确认，候选实现及数据库专项通过 |
| C-040 | UU | 数据库迁移 | `backend/internal/database/migrations_test.go` | DB-02/03 已确认，候选实现及数据库专项通过 |
| C-041 | UU | 数据库迁移 | `backend/internal/database/schema.go` | DB-02/03 已确认，候选实现及数据库专项通过 |
| C-042 | UU | 画布 Agent 与媒体 | `backend/internal/handler/agent_lesson.go` | 待讨论 |
| C-043 | UU | 跨模块基础设施 | `backend/internal/handler/api_test.go` | 路由断言保留本地改密及官方工作台/恢复/技能入口，handler 运行待验收 |
| C-044 | UU | 账号与权限 | `backend/internal/handler/auth.go` | 待讨论 |
| C-045 | UU | 跨模块基础设施 | `backend/internal/handler/openapi.yaml` | 待讨论 |
| C-046 | UU | 画布 Agent 与媒体 | `backend/internal/handler/resource_upload_session.go` | 分片准入由数据库原子预留并使用逐用户限额，handler 运行待验收 |
| C-047 | UU | 跨模块基础设施 | `backend/internal/handler/response_test.go` | 同时保留用户名次数与项目同名错误断言，handler 运行待验收 |
| C-048 | UU | 画布 Agent 与媒体 | `backend/internal/handler/skills.go` | 待讨论 |
| C-049 | UU | 跨模块基础设施 | `backend/internal/kernel/error_codes.go` | 待讨论 |
| C-050 | UU | 跨模块基础设施 | `backend/internal/kernel/errors.go` | 待讨论 |
| C-051 | UU | 账号与权限 | `backend/internal/model/models_identity.go` | 待讨论 |
| C-052 | UU | 支付积分与经营 | `backend/internal/model/models_payment.go` | 待讨论 |
| C-053 | UD | 画布 Agent 与媒体 | `backend/internal/prompts/agent-media-policy.md` | 媒体约束已进入唯一系统策略，移除重复文件；完整 prompts 包通过 |
| C-054 | UU | 画布 Agent 与媒体 | `backend/internal/prompts/agent-system-policy.md` | 保留工作台身份、服务端助手名及唯一版本化行为合同；完整 prompts 包通过 |
| C-055 | UU | 画布 Agent 与媒体 | `backend/internal/prompts/agent_policy_test.go` | 保留本地身份及完整行为断言；完整 prompts 包通过 |
| C-056 | UU | 插件与模型 | `backend/internal/protocol/builtin_test.go` | 待讨论 |
| C-057 | UU | 插件与模型 | `backend/internal/protocol/manifest.go` | 待讨论 |
| C-058 | UU | 插件与模型 | `backend/internal/protocol/package_test.go` | 待讨论 |
| C-059 | UU | 跨模块基础设施 | `backend/internal/repository/admin_audit.go` | 按既定保护项整合，选择性回归通过；ACC-05 日志专项待决 |
| C-060 | UU | 画布 Agent 与媒体 | `backend/internal/repository/admin_storage_delete.go` | 按既定保护项整合，选择性回归通过；ACC-05 日志专项待决 |
| C-061 | UU | 支付积分与经营 | `backend/internal/repository/analytics_test.go` | 按既定保护项整合，选择性回归通过；ACC-05 日志专项待决 |
| C-062 | UU | 支付积分与经营 | `backend/internal/repository/finance.go` | 待讨论 |
| C-063 | UU | 支付积分与经营 | `backend/internal/repository/payment.go` | 待讨论 |
| C-064 | UU | 跨模块基础设施 | `backend/internal/repository/repository.go` | 按既定保护项整合，选择性回归通过；ACC-05 日志专项待决 |
| C-065 | UU | 跨模块基础设施 | `backend/internal/repository/skill_packages.go` | 待讨论 |
| C-066 | UU | 画布 Agent 与媒体 | `backend/internal/repository/skills.go` | 待讨论 |
| C-067 | UU | 跨模块基础设施 | `backend/internal/service/aliases_consts.go` | 保留本地封面上限/技能开关别名，补齐已确认工作台与反馈功能别名 |
| C-068 | UU | 跨模块基础设施 | `backend/internal/service/aliases_types.go` | 待讨论 |
| C-069 | UD | 画布 Agent 与媒体 | `backend/internal/service/seed/skills.json` | 待讨论 |
| C-070 | UD | 画布 Agent 与媒体 | `backend/internal/skills/seed/skills.json` | 待讨论 |
| C-071 | UU | 画布 Agent 与媒体 | `backend/internal/skills/skills_seed_image_tools.go` | 待讨论 |
| C-072 | UU | 画布 Agent 与媒体 | `backend/internal/skills/skills_seed_test.go` | 待讨论 |
| C-073 | UU | 文档与质量 | `docs/content/docs/backend/backend-database.mdx` | 待讨论 |
| C-074 | UU | 文档与质量 | `docs/content/docs/backend/canvas-agent-appearance.mdx` | CAN-07：排除视频动画，保留 Live2D 及运行库修复说明；最终外观版本待统一 |
| C-075 | UU | 文档与质量 | `docs/content/docs/backend/code-map.mdx` | 待讨论 |
| C-076 | UU | 文档与质量 | `docs/content/docs/backend/http-api.mdx` | 待讨论 |
| C-077 | UU | 文档与质量 | `docs/content/docs/overview/features.mdx` | 待讨论 |
| C-078 | UU | 文档与质量 | `docs/content/docs/progress/pending-test.mdx` | 待讨论 |
| C-079 | UU | 文档与质量 | `docs/design/admin-ui-change-log.md` | 待讨论 |
| C-080 | DU | 文档与质量 | `docs/design/site-appearance-and-skins.mdx` | 待讨论 |
| C-081 | DU | 插件与模型 | `plugin-packages/agnes-image.yingce-plugin` | 待讨论 |
| C-082 | UU | 插件与模型 | `plugin-packages/agnes-image/docs/interface.md` | 待讨论 |
| C-083 | UU | 插件与模型 | `plugin-packages/agnes-image/manifest.json` | 待讨论 |
| C-084 | DU | 插件与模型 | `plugin-packages/autodl-comfyui.yingce-plugin` | 待讨论 |
| C-085 | DU | 插件与模型 | `plugin-packages/dashscope-wan3-video.yingce-plugin` | 待讨论 |
| C-086 | DU | 插件与模型 | `plugin-packages/google-gemini-image.yingce-plugin` | 待讨论 |
| C-087 | UU | 插件与模型 | `plugin-packages/google-gemini-image/docs/interface.md` | 待讨论 |
| C-088 | UU | 插件与模型 | `plugin-packages/google-gemini-image/manifest.json` | 待讨论 |
| C-089 | DU | 插件与模型 | `plugin-packages/image-tools.yingce-plugin` | 待讨论 |
| C-090 | UU | 插件与模型 | `plugin-packages/image-tools/docs/interface.md` | 待讨论 |
| C-091 | UU | 插件与模型 | `plugin-packages/image-tools/manifest.json` | 待讨论 |
| C-092 | UU | 插件与模型 | `plugin-packages/metaso-h3/manifest.json` | 待讨论 |
| C-093 | DU | 插件与模型 | `plugin-packages/minimax-hailuo-video-v2.yingce-plugin` | 待讨论 |
| C-094 | UU | 插件与模型 | `plugin-packages/minimax-hailuo-video-v2/docs/interface.md` | 待讨论 |
| C-095 | UU | 插件与模型 | `plugin-packages/minimax-hailuo-video-v2/manifest.json` | 待讨论 |
| C-096 | UU | 插件与模型 | `plugin-packages/newapi-video-generations-v1/manifest.json` | 待讨论 |
| C-097 | DU | 插件与模型 | `plugin-packages/openai-images.yingce-plugin` | 待讨论 |
| C-098 | UU | 插件与模型 | `plugin-packages/openai-images/docs/interface.md` | 待讨论 |
| C-099 | UU | 插件与模型 | `plugin-packages/openai-images/manifest.json` | 待讨论 |
| C-100 | DU | 插件与模型 | `plugin-packages/openai-responses.yingce-plugin` | 待讨论 |
| C-101 | UU | 插件与模型 | `plugin-packages/openai-responses/docs/interface.md` | 待讨论 |
| C-102 | UU | 插件与模型 | `plugin-packages/openai-responses/manifest.json` | 待讨论 |
| C-103 | UU | 插件与模型 | `plugin-packages/src-metaso-h3/manifest.json` | 待讨论 |
| C-104 | DU | 插件与模型 | `plugin-packages/xai-grok-images.yingce-plugin` | 待讨论 |
| C-105 | UU | 插件与模型 | `plugin-packages/xai-grok-images/docs/interface.md` | 待讨论 |
| C-106 | UU | 插件与模型 | `plugin-packages/xai-grok-images/manifest.json` | 待讨论 |
| C-107 | UU | 账号与权限 | `web/src/components/auth/require-feature.tsx` | UI-02 已确认；保留本地技能开关和失败拒绝，新增两个入口，无管理员绕过 |
| C-108 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/batch-generation-settings-dialog.tsx` | 待讨论 |
| C-109 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-agent-skill-library-modal.tsx` | 待讨论 |
| C-110 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-agent-welcome.tsx` | 待讨论 |
| C-111 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-cloud-agent-panel.tsx` | CAN-01 至 CAN-07：保留本地技能/思考能力控制，整合恢复/审批/预设，排除视频；候选文本已整合，完整验收待进行 |
| C-112 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-cloud-agent-settings.tsx` | 待讨论 |
| C-113 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-creative-interaction.tsx` | 待讨论 |
| C-114 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-image-toolbar-tools.tsx` | CAN-10：纳入图片规划入口，预填后手动发送；文本已整合，真实 UI 待验收 |
| C-115 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-node-content.tsx` | 待讨论 |
| C-116 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-node-image-edit-dialog.tsx` | 待讨论 |
| C-117 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-node-layer-decomposition-dialog.tsx` | 待讨论 |
| C-118 | UU | 画布 Agent 与媒体 | `web/src/components/canvas/canvas-resource-mention-textarea.tsx` | 待讨论 |
| C-119 | UU | 插件与模型 | `web/src/components/image-size-presets-editor.tsx` | 待讨论 |
| C-120 | UU | UI 与产品功能 | `web/src/components/layout/app-changelog-dialog.tsx` | 待讨论 |
| C-121 | UU | UI 与产品功能 | `web/src/components/layout/app-top-nav.tsx` | 保留本地积分不足提示宿主；F-09 反馈仅设置入口，移除浮动反馈挂载 |
| C-122 | UU | UI 与产品功能 | `web/src/components/layout/user-avatar.tsx` | 待讨论 |
| C-123 | UU | UI 与产品功能 | `web/src/components/layout/workspace-account-card.tsx` | 待讨论 |
| C-124 | UU | UI 与产品功能 | `web/src/components/layout/workspace-command-palette.tsx` | 待讨论 |
| C-125 | UU | UI 与产品功能 | `web/src/components/layout/workspace-sidebar-footer.tsx` | 待讨论 |
| C-126 | UU | UI 与产品功能 | `web/src/components/layout/workspace-sidebar-nav.tsx` | UI-02 已整合两个可选入口；保留首页、提示词、品牌和客服，不恢复本地已移除入口 |
| C-127 | UU | UI 与产品功能 | `web/src/components/layout/workspace-top-bar-checkin.tsx` | 待讨论 |
| C-128 | UU | 支付积分与经营 | `web/src/components/layout/workspace-wallet-modal.tsx` | 待讨论 |
| C-129 | UU | 插件与模型 | `web/src/components/model-picker.tsx` | 待讨论 |
| C-130 | UU | 画布 Agent 与媒体 | `web/src/components/skills/skill-runtime-picker.tsx` | 待讨论 |
| C-131 | UU | UI 与产品功能 | `web/src/lib/app-theme.ts` | 待讨论 |
| C-132 | UU | 画布 Agent 与媒体 | `web/src/lib/canvas/generation-contract.generated.ts` | 待讨论 |
| C-133 | UU | 画布 Agent 与媒体 | `web/src/lib/canvas/generation-contract.ts` | 待讨论 |
| C-134 | UU | UI 与产品功能 | `web/src/lib/image-utils.ts` | 待讨论 |
| C-135 | UU | 插件与模型 | `web/src/lib/model-capabilities.ts` | 待讨论 |
| C-136 | UU | 插件与模型 | `web/src/lib/model-tags.ts` | 待讨论 |
| C-137 | UU | 插件与模型 | `web/src/lib/plugins/builtin/editor/editor-subtitle-tools.tsx` | 待讨论 |
| C-138 | DU | UI 与产品功能 | `web/src/lib/skin-themes.ts` | 待讨论 |
| C-139 | UU | UI 与产品功能 | `web/src/main.tsx` | 待讨论 |
| C-140 | UU | UI 与产品功能 | `web/src/pages/admin/components/admin-shell.tsx` | F-02 平台分组新增工具入口并保留本地存储分组；其余跨模块冲突待后续决定 |
| C-141 | UU | UI 与产品功能 | `web/src/pages/admin/components/api-log-detail-drawer.tsx` | 待讨论 |
| C-142 | UU | 插件与模型 | `web/src/pages/admin/components/channel-model-cost-summary.tsx` | 待讨论 |
| C-143 | UU | 插件与模型 | `web/src/pages/admin/components/channel-model-editor-form.ts` | 待讨论 |
| C-144 | UU | 插件与模型 | `web/src/pages/admin/components/channel-model-price-tier-fields.tsx` | 待讨论 |
| C-145 | UU | 插件与模型 | `web/src/pages/admin/components/channel-model-price-tier-form.ts` | 待讨论 |
| C-146 | UU | 插件与模型 | `web/src/pages/admin/components/channel-model-pricing.ts` | 待讨论 |
| C-147 | UU | 支付积分与经营 | `web/src/pages/admin/components/credit-operations-panel.tsx` | 待讨论 |
| C-148 | UU | UI 与产品功能 | `web/src/pages/admin/components/feature-availability-panel.tsx` | UI-02/F-03/F-09 已确认；新增模块默认关闭，开关二次确认且保留本地配置 |
| C-149 | UU | 支付积分与经营 | `web/src/pages/admin/components/redemption-codes-panel.tsx` | 待讨论 |
| C-150 | UU | 画布 Agent 与媒体 | `web/src/pages/admin/components/storage-resources-panel.tsx` | 待讨论 |
| C-151 | UU | UI 与产品功能 | `web/src/pages/admin/logs/logs-page.tsx` | 待讨论 |
| C-152 | UU | 支付积分与经营 | `web/src/pages/admin/payments/payments-page.tsx` | 待讨论 |
| C-153 | UU | UI 与产品功能 | `web/src/pages/admin/settings/appearance-settings-page.tsx` | 待讨论 |
| C-154 | DU | UI 与产品功能 | `web/src/pages/admin/settings/components/welcome-setting.tsx` | 待讨论 |
| C-155 | UU | UI 与产品功能 | `web/src/pages/admin/settings/runtime-policy-settings-page.tsx` | 待讨论 |
| C-156 | UU | UI 与产品功能 | `web/src/pages/admin/theme/admin-chrome.css` | 待讨论 |
| C-157 | UU | 账号与权限 | `web/src/pages/admin/users/users-drawer.tsx` | 待讨论 |
| C-158 | UU | 账号与权限 | `web/src/pages/admin/users/users-panel.tsx` | 待讨论 |
| C-159 | UU | 账号与权限 | `web/src/pages/auth/auth-scene.tsx` | 待讨论 |
| C-160 | UU | 账号与权限 | `web/src/pages/auth/login.tsx` | 待讨论 |
| C-161 | UU | 账号与权限 | `web/src/pages/auth/register.tsx` | 待讨论 |
| C-162 | UU | 画布 Agent 与媒体 | `web/src/pages/canvas/canvas-project-top-bar.tsx` | 待讨论 |
| C-163 | UU | 画布 Agent 与媒体 | `web/src/pages/canvas/use-canvas-generation.ts` | 待讨论 |
| C-164 | DU | UI 与产品功能 | `web/src/pages/create/creation-inspirations.ts` | 待讨论 |
| C-165 | UU | 画布 Agent 与媒体 | `web/src/pages/create/creation-workspace.tsx` | F-03 保留本地精选/个人提示词的服务端调用；画布生成等跨模块文本仍待统一 |
| C-166 | UU | UI 与产品功能 | `web/src/pages/create/index.tsx` | F-03 新模块按默认关闭开关接在本地精选后；已确认取舍不替代其余跨模块冲突解决与真实首页验收 |
| C-167 | UU | UI 与产品功能 | `web/src/pages/settings/index.tsx` | 保留本地订单/积分/返利及权限开关；新增默认关闭的我的反馈，不增加重复资料页 |
| C-168 | UU | 画布 Agent 与媒体 | `web/src/pages/skills/index.tsx` | 待讨论 |
| C-169 | DU | UI 与产品功能 | `web/src/pages/welcome/index.tsx` | 待讨论 |
| C-170 | UU | UI 与产品功能 | `web/src/router.tsx` | F-02 已确认工作台工具路由及独立权限门禁；其余跨模块冲突保留待讨论 |
| C-171 | UU | UI 与产品功能 | `web/src/services/api/appearance.ts` | 待讨论 |
| C-172 | UU | 账号与权限 | `web/src/services/api/auth.ts` | 待讨论 |
| C-173 | UU | UI 与产品功能 | `web/src/services/api/image-response.ts` | 待讨论 |
| C-174 | UU | 支付积分与经营 | `web/src/services/api/payments.ts` | 待讨论 |
| C-175 | UU | 画布 Agent 与媒体 | `web/src/services/creative-agent-controller.ts` | 待讨论 |
| C-176 | UU | UI 与产品功能 | `web/src/stores/use-appearance-store.ts` | 待讨论 |
| C-177 | UU | UI 与产品功能 | `web/src/stores/use-config-store.ts` | 待讨论 |
| C-178 | UU | UI 与产品功能 | `web/src/styles/globals.css` | 待讨论 |
| C-179 | UU | 插件与模型 | `web/src/styles/shared/model-picker.css` | 待讨论 |
| C-180 | UU | UI 与产品功能 | `web/test/admin-ui-regressions.test.ts` | 待讨论 |
| C-181 | DU | UI 与产品功能 | `web/test/app-theme.test.ts` | 待讨论 |
| C-182 | UU | UI 与产品功能 | `web/test/appearance-bootstrap.test.ts` | 待讨论 |
| C-183 | UU | UI 与产品功能 | `web/test/application-bootstrap.test.mjs` | 待讨论 |
| C-184 | UU | 画布 Agent 与媒体 | `web/test/canvas-media-performance.test.ts` | 待讨论 |
| C-185 | UU | 插件与模型 | `web/test/channel-model-pricing.test.ts` | 待讨论 |
| C-186 | UU | 画布 Agent 与媒体 | `web/test/helpers/generation-storage-consistency.worker.ts` | 待讨论 |
| C-187 | UU | 插件与模型 | `web/test/model-picker-groups.test.ts` | 待讨论 |
| C-188 | UU | 插件与模型 | `web/test/model-picker-style-source.test.ts` | 待讨论 |
| C-189 | DU | UI 与产品功能 | `web/test/site-appearance-and-skins.test.ts` | 待讨论 |
| C-190 | DU | UI 与产品功能 | `web/test/welcome-contributors.test.ts` | 待讨论 |
| C-191 | DU | UI 与产品功能 | `web/test/workspace-sidebar-checkin.test.ts` | 待讨论 |

## 双方有变化但没有文本冲突

共 185 个路径。包含权限、计费、删除、存储、协议、缓存和配置的写路径仍需逐项检查，不因 Git 自动合并而默认通过。分类并不涵盖所有官方新文件，新增文件的权限与结构检查另见审查报告。

| 编号 | 路径 | 状态 |
| --- | --- | --- |
| A-001 | `.env.example` | 待语义复核 |
| A-002 | `.github/workflows/publish-images.yml` | 待语义复核 |
| A-003 | `.gitignore` | 待语义复核 |
| A-004 | `AGENTS.md` | 待语义复核 |
| A-005 | `CHANGELOG.md` | 待语义复核 |
| A-006 | `backend/internal/app/admin_audit.go` | 待语义复核 |
| A-007 | `backend/internal/app/admin_bulk_user_test.go` | 待语义复核 |
| A-008 | `backend/internal/app/admin_storage.go` | 待语义复核 |
| A-009 | `backend/internal/app/admin_storage_delete.go` | 待语义复核 |
| A-010 | `backend/internal/app/admin_storage_delete_test.go` | 待语义复核 |
| A-011 | `backend/internal/app/admin_storage_test.go` | 待语义复核 |
| A-012 | `backend/internal/app/admin_system_performance.go` | 待语义复核 |
| A-013 | `backend/internal/app/admin_update.go` | 待语义复核 |
| A-014 | `backend/internal/app/all_video_token_integration_test.go` | 待语义复核 |
| A-015 | `backend/internal/app/announcement.go` | 待语义复核 |
| A-016 | `backend/internal/app/appearance_live2d.go` | 待语义复核 |
| A-017 | `backend/internal/app/appearance_live2d_test.go` | 待语义复核 |
| A-018 | `backend/internal/app/canvas_bridge.go` | 待语义复核 |
| A-019 | `backend/internal/app/channel_model_pricing.go` | 待语义复核 |
| A-020 | `backend/internal/app/channel_model_pricing_test.go` | 待语义复核 |
| A-021 | `backend/internal/app/channel_order.go` | 待语义复核 |
| A-022 | `backend/internal/app/channel_presentation.go` | 待语义复核 |
| A-023 | `backend/internal/app/cloud_agent.go` | 待语义复核 |
| A-024 | `backend/internal/app/cloud_agent_lessons.go` | 待语义复核 |
| A-025 | `backend/internal/app/cloud_agent_test.go` | 待语义复核 |
| A-026 | `backend/internal/app/creation.go` | 待语义复核 |
| A-027 | `backend/internal/app/creation_canvas.go` | 待语义复核 |
| A-028 | `backend/internal/app/creation_test.go` | 待语义复核 |
| A-029 | `backend/internal/app/credit_cost_test.go` | 待语义复核 |
| A-030 | `backend/internal/app/credit_policy.go` | 待语义复核 |
| A-031 | `backend/internal/app/drawing_engine.go` | 待语义复核 |
| A-032 | `backend/internal/app/errors.go` | 待语义复核 |
| A-033 | `backend/internal/app/finance.go` | 待语义复核 |
| A-034 | `backend/internal/app/finance_redeem_test.go` | 待语义复核 |
| A-035 | `backend/internal/app/finance_token_arithmetic_test.go` | 待语义复核 |
| A-036 | `backend/internal/app/finance_token_test.go` | 待语义复核 |
| A-037 | `backend/internal/app/logical_model_quote.go` | 待语义复核 |
| A-038 | `backend/internal/app/logical_models.go` | 待语义复核 |
| A-039 | `backend/internal/app/payment_export.go` | 待语义复核 |
| A-040 | `backend/internal/app/payment_plugins.go` | 待语义复核 |
| A-041 | `backend/internal/app/payment_plugins_test.go` | 待语义复核 |
| A-042 | `backend/internal/app/payment_reconciliation.go` | 待语义复核 |
| A-043 | `backend/internal/app/payment_runtime_cache_test.go` | 待语义复核 |
| A-044 | `backend/internal/app/platform_bridge.go` | 待语义复核 |
| A-045 | `backend/internal/app/plugin_management.go` | 待语义复核 |
| A-046 | `backend/internal/app/project_settings_test.go` | 待语义复核 |
| A-047 | `backend/internal/app/prompts_bridge.go` | 待语义复核 |
| A-048 | `backend/internal/app/protocol_registry.go` | 待语义复核 |
| A-049 | `backend/internal/app/protocol_runtime_test.go` | 待语义复核 |
| A-050 | `backend/internal/app/provider_audio.go` | 待语义复核 |
| A-051 | `backend/internal/app/provider_http_client.go` | 待语义复核 |
| A-052 | `backend/internal/app/provider_test.go` | 待语义复核 |
| A-053 | `backend/internal/app/resource_delete.go` | 待语义复核 |
| A-054 | `backend/internal/app/resource_delete_test.go` | 待语义复核 |
| A-055 | `backend/internal/app/response_interception.go` | 待语义复核 |
| A-056 | `backend/internal/app/settings.go` | 待语义复核 |
| A-057 | `backend/internal/app/settings_ark_assets.go` | 待语义复核 |
| A-058 | `backend/internal/app/storage_location_test.go` | 待语义复核 |
| A-059 | `backend/internal/app/storage_quota.go` | 待语义复核 |
| A-060 | `backend/internal/app/storage_quota_test.go` | 待语义复核 |
| A-061 | `backend/internal/app/task_api_call_log_test.go` | 待语义复核 |
| A-062 | `backend/internal/app/task_execution_contract_test.go` | 待语义复核 |
| A-063 | `backend/internal/app/task_render.go` | 待语义复核 |
| A-064 | `backend/internal/app/task_timeline.go` | 待语义复核 |
| A-065 | `backend/internal/app/text_replay.go` | 待语义复核 |
| A-066 | `backend/internal/app/text_replay_cache.go` | 待语义复核 |
| A-067 | `backend/internal/app/video_logical_billing_test.go` | 待语义复核 |
| A-068 | `backend/internal/app/video_token_quote_test.go` | 待语义复核 |
| A-069 | `backend/internal/app/workflow_plugins.go` | 待语义复核 |
| A-070 | `backend/internal/auth/auth_user_test.go` | 待语义复核 |
| A-071 | `backend/internal/auth/email.go` | 待语义复核 |
| A-072 | `backend/internal/auth/host.go` | 待语义复核 |
| A-073 | `backend/internal/auth/libtv.go` | 待语义复核 |
| A-074 | `backend/internal/auth/registration.go` | 待语义复核 |
| A-075 | `backend/internal/generation/registry.go` | 待语义复核 |
| A-076 | `backend/internal/handler/admin_analytics.go` | 待语义复核 |
| A-077 | `backend/internal/handler/admin_storage.go` | 待语义复核 |
| A-078 | `backend/internal/handler/agent.go` | 待语义复核 |
| A-079 | `backend/internal/handler/api.go` | 待语义复核 |
| A-080 | `backend/internal/handler/appearance_live2d.go` | 待语义复核 |
| A-081 | `backend/internal/handler/finance.go` | 待语义复核 |
| A-082 | `backend/internal/handler/payment.go` | 待语义复核 |
| A-083 | `backend/internal/handler/plugin.go` | 待语义复核 |
| A-084 | `backend/internal/handler/response.go` | 待语义复核 |
| A-085 | `backend/internal/handler/routes.go` | 待语义复核 |
| A-086 | `backend/internal/handler/user_data.go` | 待语义复核 |
| A-087 | `backend/internal/model/models.go` | 待语义复核 |
| A-088 | `backend/internal/model/models_finance.go` | 待语义复核 |
| A-089 | `backend/internal/model/models_platform.go` | 待语义复核 |
| A-090 | `backend/internal/model/models_project.go` | 待语义复核 |
| A-091 | `backend/internal/payment/rpc.go` | 待语义复核 |
| A-092 | `backend/internal/platform/feature_availability.go` | 待语义复核 |
| A-093 | `backend/internal/platform/host.go` | 待语义复核 |
| A-094 | `backend/internal/platform/runtime_policy.go` | 待语义复核 |
| A-095 | `backend/internal/protocol/dashscope_wan3_video_test.go` | 待语义复核 |
| A-096 | `backend/internal/protocol/image_tools_test.go` | 待语义复核 |
| A-097 | `backend/internal/protocol/minimax_video_test.go` | 待语义复核 |
| A-098 | `backend/internal/protocol/official_catalog_test.go` | 待语义复核 |
| A-099 | `backend/internal/protocol/package.go` | 待语义复核 |
| A-100 | `backend/internal/repository/admin_storage.go` | 待语义复核 |
| A-101 | `backend/internal/repository/agent_lesson.go` | 待语义复核 |
| A-102 | `backend/internal/repository/analytics.go` | 待语义复核 |
| A-103 | `backend/internal/repository/finance_recovery_test.go` | 待语义复核 |
| A-104 | `backend/internal/repository/finance_token_arithmetic_test.go` | 待语义复核 |
| A-105 | `backend/internal/repository/finance_token_test.go` | 待语义复核 |
| A-106 | `backend/internal/repository/finance_video_formula_test.go` | 待语义复核 |
| A-107 | `backend/internal/repository/finance_video_usage_test.go` | 待语义复核 |
| A-108 | `backend/internal/repository/payment_test.go` | 待语义复核 |
| A-109 | `backend/internal/repository/resource_cleanup.go` | 待语义复核 |
| A-110 | `backend/internal/repository/resource_deletion.go` | 待语义复核 |
| A-111 | `backend/internal/repository/resource_reference.go` | 待语义复核 |
| A-112 | `backend/internal/service/aliases_funcs.go` | 待语义复核 |
| A-113 | `backend/internal/skills/skill_packages.go` | 待语义复核 |
| A-114 | `backend/internal/skills/skill_packages_test.go` | 待语义复核 |
| A-115 | `backend/internal/skills/skills.go` | 待语义复核 |
| A-116 | `backend/payment-plugins/aliases.go` | 待语义复核 |
| A-117 | `docker-compose.deploy.yml` | OPS-01a/b 已确认：显式自有匹配 Agent、独立凭据及自有正式发布；Compose 正负配置通过 |
| A-118 | `docker-compose.dev.yml` | OPS-01a 已补齐独立 Agent、内部桥接与健康依赖，开发数据目录保留 |
| A-119 | `docker-compose.local.yml` | OPS-01a 已补齐匹配构建、独立 Agent 和运行时重建关系 |
| A-120 | `docs/content/docs/backend/local-development.mdx` | 待语义复核 |
| A-121 | `docs/content/docs/backend/meta.json` | 待语义复核 |
| A-122 | `docs/index.md` | 待语义复核 |
| A-123 | `plugin-packages/generate-catalog.mjs` | 待语义复核 |
| A-124 | `plugin-packages/image-tools/README.md` | 待语义复核 |
| A-125 | `plugin-packages/metaso-h3/docs/interface.md` | 待语义复核 |
| A-126 | `plugin-packages/newapi-video-generations-v1/docs/interface.md` | 待语义复核 |
| A-127 | `plugin-packages/src-metaso-h3/docs/interface.md` | 待语义复核 |
| A-128 | `scripts/install-server.sh` | 待语义复核 |
| A-129 | `web/index.html` | 待语义复核 |
| A-130 | `web/src/components/canvas/canvas-node-prompt-panel.tsx` | 待语义复核 |
| A-131 | `web/src/components/layout/app-providers.tsx` | 待语义复核 |
| A-132 | `web/src/components/layout/system-announcement-center.tsx` | 待语义复核 |
| A-133 | `web/src/components/layout/workspace-account-menu.tsx` | 待语义复核 |
| A-134 | `web/src/components/layout/workspace-sidebar-checkin.tsx` | 待语义复核 |
| A-135 | `web/src/components/layout/workspace-top-bar.tsx` | 待语义复核 |
| A-136 | `web/src/lib/canvas/agent-appearance.ts` | 待语义复核 |
| A-137 | `web/src/lib/canvas/canvas-file-upload.ts` | 待语义复核 |
| A-138 | `web/src/lib/canvas/canvas-node-visibility.ts` | 待语义复核 |
| A-139 | `web/src/lib/canvas/canvas-task-state.ts` | 待语义复核 |
| A-140 | `web/src/lib/creation/creative-agent-state.ts` | 待语义复核 |
| A-141 | `web/src/lib/generation-error.ts` | 待语义复核 |
| A-142 | `web/src/lib/image-size-presets.ts` | 待语义复核 |
| A-143 | `web/src/lib/model-picker-groups.ts` | 待语义复核 |
| A-144 | `web/src/lib/model-pricing.ts` | 待语义复核 |
| A-145 | `web/src/lib/model-protocols.ts` | 待语义复核 |
| A-146 | `web/src/lib/workspace-route-modules.ts` | 待语义复核 |
| A-147 | `web/src/pages/admin/admin-route-pages.tsx` | 待语义复核 |
| A-148 | `web/src/pages/admin/components/admin-ui.tsx` | 待语义复核 |
| A-149 | `web/src/pages/admin/components/admin-user-detail-drawer.tsx` | 待语义复核 |
| A-150 | `web/src/pages/admin/components/agent-lessons-panel.tsx` | 待语义复核 |
| A-151 | `web/src/pages/admin/components/analytics-panel.tsx` | 待语义复核 |
| A-152 | `web/src/pages/admin/components/channel-model-manager.tsx` | 待语义复核 |
| A-153 | `web/src/pages/admin/components/channel-model-reprice-dialog.tsx` | 待语义复核 |
| A-154 | `web/src/pages/admin/components/channel-model-tags-editor.tsx` | 待语义复核 |
| A-155 | `web/src/pages/canvas/canvas-image-generation-executor.ts` | 待语义复核 |
| A-156 | `web/src/pages/canvas/project.tsx` | 待语义复核 |
| A-157 | `web/src/pages/canvas/shared.tsx` | 待语义复核 |
| A-158 | `web/src/pages/create/creation-conversations.ts` | 待语义复核 |
| A-159 | `web/src/pages/create/creation-product.css` | 待语义复核 |
| A-160 | `web/src/pages/create/creation-types.ts` | 待语义复核 |
| A-161 | `web/src/pages/projects/detail/workflow-production-workbench.tsx` | 待语义复核 |
| A-162 | `web/src/pages/settings/model-default-grid.tsx` | 待语义复核 |
| A-163 | `web/src/services/api/admin-storage.ts` | 待语义复核 |
| A-164 | `web/src/services/api/image.ts` | 待语义复核 |
| A-165 | `web/src/services/api/resources.ts` | 待语义复核 |
| A-166 | `web/src/services/api/task-center.ts` | 待语义复核 |
| A-167 | `web/src/services/api/wallet.ts` | 待语义复核 |
| A-168 | `web/src/services/creation-canvas-conversation.ts` | 待语义复核 |
| A-169 | `web/src/services/diagnostics/diagnostics-api.ts` | 待语义复核 |
| A-170 | `web/src/stores/use-user-store.ts` | 待语义复核 |
| A-171 | `web/src/styles/admin-ui.css` | 待语义复核 |
| A-172 | `web/src/styles/workspace-product.css` | 待语义复核 |
| A-173 | `web/src/types/canvas.ts` | 待语义复核 |
| A-174 | `web/test/asset-batch-delete.test.ts` | 待语义复核 |
| A-175 | `web/test/assets-collection-layout.test.ts` | 待语义复核 |
| A-176 | `web/test/canvas-appearance.test.ts` | 待语义复核 |
| A-177 | `web/test/canvas-batch-table-merge.test.tsx` | 待语义复核 |
| A-178 | `web/test/canvas-image-upload.test.tsx` | 待语义复核 |
| A-179 | `web/test/canvas-node-generation-mentions.test.ts` | 待语义复核 |
| A-180 | `web/test/channel-model-editor-form.test.ts` | 待语义复核 |
| A-181 | `web/test/creative-agent-controller.test.ts` | 待语义复核 |
| A-182 | `web/test/model-picker-prices.test.tsx` | 待语义复核 |
| A-183 | `web/test/model-pricing.test.ts` | 待语义复核 |
| A-184 | `web/test/scrollbar-style-source.test.ts` | 待语义复核 |
| A-185 | `web/test/workspace-route-loading.test.ts` | 待语义复核 |

## DB-02/03 决策补充

最终批准保留已部署 1–51 两套谱系，52–77 接入 26 项官方缺失迁移，78–83 接入六项业务扩展；排除推荐目录迁移，不改旧号或签名。极旧充值权益 16、官方极旧工作台 51 停止并报告。已实现完整账本预检、事务升级及反馈资源回填；四套原始历史夹具和 SQLite/PostgreSQL 专项通过。真实业务副本与跨库转换入口另行验收，未运行真实库。

DB-04：跨库转换入口不复制原账本、目标初始化种子与复制数据存在交叉；完整讨论、推荐方案和验收见 [转库讨论单](upstream-v1.6.9.2-migrations-conversion.md)。用户已选择方案 1，转换实现、两套历史夹具、只读重放、拒绝差异与回滚以及实际命令验证完成。

ACC-05：本地成功审计 90 天/每用户 200 条与官方新审计无自动上限发生交叉，已单独提交 [登录审计保留规则](upstream-v1.6.9.2-login-retention.md)；用户已确认方案 1，清理及完整仓储包 130/266 验证通过。共享层过程与选择性验证见 [整合记录](upstream-v1.6.9.2-repository-integration.md)。

ANA-01 已确认采用本地已结算订单去重指标；PAY-15 已确认可选自动退款默认关闭。完整 Backend 39 包、2,224 顶层用例及 4,121 项含子项检查通过，另有三项明确跳过。UI-03 可选公告默认关闭、UI-04 访客浏览、CAN-11 连续拆层、CAN-12 新媒体调度与 PLG-07 成本加价率均已确认并实现；详见[公共页面与画布补充决策](upstream-v1.6.9.2-ui-public-and-canvas.md)。前端构建、lint 和完整 2,982 项测试已通过；媒体接线修复后的最终复核另记。
