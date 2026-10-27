# DB-02 最终追加迁移候选映射

状态：DB-02、DB-03 已由用户确认并实现。隔离 SQLite/PostgreSQL 及四套原始 Schema 51 夹具升级通过；用户后续明确指定本机现有业务 SQLite，已完成完整备份、副本升级/重复校验、现有库升级至 83 及 22 张业务表核对。未操作生产库或执行真实转库。固定本地起点为 `15861487c12780d08043ed7d7eca864deea73c53`，官方内容固定为 `19eec9bc4e71cbb5941b9dc4139fd6487fc91015`；最新证据见[候选验收记录](upstream-v1.6.9.2-candidate-validation.md)。

已确认保留两套已支持历史的 1–51，追加 26 项已确认功能所需的官方迁移至 52–77，再追加 6 项合并业务扩展至 78–83，最终候选 Schema 为 83。两套历史在 52 及以后使用同一顺序和签名；不是改用官方编号 60，也不把历史账本转换成另一套编号。

原讨论预编号 73 对应的 provider_catalog_source 已按 PLG-01/F-04 排除。本次已确认压缩未部署的新编号：原预编号 74–78 分别成为 73–77，扩展从 78 开始。没有任何已生效记录改号；此前预编号仅是文档草案。

## 已部署 1–51：按原谱系保留

下表由原迁移映射文档的 32 项共享声明及 19 项本地声明汇总。名称和 checksum 原样保留，数字仅用于选择该数据库原本所属的计划，不做账本 UPDATE。当前“官方优先谱系”特指本地代码已支持的 1–51 历史排列，不等于任意纯官方数据库。

| 本地优先号 | 官方优先号 | 名称 | 原 checksum |
| ---: | ---: | --- | --- |
| 1 | 1 | `baseline_gorm_schema` | `sha256:open-ai-canvas-schema-v1-20260830` |
| 2 | 2 | `schema_migrations_applied_at_index` | `sha256:schema-migrations-applied-at-index-v2-20260830` |
| 3 | 3 | `asset_taxonomy_candidate_identity` | `sha256:asset-taxonomy-candidate-identity-v3-20260831-r1` |
| 4 | 4 | `resource_upload_key` | `sha256:resource-upload-key-v4-20260901` |
| 5 | 5 | `payment_topup` | `sha256:payment-topup-v5-20260902` |
| 6 | 6 | `resource_playback_variant` | `sha256:resource-playback-v6-20260902` |
| 7 | 7 | `asset_library_folders` | `sha256:asset-library-folders-v6-20260902` |
| 8 | 8 | `logical_model_active_code` | `sha256:logical-model-active-code-v8-20260905` |
| 9 | 9 | `channel_presentation` | `sha256:channel-presentation-v9-20260908` |
| 10 | 10 | `creation_runtime` | `sha256:creation-runtime-v10-20260909` |
| 11 | 11 | `cloud_agent_runtime` | `sha256:cloud-agent-runtime-v11-20260912` |
| 12 | 12 | `agent_token_charge_limit` | `sha256:agent-token-charge-limit-v12-20260913` |
| 13 | 13 | `cloud_agent_canvas_mutation` | `sha256:cloud-agent-canvas-mutation-v13-20260913` |
| 14 | 14 | `cloud_agent_recovery_control` | `sha256:cloud-agent-recovery-control-v14` |
| 15 | 15 | `agent_profiles` | `sha256:agent-profiles-v15-20260914` |
| 16 | 16 | `agent_lessons` | `sha256:agent-lessons-v16-20260917` |
| 17 | 17 | `agent_lessons_owner_index` | `sha256:agent-lessons-owner-index-v17-20260917` |
| 18 | 18 | `agent_memory_settings` | `sha256:agent-memory-settings-v18-20260917` |
| 19 | 19 | `payment_plugin_version` | `sha256:payment-plugin-version-v19-20260917` |
| 20 | 20 | `banner_announcements` | `sha256:banner-announcements-v20-20260917` |
| 21 | 21 | `banner_announcement_title_runs` | `sha256:banner-announcement-title-runs-v21-20260917` |
| 22 | 22 | `banner_announcement_notice_type` | `sha256:banner-announcement-notice-type-v22-20260917` |
| 23 | 23 | `canvas_revision_history` | `sha256:canvas-revision-history-v23-20260918` |
| 24 | 33 | `topup_product_benefits` | `sha256:topup-product-benefits-v16-20260916` |
| 25 | 34 | `featured_inspirations` | `sha256:featured-inspirations-v25-20260920` |
| 26 | 35 | `user_prompt_library` | `sha256:user-prompt-library-v26-20260920` |
| 27 | 36 | `inspiration_cover_dimensions` | `sha256:inspiration-cover-dimensions-v27-20260921` |
| 28 | 24 | `channel_model_label` | `sha256:channel-model-label-v24` |
| 29 | 25 | `video_token_formula_snapshot` | `sha256:video-token-formula-snapshot-v25` |
| 30 | 26 | `channel_model_description` | `sha256:channel-model-description-v26` |
| 31 | 27 | `channel_credit_cost` | `sha256:channel-credit-cost-v27` |
| 32 | 28 | `agent_execution_journal` | `sha256:agent-execution-journal-v28` |
| 33 | 29 | `agent_resource_leases` | `sha256:agent-resource-leases-v29-20260919` |
| 34 | 30 | `builtin_tools` | `sha256:builtin-tools-v30` |
| 35 | 31 | `tool_favorites` | `sha256:tool-favorites-v31` |
| 36 | 32 | `channel_model_tags` | `sha256:channel-model-tags-v32` |
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

