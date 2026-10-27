# OPS-01 自有构建、Agent 与更新来源

本轮交付本地候选分支，不推送、正式发布或部署生产。用户后续明确授权本机现有库升级与预览：完整备份和副本演练后已升级至 83，3140 预览运行。正式 Backend/Agent 镜像拉取受 Docker Hub 连接超时阻断，当前预览使用缓存 Linux 环境及候选源码编译产物，不能代替正式部署验收。真实 Token 和生产数据不在本文件填写。OPS-01a 已整合独立 Agent；OPS-01b 已接入自有完整正式 Release，由管理员手动确认升级。最新状态见[候选验收记录](upstream-v1.6.9.2-candidate-validation.md)，下文未启动等描述保留配置阶段过程。

## OPS-01a：Agent 服务拓扑

调用链为浏览器 `/api/agent` → Backend 的云端 Agent 编排 → `runCloudAgentPi`。设置 `YINGCE_AGENT_URL` 时调用独立服务；未设置时使用后端内的 Node 运行时。当前后端 Dockerfile 仍包含该运行时，两条路线均有代码基础。

| 项目 | 本批检查起点 | 整合要求 |
| --- | --- | --- |
| 生产编排 | 官方 deploy 有独立 `yingce-agent`；本地 server/local 编排未补齐 | 生产使用 PostgreSQL、Redis、Web、Backend、独立 Agent，迁移单次执行；Web 对外，其他服务内部通信 |
| 构建关系 | Web、Backend 有自建配置，Agent 有单独 Dockerfile，migrate 与 Backend 共用镜像 | 四个程序来自同一候选提交和匹配运行时；不得回退到官方成品业务镜像 |
| Agent 镜像 | deploy 的 Agent 仍可兜底到官方 ghcr.io 地址 | 自建路线只用自建 Agent；镜像发布路线必须显式指定自有不可变引用，不保留官方兜底 |
| 服务认证 | Agent Token 可以复用 Updater Token | Agent 使用独立凭据，至少 32 字符；不写入源码，不因关闭更新器失去认证 |
| Node 依赖 | Agent Dockerfile 从 `backend/agent-runtime/pi` 安装 package-lock | 保留该独立运行时的 npm 锁文件；Web 继续 Bun，不更换 Web 锁文件 |
| 旧会话、权限、预算 | 业务状态与工具审批在 Backend | 继续由 Backend 承担，不因拆出进程重置账号、额度或模型配置 |

- 方案 A，推荐：生产补齐独立 Agent，自有源码匹配构建；宿主机开发保留显式可用的内嵌路径。
- 方案 B：本轮生产继续内嵌运行时，不新增常驻 Agent 服务，相关会话并发、内存和恢复检查仍需覆盖。
- 验收：Compose 静态配置、四个构建版本、独立凭据、内部桥接、审批/SSE/取消/重连与停机恢复；不以健康检查代替完整链路，不默认真实付费调用。
- 最终决定：**用户已确认采用独立 Agent，全部使用自有匹配构建和独立凭据。**

已补齐默认/local/server/dev 的独立 Agent 构建、内部桥接、健康依赖和可追溯构建参数，deploy 不再兜底到官方 Agent 镜像或复用 Updater Token。migrate 与 Backend 保持同一镜像。Web、Backend、Agent 镜像登记同一组版本/提交/时间参数；Backend 的第一方 ldflags 修正到现有 `infinite-canvas/backend` 模块。运行时 npm 锁文件和 Web 的 Bun 锁文件保持各自边界。宿主机未设置独立服务地址时仍可显式使用原内嵌开发路径。

验证：6 组真实 `docker compose config --format json` 配置解析通过，核对 Agent 不发布端口、两端凭据一致、内部桥接、健康依赖、相同构建参数和迁移镜像；7 组缺少独立凭据/镜像的负向配置均拒绝，没有复用更新器凭据或官方镜像。部署配置专项 Go 用例通过。Host Updater 全包在 Windows 首次运行失败，含 POSIX 文件权限断言不支持及旧官方兜底断言；后者按确认规则更新。随后已在禁用外网、源码和 Compose 只读的隔离 Linux 测试容器完整通过，覆盖文件权限和新增发布源/镜像合同；不代表 Backend 全包或真实更新通过。未执行实际镜像构建、服务启动、数据库迁移、安装脚本或生产操作。

