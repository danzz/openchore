package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	dbpkg "github.com/liftedkilt/openchore/internal/db"
	_ "modernc.org/sqlite"
)

func main() {
	dbpkg.LoadEnv("")

	sqlitePath := os.Getenv("SQLITE_DB_PATH")
	if sqlitePath == "" {
		sqlitePath = os.Getenv("DB_PATH")
	}
	if sqlitePath == "" {
		sqlitePath = "openchore.db"
	}

	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		log.Fatalf("source SQLite database %q does not exist", sqlitePath)
	}

	log.Printf("Starting migration from SQLite (%s) to PostgreSQL...", sqlitePath)

	// Open source SQLite DB
	sqliteDB, err := sql.Open("sqlite", sqlitePath+"?_foreign_keys=off&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("failed to open sqlite: %v", err)
	}
	defer sqliteDB.Close()

	// Resolve PostgreSQL config and open target DB
	os.Setenv("DB_TYPE", "postgres")
	pgDB, dialect, err := dbpkg.OpenAndMigrate(context.Background())
	if err != nil {
		log.Fatalf("failed to connect/migrate target postgres db: %v", err)
	}
	defer pgDB.Close()

	log.Printf("Connected to target PostgreSQL database (dialect: %s)", dialect)

	// List of tables in dependency order
	tables := []struct {
		name       string
		hasID      bool
		conflictOn string // e.g. "id" or "key" or "household_id, key"
	}{
		{name: "accounts", hasID: true, conflictOn: "id"},
		{name: "households", hasID: true, conflictOn: "id"},
		{name: "household_devices", hasID: true, conflictOn: "id"},
		{name: "household_settings", hasID: false, conflictOn: "household_id, key"},
		{name: "app_settings", hasID: false, conflictOn: "key"},
		{name: "users", hasID: true, conflictOn: "id"},
		{name: "chores", hasID: true, conflictOn: "id"},
		{name: "rewards", hasID: true, conflictOn: "id"},
		{name: "reward_redemptions", hasID: true, conflictOn: "id"},
		{name: "reward_assignments", hasID: true, conflictOn: "id"},
		{name: "user_streaks", hasID: false, conflictOn: "user_id"},
		{name: "webhooks", hasID: true, conflictOn: "id"},
		{name: "webhook_deliveries", hasID: true, conflictOn: "id"},
		{name: "user_decay_config", hasID: false, conflictOn: "user_id"},
		{name: "chore_triggers", hasID: true, conflictOn: "id"},
		{name: "api_tokens", hasID: true, conflictOn: "id"},
		{name: "chore_schedules", hasID: true, conflictOn: "id"},
		{name: "shared_commitment_pools", hasID: true, conflictOn: "id"},
		{name: "reward_commitments", hasID: true, conflictOn: "id"},
		{name: "point_transactions", hasID: true, conflictOn: "id"},
		{name: "user_identities", hasID: true, conflictOn: "id"},
		{name: "chore_completions", hasID: true, conflictOn: "id"},
		{name: "weekly_summaries", hasID: false, conflictOn: "user_id, week_start"},
		{name: "oidc_providers", hasID: false, conflictOn: "id"},
		{name: "streak_rewards", hasID: true, conflictOn: "id"},
	}

	ctx := context.Background()

	// Disable foreign keys temporarily during data copy in PostgreSQL
	tx, err := pgDB.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("failed to start pg transaction: %v", err)
	}
	defer tx.Rollback()

	// Set session replication role to replica to bypass foreign key checks during bulk migration
	if _, err := tx.ExecContext(ctx, "SET session_replication_role = 'replica';"); err != nil {
		log.Printf("note: could not set session_replication_role (needs superuser), continuing with standard order: %v", err)
	}

	totalMigrated := 0

	for _, tbl := range tables {
		count, err := migrateTable(ctx, sqliteDB, tx, tbl.name, tbl.conflictOn)
		if err != nil {
			log.Fatalf("failed migrating table %s: %v", tbl.name, err)
		}
		log.Printf("  ✓ %s: %d rows migrated", tbl.name, count)
		totalMigrated += count
	}

	// Restore replication role
	_, _ = tx.ExecContext(ctx, "SET session_replication_role = 'origin';")

	if err := tx.Commit(); err != nil {
		log.Fatalf("failed committing migration transaction: %v", err)
	}

	// Reset sequences for all auto-increment tables
	log.Println("Resetting PostgreSQL serial sequences...")
	for _, tbl := range tables {
		if tbl.hasID {
			resetSeq := fmt.Sprintf(
				"SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE((SELECT MAX(id) FROM %s), 1));",
				tbl.name, tbl.name,
			)
			if _, err := pgDB.ExecContext(ctx, resetSeq); err != nil {
				log.Printf("note: could not reset sequence for table %s: %v", tbl.name, err)
			}
		}
	}

	log.Printf("🎉 Successfully migrated %d total rows from SQLite to PostgreSQL!", totalMigrated)
}

func migrateTable(ctx context.Context, src *sql.DB, dest *sql.Tx, table, conflictCol string) (int, error) {
	// Check if source table exists in SQLite
	var exists int
	err := src.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&exists)
	if err != nil || exists == 0 {
		return 0, nil
	}

	rows, err := src.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, err
	}

	if len(cols) == 0 {
		return 0, nil
	}

	// Prepare INSERT query for Postgres
	colNames := strings.Join(cols, ", ")
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	phStr := strings.Join(placeholders, ", ")

	insertQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, colNames, phStr)
	if conflictCol != "" {
		insertQuery += fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", conflictCol)
	}

	stmt, err := dest.PrepareContext(ctx, insertQuery)
	if err != nil {
		return 0, fmt.Errorf("prepare %s insert: %w", table, err)
	}
	defer stmt.Close()

	count := 0
	values := make([]any, len(cols))
	scanArgs := make([]any, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return count, fmt.Errorf("scan %s: %w", table, err)
		}

		// Convert bytes to string if needed
		insertVals := make([]any, len(cols))
		for i, val := range values {
			if b, ok := val.([]byte); ok {
				insertVals[i] = string(b)
			} else {
				insertVals[i] = val
			}
		}

		if _, err := stmt.ExecContext(ctx, insertVals...); err != nil {
			return count, fmt.Errorf("exec %s insert: %w", table, err)
		}
		count++
	}

	return count, rows.Err()
}
