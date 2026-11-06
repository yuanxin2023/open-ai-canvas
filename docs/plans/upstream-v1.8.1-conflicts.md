# 官方 v1.8.1 整合：冲突与决策记录

## 基线与当前状态

- 本地起点：`114a9e19f67460bdacd59a0f171f480e708d28f2`；备份：`codex/backup-before-upstream-v1.8.1-114a9e19`。
- 独立工作树分支：`codex/merge-upstream-v1.8.1`；原工作树保持原样。
- 官方标签：`v1.8.1`，标签对象 `16b4a7a014efae29189f9bb4d371800e64f5ad5d`；固定目标 `8611fc0eb0dadf30f62db7b3e196a63301c68b28`。
- 共同祖先：`878b515b7d45ab4fe4a64c07043ad95028e4ac3b`（官方 v1.7.3.7）；官方新增 54 个提交、494 个变更文件。
- Git 拉取、目标校验、merge-tree 预检及不自动提交的三方合并已完成；没有持久修改网络代理。首次合并有 116 个冲突路径（112 UU、2 AA、2 DU），含 41 个二进制冲突；下方保留初始清单。
- 已按 D-01 至 D-15 完成模块整合，所有冲突已解决，完整质量检查和最终复核通过；交付为保留两条历史的本地合并提交。
- 本轮未推送、发布、部署、创建标签或启动预览；仅新建隔离测试数据库，未访问现有业务数据库。

## 用户决策与落实

| 编号 | 用户选择 | 已落实的边界 |
| --- | --- | --- |
| D-01 文本价格规则 | 接入新规则并保留本地账务 | 建单锁定上海时间时段，实际输入选档，保留微积分、两位小数、预留和退款恢复保护；不引入多币种及分组倍率 |
| D-02 参考视频 | 按官方走 | 售价、成本、实测参考时长、报价及历史快照同步；纯 Token 旧模型开启并回填现价，含按秒档模型保持关闭 |
| D-03 技能管理 | 舍弃本地技能管理，完全按官方走 | 官方列表、上架/下架、编辑及工作台；删除旧管理路由和两层可用性判定；保留历史技能、版本、安装关系及 `admin.platform.skills` 权限 |
| D-04 方舟素材 | 按官方走 | 真人/虚拟/版权素材组、审核、授权与旧素材转换；管理操作仍检查本站方舟细分权限 |
| D-05 姿势库 | 接入并新增独立权限 | `admin.content.pose_library` 接入后台内容管理、前端路由和服务端；普通登录用户选用后复制为本人资源 |
| D-06 灵感 | 官方大厅及同步，保留个人提示词 | 官方每日推荐及上游中文同步；保留个人提示词入口、用户隔离和等待保存结果的写合同 |
| D-07 侧栏 | 官方侧栏和账号菜单布局，保留自有品牌名称 | 采用官方组件及签到/容量显示；读取本站品牌，保留手机导航、焦点恢复和游客登录后继续导航合同 |
| D-08 首页 | 官方页签与修复，保留本地对话布局 | 官方模式卡与大厅；保留桌面对话侧栏和手机历史抽屉 |
| D-09 设置 | 官方分组，保留本地业务栏目 | 接入方舟及提醒偏好，保留账号安全、订单、积分流水及功能开关 |
| D-10 画布与弹窗 | 界面和交互整体采用官方 | 工具条、引用、转换、素材、反馈、任务列表按官方；不恢复斜杠输入菜单；保留归属、隐私及功能开关校验 |
| D-11 模型目录向导 | 继续不提供 | 两个删除/修改冲突保持删除 |
| D-12 支付插件 | 本地支付合同，接入修复并重建 | RPC 业务错误识别；人民币、促销、库存及返利合同保留；六个支付插件及 OpenAI Images 包重建并校验 |
| D-13 功能开放 | 保留本地双栏及确认，加入官方新功能 | 保留逐项确认与服务端保存确认，接入官方新增能力及依赖提示 |
| D-14 版本 | 自有 v1.8.2 | VERSION 与本地整合发布说明同步，不创建标签、不发布 |
| D-15 支付关单 | 采用官方确认及复查，保留本地支付合同 | 查单异常保留待支付；关单失败复查，已付则入账；三次确认不存在才按渠道不存在关单，本地库存/返利和人民币核验保留 |

