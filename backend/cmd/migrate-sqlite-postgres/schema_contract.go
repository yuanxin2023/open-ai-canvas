package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/database"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func copySchemas(db *gorm.DB) ([]*schema.Schema, error) {
	entities := append(database.Models(), &database.MigrationLedgerRecord{})
	result := make([]*schema.Schema, 0, len(entities))
	for _, entity := range entities {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(entity); err != nil {
			return nil, err
		}
		result = append(result, statement.Schema)
	}
	return result, nil
}

func verifyTargetStructure(source, target *gorm.DB) error {
	entities, err := copySchemas(source)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		fields, err := persistedFields(source, entity)
		if err != nil {
			return err
		}
		if _, err := persistedFields(target, entity); err != nil {
			return err
		}
		columns, err := target.Migrator().ColumnTypes(entity.Table)
		if err != nil {
			return err
		}
		byName := map[string]gorm.ColumnType{}
		for _, column := range columns {
			byName[column.Name()] = column
		}
		for _, field := range fields {
			column := byName[field.DBName]
			expected := strings.ToLower(target.Dialector.DataTypeOf(field))
			base, size, _ := strings.Cut(expected, "(")
			if normalizedPGType(base) != normalizedPGType(column.DatabaseTypeName()) {
				return fmt.Errorf("目标 %s.%s 类型不匹配", entity.Table, field.DBName)
			}
			if size != "" {
				length, err := strconv.ParseInt(strings.TrimSuffix(size, ")"), 10, 64)
				actual, ok := column.Length()
				if err != nil || !ok || actual != length {
					return fmt.Errorf("目标 %s.%s 长度不匹配", entity.Table, field.DBName)
				}
			}
			if nullable, ok := column.Nullable(); !ok || nullable == (field.NotNull || field.PrimaryKey) {
				return fmt.Errorf("目标 %s.%s 空值约束不匹配", entity.Table, field.DBName)
			}
			if auto, ok := column.AutoIncrement(); field.AutoIncrement && (!ok || !auto) {
				return fmt.Errorf("目标 %s.%s 自动编号合同不匹配", entity.Table, field.DBName)
			}
			if !field.AutoIncrement {
				actual, exists := column.DefaultValue()
				expectedDefault := field.DefaultValue
				// GORM retains SQL quotes for serialized slices, while the
				// PostgreSQL column inspector returns the decoded text default.
				if field.Serializer != nil {
					expectedDefault = strings.Trim(expectedDefault, "'")
				}
				if field.HasDefaultValue != exists || (exists && actual != expectedDefault) {
					return fmt.Errorf("目标 %s.%s 默认值不匹配", entity.Table, field.DBName)
				}
			}
		}
		var primary string
		if err := target.Raw(`SELECT string_agg(a.attname, ',' ORDER BY k.ordinality)
 FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace
 CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY k(attnum, ordinality)
 JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.attnum
 WHERE n.nspname='public' AND t.relname=? AND c.contype='p'`, entity.Table).Scan(&primary).Error; err != nil {
			return err
		}
		var expectedKeys []string
		for _, field := range entity.PrimaryFields {
			expectedKeys = append(expectedKeys, field.DBName)
		}
		if primary != strings.Join(expectedKeys, ",") {
			return fmt.Errorf("目标 %s 主键不匹配", entity.Table)
		}
		for _, index := range entity.ParseIndexes() {
			var keys []string
			for _, option := range index.Fields {
				key := option.Expression
				if key == "" {
					key = option.DBName
				}
				if strings.EqualFold(option.Sort, "desc") {
					key += " DESC"
				}
				keys = append(keys, key)
			}
			if err := verifyPGIndex(target, entity.Table, index.Name, index.Class == "UNIQUE", strings.Join(keys, ","), index.Where); err != nil {
				return err
			}
		}
		for _, relation := range entity.Relationships.Relations {
			constraint := relation.ParseConstraint()
			if constraint == nil {
				continue
			}
			var foreign struct {
				ReferenceTable, Columns, ReferenceColumns, DeleteAction, UpdateAction string
				Valid                                                                 bool
			}
			if err := target.Raw(`SELECT rt.relname AS reference_table, c.convalidated AS valid,
 c.confdeltype::text AS delete_action, c.confupdtype::text AS update_action,
 (SELECT string_agg(a.attname, ',' ORDER BY k.ordinality) FROM unnest(c.conkey) WITH ORDINALITY k(num,ordinality) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.num) AS columns,
 (SELECT string_agg(a.attname, ',' ORDER BY k.ordinality) FROM unnest(c.confkey) WITH ORDINALITY k(num,ordinality) JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.num) AS reference_columns
 FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_class rt ON rt.oid=c.confrelid JOIN pg_namespace n ON n.oid=t.relnamespace
 WHERE n.nspname='public' AND t.relname=? AND c.conname=? AND c.contype='f'`, constraint.Schema.Table, constraint.Name).Scan(&foreign).Error; err != nil {
				return err
			}
			var local, remote []string
			for _, field := range constraint.ForeignKeys {
				local = append(local, field.DBName)
			}
			for _, field := range constraint.References {
				remote = append(remote, field.DBName)
			}
			if !foreign.Valid || foreign.ReferenceTable != constraint.ReferenceSchema.Table || foreign.Columns != strings.Join(local, ",") || foreign.ReferenceColumns != strings.Join(remote, ",") || foreign.DeleteAction != foreignAction(constraint.OnDelete) || foreign.UpdateAction != foreignAction(constraint.OnUpdate) {
				return fmt.Errorf("目标外键 %s 不匹配", constraint.Name)
			}
		}
	}
	for _, index := range []struct {
		table, name, columns, predicate string
		unique                          bool
	}{
		{"schema_migrations", "idx_schema_migrations_applied_at", "applied_at", "", false},
		{"users", "idx_users_username_ci", "lower(username)", "", true},
		{"users", "idx_users_email_nonempty", "lower(email)", "email <> ''", true},
		{"users", "idx_users_phone_nonempty", "phone", "phone <> ''", true},
		{"logical_models", "idx_logical_model_source_active", "source_channel_model_id", "source_channel_model_id <> '' AND archived_at IS NULL", true},
		{"project_asset_candidates", "idx_project_asset_candidates_pending_identity", "project_id, category, name_key", "status = 'pending_confirmation' AND name_key <> ''", true},
		{"cloud_agent_executions", "idx_cloud_agent_canvas_active", "user_id, id", "status NOT IN ('completed', 'failed', 'cancelled', 'rejected') OR cleanup_pending = true", false},
		{"tasks", "idx_tasks_canvas_agent_reservation", "user_id, project_id, created_at, id", "operation = 'cloud_agent' AND agent_execution_ready = false AND status NOT IN ('succeeded', 'failed', 'cancelled')", false},
	} {
		if err := verifyPGIndex(target, index.table, index.name, index.unique, index.columns, index.predicate); err != nil {
			return err
		}
	}
	return nil
}

