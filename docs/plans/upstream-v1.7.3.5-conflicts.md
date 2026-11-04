# 官方 v1.7.3.5 合并记录

## 当前状态与固定基线

本文件记录固定基线、用户确认的业务取舍和验证依据。已发现的冲突均有明确决定并已落实，必要自动检查通过，整合结果建立本地双亲合并提交。下方初始冲突表保留首次合并快照，历史建议不替代用户最终决定；浏览器及外部环境验收仍列入待测试文档。

| 项目 | 值 |
| --- | --- |
| 本地起点 | `28089ad0ea9ddc6eaa38cf2978d4e1e975b8c3fc` |
| 本地原分支 | `codex/merge-upstream-v1.7.1` |
| 官方目标 | `v1.7.3.5` / `7c14ffdd8c5480eefc29e6f1ab7da4ef25da2856` |
| 官方标签对象 | `c4501fdd08d6a0f0ef87aa2c981e3b6834219315` |
| 共同祖先 | `89fcb53fd65969cb58ae1eba5d18381db51819d1`（官方 v1.7.1） |
| 备份分支 | `codex/backup-before-upstream-v1.7.3.5-28089ad0` |
| 候选分支 | `codex/merge-upstream-v1.7.3.5` |
| 整合方式 | `merge --no-commit --no-ff`，diff3 标记，禁用 rerere 自动解决 |
| 本地 / 官方独有提交 | 122 / 133 |
| 官方增量 | 1,184 个文件；141,372 行新增，23,740 行删除 |
| 初始冲突 | 158 个路径，449 个文本冲突块 |
| 原工作区未提交修改 | 两份专题文档，已备份完整补丁并带入候选工作树 |

完整 Git 历史已通过现有本机代理和 Windows TLS 后端拉取，配置只作用于本次命令，没有修改全局网络设置。原工作树及其两份未提交文档保持不变。补丁 SHA-256：`AB41C3A5027AC9D6EB3E9177DAD2862CBBF920B9C1F1D2C244C0FA1E47EE70E6`。原始合并日志、未合并索引和补丁存入 Git 忽略的本地缓存，不提交机器路径或运行数据。

## 已确认决策

