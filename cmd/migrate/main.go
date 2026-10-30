package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/platform/db"
)

func main() {
	action := "up"
	if len(os.Args) > 1 {
		action = strings.ToLower(os.Args[1])
	}

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer database.Close()

	if err := ensureMigrationsTable(ctx, database); err != nil {
		log.Fatalf("ensure schema_migrations table: %v", err)
	}

	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		if _, err2 := os.Stat("backend/migrations"); err2 == nil {
			migrationsDir = "backend/migrations"
		}
	}

	switch action {
	case "up":
		if err := migrateUp(ctx, database, migrationsDir); err != nil {
			log.Fatalf("migrate up failed: %v", err)
		}
	case "down":
		if err := migrateDown(ctx, database, migrationsDir); err != nil {
			log.Fatalf("migrate down failed: %v", err)
		}
	case "status":
		if err := migrateStatus(ctx, database, migrationsDir); err != nil {
			log.Fatalf("migrate status failed: %v", err)
		}
	default:
		log.Fatalf("unknown command %q (expected: up, down, status)", action)
	}
}

func ensureMigrationsTable(ctx context.Context, database *db.DB) error {
	const q = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`
	_, err := database.Pool.Exec(ctx, q)
	return err
}

func appliedVersions(ctx context.Context, database *db.DB) (map[string]time.Time, error) {
	rows, err := database.Pool.Query(ctx, `SELECT version, applied_at FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]time.Time)
	for rows.Next() {
		var v string
		var t time.Time
		if err := rows.Scan(&v, &t); err != nil {
			return nil, err
		}
		res[v] = t
	}
	return res, rows.Err()
}

func migrateUp(ctx context.Context, database *db.DB, dir string) error {
	applied, err := appliedVersions(ctx, database)
	if err != nil {
		return err
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(matches)

	appliedCount := 0
	for _, path := range matches {
		base := filepath.Base(path)
		version := strings.TrimSuffix(base, ".up.sql")
		if _, ok := applied[version]; ok {
			continue
		}

		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		if err := database.WithinTx(ctx, func(txCtx context.Context) error {
			if _, err := database.Conn(txCtx).Exec(txCtx, string(sqlBytes)); err != nil {
				return fmt.Errorf("exec %s: %w", base, err)
			}
			if _, err := database.Conn(txCtx).Exec(txCtx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
				return fmt.Errorf("record %s: %w", version, err)
			}
			return nil
		}); err != nil {
			return err
		}

		fmt.Printf("[MIGRATE UP] applied %s\n", version)
		appliedCount++
	}

	if appliedCount == 0 {
		fmt.Println("[MIGRATE UP] database is already up to date")
	}
	return nil
}

func migrateDown(ctx context.Context, database *db.DB, dir string) error {
	var lastVersion string
	err := database.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&lastVersion)
	if err != nil {
		fmt.Println("[MIGRATE DOWN] no applied migrations to roll back")
		return nil
	}

	downPath := filepath.Join(dir, lastVersion+".down.sql")
	sqlBytes, err := os.ReadFile(downPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", downPath, err)
	}

	if err := database.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := database.Conn(txCtx).Exec(txCtx, string(sqlBytes)); err != nil {
			return fmt.Errorf("exec %s: %w", filepath.Base(downPath), err)
		}
		if _, err := database.Conn(txCtx).Exec(txCtx, `DELETE FROM schema_migrations WHERE version = $1`, lastVersion); err != nil {
			return fmt.Errorf("remove %s: %w", lastVersion, err)
		}
		return nil
	}); err != nil {
		return err
	}

	fmt.Printf("[MIGRATE DOWN] rolled back %s\n", lastVersion)
	return nil
}

func migrateStatus(ctx context.Context, database *db.DB, dir string) error {
	applied, err := appliedVersions(ctx, database)
	if err != nil {
		return err
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(matches)

	fmt.Printf("%-35s | %-10s | %s\n", "MIGRATION VERSION", "STATUS", "APPLIED AT")
	fmt.Println(strings.Repeat("-", 75))
	for _, path := range matches {
		version := strings.TrimSuffix(filepath.Base(path), ".up.sql")
		if t, ok := applied[version]; ok {
			fmt.Printf("%-35s | %-10s | %s\n", version, "APPLIED", t.Format(time.RFC3339))
		} else {
			fmt.Printf("%-35s | %-10s | -\n", version, "PENDING")
		}
	}
	return nil
}
