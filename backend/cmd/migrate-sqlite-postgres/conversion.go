package main

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"infinite-canvas/backend/internal/database"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type copyResult struct {
	Copied bool
	Tables int
	Rows   int
}

func runConversion(sourcePath, targetDSN string) (copyResult, error) {
	var result copyResult
	dsn, err := readOnlySnapshotDSN(sourcePath)
	if err != nil {
		return result, err
	}
	before, err := fileDigest(sourcePath)
	if err != nil {
		return result, errors.New("无法读取 SQLite 副本")
	}
	source, err := database.Open(database.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		return result, errors.New("无法打开只读 SQLite 副本")
	}
	sourceSQL, err := source.DB()
	if err != nil {
		return result, err
	}
	defer sourceSQL.Close()
	source = source.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if err := database.RequireSchemaVersion(source); err != nil {
		return result, fmt.Errorf("源副本必须先升级并 verify 到 Schema %d：%w", database.CurrentSchemaVersion, err)
	}
	target, err := database.Open(database.Config{Driver: "postgres", DSN: targetDSN})
	if err != nil {
		return result, errors.New("无法连接 PostgreSQL 目标，请检查连接配置")
	}
	targetSQL, err := target.DB()
	if err != nil {
		return result, err
	}
	defer targetSQL.Close()
	return convertSnapshot(source, target, func() error {
		after, err := fileDigest(sourcePath)
		if err != nil || after != before {
			return errors.New("SQLite 副本在转换期间变化，目标事务已停止")
		}
		_, err = readOnlySnapshotDSN(sourcePath)
		return err
	})
}

func readOnlySnapshotDSN(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("SQLite 副本路径无效")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("SQLite 源必须是已存在的独立副本文件")
	}
	// immutable avoids journal/SHM writes. It is safe only for an offline,
	// checkpointed snapshot, so never ignore an existing WAL or hot journal.
	for _, suffix := range []string{"-wal", "-journal"} {
		info, err := os.Stat(absolute + suffix)
		if err == nil && info.Size() > 0 {
			return "", errors.New("SQLite 副本仍有 WAL 或 journal，请先生成完整离线副本并检查点")
		}
		if err != nil && !os.IsNotExist(err) {
			return "", errors.New("无法检查 SQLite 副本的日志文件")
		}
	}
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath}
	query := url.Values{"mode": {"ro"}, "immutable": {"1"}, "_query_only": {"1"}, "_foreign_keys": {"on"}, "_busy_timeout": {"5000"}}
	uri.RawQuery = query.Encode()
	return uri.String(), nil
}