## 决策完成

以上已提出冲突均收到明确选择。后续验证若发现新的业务取舍，仍需重新讨论，不以已有决定覆盖新冲突。

## 实施核对

- 历史 Schema 1–105 从本地基线保留，仅追加独立的 106–112；没有用官方 1–89 覆盖本地身份。
- 新 Go 文件沿用 `infinite-canvas/backend`；新增业务 API 沿用前端 `http`；保留自有 Release 更新源、独立 Agent 和插件体系。
- 修复自动合并造成的重复技能路由、同名不同签名方法与 service 别名；强制检查新姿势、灵感和方舟操作的细分权限。
- 修复反馈开关/诊断路径脱敏、任务响应跨账号通知及大厅异步保存的整合遗漏。更新与已确认官方布局不符的旧 UI 断言，保留权限、归属和业务失败的回归断言。
- 列表、详情、刷新和重试也在请求发起时固定账号 scope，旧账号响应不进入新账号完成跟踪；相关 15 项 API 回归通过。

## 验证记录

- 宿主机模型及 kernel 专项通过；CGO 未启用的 SQLite 失败不算通过，后续改用 Go 1.25.14 的隔离 Linux CGO 环境。
- 技能同步、旧身份/版本/安装引用、官方上架状态及管理员编辑保留专项通过；参考视频与 Token 账务专项通过。
- 新技能、姿势、灵感及方舟管理的无关受限管理员拒绝测试通过（服务端写入及外部请求之前）。
- 前端类型检查、lint、格式检查及全量测试通过：3452 项、0 失败。前期旧界面断言、遗漏样式和账号跟踪问题均已修复，没有删掉权限、归属和失败传播的回归要求。
- Linux 数据库全包：145 项通过、4 项按环境跳过、0 失败。初次磁盘同步缓慢的运行已停止，改用隔离内存临时目录重跑；未把中止算通过。
- 从原始 `114a9e19` 源码建立真正的 Schema 105 夹具，分别在 SQLite 与 PostgreSQL 17 升到 112，连续三次 up、一次 verify 全部通过；比较 155 张旧表原有列，105 条旧迁移记录及应用时间、余额、账单、人民币支付、技能版本与安装关系、项目均保留。仅官方回填修改的模型和价格档 `updated_at` 按预期变化；纯 Token 回填售价/成本和混合档关闭也通过，SQLite 完整性/外键检查通过。
- PostgreSQL 的账本预检和升级专项通过。完整脚本覆盖从本地起点开始的全部 546 个整合文件：`bun scripts/verify-before-push.mjs --base 114a9e19 --jobs 4 --skip-e2e`，总结为「共 11 项，失败 0 项，用时 153.3s（并行 4）」。后端非 app 包及 app 四分片（列表含 1662 个顶层测试/示例/模糊测试入口）、前端格式/类型/lint/全量测试均通过。
- 完整检查在 Git 导出的隔离 Linux 副本运行，Bun 1.3.9、Go 1.25.14、Node 22.23.2，使用锁文件对应的 Linux 依赖和隔离 PostgreSQL。初次环境准备中的 Git 身份/换行及 Git 环境变量问题未计为通过；最后一轮用干净副本和正常 Git 发现机制通过。
- 完整检查发现并修复新旧测试夹具差异：视频报价沿用本站积分取整；方舟余额夹具显式设零；图片资源测试包含本站配额模型；RPC 子进程夹具补齐本地多服务商插件描述信息。支付确认/复查与受限管理员专项通过。
- 支付六包、OpenAI Images 包逐项与当前源文件字节一致，压缩包完整性及全部 111 个 SHA-256 校验通过；支付跨平台二进制已从合并源码重建，尚未在每个平台实机运行。