## 26 项官方追加：52–77

采用对应功能已在 ACC/PAY/PLG/CAN/UI/F 批次决定；这里确认的是迁移次序和编号。沿用官方名称及签名，已部署身份不变。即便部分 AutoMigrate 因当前模型提前建立了相同字段，追加扩展仍需显式声明并验证，不修改历史 checksum 来替代。

| 新号 | 官方号 | 原预编号 | 名称 | checksum |
| ---: | ---: | ---: | --- | --- |
| 52 | 33 | 52 | `oauth_state_accepted_terms` | `sha256:oauth-state-accepted-terms-v33` |
| 53 | 34 | 53 | `task_media_recovery` | `sha256:task-media-recovery-v34` |
| 54 | 35 | 54 | `auth_notifications` | `sha256:auth-notifications-v35-20260924` |
| 55 | 36 | 55 | `cloud_agent_gemini_cache` | `sha256:cloud-agent-gemini-cache-v36-20260924` |
| 56 | 37 | 56 | `cloud_agent_gemini_cache_identity` | `sha256:cloud-agent-gemini-cache-identity-v37-20260925` |
| 57 | 38 | 57 | `prefixed_id_sequence_reconcile` | `sha256:prefixed-id-sequence-reconcile-v38-20260926` |
| 58 | 39 | 58 | `skill_library_categories` | `sha256:skill-library-categories-v39-20260926` |
| 59 | 40 | 59 | `builtin_skill_tombstones` | `sha256:builtin-skill-tombstones-v40-20260927` |
| 60 | 41 | 60 | `resource_thumbnail` | `sha256:resource-thumbnail-v41-20260927` |
| 61 | 42 | 61 | `cloud_agent_pi_sessions` | `sha256:cloud-agent-pi-sessions-v42-20260928` |
| 62 | 43 | 62 | `topup_sale_strategies` | `sha256:topup-sale-strategies-v43-20260929` |
| 63 | 45 | 63 | `upload_reservations` | `sha256:upload-reservations-v45-20261005` |
| 64 | 46 | 64 | `skill_curation` | `sha256:skill-curation-v46-20261005` |
| 65 | 47 | 65 | `skill_curation_roots` | `sha256:skill-curation-roots-v47-20261005` |
| 66 | 48 | 66 | `channel_proxy_url` | `sha256:channel-proxy-url-v48-20261009` |
| 67 | 49 | 67 | `banner_sort_order` | `sha256:banner-sort-order-v49-20261010` |
| 68 | 50 | 68 | `canvas_mutation_receipts` | `sha256:canvas-mutation-receipts-v50-20261011` |
| 69 | 51 | 69 | `personal_profile_security` | `sha256:personal-profile-security-v51-20261012` |
| 70 | 52 | 70 | `user_avatar_presets` | `sha256:user-avatar-presets-v52-20261012` |
| 71 | 53 | 71 | `user_login_logs` | `sha256:user-login-logs-v53` |
| 72 | 54 | 72 | `workbench_tools_and_presets` | `sha256:workbench-tools-presets-v54-20261014` |
| 73 | 56 | 74 | `sql_perf_indexes` | `sha256:sql-perf-indexes-v56-20261016` |
| 74 | 57 | 75 | `canvas_multi_session_sync` | `sha256:canvas-multi-session-sync-v57` |
| 75 | 58 | 76 | `agent_memory_proposals` | `sha256:agent-memory-proposals-v58` |
| 76 | 59 | 77 | `channel_model_protocol_length` | `sha256:channel-model-protocol-length-v59` |
| 77 | 60 | 78 | `user_feedbacks` | `sha256:user-feedbacks-v60-20261020` |

