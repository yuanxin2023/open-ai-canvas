# Schema 51 历史测试夹具

`schema51_export_test.go` 只用于固定旧源码快照，不放入当前 database 包编译。先将提交 `15861487c12780d08043ed7d7eca864deea73c53` 的 `backend/` 导出至隔离目录，再将本文件作为该快照的 `internal/database/merge_export_test.go` 测试文件挂载。导出器明确要求原 `CurrentSchemaVersion == 51`，不能用当前模型伪造旧结构。

在隔离 Linux / GCC / CGO 环境运行旧快照的 `go test ./internal/database -run '^TestExportOriginalSchema51$' -count=1`。`CANVAS_FIXTURE_OUTPUT` 必须为专用可写挂载 `/results`；生成文件仅为虚构用户和资金，不是生产副本。

PostgreSQL 测试主机固定为 `codex-canvas-merge-v1692-postgres:5432`，入口数据库为 `canvas_merge_fixture`，连接通过 `CANVAS_TEST_POSTGRES_DSN` 传入。仅在不暴露宿主端口、使用临时数据目录的独立测试实例上运行。导出器创建 `postgres_local_51` 和 `postgres_upstream_51` 两个测试数据库；SQLite 创建两个 `.db`。每套同时输出 20 张关键表的原始列和数据 JSON 清单。重复导出须换全新测试实例与空输出目录，不能覆盖真实数据。

之后把夹具目录只读挂载到候选测试环境，将 `CANVAS_TEST_MIGRATION_FIXTURES` 指向该挂载，设置同一隔离 PostgreSQL DSN。运行候选的 `go test ./internal/database -count=1`。历史测试复制 SQLite 文件，使用 PostgreSQL TEMPLATE 创建独立数据库副本；每套执行两遍 up / verify，按原始列精确比较，保留 1–51 的名称、校验和及执行时间。测试清理自己创建的 PostgreSQL 副本，不修改原夹具。

六类业务扩展可额外在 PostgreSQL 执行：设置 `CANVAS_TEST_MIGRATION_DRIVER=postgres`，运行 `go test ./internal/database -run '^(TestLedgerPreflight|TestPaymentConfirmationMigration|TestRefundRecoveryMigration|TestFeedbackMigration|TestNewPersonalStores)' -count=1`。它们只建立测试专用 schema 并在结束后清理。

未设置所需环境时，历史演练和既有 PostgreSQL 用例会明确跳过；跳过不能视为已通过。导出结果、数据库文件与运行日志均放 Git 忽略的本地缓存，不提交到仓库。完整业务副本仍需另行执行上线前验收。

Schema 83 转库验证复用这些原始 SQLite 夹具，测试中只升级临时复制文件；运行 go test ./cmd/migrate-sqlite-postgres ./internal/database -count=1，须设置同样的两个 CANVAS_TEST 环境变量。转库测试建立并清理独立 PostgreSQL 数据库，包含原账本、软删除/NULL/二进制、默认种子、已有目标强制只读核对、失败回滚及实际 CLI 编译执行。新建测试实例时先重建历史 PostgreSQL 模板，不能只挂载旧的 JSON 清单。