## 未覆盖与交付边界

- 按本轮不启动预览的约束跳过浏览器 E2E；真实支付、第三方素材认证、付费模型生成和真实浏览器验收未运行。
- 脚本列出的未覆盖项：支付插件 Docker 原生双架构构建、Agent `golangci-lint v2.5.0`（环境未安装）。支付插件多平台交叉编译、包完整性及校验和已通过，不能替代各平台实机运行。
- OpenAPI 223 个路径无重复，799 个内部引用均可解析；旧技能管理写接口已移除，新技能及姿势接口、类型和 http 调用方一致。
- 检查输出及隔离夹具保存在 Git 忽略的 `.local/cache/merge-v1.8.1/`，完整脚本详细日志在 `.local/cache/verify/`。最终通过的源码快照为 `0f7959e317fd058c8fcd19c9de24ebbfb8099299`，随后只更新本合并记录和待验收文档。
- 原工作树保持干净、停在原始提交；不推送、不发布、不部署、不创建标签，不操作现有业务数据库。

## 初始冲突路径快照

本节记录首次合并结果，编号稳定。UU 为双方修改，AA 为双方新增，DU 为本地删除/官方修改。二进制路径包含在同一清单中；后续落实状态以决策记录为准。

### 计费、模型与数据迁移（13 个路径）

| 编号 | 状态 | 路径 |
| --- | --- | --- |
| C-001 | UU | `backend/internal/app/channel_models.go` |
| C-002 | UU | `backend/internal/app/channel_transfer.go` |
| C-003 | UU | `backend/internal/app/finance.go` |
| C-004 | UU | `backend/internal/app/logical_model_quote.go` |
| C-005 | UU | `backend/internal/app/model_catalog.go` |
| C-006 | UU | `backend/internal/database/migrations.go` |
| C-007 | UU | `backend/internal/model/models_channel.go` |
| C-008 | UU | `backend/internal/model/models_finance.go` |
| C-009 | UU | `backend/internal/repository/finance_billing.go` |
| C-010 | UU | `backend/internal/repository/finance_video_usage_test.go` |
| C-011 | UU | `web/src/lib/model-pricing.ts` |
| C-012 | UU | `web/src/pages/admin/components/channel-model-editor-form.ts` |
| C-013 | UU | `web/src/pages/admin/components/channel-model-editor.tsx` |

### 技能管理（5 个路径）

| 编号 | 状态 | 路径 |
| --- | --- | --- |
| C-014 | UU | `backend/internal/repository/skills.go` |
| C-015 | UU | `backend/internal/skills/skills_builtin.go` |
| C-016 | AA | `web/src/pages/admin/skills/skills-page.tsx` |
| C-017 | AA | `web/src/services/api/admin-skills.ts` |
| C-018 | UU | `web/src/services/api/skills.ts` |

### 插件与支付（44 个路径）