## 6 项合并扩展：78–83

| 新号 | 名称 | 候选 checksum |
| ---: | --- | --- |
| 78 | `merged_login_environment_contract` | `sha256:merged-login-environment-contract-v1` |
| 79 | `payment_order_channel_confirmation` | `sha256:payment-order-channel-confirmation-v1` |
| 80 | `billing_refund_recovery_state` | `sha256:billing-refund-recovery-state-v1` |
| 81 | `workbench_submission_receipts` | `sha256:workbench-submission-receipts-v1` |
| 82 | `user_inspiration_library_state` | `sha256:user-inspiration-library-state-v1` |
| 83 | `feedback_versions_and_resource_references` | `sha256:feedback-versions-resource-references-v1` |

### 78：merged_login_environment_contract

login_logs 增加 DeviceType、Browser、BrowserVersion、OS、OSVersion；UserAgent 扩至 1024。

保留 user_login_events 历史，由现有统一查询读取，不批量复制或删旧表；旧新日志的新增环境字段可为空。

### 79：payment_order_channel_confirmation

payment_orders 增加 sale_strategy_snapshot、provider_closed_at、inventory_oversold。

已支持的本地历史订单按原不限量行为处理；不推定旧关单已获渠道确认，不重算金额/积分。若副本发现无法确定策略的旧库存订单，停止并报告，不能从当前商品猜历史策略。

### 80：billing_refund_recovery_state

billing_orders 增加 refund_recovery_pending，非空，默认 false。

不重算历史已扣、退款或欠款；新恢复待核对标志防止重复释放已退还授权。

### 81：workbench_submission_receipts

新建 workbench_submissions；主键 user_id/request_key，保存请求摘要、任务 ID、原报价和创建时间。

不根据旧任务猜请求身份，不存原请求正文或密钥，不批量改旧任务/账单；新提交事务去重。

### 82：user_inspiration_library_state

新建 user_inspiration_libraries；用户主键、revision、状态 JSON 及创建/更新时间。

既有 user_prompts ID、正文、封面不变；不自动导入全部目录或浏览器数据，首次写状态时初始化版本。

### 83：feedback_versions_and_resource_references

user_feedbacks.version 非空默认 1；新建 feedback_resource_references，资源 RESTRICT、反馈 CASCADE。

按附件 ID/可识别原资源路径核实用户归属、回填 resourceId 和引用；归档反馈同样保留引用。缺失、跨账号、不一致或无法识别时停止整步，不清空附件；重跑不重置已有版本或重复插入关系。

## 执行和重复能力边界

