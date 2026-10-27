# 官方 v1.6.9.2 迁移身份与候选映射

状态：**DB-01/02/03 已确认；保留两套历史 1–51，按最终表连续追加 52–83，极旧账本停止并报告。候选 database 包和四套原始历史夹具升级已通过隔离验证；未读取或升级真实业务数据库。**

最终实施映射见 [DB-02 最终追加迁移候选映射](upstream-v1.6.9.2-migrations-final.md)：26 项官方迁移为 52–77，6 项业务扩展为 78–83。本文余下 52–78 表格保留初次讨论记录，已被最终表取代，不能作为执行编号。

来源：

- 本地固定提交 `15861487c12780d08043ed7d7eca864deea73c53` 的迁移声明与 `upstreamFirstMigrationPlan`。
- 官方固定提交 `19eec9bc4e71cbb5941b9dc4139fd6487fc91015` 的迁移声明。
- 本地最大版本与声明条数均为 51；官方最大版本 60、声明 59 条，44 缺位。
- 按名称与校验和比对：32 项共享、19 项仅本地、27 项仅官方；没有发现同名但校验和不同的声明。
- 声明相同不保证迁移函数体或模型定义相同；已执行迁移不重复运行，新字段仍须确认由新增迁移覆盖。

## 方案 A：保留历史，追加缺失迁移

1. 保留本地已部署 1–51 的版本、名称和校验和。
2. 保留当前已支持的官方优先历史谱系：本地优先 24–27 对应官方优先 33–36；本地优先 28–36 对应官方优先 24–32；1–23 和 37–51 相同。
3. 将缺失官方迁移按原依赖顺序追加；下表假设完整接入 27 项，候选编号 52–78。产品功能取舍可能改变新增范围，实施前须重新确认最终映射。
4. 用显式旧版本映射替换依赖总长度的切片计算；追加新迁移不得改变原 24–51 映射。52 及以后在两套已有谱系中使用相同编号。
5. 不修改已生效账本；未知名称或校验和失败退出。纯官方较新数据库与当前已支持的官方优先谱系不是同一回事，不自动导入。
6. 外部服务、生产资源和真实支付不参与迁移；在隔离副本验证数据一致性与恢复。

用户已选择方案 A。方案 B 不采用，不设计或执行统一官方编号的账本转换。

## 官方全部迁移到候选版本的映射

“本地优先”“官方优先”是现有代码已声明的两套谱系；不是从真实数据库读取的记录。“候选”列仅描述方案 A，未写入源码或数据库。

