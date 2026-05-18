package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Migrate(database *sql.DB, migrationsDir string) {
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version    INT NOT NULL,
		applied_at BIGINT NOT NULL,
		PRIMARY KEY (version)
	)`); err != nil {
		panic(fmt.Sprintf("ensure schema_version: %v", err))
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		panic(fmt.Sprintf("read migrations dir: %v", err))
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for i, f := range files {
		version := i + 1

		var count int
		if err := database.QueryRow(`SELECT COUNT(*) FROM schema_version WHERE version = ?`, version).Scan(&count); err != nil {
			panic(fmt.Sprintf("check version %d: %v", version, err))
		}
		if count > 0 {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			panic(fmt.Sprintf("read %s: %v", f, err))
		}

		for _, stmt := range splitSQL(string(sqlBytes)) {
			if _, err := database.Exec(stmt); err != nil {
				panic(fmt.Sprintf("apply %s statement: %v\nSQL: %s", f, err, stmt))
			}
		}

		if _, err := database.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (?, ?)`,
			version, time.Now().Unix()); err != nil {
			panic(fmt.Sprintf("record version %d: %v", version, err))
		}
		fmt.Printf("applied migration %s\n", f)
	}
}

func splitSQL(content string) []string {
	var stmts []string
	for _, s := range strings.Split(content, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}