| 编号 | 状态 | 路径 |
| --- | --- | --- |
| C-019 | UU | `backend/internal/payment/rpc.go` |
| C-020 | UU | `plugin-packages/official-payment-alipay-page.yingce-plugin` |
| C-021 | UU | `plugin-packages/official-payment-alipay-page/backend/provider` |
| C-022 | UU | `plugin-packages/official-payment-alipay-page/backend/provider-darwin-amd64` |
| C-023 | UU | `plugin-packages/official-payment-alipay-page/backend/provider-darwin-arm64` |
| C-024 | UU | `plugin-packages/official-payment-alipay-page/backend/provider-linux-amd64` |
| C-025 | UU | `plugin-packages/official-payment-alipay-page/backend/provider-linux-arm64` |
| C-026 | UU | `plugin-packages/official-payment-alipay-page/backend/provider-windows-amd64.exe` |
| C-027 | UU | `plugin-packages/official-payment-alipay-page/backend/provider-windows-arm64.exe` |
| C-028 | UU | `plugin-packages/official-payment-epay.yingce-plugin` |
| C-029 | UU | `plugin-packages/official-payment-epay/backend/provider` |
| C-030 | UU | `plugin-packages/official-payment-epay/backend/provider-darwin-amd64` |
| C-031 | UU | `plugin-packages/official-payment-epay/backend/provider-darwin-arm64` |
| C-032 | UU | `plugin-packages/official-payment-epay/backend/provider-linux-amd64` |
| C-033 | UU | `plugin-packages/official-payment-epay/backend/provider-linux-arm64` |
| C-034 | UU | `plugin-packages/official-payment-epay/backend/provider-windows-amd64.exe` |
| C-035 | UU | `plugin-packages/official-payment-epay/backend/provider-windows-arm64.exe` |
| C-036 | UU | `plugin-packages/official-payment-wechat-native.yingce-plugin` |
| C-037 | UU | `plugin-packages/official-payment-wechat-native/backend/provider` |
| C-038 | UU | `plugin-packages/official-payment-wechat-native/backend/provider-darwin-amd64` |
| C-039 | UU | `plugin-packages/official-payment-wechat-native/backend/provider-darwin-arm64` |
| C-040 | UU | `plugin-packages/official-payment-wechat-native/backend/provider-linux-amd64` |
| C-041 | UU | `plugin-packages/official-payment-wechat-native/backend/provider-linux-arm64` |
| C-042 | UU | `plugin-packages/official-payment-wechat-native/backend/provider-windows-amd64.exe` |
| C-043 | UU | `plugin-packages/official-payment-wechat-native/backend/provider-windows-arm64.exe` |
| C-044 | UU | `plugin-packages/official-payment-xunhupay.yingce-plugin` |
| C-045 | UU | `plugin-packages/official-payment-xunhupay/backend/provider` |
| C-046 | UU | `plugin-packages/official-payment-xunhupay/backend/provider-darwin-amd64` |
| C-047 | UU | `plugin-packages/official-payment-xunhupay/backend/provider-darwin-arm64` |
| C-048 | UU | `plugin-packages/official-payment-xunhupay/backend/provider-linux-amd64` |
| C-049 | UU | `plugin-packages/official-payment-xunhupay/backend/provider-linux-arm64` |
| C-050 | UU | `plugin-packages/official-payment-xunhupay/backend/provider-windows-amd64.exe` |
| C-051 | UU | `plugin-packages/official-payment-xunhupay/backend/provider-windows-arm64.exe` |
| C-052 | UU | `plugin-packages/official-payment-zhifufm.yingce-plugin` |
| C-053 | UU | `plugin-packages/official-payment-zhifufm/backend/provider` |
| C-054 | UU | `plugin-packages/official-payment-zhifufm/backend/provider-darwin-amd64` |
| C-055 | UU | `plugin-packages/official-payment-zhifufm/backend/provider-darwin-arm64` |
| C-056 | UU | `plugin-packages/official-payment-zhifufm/backend/provider-linux-amd64` |
| C-057 | UU | `plugin-packages/official-payment-zhifufm/backend/provider-linux-arm64` |
| C-058 | UU | `plugin-packages/official-payment-zhifufm/backend/provider-windows-amd64.exe` |
| C-059 | UU | `plugin-packages/official-payment-zhifufm/backend/provider-windows-arm64.exe` |
| C-060 | UU | `plugin-packages/openai-images.yingce-plugin` |
| C-061 | UU | `plugin-packages/openai-images/docs/interface.md` |
| C-062 | UU | `plugin-packages/openai-images/manifest.json` |

### 文档（7 个路径）

| 编号 | 状态 | 路径 |
| --- | --- | --- |
| C-063 | UU | `docs/content/docs/backend/backend-database.mdx` |
| C-064 | UU | `docs/content/docs/backend/code-map.mdx` |
| C-065 | UU | `docs/content/docs/backend/http-api.mdx` |
| C-066 | UU | `docs/content/docs/overview/features.mdx` |
| C-067 | UU | `docs/content/docs/progress/pending-test.mdx` |
| C-068 | UU | `docs/design/admin-ui-design-standard.md` |
| C-069 | UU | `docs/index.md` |