| 官方版本 | 迁移名称 | 现有本地优先 | 现有官方优先 | 候选版本 | 状态 | 官方校验和 |
| ---: | --- | ---: | ---: | ---: | --- | --- |
| 1 | `baseline_gorm_schema` | 1 | 1 | 1 | 已有，同校验和 | `sha256:open-ai-canvas-schema-v1-20260830` |
| 2 | `schema_migrations_applied_at_index` | 2 | 2 | 2 | 已有，同校验和 | `sha256:schema-migrations-applied-at-index-v2-20260830` |
| 3 | `asset_taxonomy_candidate_identity` | 3 | 3 | 3 | 已有，同校验和 | `sha256:asset-taxonomy-candidate-identity-v3-20260831-r1` |
| 4 | `resource_upload_key` | 4 | 4 | 4 | 已有，同校验和 | `sha256:resource-upload-key-v4-20260901` |
| 5 | `payment_topup` | 5 | 5 | 5 | 已有，同校验和 | `sha256:payment-topup-v5-20260902` |
| 6 | `resource_playback_variant` | 6 | 6 | 6 | 已有，同校验和 | `sha256:resource-playback-v6-20260902` |
| 7 | `asset_library_folders` | 7 | 7 | 7 | 已有，同校验和 | `sha256:asset-library-folders-v6-20260902` |
| 8 | `logical_model_active_code` | 8 | 8 | 8 | 已有，同校验和 | `sha256:logical-model-active-code-v8-20260905` |
| 9 | `channel_presentation` | 9 | 9 | 9 | 已有，同校验和 | `sha256:channel-presentation-v9-20260908` |
| 10 | `creation_runtime` | 10 | 10 | 10 | 已有，同校验和 | `sha256:creation-runtime-v10-20260909` |
| 11 | `cloud_agent_runtime` | 11 | 11 | 11 | 已有，同校验和 | `sha256:cloud-agent-runtime-v11-20260912` |
| 12 | `agent_token_charge_limit` | 12 | 12 | 12 | 已有，同校验和 | `sha256:agent-token-charge-limit-v12-20260913` |
| 13 | `cloud_agent_canvas_mutation` | 13 | 13 | 13 | 已有，同校验和 | `sha256:cloud-agent-canvas-mutation-v13-20260913` |
| 14 | `cloud_agent_recovery_control` | 14 | 14 | 14 | 已有，同校验和 | `sha256:cloud-agent-recovery-control-v14` |
| 15 | `agent_profiles` | 15 | 15 | 15 | 已有，同校验和 | `sha256:agent-profiles-v15-20260914` |
| 16 | `agent_lessons` | 16 | 16 | 16 | 已有，同校验和 | `sha256:agent-lessons-v16-20260917` |
| 17 | `agent_lessons_owner_index` | 17 | 17 | 17 | 已有，同校验和 | `sha256:agent-lessons-owner-index-v17-20260917` |
| 18 | `agent_memory_settings` | 18 | 18 | 18 | 已有，同校验和 | `sha256:agent-memory-settings-v18-20260917` |
| 19 | `payment_plugin_version` | 19 | 19 | 19 | 已有，同校验和 | `sha256:payment-plugin-version-v19-20260917` |
| 20 | `banner_announcements` | 20 | 20 | 20 | 已有，同校验和 | `sha256:banner-announcements-v20-20260917` |
| 21 | `banner_announcement_title_runs` | 21 | 21 | 21 | 已有，同校验和 | `sha256:banner-announcement-title-runs-v21-20260917` |
| 22 | `banner_announcement_notice_type` | 22 | 22 | 22 | 已有，同校验和 | `sha256:banner-announcement-notice-type-v22-20260917` |
| 23 | `canvas_revision_history` | 23 | 23 | 23 | 已有，同校验和 | `sha256:canvas-revision-history-v23-20260918` |
| 24 | `channel_model_label` | 28 | 24 | 28 | 已有，同校验和 | `sha256:channel-model-label-v24` |
| 25 | `video_token_formula_snapshot` | 29 | 25 | 29 | 已有，同校验和 | `sha256:video-token-formula-snapshot-v25` |
| 26 | `channel_model_description` | 30 | 26 | 30 | 已有，同校验和 | `sha256:channel-model-description-v26` |
| 27 | `channel_credit_cost` | 31 | 27 | 31 | 已有，同校验和 | `sha256:channel-credit-cost-v27` |
| 28 | `agent_execution_journal` | 32 | 28 | 32 | 已有，同校验和 | `sha256:agent-execution-journal-v28` |
| 29 | `agent_resource_leases` | 33 | 29 | 33 | 已有，同校验和 | `sha256:agent-resource-leases-v29-20260919` |
| 30 | `builtin_tools` | 34 | 30 | 34 | 已有，同校验和 | `sha256:builtin-tools-v30` |
| 31 | `tool_favorites` | 35 | 31 | 35 | 已有，同校验和 | `sha256:tool-favorites-v31` |
| 32 | `channel_model_tags` | 36 | 32 | 36 | 已有，同校验和 | `sha256:channel-model-tags-v32` |
| 33 | `oauth_state_accepted_terms` | — | — | 52 | 新增，待确认 | `sha256:oauth-state-accepted-terms-v33` |
| 34 | `task_media_recovery` | — | — | 53 | 新增，待确认 | `sha256:task-media-recovery-v34` |
| 35 | `auth_notifications` | — | — | 54 | 新增，待确认 | `sha256:auth-notifications-v35-20260924` |
| 36 | `cloud_agent_gemini_cache` | — | — | 55 | 新增，待确认 | `sha256:cloud-agent-gemini-cache-v36-20260924` |
| 37 | `cloud_agent_gemini_cache_identity` | — | — | 56 | 新增，待确认 | `sha256:cloud-agent-gemini-cache-identity-v37-20260925` |
| 38 | `prefixed_id_sequence_reconcile` | — | — | 57 | 新增，待确认 | `sha256:prefixed-id-sequence-reconcile-v38-20260926` |
| 39 | `skill_library_categories` | — | — | 58 | 新增，待确认 | `sha256:skill-library-categories-v39-20260926` |
| 40 | `builtin_skill_tombstones` | — | — | 59 | CAN-05 已确认采用，最终编号待统一 | `sha256:builtin-skill-tombstones-v40-20260927` |
| 41 | `resource_thumbnail` | — | — | 60 | 新增，待确认 | `sha256:resource-thumbnail-v41-20260927` |
| 42 | `cloud_agent_pi_sessions` | — | — | 61 | 新增，待确认 | `sha256:cloud-agent-pi-sessions-v42-20260928` |
| 43 | `topup_sale_strategies` | — | — | 62 | PAY-01 已确认采用；编号待总体定稿 | `sha256:topup-sale-strategies-v43-20260929` |
| 45 | `upload_reservations` | — | — | 63 | 新增，待确认 | `sha256:upload-reservations-v45-20261005` |
| 46 | `skill_curation` | — | — | 64 | 新增，待确认 | `sha256:skill-curation-v46-20261005` |
| 47 | `skill_curation_roots` | — | — | 65 | 新增，待确认 | `sha256:skill-curation-roots-v47-20261005` |
| 48 | `channel_proxy_url` | — | — | 66 | 新增，待确认 | `sha256:channel-proxy-url-v48-20261009` |
| 49 | `banner_sort_order` | — | — | 67 | 新增，待确认 | `sha256:banner-sort-order-v49-20261010` |
| 50 | `canvas_mutation_receipts` | — | — | 68 | 新增，待确认 | `sha256:canvas-mutation-receipts-v50-20261011` |
| 51 | `personal_profile_security` | — | — | 69 | 新增，待确认 | `sha256:personal-profile-security-v51-20261012` |
| 52 | `user_avatar_presets` | — | — | 70 | 新增，待确认 | `sha256:user-avatar-presets-v52-20261012` |
| 53 | `user_login_logs` | — | — | 71 | 新增，待确认 | `sha256:user-login-logs-v53` |
| 54 | `workbench_tools_and_presets` | — | — | 72 | F-02 已确认采用；最终编号/顺序待总表确认 | `sha256:workbench-tools-presets-v54-20261014` |
| 55 | `provider_catalog_source` | — | — | 原预编号 73 | PLG-01/F-04 已确认排除；总表定稿时处理空位 | `sha256:provider-catalog-source-v55` |
| 56 | `sql_perf_indexes` | — | — | 74 | 新增，待确认 | `sha256:sql-perf-indexes-v56-20261016` |
| 57 | `canvas_multi_session_sync` | — | — | 75 | 新增，待确认 | `sha256:canvas-multi-session-sync-v57` |
| 58 | `agent_memory_proposals` | — | — | 76 | 新增，待确认 | `sha256:agent-memory-proposals-v58` |
| 59 | `channel_model_protocol_length` | — | — | 77 | 新增，待确认 | `sha256:channel-model-protocol-length-v59` |
| 60 | `user_feedbacks` | — | — | 78 | 新增，待确认 | `sha256:user-feedbacks-v60-20261020` |