- D-01：保留本地返利体系，排除官方重复返佣链路；保留本地邀请关系、规则版本、审核与支付事务保护。
- D-02：只纳入用户分组与渠道权限，继续只用积分计价；不引入官方多货币、汇率和分组价格覆盖，也不借此修改既有价格精度。
- D-03：Agent 记忆与技能发现机制完全按照官方实现；不延续本地记忆提案交互。独立技能库开关不禁用 Agent 已安装技能，新技能浏览和安装仍受开关约束。保留用户隔离、资源归属、预算和媒体审批等既有安全边界，接入本地模块路径。
- D-04：保持本地删除状态，不恢复皮肤系统、官方欢迎页和模型供应商推荐向导；对应删除/修改冲突已按此处理。
- D-05：逐页向用户说明并选择，不对所有页面统一采用任意一方。已选页面如下。
- D-06：用户分组、电商配置、知识库分别新增可授予权限 `admin.users.groups`、`admin.ecommerce.config`、`admin.ecommerce.knowledge`，沿用本地全权限与受限管理员机制。
- 工作区侧栏：保留本地品牌、提示词等导航与账号布局，增加电商入口和账号分组徽标。
- 创作首页：保留本地灵感及桌面对话侧栏、手机历史抽屉，增加最近画布和官方作品恢复、重新获取。
- 后台功能开放：保留本地五组双栏与逐项确认，补充电商创作、商品视频、创作助手三个开关。
- 后台导航：保留本地分组与折叠，在用户与服务中加入用户分组，增加电商 Agent 分组。
- 用户管理：保留普通用户与管理员分开、备注和抽屉，加入分组筛选与编辑，充值、消耗、最近登录为可选列。
- 支付管理：保留本地商品运营、支付渠道、订单、对账的独立入口和页面；保留促销、库存、本地返利和人民币支付，不加入多货币。
- 画布 Agent 面板：图片规划与场景配方交互也完全跟随官方，保持本地品牌、已配置 Live2D 与内置头像，不恢复自定义头像 ZIP。
- 共享引用输入框：保留本地 `/` 快捷命令，融合官方引用悬停预览、边界及选区修复。
- 访客模型目录：继续允许未登录浏览，按默认分组范围过滤；提交生成、报价写路径仍要求真实登录。
- 个人设置：保留个人资料、账号安全、我的订单、积分流水等本地独立栏目，记忆内容采用官方。
- 图片／视频工作台：采用官方编辑侧栏、批次画廊、场景库与素材选择，迁入本地提交去重、待确认请求恢复、账号隔离和历史渠道恢复保护。
- 请求成功率：采用官方口径，以成功和失败的请求作为分母；请求总数保留完整展示。
- 后台请求明细：保留本地自动刷新、中文输入保护、用户备注、居中详情弹窗与财务详情入口，增加日期、用户、模型、渠道筛选。
- 积分运营：保留本地居中弹窗和请求详情入口，加入正数赠送按 1000 次注册／签到计算的发放量预估确认。
- 画布节点生成状态：采用官方阶段、失败和恢复展示，补回本地购买积分入口。
- 图片文字编辑：保留弹窗，采用官方点击后识别及模型、计费提示。
- 后台概览：采用官方余额和结算卡片名称、说明及已结算笔数。
- 全站普通按钮：采用官方底色、边框 token 和图标间距，保留本地品牌及 Glow；模型选择弹窗采用官方不透明表面和菜单间距。
- 插件：保留本地 `open-ai-canvas.plugin/v2`、`.canvas-plugin` 和品牌，融合官方协议修复并重新打包。AutoDL 以官方随 Git 提供的 2.4.0 插件包内声明为来源补齐本地源码；8 个更新插件的声明与文档已核对。
- 内置工作台工具：首次导入即启用，已有工具的发布状态保持原值，工作台总开关仍独立生效。
- 技能库开关：关闭独立技能库页面及其浏览、安装接口；Agent 继续使用已安装技能及其自动／手动设置，专用接口仍要求登录和资源归属。Agent 市场推荐不直接安装技能，安装仍须通过受开关约束的接口。
- 画布并发：采用官方目标值收敛规则，当前值已等于目标值时返回成功；旧值不匹配且目标不同仍拒绝。既有请求 ID、摘要和回执去重保留，旧画布字段转换后也遵循同一规则。
- 注册奖励：采用官方规则，只在新注册时发放；旧账号登录不再补发注册赠送，原奖励配置和已发记录保持原值。
- 工作台关闭：采用官方规则，图片／视频工作台历史仍可按当前用户读取；新建和重试继续受总开关约束，不能读取其他账号作品。
- 上述选择均已落实，初始冲突表保留原始快照，不作为实时完成清单。

## 第一批业务差异与选项（历史记录，以已确认决策为准）

### D-01 邀请返利

- 本地：六位邀请码、阶梯规则、版本化与个人规则、暂停与人工审核；使用本地推广权限和支付事务内返利链路。
- 官方：八位推广码、全员/指定推广人资格、固定比例佣金；使用独立绑定表与佣金表，在支付通知后发放并按订单去重。
- 影响：同名 `ReferralProfile`、service/repository/handler 文件均为双方新增冲突。直接并存可能产生双重返佣、两套邀请关系和不一致规则。
- 选项：保留本地体系；改用官方体系并迁移历史；保留本地体系并加入官方推广人资格。
- 建议：保留本地体系，排除官方重复返佣链路，逐一清理新增入口和迁移依赖。未决定前不解决相关冲突。

### D-02 用户分组与多货币

- 本地：既有积分计价、两位小数的价格输入、本地实测结算与上限保护、商品促销和订单快照。
- 官方：用户分组、渠道可见范围、分组倍率及模型级覆盖，人民币/美元价格换算积分，商品货币和账单汇率快照；部分价格输入改为六位小数。
- 影响：报价函数增加用户上下文，目录、准入、预留与结算必须使用一致分组；不能将不同单位的售价与成本直接相除。
- 选项：完整纳入分组与多货币并保留本地结算保护；只纳入分组与渠道权限；继续现有计费和渠道体系。
- 建议：完整纳入，既有账号默认组与既有积分价格保持原结果。精度和零价规则另按实际冲突逐项复核，不全局改为六位小数。

