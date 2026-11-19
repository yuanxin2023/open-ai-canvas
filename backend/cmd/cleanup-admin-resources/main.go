package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/repository"
)

func main() {
	apply := flag.Bool("apply", false, "执行清理；默认只输出预览")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatal("用法：cleanup-admin-resources [--apply]")
	}
	dir := strings.TrimSpace(os.Getenv("CANVAS_BACKEND_DATA_DIR"))
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dir == "" && dsn == "" {
		log.Fatal("必须显式指定 CANVAS_BACKEND_DATA_DIR 或 DATABASE_URL；拒绝使用默认数据库")
	}
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("CANVAS_DATABASE_DRIVER")))
	if driver == "" || driver == "sqlite" {
		if dsn == "" {
			absolute, err := filepath.Abs(filepath.Join(dir, "open_ai_canvas.db"))
			if err != nil {
				log.Fatal(err)
			}
			dsn = "file:" + filepath.ToSlash(absolute)
		}
		name, query, _ := strings.Cut(dsn, "?")
		params, err := url.ParseQuery(query)
		if err != nil {
			log.Fatal("SQLite DSN 参数无效")
		}
		params.Set("mode", "rw")
		if !*apply {
			params.Set("mode", "ro")
			params.Set("_query_only", "1")
			params.Del("_journal_mode")
		}
		dsn = name + "?" + params.Encode()
	}
	db, err := database.Open(database.Config{Driver: driver, DSN: dsn, DataDir: dir})
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	if !*apply {
		db = db.Begin(&sql.TxOptions{ReadOnly: true})
		if db.Error != nil {
			log.Fatal(db.Error)
		}
		defer db.Rollback()
	}
	if err := database.RequireSchemaVersion(db); err != nil {
		log.Fatal(err)
	}
	report, err := app.CleanupAdminResourceHistory(repository.New(db), *apply)
	raw, encodeErr := json.Marshal(report)
	if encodeErr != nil {
		log.Fatal(encodeErr)
	}
	fmt.Println(string(raw))
	if err != nil {
		log.Fatal(err)
	}
	if len(report.Skipped) > 0 {
		os.Exit(2)
	}
}