## 官方没有的 19 项本地迁移

这些迁移不因接入官方而删除或重编号。

| 本地优先 | 官方优先 | 名称 | 本地校验和 |
| ---: | ---: | --- | --- |
| 24 | 33 | `topup_product_benefits` | `sha256:topup-product-benefits-v16-20260916` |
| 25 | 34 | `featured_inspirations` | `sha256:featured-inspirations-v25-20260920` |
| 26 | 35 | `user_prompt_library` | `sha256:user-prompt-library-v26-20260920` |
| 27 | 36 | `inspiration_cover_dimensions` | `sha256:inspiration-cover-dimensions-v27-20260921` |
| 37 | 37 | `user_profiles` | `sha256:user-profiles-v37-20260927` |
| 38 | 38 | `user_login_names` | `sha256:user-login-names-v38-20260927` |
| 39 | 39 | `user_login_environment` | `sha256:user-login-environment-v39-20260930` |
| 40 | 40 | `platform_skill_availability` | `sha256:platform-skill-availability-v40-20260930` |
| 41 | 41 | `short_login_usernames` | `sha256:short-login-usernames-v41-20261001` |
| 42 | 42 | `user_admin_remarks` | `sha256:user-admin-remarks-v42-20261002` |
| 43 | 43 | `topup_product_card_fields` | `sha256:topup-product-card-fields-v43-20261002` |
| 44 | 44 | `topup_product_accent_color` | `sha256:topup-product-accent-color-v44-20261002` |
| 45 | 45 | `scoped_admin_permissions` | `sha256:scoped-admin-permissions-v45-20261002` |
| 46 | 46 | `payment_promotion_image_drafts` | `sha256:payment-promotion-image-drafts-v46-20261003` |
| 47 | 47 | `scoped_redeem_funding` | `sha256:scoped-redeem-funding-v47-20261003` |
| 48 | 48 | `redeem_batch_lifecycle` | `sha256:redeem-batch-lifecycle-v48-20261005` |
| 49 | 49 | `referral_rewards` | `sha256:referral-rewards-v49-20261006` |
| 50 | 50 | `six_character_referral_codes` | `sha256:six-character-referral-codes-v50-20261006` |
| 51 | 51 | `user_resource_quotas` | `sha256:user-resource-quotas-v51` |