### D-03 Agent 记忆与技能

- 本地：记忆提案界面、既有记忆注入与统计、本地技能开关。
- 官方：用户记忆文档与修订、Agent 写入工具、自动发现与本轮固定技能、建 run 时冻结技能及记忆快照；删除部分旧提案界面与记忆测试。
- 影响：不仅是页面重排，涉及历史数据、工具注册、模型上下文和用户隔离。不能只解决前端删除冲突。
- 选项：采用新机制并保留/迁移旧记忆和本地开关；维持旧机制；新旧流程同时保留。
- 建议：采用新机制、保留本地开关和历史内容；先检查旧数据的归属和映射，再决定是否追加数据转换，不丢弃旧记忆。

## 初始逐模块复核范围（历史建议，以已确认决策为准）

| 编号 | 模块 | 实际差异与影响 | 建议方向 |
| --- | --- | --- | --- |
| D-04 | 皮肤、欢迎页与模型目录向导 | 本地删除，官方继续修改；Git 将官方文件留在候选树，不代表已恢复功能 | 沿用本地删除决定；如需恢复由用户另选，关联入口和测试同步处理 |
| D-05 | 品牌、主题、导航与后台布局 | 本地首页、Glow、账号与后台布局和功能开关，与官方电商入口、分组徽标及新版布局相交 | 保留本地视觉和导航结构，接入已选的新功能；核对明暗主题、手机和弹窗 |
| D-06 | 新增后台权限边界 | 自动合并的新 user_groups/ecommerce 等文件沿用官方 RequireAdmin；本地还区分模块权限和全权限管理员 | 必须映射本地权限；分组、知识与电商配置的具体归属在接入时交用户决定，不把角色检查当作完整授权 |
| D-07 | 支付、库存、促销与返利 | 本地支付目录返回促销与服务时间；官方返回按货币排序的商品；回调返利也受 D-01 影响 | 保留本地促销、库存与快照保护，按 D-01/D-02 接入货币与返利，不覆盖现有写路径 |
| D-08 | 注册、登录和奖励 | 官方登录不再补发注册奖励，新注册绑定官方推广关系；本地登录上下文和返利协议不同 | 注册绑定跟随 D-01；奖励默认值及旧账号补发行为单独确认，不覆盖现有配置 |
| D-09 | 数据库迁移 | 本地 Schema 96；官方从 71 追加到 79，含推广、短剧来源与引用、分组货币、记忆、角色声音造型、电商知识库 | 保留本地 1–96 历史，仅追加选定功能迁移；推广迁移随 D-01 调整，不能直接设置为官方 79 |
| D-10 | 协议插件 | 本地 .canvas-plugin 与官方 .yingce-plugin 包名冲突；官方插件版本及参数更新 | 保留本地包名和品牌，逐项纳入协议修复；重新生成包须与真实构建脚本一致 |
| D-11 | 画布、短剧、创作页与资源 | 官方新增作品载入、引用清理、短剧声音造型及新交互；本地有控件、页面、权限和资源保护 | 接入官方功能时保留本地配额、引用检查、计费和交互边界；每个行为冲突按实际代码再确认 |
| D-12 | 构建、模块路径和自有更新 | 新 Go 文件包含官方 yingce/backend 导入，而本地 go.mod 仍为 infinite-canvas/backend；质量流程有冲突 | 保留本地模块路径、自有正式 Release 更新源与独立 Agent，修复新增导入并同步依赖和质量流程 |

自动合并成功的新增文件同样需要业务审查。当前暂存的新文件只是 Git 合并结果，不代表已确认这些功能和权限可直接交付。数据库迁移编号和接口最终形态在用户决策落实后记录，不在此时提前固定。

## 当前实施进度