func fileDigest(path string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	f, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return digest, err
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func convertSnapshot(source, target *gorm.DB, verifyFile func() error) (copyResult, error) {
	var result copyResult
	if source.Dialector.Name() != "sqlite" || target.Dialector.Name() != "postgres" {
		return result, errors.New("只支持已验证 SQLite 副本到 PostgreSQL 的转换")
	}
	source = source.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	target = target.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	err := target.Transaction(func(targetTx *gorm.DB) error {
		return source.Transaction(func(sourceTx *gorm.DB) error {
			if err := verifySQLite(sourceTx); err != nil {
				return err
			}
			if err := database.RequireSchemaVersion(sourceTx); err != nil {
				return fmt.Errorf("源副本必须先验证到 Schema %d：%w", database.CurrentSchemaVersion, err)
			}
			if err := verifyMigrationCoverage(sourceTx); err != nil {
				return err
			}
			if err := validateSourceTables(sourceTx); err != nil {
				return err
			}
			if err := database.LockSchemaForCopy(targetTx); err != nil {
				return err
			}
			tables, err := targetTableNames(targetTx)
			if err != nil {
				return err
			}
			result.Copied = len(tables) == 0
			if result.Copied {
				if err := database.InitializeSchemaForCopy(targetTx); err != nil {
					return fmt.Errorf("创建空目标转库结构：%w", err)
				}
			} else {
				if err := validateTargetTableNames(tables); err != nil {
					return err
				}
				if err := database.RequireSchemaVersion(targetTx); err != nil {
					return fmt.Errorf("非空目标账本不匹配，拒绝补写：%w", err)
				}
			}
			if err := verifyTargetStructure(sourceTx, targetTx); err != nil {
				return err
			}
			for _, item := range migrations() {
				count, err := item.run(sourceTx, targetTx, result.Copied)
				if err != nil {
					return fmt.Errorf("转库表 %s 核对失败：%w", item.name, safeConversionError(err))
				}
				result.Tables++
				result.Rows += count
			}
			if err := database.RequireSchemaVersion(targetTx); err != nil {
				return err
			}
			if result.Copied {
				if err := synchronizeNewSerialSequences(targetTx); err != nil {
					return err
				}
			}
			if verifyFile != nil {
				return verifyFile()
			}
			return nil
		}, &sql.TxOptions{ReadOnly: true})
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return copyResult{}, safeConversionError(err)
	}
	return result, nil
}

func verifySQLite(db *gorm.DB) error {
	var result string
	if err := db.Raw("PRAGMA quick_check").Scan(&result).Error; err != nil {
		return err
	}
	if result != "ok" {
		return errors.New("SQLite quick_check 未通过")
	}
	rows, err := db.Raw("PRAGMA foreign_key_check").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return errors.New("SQLite 副本存在无效外键，停止转库")
	}
	return rows.Err()
}

func validateSourceTables(db *gorm.DB) error {
	var extra int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type IN ('view','trigger') AND name NOT LIKE 'sqlite_%'").Scan(&extra).Error; err != nil {
		return err
	}
	if extra != 0 {
		return errors.New("源副本存在额外视图或触发器，停止转换并核对结构")
	}
	names, err := db.Migrator().GetTables()
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if !strings.HasPrefix(name, "sqlite_") {
			filtered = append(filtered, name)
		}
	}
	return validateTargetTableNames(filtered)
}

func targetTableNames(db *gorm.DB) ([]string, error) {
	var schema string
	if err := db.Raw("SELECT current_schema()").Scan(&schema).Error; err != nil {
		return nil, err
	}
	if schema != "public" {
		return nil, errors.New("转库目标必须使用独立数据库的 public schema")
	}
	var foreignObjects int64
	if err := db.Raw(`SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
 AND (n.nspname <> 'public' OR c.relkind IN ('v', 'm', 'f'))`).Scan(&foreignObjects).Error; err != nil {
		return nil, err
	}
	if foreignObjects != 0 {
		return nil, errors.New("目标存在额外 schema、视图或外部表，拒绝转库")
	}
	var extra int64
	if err := db.Raw(`SELECT COUNT(*) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT t.tgisinternal`).Scan(&extra).Error; err != nil {
		return nil, err
	}
	if extra != 0 {
		return nil, errors.New("目标存在额外触发器，拒绝转换或补写")
	}
	if err := db.Raw(`SELECT COUNT(*) FROM pg_class s JOIN pg_namespace n ON n.oid=s.relnamespace
 WHERE n.nspname='public' AND s.relkind='S' AND NOT EXISTS (
 SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=s.oid AND d.deptype IN ('a','i'))`).Scan(&extra).Error; err != nil {
		return nil, err
	}
	if extra != 0 {
		return nil, errors.New("目标存在未归属模型列的序列，拒绝转换或补写")
	}
	var names []string
	if err := db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename").Scan(&names).Error; err != nil {
		return nil, err
	}
	if len(names) == 0 {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'").Scan(&count).Error; err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, errors.New("空目标存在预先创建的对象，拒绝初始化")
		}
	}
	return names, nil
}

func validateTargetTableNames(actual []string) error {
	expected := map[string]bool{}
	for _, item := range migrations() {
		expected[item.name] = true
	}
	actualSet := map[string]bool{}
	for _, name := range actual {
		actualSet[name] = true
	}
	var missing, extra []string
	for name := range expected {
		if !actualSet[name] {
			missing = append(missing, name)
		}
	}
	for name := range actualSet {
		if !expected[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing)+len(extra) != 0 {
		return fmt.Errorf("数据库表清单不匹配，缺少 %s，额外 %s；拒绝覆盖或补写", strings.Join(missing, ","), strings.Join(extra, ","))
	}
	return nil
}

func safeConversionError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		return fmt.Errorf("数据库约束或结构检查失败（SQLSTATE %s），转库事务回滚", postgresError.Code)
	}
	return err
}