func foreignAction(action string) string {
	switch strings.ToUpper(action) {
	case "CASCADE":
		return "c"
	case "RESTRICT":
		return "r"
	case "SET NULL":
		return "n"
	case "SET DEFAULT":
		return "d"
	default:
		return "a"
	}
}

func normalizedPGType(value string) string {
	switch strings.ToLower(value) {
	case "bigserial", "bigint", "int8":
		return "int8"
	case "serial", "integer", "int4":
		return "int4"
	case "smallserial", "smallint", "int2":
		return "int2"
	case "boolean", "bool":
		return "bool"
	case "character varying", "varchar":
		return "varchar"
	case "double precision", "float8":
		return "float8"
	case "real", "float4":
		return "float4"
	case "timestamp with time zone", "timestamptz":
		return "timestamptz"
	default:
		return strings.ToLower(value)
	}
}

func verifyPGIndex(db *gorm.DB, table, name string, unique bool, columns, predicate string) error {
	var actual struct {
		Unique, Valid, Ready, NullsNotDistinct bool
		Columns, Predicate, Method             string
	}
	if err := db.Raw(`SELECT i.indisunique AS unique, i.indisvalid AS valid, i.indisready AS ready, i.indnullsnotdistinct AS nulls_not_distinct,
 am.amname AS method,
 array_to_string(ARRAY(SELECT pg_get_indexdef(i.indexrelid,k,true)
 || CASE WHEN (i.indoption[k-1] & 1)=1 THEN ' DESC' ELSE '' END
 || CASE WHEN ((i.indoption[k-1] & 2)=2) <> ((i.indoption[k-1] & 1)=1)
 THEN CASE WHEN (i.indoption[k-1] & 2)=2 THEN ' NULLS FIRST' ELSE ' NULLS LAST' END ELSE '' END
 FROM generate_series(1,i.indnkeyatts) k), ',') AS columns,
 COALESCE(pg_get_expr(i.indpred,i.indrelid,true),'') AS predicate
 FROM pg_index i JOIN pg_class t ON t.oid=i.indrelid JOIN pg_namespace n ON n.oid=t.relnamespace JOIN pg_class x ON x.oid=i.indexrelid JOIN pg_am am ON am.oid=x.relam
 WHERE n.nspname='public' AND t.relname=? AND x.relname=?`, table, name).Scan(&actual).Error; err != nil {
		return err
	}
	if !actual.Valid || !actual.Ready || actual.Unique != unique || actual.NullsNotDistinct || actual.Method != "btree" || normalizedIndexSQL(actual.Columns) != normalizedIndexSQL(columns) || normalizedIndexSQL(actual.Predicate) != normalizedIndexSQL(predicate) {
		return fmt.Errorf("目标索引 %s 合同不匹配", name)
	}
	return nil
}