1. 在迁移事务和数据库锁内先核对已有全部账本身份与所选历史，不先改号、改签名或掩盖未知项；未知记录明确拒绝。全新库、本地优先及既有官方优先 1–51 分别测试；纯官方较新账本不自动导入。
2. 先按表内顺序应用 52–77，再按 78–83；即使 AutoMigrate 已创建过相同列/索引，也要显式核对字段、约束和签名。`sql_perf_indexes` 已覆盖 ApiCallLog/Task 的 epoch 及 CreditLedgerEntry 新索引；协议长度、缩略图、OAuth/认证/短信表和 Agent 结构分别由相应官方追加项覆盖，不再无理由重复加迁移。
3. 旧登录记录保留表和身份，由统一查询展示；不复制成双份。旧套餐保持不限量；不可把本地关闭推定为渠道已确认，不回算资金。既有用户、管理员权限、用户配额、余额/冻结、兑换码、返利、个人提示词和资源引用须升级前后核对。
4. 反馈引用回填必须同时验证同用户资源；无法可靠转换时整个事务停止，不以跳过异常数据换取通过。关系主键去重，版本初始值不覆盖已修改记录；幂等须通过重复运行验证。
5. SQLite 与 PostgreSQL 分别验证；仅在副本或隔离夹具执行 up/verify。未知 checksum、失败注入、跨用户附件、外键拒绝、重复执行和升级回滚都需覆盖。没有真实副本依据的项目保留待验收。

## 模型覆盖核对依据

已对当前注册 GORM 模型分别与本地起点和官方目标做 Go AST 字段/类型/ORM 标签比较，没有发现当前注册模型删除本地原有持久字段。两者之外的新增持久化合同集中在本表 6 项：登录环境、支付渠道确认、退款恢复状态、工作台回执、服务端灵感状态及反馈版本/引用；远程目录来源字段已排除。该静态核对不替代数据库结构及数据演练，也不证明 AutoMigrate 会执行正确的数据回填。

## DB-03：极旧账本停止并报告

原本地代码会将精确识别的充值权益旧记录从 16 自动改为 24；官方还会把极旧工作台记录从 51 改为 54 并换校验和。用户已确认本轮均不自动修复：检测到时停止并报告，基于副本另议。当前实现不包含这两个自动改号函数，也不导入纯官方较新数据库。

保留原有 6/7 文件夹与播放变体的已知历史识别，仅选用对应迁移身份，不写回旧账本。两套 1–51 正常追加使用相同 52–83 尾部。

## 实现与验证记录

- 主声明保持原 1–51，26 项官方追加单独放在 migrations_upstream_v1692.go，6 项合并扩展放在 migrations_merged_business.go；CurrentSchemaVersion 为 83。
- 升级和 verify 均先检查全部账本身份及连续前缀；未知名称、未知校验和、缺号、超出版本和两种极旧记录明确拒绝。PostgreSQL 使用事务级迁移锁。
- Models 总表补齐原已部署的 AdminPermissionGrant 和官方五张策展表，防止跨库清单遗漏。未采用的 Canvas/CanvasNode/Approval 等孤立脚手架不据此增加新表。
- Linux Go 1.25.14 / GCC / CGO 下，database 全包 86 个顶层用例、140 项含子用例检查通过，配置 PostgreSQL 17 后无跳过。业务扩展另在 PostgreSQL 通过 6 个顶层用例、17 项含子用例检查。
- 从固定原提交导出源码，由原 Schema 51 代码生成 SQLite / PostgreSQL × 本地 / 官方优先四套夹具。每套复制后两遍 up / verify，比较 20 张关键表的原始列及所有夹具行，保留账本执行时间与本地业务数据。
- 专项覆盖异常账本写入前拒绝、资金快照不重算、库存策略不明拒绝、退款历史保留、账号请求键唯一性、归档反馈引用、外键删除保护、附件未知字段/整数精度保留，以及缺失/越权/冲突附件整批回滚。
- 历史夹具导出器和复现约定位于 backend/internal/database/testdata/；实际数据库文件、旧源码快照和 JSONL 日志仅在忽略的本地缓存。它们是虚构业务夹具，不是生产副本。

真实业务数据库副本升级、完整业务事务/权限测试、迁移可执行文件、全应用构建和浏览器验收仍待完成。不能用本批夹具结果代替生产数据演练，也未启动候选应用。