## 需要随功能取舍确认的新增迁移

| 功能议题 | 相关官方迁移 | 候选版本 |
| --- | --- | --- |
| F-07 资料与认证 | auth_notifications、personal_profile_security、user_avatar_presets | 54、69、70 |
| F-05 技能分类/策展 | skill_library_categories、skill_curation、skill_curation_roots | 58、64、65 |
| F-10 商品售卖策略 | topup_sale_strategies | 62 |
| F-08 登录日志 | user_login_logs | 71 |
| F-02 工作台工具目录（118 工具/354 预设，新增首次停用） | workbench_tools_and_presets | 原预编号 72，最终映射待确认 |
| F-04 供应商目录 | provider_catalog_source | 73 |
| F-13 多窗口实时交互 | canvas_multi_session_sync | 75 |
| F-14 Agent 记忆与修订 | agent_memory_proposals | 76 |
| F-09 用户反馈 | user_feedbacks | 78 |

部分表结构可能被其他能力引用，排除入口不等于可以排除其迁移。决定删除迁移前需核对模型、schema bootstrap、查询路径、启动任务与编译依赖。

## 验证门槛

- 两套已有历史谱系、全新库、本地 Schema 51 均能升级；原有记录的编号、名称、校验和不变。
- 重复执行幂等，verify 成功，未知账本被明确拒绝。
- SQLite 与 PostgreSQL 均有实际数据库测试；Redis 相关集成检查不跳过。
- 升级前后权限、用户配额、可用/冻结积分、兑换码、返利、提示词与资源引用一致。
- 官方新模型字段、索引和约束的实际结构核对通过；不只检查 schema 版本号。
- 最终版本号与映射需先获得讨论结论。当前不满足执行迁移条件。

## 决策记录

历史映射采用固定版本转换，24–27 对应 33–36，28–36 对应 24–32，其他版本保持不变，再按版本排序。独立版本映射测试的 3 组、30 个子场景通过；完整计划与数据库集成测试仍待合并依赖解决后执行。

| 议题 | 推荐 | 用户结论 | 实施状态 |
| --- | --- | --- | --- |
| DB-01 历史保留与追加路线 | 方案 A | 已确认保留历史并追加迁移 | 历史映射已修正，新增声明待范围确认 |
| 两套现有谱系的保留 | 显式固定旧映射 | 随方案 A 确认 | 版本映射独立专项测试通过，数据库集成待验证 |
| 27 项新增迁移的采用范围 | 随相关功能决定并核对依赖 | 待讨论 | 未实施 |
| 最终 Schema 版本 | 官方追加项完整接入时到 78；还需合并扩展，不直接采用官方 60 | 待讨论 | 未实施 |

## 已确认功能带来的追加扩展

F-03b 已确认统一服务端个人库，新增候选表 `user_inspiration_libraries`（迁移候选名 `user_inspiration_library_state`）：主键为用户 ID，字段为 revision、状态 JSON 及创建/更新时间；主要正文和封面复用既有 `user_prompts`。状态保存目录 ID、个人副本关联、显式导入映射及双语/来源补充信息，不自动导入全目录。原个人库和大厅写入共同推进账号版本，账号注销清理新增状态。两个新功能开关仅扩展既有功能配置 JSON，默认关闭，不另建列。此项须进入最终追加映射，编号、顺序、校验和仍待总表确认；尚未编写迁移声明或执行升级，不能用旧库直接启动候选。