- 初始 158 个未合并索引路径已全部解决，整合后的源码没有冲突标记，暂存差异检查通过。最终提交保留本地起点与官方固定提交两个父提交。
- D-01：核心 model/repository/service/handler 保留本地实现；删除官方重复的推广页及 service 别名，清除支付通知、对账与 auth Host 的第二套返佣入口；保留本地支付事务内返利。
- D-02：分组模型、service、API、表单与管理页只保留分组信息及渠道范围；目录过滤、报价和账单准入保留渠道检查，移除分组倍率与货币换算；用户管理、登录态和分组接口已衔接。本地价格输入精度及报价结算上限保留。
- 支付页与商品多货币入口已排除，分组路由、用户编辑控件及新模块权限已接入；同步 OpenAPI、API 调用方和数据库、部署、代码地图等专题文档。OpenAPI 的 211 个路径均可解析，内部引用完整。
- D-03：后端技能池、run 记忆快照和工具注册采用官方机制；画布 Agent 面板、队列与设置已采用官方实现并适配本地头像合同。旧提案及旧图片规划确认测试不恢复。独立技能库关闭时，Agent 专用的已安装技能读取、模式修改和用户隔离检查已通过。
- D-04：皮肤、欢迎页和模型推荐向导关联的 11 个删除/修改冲突已维持本地删除，未恢复入口。
- 将 138 个文件中新引入的 Go 模块导入改为本地模块路径；没有修改 go.mod 的模块身份或现有部署。
- 冲突标记与解析检查已覆盖整合后的源码；早期冲突处理脚本留下的提交号行已清理。新增 Go 导入适配本地模块并按 gofmt 校正。
- Schema 在本地 96 后追加 97–103，保留既有账本身份；不采用官方重复返佣迁移，分组迁移只包含渠道权限，不含汇率和倍率。
- 自动合并复核发现短剧资源引用方法重复；保留官方完整引用遍历，补充配音样本引用保护，排除重复定义。后台侧栏也发现双实现残留，已按用户所选本地折叠方案收敛。
- 后端回归发现并修复短剧素材仍被引用时可误删、后台遗漏电商／短剧引用投影、迁移复制清单漏表，以及生成准入新增检查未使用个人配额的问题。测试夹具补齐本地版本化返利规则，未放宽生产返利或账本校验。

## 验证与后续步骤

- 已核对：官方标签对象及目标提交、完整非浅克隆历史、共同祖先、备份补丁、初始未合并索引和原工作树状态。
- 已操作：在独立工作树执行三方合并；两份原未提交文档补丁已带入候选；全部已发现冲突按用户决定处理，原目录与原补丁 SHA-256 未变。
- 已通过：Go model 包测试；workbench 和 app 包编译（`-run '^$'`，并非业务用例）；SQLite 新库迁移与重复执行；隔离 PostgreSQL 的新增迁移、重复执行与资产 ID 迁移。
- 已通过：从本地起点提交的旧版代码生成 Schema 96 临时 SQLite，再由候选代码升级到 103，连续执行三次，原 96 条账本（含校验值及应用时间）、测试积分余额和返利记录均不变。
- 前端已通过：引用／快捷命令等 19 项，新增模块权限 5 项，首页手机与桌面对话真实组件用例，以及官方 Agent 布局和跨设备接续相关 18 项（跨设备组件内部 15 项）。旧图片规划的 5 项测试随已批准的官方交互替换。
- 前端完整类型检查和生产构建通过；最终集中专项 91 项通过、0 失败，覆盖后台导航与权限、工作台去重／待确认请求／历史恢复／账号切换、引用快捷命令、首页和 Agent。工作台组件内部 19 项通过。`bun run lint` 通过。
- 插件包保留本地标识和后缀，8 个更新包已重打包；111 份包的 SHA-256 均与校验清单一致，声明式协议和内嵌文档合同测试通过。
- 文档已按专题同步；此工作树没有 `docs/package.json`，未执行文档站构建，不将链接或 YAML 静态检查记为文档站运行验证。
- 临时 PostgreSQL 也已完成由历史 Schema 96 升至 103 的演练与连续重放，测试容器已清理。
- 后端最终运行通过：隔离 Linux（Go 1.25.14、CGO、Node 22）执行 `go test -mod=readonly -json -p 1 -timeout=20m ./...`，45 个包通过、4,675 项检查（含子用例）通过、0 失败。早期失败及中止不计入此结果；Agent 使用新版独立 Node 依赖。
- 全量默认运行有 50 项条件用例跳过，包括额外 PostgreSQL 集成、旧 Schema 51/83 夹具、真实外部模型、媒体工具、浏览器及平台限定路径。Schema 96→103 的 SQLite/PostgreSQL 演练已另行执行，其余跳过项不宣称通过；未做真实模型付费生成、生产编排及浏览器页面验收。
- 专项回归覆盖本地返利与注册验证、默认组／指定组的目录与报价准入、独立管理员权限、工作台关闭后的历史归属与新建／重试拒绝、个人资源配额、短剧／电商资源引用、Agent 记忆和运行、插件协议与迁移账本。
- 本轮不推送、不发布、不部署、不启动预览、不升级或访问现有业务数据库。