## OPS-01b：在线更新来源

本地保护边界是自建程序与定制数据。新增的 Host Updater 会从指定 GitHub 仓库读取 Release、Compose、校验和和更新器二进制，再选择业务镜像。`scripts/install-server-image.sh`、`scripts/install-host-updater.sh` 及源码安装脚本仍存在官方仓库/镜像默认地址；直接保留这些入口可能把自有构建切换回官方版本。

- 方案 A：本轮关闭在线更新执行及自动安装 Host Updater；保留手工构建/发布说明和必要的自建 Compose。后台明确说明自建版本的手动发布方式，不能显示可执行的官方一键升级。现有生产更新器不在本轮操作。
- 方案 B：本轮适配自有更新源。所有 Compose、镜像和更新器必须来自匹配发布，不保留官方兜底；正式版本就绪后由具备系统更新权限的管理员手动确认，普通提交不直接切换生产。
- 无论选择哪条路线，数据库升级仍需确认后的独立迁移、备份与恢复演练；不能用回退旧镜像代替新 Schema 的恢复方案。
- 最终决定：**用户已确认接入 yuanxin2023/open-ai-canvas 的完整正式 Release，后台手动确认更新；普通提交和预发布不自动上线。**

### 自有正式发布路线与实现

已核对本地 origin 及 GitHub 仓库元数据：自有公开仓库为 `yuanxin2023/open-ai-canvas`。候选工作流 `.github/workflows/publish-images.yml` 支持 main/tag 推送或手动运行；镜像按 repository_owner 进入自己的 GHCR 命名空间，版本标签的发布步骤在检查和镜像发布成功后生成 Release、Updater 二进制和 SHA256SUMS。后台读取的是 Release，普通提交本身不会成为后台可更新版本。

可以形成「修改源码 → 推送新版本标签 → Actions 检查/构建匹配镜像与发布资产 → 生成正式 Release → 管理后台检查 → 管理员确认升级」的流程。三类镜像可分别使用 `ghcr.io/yuanxin2023/open-ai-canvas-{web,backend,yingce-agent}`，迁移复用 Backend 镜像；上线执行继续走独立 Host Updater 的备份、迁移和健康验证，正式启用前须完成副本及恢复演练。

已统一源码/Release 默认仓库、按同版本标签下载的 Compose/安装脚本、自有镜像来源及 Updater 运行配置；安装脚本明确写入 CANVAS_UPDATER_REPOSITORY。后台过滤预发布、草稿、非法标签及缺少完整已上传资产的 Release，启动只接受最近成功检查的正式版本。不同来源的旧发布缓存失效，既有备份保留。镜像校验增加版本、源码来源、有效且一致的提交与仓库 digest；不匹配时在停止旧服务及迁移前拒绝。后台沿用细分系统更新权限，写请求绑定当前管理员，账号切换关闭旧确认弹窗，不在读取/检查时自动升级。仅整合本地候选；未启用远端 Actions、创建标签/Release、推送镜像、安装 Updater 或实际升级。

## 交叉修正清单

确认后同步处理 deploy/build/server/local/dev Compose、Backend 的第一方 Go ldflags 模块路径、Agent 环境变量、构建流程、安装脚本、后台更新状态及 README/专题部署文档。检查 nginx 的 WebSocket、文本/Agent SSE 和私有媒体；不统一放宽所有 API 缓冲、缓存或超时。

OPS-01a/b 已完成上述候选配置和源码整合。前端更新/反馈联合专项 12 项外层测试通过，两个包装另执行 11 项真实组件交互用例；变更前端文件 ESLint 通过。三个安装脚本 Bash 语法通过，临时夹具验证服务配置写入自有仓库、保留其他环境字段并拒绝预发布；没有调用安装/服务函数。OpenAPI 补齐四个系统更新入口及预期账号头。完整应用构建、真实发布、首次安装、Agent 桥接、权限/数据库集成与迁移恢复仍待验收。未执行业务镜像构建、Compose up、真实安装或服务升级。