CAN-02/F-14a 已确认采用 Agent 记忆提案与修订，`agent_memory_proposals` 纳入采用范围（原预编号 76），仍须与其他迁移统一编号。CAN-03 步骤撤销所需快照/回执与保存版本结构需联合核对，不以实时提示默认关闭为由排除保存依赖；当前未新增或执行迁移。

CAN-04/F-05a 已确认采用个人分类、平台分类与策展，`skill_library_categories`、`skill_curation`、`skill_curation_roots` 纳入采用范围（原预编号 58、64、65）。新建策展设置默认关闭，不重置既有状态；旧身份与安装回填仍随 CAN-05 的内容/映射决定统一审查。当前未新增或执行这些迁移。

CAN-05/06 已确认采用新增技能内容与预设，同时采用 `builtin_skill_tombstones`（原预编号 59）保护已删除技能不被启动重新创建。37 项首次停用复用已有可用性表，不重写历史授权；启动目录/版本/默认状态事务需要先完成缺失表的追加迁移。若副本发现旧官方图片身份记录，停止自动转换并提交映射讨论。当前未编码或执行本批迁移。

PAY-01 至 PAY-06 要求在官方商品策略之外增加 `payment_orders.sale_strategy_snapshot`、`provider_closed_at`、`inventory_oversold`，分别固化下单时策略、区分站内关闭和渠道确认、保留超售异常。旧本地商品及订单默认为不限量，旧关单不自动推定渠道已确认。

PAY-12/13 另增加 `billing_orders.refund_recovery_pending`（非空，默认 false），明确退款恢复待核对时原预授权已返还，防止二次释放。已有退款日期和历史欠款不批量改写。

PLG-01/F-04 已排除远程推荐目录及其来源字段，官方缺失迁移的采用上限已从最初 27 项降为 26 项；其余功能还在逐项决定。原预编号 73 不静默分给另一迁移，追加映射、最终版本和测试期待须整体确认后统一编码。当前冲突中的官方 55 声明及其历史测试仍待处理，不满足执行迁移条件。

这些字段不能改写已经部署的迁移或直接改变官方迁移的原校验和；需与账号审计/验证票据等合并扩展一并制定追加声明。若官方缺失项完整采用 52–78，扩展最早从 79 编号；最终版本仍待范围和依赖统一确认。当前只修改候选模型，未编写或执行支付扩展迁移，不能启动候选访问旧库。

UI-02a 已确认增加本地扩展表 `workbench_submissions`，候选迁移名 `workbench_submission_receipts`。依赖既有任务/账单表及工作台任务分类，复合主键 `(user_id, request_key)` 保证同账号请求不重复创建任务和积分预占。字段为请求摘要、类型、原任务 ID、不可变报价 JSON 和首次时间；没有原请求正文、密钥或签名地址。任务删除保留回执，完整账号注销按原规则清理。此项须加入最终追加映射；编号、顺序与校验和统一确认后再编码，不能修改任一历史声明，也不能将登记 `Models()` 视为已完成历史升级。尚未执行迁移或副本演练。

## F-09 反馈扩展（已确认，编号待总表）

用户已确认采用官方 `user_feedbacks`。本轮扩展新增 `user_feedbacks.version`（非空，默认 1）和 `feedback_resource_references`（复合主键 feedback_id/resource_id、user_id 索引；资源外键 RESTRICT，反馈外键 CASCADE），候选迁移名 `feedback_versions_and_resource_references`。提交以账号和 requestId 派生确定 ID，复用反馈主键去重，不另建请求正文表。已有反馈版本回填 1；附件 JSON 需要按原 ID/URL 识别资源、核实反馈用户归属并回填关系及 resourceId，归档反馈同样保留引用。缺失、跨账号或无法识别的附件不得静默删除，停止该副本升级并提交讨论。历史原迁移名称/编号/校验和不改，扩展顺序须在反馈表与资源表之后；最终编号、校验和、升级/verify/幂等与回填演练尚未完成。功能开关只扩展既有 JSON，默认 false。未执行迁移或真实数据改写。
