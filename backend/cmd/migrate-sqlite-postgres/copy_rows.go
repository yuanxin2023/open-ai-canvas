package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func migrateTable[T any](name string) tableMigration {
	return tableMigration{name: name, run: func(source, target *gorm.DB, copyRows bool) (int, error) {
		statement := &gorm.Statement{DB: source}
		if err := statement.Parse(new(T)); err != nil {
			return 0, err
		}
		if statement.Schema.Table != name {
			return 0, errors.New("转库模型与表名不一致")
		}
		fields, err := persistedFields(source, statement.Schema)
		if err != nil {
			return 0, err
		}
		if _, err := persistedFields(target, statement.Schema); err != nil {
			return 0, err
		}
		sourceHash, targetHash := sha256.New(), sha256.New()
		batch := make([]map[string]any, 0, 100)
		flush := func() error {
			if len(batch) == 0 {
				return nil
			}
			// Map insertion bypasses model defaults, timestamps, hooks and
			// associations. SQL NULL, zero and soft-deleted rows stay explicit.
			if err := target.Session(&gorm.Session{SkipHooks: true}).Table(name).CreateInBatches(batch, 100).Error; err != nil {
				return err
			}
			batch = make([]map[string]any, 0, 100)
			return nil
		}
		sourceCount, err := scanPersistedRows(source, statement.Schema, fields, func(values []any) error {
			if err := hashPersistedRow(sourceHash, values); err != nil {
				return err
			}
			if !copyRows {
				return nil
			}
			row := make(map[string]any, len(fields))
			for i, field := range fields {
				row[field.DBName] = values[i]
			}
			batch = append(batch, row)
			if len(batch) == cap(batch) {
				return flush()
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
		if copyRows {
			if err := flush(); err != nil {
				return 0, err
			}
		}
		targetCount, err := scanPersistedRows(target, statement.Schema, fields, func(values []any) error { return hashPersistedRow(targetHash, values) })
		if err != nil {
			return 0, err
		}
		if sourceCount != targetCount || !reflect.DeepEqual(sourceHash.Sum(nil), targetHash.Sum(nil)) {
			return 0, errors.New("源快照与目标数据逐字段核对不一致，拒绝覆盖或补写")
		}
		return sourceCount, nil
	}}
}

func persistedFields(db *gorm.DB, entity *schema.Schema) ([]*schema.Field, error) {
	if len(entity.PrimaryFields) == 0 {
		return nil, fmt.Errorf("表 %s 缺少主键", entity.Table)
	}
	fields := make([]*schema.Field, 0, len(entity.DBNames))
	expected := map[string]bool{}
	for _, name := range entity.DBNames {
		field := entity.FieldsByDBName[name]
		if field == nil || field.IgnoreMigration {
			continue
		}
		expected[name] = true
		fields = append(fields, field)
	}
	columns, err := db.Migrator().ColumnTypes(entity.Table)
	if err != nil {
		return nil, err
	}
	actual := map[string]bool{}
	for _, column := range columns {
		if !expected[column.Name()] {
			return nil, fmt.Errorf("表 %s 存在未知持久化列，拒绝丢弃数据", entity.Table)
		}
		actual[column.Name()] = true
	}
	for name := range expected {
		if !actual[name] {
			return nil, fmt.Errorf("表 %s 缺少列 %s，不能按当前模型猜默认值", entity.Table, name)
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].DBName < fields[j].DBName })
	return fields, nil
}

func scanPersistedRows(db *gorm.DB, entity *schema.Schema, fields []*schema.Field, consume func([]any) error) (int, error) {
	columns := make([]clause.Column, len(fields))
	for i, field := range fields {
		columns[i] = clause.Column{Name: field.DBName}
	}
	var ordering []string
	for _, field := range entity.PrimaryFields {
		column := quoteIdentifier(db, field.DBName)
		if field.GORMDataType == schema.String {
			if db.Dialector.Name() == "postgres" {
				column += ` COLLATE "C"`
			} else {
				column += " COLLATE BINARY"
			}
		}
		ordering = append(ordering, column)
	}
	rows, err := db.Unscoped().Table(entity.Table).Clauses(clause.Select{Columns: columns}).Order(strings.Join(ordering, ",")).Rows()
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		values := make([]any, len(fields))
		pointers := make([]any, len(fields))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return count, err
		}
		for i, field := range fields {
			value, err := normalizedColumnValue(field, values[i])
			if err != nil {
				return count, fmt.Errorf("表 %s 第 %d 行的列 %s 无法无损复制：%w", entity.Table, count+1, field.DBName, err)
			}
			values[i] = value
		}
		if err := consume(values); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func normalizedColumnValue(field *schema.Field, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	if field.Serializer != nil || field.GORMDataType == schema.String {
		var text string
		switch item := value.(type) {
		case string:
			text = item
		case []byte:
			text = string(item)
		default:
			return nil, errors.New("文本值类型不匹配")
		}
		if !utf8.ValidString(text) {
			return nil, errors.New("文本不是有效 UTF-8")
		}
		if limit, exists := field.TagSettings["SIZE"]; exists && !strings.EqualFold(field.TagSettings["TYPE"], "text") {
			maximum, err := strconv.Atoi(limit)
			if err != nil {
				return nil, errors.New("字段长度合同无效")
			}
			if utf8.RuneCountInString(text) > maximum {
				return nil, errors.New("文本超出目标字段长度")
			}
		}
		return text, nil
	}
	switch field.GORMDataType {
	case schema.Bool:
		switch item := value.(type) {
		case bool:
			return item, nil
		case int64:
			if item == 0 || item == 1 {
				return item == 1, nil
			}
		}
	case schema.Int, schema.Uint:
		if item, ok := value.(int64); ok {
			return item, nil
		}
	case schema.Float:
		switch item := value.(type) {
		case float64:
			if !math.IsNaN(item) && !math.IsInf(item, 0) {
				return item, nil
			}
		case int64:
			return float64(item), nil
		}
	case schema.Time:
		if item, ok := value.(time.Time); ok {
			return item.UTC().Truncate(time.Microsecond), nil
		}
	case schema.Bytes:
		if item, ok := value.([]byte); ok {
			return append([]byte{}, item...), nil
		}
	}
	return nil, errors.New("值类型与持久化合同不一致")
}

func hashPersistedRow(digest hash.Hash, values []any) error {
	encoded, err := json.Marshal(values)
	if err != nil {
		return errors.New("无法构建逐字段核对摘要")
	}
	if _, err := digest.Write(encoded); err != nil {
		return err
	}
	_, err = digest.Write([]byte{'\n'})
	return err
}

func quoteIdentifier(db *gorm.DB, name string) string {
	statement := &gorm.Statement{DB: db}
	statement.WriteQuoted(clause.Column{Name: name})
	return statement.SQL.String()
}