## 初始冲突清单

状态：UU 为双方修改；AA 为双方新增；DU 为本地删除/官方修改；UD 为本地修改/官方删除。文本块为初始 diff3 标记数量，二进制或删除冲突可为 0。以下为首次合并快照，部分已经解决；最终取舍以已确认决策及实施记录为准。

### 返利体系（5 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-001 | AA | 1 | `backend/internal/app/referral.go` |
| C-002 | AA | 1 | `backend/internal/app/referral_test.go` |
| C-003 | AA | 1 | `backend/internal/handler/referral.go` |
| C-004 | AA | 1 | `backend/internal/model/models_referral.go` |
| C-005 | AA | 1 | `backend/internal/repository/referral.go` |

### 迁移与数据（3 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-006 | UU | 2 | `backend/cmd/migrate-sqlite-postgres/main.go` |
| C-007 | UU | 2 | `backend/internal/database/migrations.go` |
| C-008 | UU | 2 | `backend/internal/database/schema.go` |

### 插件协议与打包（15 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-009 | DU | 0 | `plugin-packages/autodl-comfyui.yingce-plugin` |
| C-010 | UU | 1 | `plugin-packages/build-packages.sh` |
| C-011 | DU | 0 | `plugin-packages/dashscope-wan3-video.yingce-plugin` |
| C-012 | UU | 1 | `plugin-packages/dashscope-wan3-video/docs/interface.md` |
| C-013 | UU | 2 | `plugin-packages/dashscope-wan3-video/manifest.json` |
| C-014 | DU | 0 | `plugin-packages/metaso-h3.yingce-plugin` |
| C-015 | UU | 1 | `plugin-packages/metaso-h3/manifest.json` |
| C-016 | DU | 0 | `plugin-packages/minimax-hailuo-video-v2.yingce-plugin` |
| C-017 | UU | 1 | `plugin-packages/minimax-hailuo-video-v2/docs/interface.md` |
| C-018 | UU | 2 | `plugin-packages/minimax-hailuo-video-v2/manifest.json` |
| C-019 | DU | 0 | `plugin-packages/newapi-video-generations-v1.yingce-plugin` |
| C-020 | UU | 1 | `plugin-packages/newapi-video-generations-v1/docs/interface.md` |
| C-021 | UU | 2 | `plugin-packages/newapi-video-generations-v1/manifest.json` |
| C-022 | DU | 0 | `plugin-packages/xai-video.yingce-plugin` |
| C-023 | UU | 1 | `plugin-packages/xai-video/manifest.json` |

