package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/database"
)

func TestCleanupPreviewCLI(t *testing.T) {
	if os.Getenv("CANVAS_CLEANUP_CLI_HELPER") == "1" {
		main()
		return
	}
	dir := t.TempDir()
	db, err := database.Open(database.Config{Driver: "sqlite", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	beforeDatabase, err := os.ReadFile(filepath.Join(dir, "open_ai_canvas.db"))
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestCleanupPreviewCLI$")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "CANVAS_DATABASE_DRIVER" && key != "DATABASE_URL" && key != "CANVAS_BACKEND_DATA_DIR" && key != "CANVAS_CLEANUP_CLI_HELPER" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "CANVAS_CLEANUP_CLI_HELPER=1", "CANVAS_DATABASE_DRIVER=sqlite", "DATABASE_URL=", "CANVAS_BACKEND_DATA_DIR="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), `"apply":false`) {
		t.Fatalf("preview failed: %v\n%s", err, out)
	}
	after, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := func(entries []os.DirEntry) []string {
		result := []string{}
		for _, entry := range entries {
			// SQLite may create WAL coordination files even for a read-only connection.
			if strings.HasSuffix(entry.Name(), "-wal") || strings.HasSuffix(entry.Name(), "-shm") {
				continue
			}
			result = append(result, entry.Name())
		}
		return result
	}
	if !reflect.DeepEqual(names(before), names(after)) {
		t.Fatalf("preview created files: before=%v after=%v", names(before), names(after))
	}
	afterDatabase, err := os.ReadFile(filepath.Join(dir, "open_ai_canvas.db"))
	if err != nil || !reflect.DeepEqual(beforeDatabase, afterDatabase) {
		t.Fatalf("preview modified database: %v", err)
	}
}