### 界面、画布与创作（37 个路径）

| 编号 | 状态 | 路径 |
| --- | --- | --- |
| C-070 | UU | `web/index.html` |
| C-071 | UU | `web/src/components/canvas/canvas-resource-mention-textarea.tsx` |
| C-072 | UU | `web/src/components/canvas/canvas-toolbar.tsx` |
| C-073 | UU | `web/src/components/canvas/nodes/media-conversion-node.tsx` |
| C-074 | UU | `web/src/components/feedback/feedback-modal.tsx` |
| C-075 | UU | `web/src/components/layout/app-top-nav.tsx` |
| C-076 | UU | `web/src/components/layout/workspace-sidebar-nav.tsx` |
| C-077 | UU | `web/src/components/layout/workspace-top-bar.tsx` |
| C-078 | UU | `web/src/components/layout/workspace-wallet-modal.tsx` |
| C-079 | UU | `web/src/components/ui/aceternity/full-screen-loader.tsx` |
| C-080 | DU | `web/src/pages/admin/channels/provider-catalog-wizard.css` |
| C-081 | DU | `web/src/pages/admin/channels/provider-catalog-wizard.tsx` |
| C-082 | UU | `web/src/pages/admin/components/admin-shell.tsx` |
| C-083 | UU | `web/src/pages/admin/components/feature-availability-panel.tsx` |
| C-084 | UU | `web/src/pages/assets/asset-batch-upload-modal.tsx` |
| C-085 | UU | `web/src/pages/assets/index.tsx` |
| C-086 | UU | `web/src/pages/canvas/index.tsx` |
| C-087 | UU | `web/src/pages/canvas/use-canvas-media-tools.ts` |
| C-088 | UU | `web/src/pages/canvas/use-canvas-node-editor.ts` |
| C-089 | UU | `web/src/pages/create/creation-workspace-messages.tsx` |
| C-090 | UU | `web/src/pages/create/creation-workspace.tsx` |
| C-091 | UU | `web/src/pages/create/index.tsx` |
| C-092 | UU | `web/src/pages/inspirations/index.tsx` |
| C-093 | UU | `web/src/pages/settings/index.tsx` |
| C-094 | UU | `web/src/pages/tasks/index.tsx` |
| C-095 | UU | `web/src/pages/tasks/task-grid-card.tsx` |
| C-096 | UU | `web/src/pages/tasks/task-list-row.tsx` |
| C-097 | UU | `web/src/pages/tasks/task-status-filter.tsx` |
| C-098 | UU | `web/src/pages/tasks/task-video-thumbnail.tsx` |
| C-099 | UU | `web/src/router.tsx` |
| C-100 | UU | `web/src/services/api/task-center.ts` |
| C-101 | UU | `web/src/services/api/wallet.ts` |
| C-102 | UU | `web/src/services/api/workbench.ts` |
| C-103 | UU | `web/src/services/inspiration-catalog.ts` |
| C-104 | UU | `web/src/stores/use-appearance-store.ts` |
| C-105 | UU | `web/src/stores/use-config-store.ts` |
| C-106 | UU | `web/src/styles/globals.css` |

### 其他后端与版本配置（10 个路径）

| 编号 | 状态 | 路径 |
| --- | --- | --- |
| C-107 | UU | `.gitignore` |
| C-108 | UU | `CHANGELOG.md` |
| C-109 | UU | `VERSION` |
| C-110 | UU | `backend/internal/app/ark_portraits.go` |
| C-111 | UU | `backend/internal/app/drama_models.go` |
| C-112 | UU | `backend/internal/app/error_codes.go` |
| C-113 | UU | `backend/internal/app/provider_media_hydration.go` |
| C-114 | UU | `backend/internal/app/service.go` |
| C-115 | UU | `backend/internal/handler/openapi.yaml` |
| C-116 | UU | `backend/internal/service/aliases_types.go` |