### 外观与已删除页面（11 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-024 | DU | 0 | `backend/internal/app/appearance_button_fill_test.go` |
| C-025 | DU | 0 | `backend/internal/app/appearance_skins.go` |
| C-026 | DU | 0 | `docs/design/site-appearance-and-skins.mdx` |
| C-027 | UU | 8 | `web/src/components/canvas/agent-avatar-video.tsx` |
| C-028 | DU | 0 | `web/src/lib/skin-themes.ts` |
| C-029 | DU | 0 | `web/src/pages/admin/settings/components/skin-theme-editor.tsx` |
| C-030 | DU | 0 | `web/src/pages/welcome/index.tsx` |
| C-031 | DU | 0 | `web/src/pages/welcome/welcome.css` |
| C-032 | DU | 0 | `web/test/site-appearance-and-skins.test.ts` |
| C-033 | DU | 0 | `web/test/skin-button-fill.test.ts` |
| C-034 | DU | 0 | `web/test/welcome-contributors.test.ts` |

### Agent 与技能（18 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-035 | UU | 1 | `backend/internal/app/cloud_agent.go` |
| C-036 | UU | 1 | `backend/internal/app/cloud_agent_character_create.go` |
| C-037 | UU | 1 | `backend/internal/app/cloud_agent_contract_test.go` |
| C-038 | UU | 1 | `backend/internal/app/cloud_agent_lessons.go` |
| C-039 | UD | 0 | `backend/internal/app/cloud_agent_memory_candidates_test.go` |
| C-040 | UU | 1 | `backend/internal/app/cloud_agent_runtime.go` |
| C-041 | UU | 1 | `backend/internal/app/cloud_agent_tool_registry.go` |
| C-042 | UU | 1 | `backend/internal/app/skills_bridge.go` |
| C-043 | UU | 6 | `web/src/components/canvas/canvas-cloud-agent-panel-parts.tsx` |
| C-044 | UU | 18 | `web/src/components/canvas/canvas-cloud-agent-panel.tsx` |
| C-045 | UU | 2 | `web/src/components/canvas/canvas-cloud-agent-settings.tsx` |
| C-046 | UU | 1 | `web/src/components/canvas/canvas-cloud-agent.css` |
| C-047 | UU | 4 | `web/src/pages/admin/components/agent-lessons-panel.tsx` |
| C-048 | UD | 0 | `web/src/pages/settings/agent-memory-proposals.tsx` |
| C-049 | DU | 0 | `web/src/pages/skills/skill-install-modal.tsx` |
| C-050 | UU | 2 | `web/test/agent-interjection.test.ts` |
| C-051 | UU | 1 | `web/test/agent-panel-layout.test.ts` |
| C-052 | UU | 1 | `web/test/fixtures/cloud-agent-panel-sync.harness.tsx` |

### 画布与短剧（10 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-053 | UU | 1 | `web/src/components/canvas/batch-generation-settings-dialog.tsx` |
| C-054 | UU | 2 | `web/src/components/canvas/canvas-agent-welcome.tsx` |
| C-055 | UU | 3 | `web/src/components/canvas/canvas-file-upload.css` |
| C-056 | UU | 2 | `web/src/components/canvas/canvas-image-toolbar-tools.tsx` |
| C-057 | UU | 2 | `web/src/components/canvas/canvas-node-image-edit-dialog.tsx` |
| C-058 | UU | 2 | `web/src/components/canvas/canvas-node-status-content.tsx` |
| C-059 | UU | 2 | `web/src/components/canvas/canvas-node-text-edit-dialog.tsx` |
| C-060 | UU | 11 | `web/src/components/canvas/canvas-resource-mention-textarea.tsx` |
| C-061 | UU | 2 | `web/src/components/canvas/canvas-short-drama-entry.css` |
| C-062 | UU | 1 | `web/src/components/canvas/previs/canvas-previs-node-panel.tsx` |