var sqlCast = regexp.MustCompile(`::(?:character varying|text|bigint|integer|boolean)(?:\[\])?`)

// Compare the limited expressions used by this schema, ignoring PostgreSQL's
// implicit casts and parenthesis formatting. Quoted values remain case-sensitive.
func normalizedIndexSQL(sql string) string {
	var code strings.Builder
	var literals []string
	for i := 0; i < len(sql); {
		if sql[i] == '\'' {
			start := i
			i++
			for i < len(sql) {
				if sql[i] == '\'' {
					i++
					if i < len(sql) && sql[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			literals = append(literals, sql[start:i])
			fmt.Fprintf(&code, "#%d#", len(literals))
		} else {
			code.WriteByte(sql[i])
			i++
		}
	}
	text := sqlCast.ReplaceAllString(strings.ToLower(code.String()), "")
	text = strings.Map(func(r rune) rune {
		if r == '(' || r == ')' || r == '[' || r == ']' || r == '"' || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, text)
	text = strings.ReplaceAll(text, "<>allarray", "notin")
	// Length prefixes keep quoted values unambiguous without parsing SQL content.
	var result strings.Builder
	for _, value := range literals {
		fmt.Fprintf(&result, "%d:%s", len(value), value)
	}
	return text + "|" + result.String()
}

func synchronizeNewSerialSequences(db *gorm.DB) error {
	entities, err := copySchemas(db)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		for _, field := range entity.PrimaryFields {
			if !field.AutoIncrement {
				continue
			}
			var sequence string
			if err := db.Raw("SELECT COALESCE(pg_get_serial_sequence(?, ?),'')", "public."+entity.Table, field.DBName).Scan(&sequence).Error; err != nil {
				return err
			}
			if sequence == "" {
				return errors.New("新目标自动编号序列缺失")
			}
			var maximum int64
			if err := db.Table(entity.Table).Select("COALESCE(MAX(" + quoteIdentifier(db, field.DBName) + "),0)").Scan(&maximum).Error; err != nil {
				return err
			}
			if maximum > 0 {
				if err := db.Exec("SELECT setval(?::regclass, ?, true)", sequence, maximum).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}