### 后台管理与经营（29 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-063 | UU | 4 | `backend/internal/app/admin.go` |
| C-064 | UU | 1 | `backend/internal/app/admin_bulk_user_test.go` |
| C-065 | UU | 3 | `backend/internal/app/admin_users.go` |
| C-066 | UU | 1 | `backend/internal/app/analytics_build.go` |
| C-067 | UU | 1 | `backend/internal/app/analytics_finance.go` |
| C-068 | UU | 1 | `web/src/pages/admin/admin-route-pages.tsx` |
| C-069 | UU | 1 | `web/src/pages/admin/channels/channels-page.tsx` |
| C-070 | DU | 0 | `web/src/pages/admin/channels/provider-catalog-wizard.tsx` |
| C-071 | UU | 15 | `web/src/pages/admin/components/admin-shell.tsx` |
| C-072 | UU | 1 | `web/src/pages/admin/components/analytics-panel.tsx` |
| C-073 | UU | 3 | `web/src/pages/admin/components/api-log-detail-drawer.tsx` |
| C-074 | UU | 1 | `web/src/pages/admin/components/channel-model-cost-summary.tsx` |
| C-075 | UU | 1 | `web/src/pages/admin/components/channel-model-editor-form.ts` |
| C-076 | UU | 1 | `web/src/pages/admin/components/channel-model-manager.tsx` |
| C-077 | UU | 5 | `web/src/pages/admin/components/channel-model-price-tier-fields.tsx` |
| C-078 | UU | 11 | `web/src/pages/admin/components/credit-operations-panel.tsx` |
| C-079 | UU | 11 | `web/src/pages/admin/components/feature-availability-panel.tsx` |
| C-080 | DU | 0 | `web/src/pages/admin/components/redemption-codes-modals.tsx` |
| C-081 | UU | 5 | `web/src/pages/admin/feedbacks/feedbacks-page.tsx` |
| C-082 | UU | 1 | `web/src/pages/admin/lib/use-table-url-state.ts` |
| C-083 | UU | 14 | `web/src/pages/admin/logs/logs-page.tsx` |
| C-084 | UU | 12 | `web/src/pages/admin/payments/payments-page.tsx` |
| C-085 | UU | 1 | `web/src/pages/admin/settings/runtime-policy-settings-page.tsx` |
| C-086 | UU | 1 | `web/src/pages/admin/theme/admin-ant-theme.ts` |
| C-087 | UU | 1 | `web/src/pages/admin/theme/admin-chrome.css` |
| C-088 | UU | 12 | `web/src/pages/admin/users/users-columns.tsx` |
| C-089 | UU | 12 | `web/src/pages/admin/users/users-modals.tsx` |
| C-090 | UU | 10 | `web/src/pages/admin/users/users-panel.tsx` |
| C-091 | UU | 4 | `web/src/pages/admin/workbench-tools/workbench-tools-page.tsx` |

### 支付与计费（10 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-092 | UU | 1 | `backend/internal/app/finance.go` |
| C-093 | UU | 2 | `backend/internal/app/logical_model_quote.go` |
| C-094 | UU | 2 | `backend/internal/app/payment.go` |
| C-095 | UU | 3 | `backend/internal/app/payment_products.go` |
| C-096 | UU | 1 | `backend/internal/model/models_payment.go` |
| C-097 | UU | 1 | `backend/internal/repository/payment.go` |
| C-098 | UU | 1 | `web/src/components/layout/workspace-wallet-topup.tsx` |
| C-099 | UU | 1 | `web/src/components/payment/use-payment-order-flow.tsx` |
| C-100 | UU | 1 | `web/src/services/api/payments.ts` |
| C-101 | UU | 1 | `web/src/services/api/wallet.ts` |

### 账号与工作区（14 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-102 | UU | 1 | `backend/internal/auth/auth.go` |
| C-103 | UU | 2 | `backend/internal/auth/auth_registration_test.go` |
| C-104 | UU | 2 | `backend/internal/handler/auth.go` |
| C-105 | UU | 1 | `web/src/components/layout/app-providers.tsx` |
| C-106 | UU | 3 | `web/src/components/layout/workspace-account-card.tsx` |
| C-107 | UU | 2 | `web/src/components/layout/workspace-account-menu.tsx` |
| C-108 | UU | 2 | `web/src/components/layout/workspace-sidebar-footer.tsx` |
| C-109 | UU | 5 | `web/src/components/layout/workspace-sidebar-nav.tsx` |
| C-110 | UU | 4 | `web/src/components/layout/workspace-top-bar.tsx` |
| C-111 | UU | 2 | `web/src/pages/auth/register.tsx` |
| C-112 | UU | 1 | `web/src/pages/settings/feedback-inbox-pane.tsx` |
| C-113 | UU | 2 | `web/src/pages/settings/index.tsx` |
| C-114 | UU | 5 | `web/src/services/api/auth.ts` |
| C-115 | UU | 1 | `web/src/stores/use-user-store.ts` |

### 创作与工具（14 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-116 | UU | 1 | `backend/internal/app/creation_test.go` |
| C-117 | UU | 1 | `backend/internal/app/provider_task_cancellation.go` |
| C-118 | UU | 2 | `backend/internal/app/task_creation.go` |
| C-119 | UU | 1 | `backend/internal/app/task_output.go` |
| C-120 | UU | 2 | `backend/internal/app/workbench_bridge.go` |
| C-121 | UU | 1 | `backend/internal/tools/tools_integration_test.go` |
| C-122 | UU | 1 | `backend/internal/workbench/tasks.go` |
| C-123 | UU | 1 | `web/src/pages/create/creation-agent-entry.tsx` |
| C-124 | UU | 1 | `web/src/pages/create/creation-product.css` |
| C-125 | UU | 3 | `web/src/pages/create/creation-workspace-messages.tsx` |
| C-126 | UU | 2 | `web/src/pages/create/creation-workspace.tsx` |
| C-127 | UU | 4 | `web/src/pages/create/index.tsx` |
| C-128 | UU | 36 | `web/src/pages/workbench/image-workbench-page.tsx` |
| C-129 | UU | 10 | `web/src/pages/workbench/workbench.css` |

### 文档（7 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-130 | UU | 4 | `docs/content/docs/backend/backend-database.mdx` |
| C-131 | UU | 3 | `docs/content/docs/backend/code-map.mdx` |
| C-132 | UU | 2 | `docs/content/docs/backend/http-api.mdx` |
| C-133 | UU | 1 | `docs/content/docs/backend/meta.json` |
| C-134 | UU | 3 | `docs/content/docs/overview/features.mdx` |
| C-135 | UU | 2 | `docs/content/docs/progress/pending-test.mdx` |
| C-136 | UU | 1 | `docs/design/admin-ui-design-standard.md` |

### 接口与构建基础（22 个路径）

| 编号 | 状态 | 文本块 | 路径 |
| --- | --- | ---: | --- |
| C-137 | UU | 1 | `.github/workflows/quality.yml` |
| C-138 | UU | 1 | `.gitignore` |
| C-139 | UU | 2 | `README.md` |
| C-140 | UU | 1 | `backend/internal/app/error_codes.go` |
| C-141 | UU | 1 | `backend/internal/app/project_settings_test.go` |
| C-142 | UU | 1 | `backend/internal/app/service.go` |
| C-143 | UU | 1 | `backend/internal/handler/api.go` |
| C-144 | UU | 1 | `backend/internal/handler/model_catalog.go` |
| C-145 | UU | 27 | `backend/internal/handler/openapi.yaml` |
| C-146 | UU | 1 | `backend/internal/kernel/error_codes.go` |
| C-147 | UU | 1 | `backend/internal/model/models.go` |
| C-148 | UU | 1 | `backend/internal/model/models_channel.go` |
| C-149 | UU | 1 | `backend/internal/model/models_identity.go` |
| C-150 | UU | 2 | `backend/internal/service/aliases_types.go` |
| C-151 | UU | 1 | `web/src/constant/navigation-tools.ts` |
| C-152 | UU | 2 | `web/src/lib/app-theme.ts` |
| C-153 | UU | 5 | `web/src/router.tsx` |
| C-154 | UU | 1 | `web/src/services/api/workbench.ts` |
| C-155 | UU | 24 | `web/src/styles/globals.css` |
| C-156 | UU | 8 | `web/src/styles/shared/model-picker.css` |
| C-157 | UU | 1 | `web/test/admin-ui-regressions.test.ts` |
| C-158 | UU | 1 | `web/test/settings-preference-spacing.test.ts` |
