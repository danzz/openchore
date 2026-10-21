package db

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	mpostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/migrations"
	pgmigrations "github.com/liftedkilt/openchore/migrations/postgres"
	_ "modernc.org/sqlite"
)

// LoadEnv loads environment variables from a .env file if it exists,
// without overriding any variables already present in the environment.
func LoadEnv(path string) {
	if path == "" {
		path = ".env"
	}
	f, err := os.Open(path)
	if err != nil {
		return // File doesn't exist, ignore
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		// Strip quotes if present
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

// Config holds resolved database configuration.
type Config struct {
	Driver   string
	DSN      string
	DBName   string
	Dialect  store.Dialect
	AdminDSN string // Used for auto-creating postgres db if needed
}

// ResolveConfig inspects environment variables and determines database settings.
func ResolveConfig() (*Config, error) {
	dbType := strings.ToLower(os.Getenv("DB_TYPE"))
	postgresHost := os.Getenv("POSTGRES_HOST")
	databaseURL := os.Getenv("DATABASE_URL")

	isPostgres := dbType == "postgres" || (dbType != "sqlite" && (postgresHost != "" || databaseURL != ""))

	if isPostgres {
		if databaseURL != "" {
			u, err := url.Parse(databaseURL)
			if err != nil {
				return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
			}
			dbName := strings.TrimPrefix(u.Path, "/")
			if dbName == "" {
				dbName = "openchore"
			}
			// Build admin DSN pointing to postgres maintenance db
			adminU := *u
			adminU.Path = "/postgres"
			return &Config{
				Driver:   "postgres",
				DSN:      databaseURL,
				DBName:   dbName,
				Dialect:  store.DialectPostgres,
				AdminDSN: adminU.String(),
			}, nil
		}

		host := postgresHost
		port := os.Getenv("POSTGRES_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("POSTGRES_USER")
		if user == "" {
			user = "postgres"
		}
		pass := os.Getenv("POSTGRES_PASSWORD")
		dbName := os.Getenv("POSTGRES_DB")
		if dbName == "" {
			dbName = os.Getenv("POSTGRES_DATABASE")
		}
		if dbName == "" {
			dbName = "openchore"
		}
		sslMode := os.Getenv("POSTGRES_SSLMODE")
		if sslMode == "" {
			sslMode = "disable"
		}

		dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			url.QueryEscape(user), url.QueryEscape(pass), host, port, dbName, sslMode)
		adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=%s",
			url.QueryEscape(user), url.QueryEscape(pass), host, port, sslMode)

		return &Config{
			Driver:   "postgres",
			DSN:      dsn,
			DBName:   dbName,
			Dialect:  store.DialectPostgres,
			AdminDSN: adminDSN,
		}, nil
	}

	// SQLite default
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "openchore.db"
	}
	dsn := dbPath + "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000"

	return &Config{
		Driver:  "sqlite",
		DSN:     dsn,
		DBName:  dbPath,
		Dialect: store.DialectSQLite,
	}, nil
}

var safeIdent = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// ensurePostgresDatabase verifies that the target PostgreSQL database exists,
// attempting to create it via the maintenance connection if it does not.
func ensurePostgresDatabase(cfg *Config) error {
	if cfg.AdminDSN == "" || cfg.DBName == "" {
		return nil
	}

	// Connect to maintenance DB
	adminDB, err := sql.Open("postgres", cfg.AdminDSN)
	if err != nil {
		return fmt.Errorf("failed to open admin postgres connection: %w", err)
	}
	defer adminDB.Close()

	var exists bool
	err = adminDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.DBName).Scan(&exists)
	if err != nil {
		log.Printf("db: could not check if database %q exists: %v", cfg.DBName, err)
		return nil // Non-fatal, try connecting anyway
	}

	if !exists {
		if !safeIdent.MatchString(cfg.DBName) {
			return fmt.Errorf("database name %q contains invalid characters", cfg.DBName)
		}
		log.Printf("db: database %q not found, creating...", cfg.DBName)
		_, err = adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, cfg.DBName))
		if err != nil {
			return fmt.Errorf("failed to create database %q: %w", cfg.DBName, err)
		}
		log.Printf("db: database %q created successfully", cfg.DBName)
	}

	return nil
}

// OpenAndMigrate establishes the database connection, ensures schema migration,
// and returns the initialized *sql.DB along with the dialect.
func OpenAndMigrate(ctx context.Context) (*sql.DB, store.Dialect, error) {
	cfg, err := ResolveConfig()
	if err != nil {
		return nil, "", err
	}

	if cfg.Driver == "postgres" {
		log.Printf("db: using PostgreSQL database %q", cfg.DBName)
		if err := ensurePostgresDatabase(cfg); err != nil {
			log.Printf("db: auto-create database warning: %v", err)
		}

		db, err := sql.Open("postgres", cfg.DSN)
		if err != nil {
			return nil, "", fmt.Errorf("failed to open postgres database: %w", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		if err := db.PingContext(ctx); err != nil {
			db.Close()
			return nil, "", fmt.Errorf("failed to ping postgres database: %w", err)
		}

		if err := runPostgresMigrations(db); err != nil {
			db.Close()
			return nil, "", fmt.Errorf("failed to run postgres migrations: %w", err)
		}

		return db, store.DialectPostgres, nil
	}

	// SQLite
	log.Printf("db: using SQLite database %q", cfg.DBName)
	db, err := sql.Open("sqlite", cfg.DSN)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := runSQLiteMigrations(db); err != nil {
		db.Close()
		return nil, "", fmt.Errorf("failed to run sqlite migrations: %w", err)
	}

	return db, store.DialectSQLite, nil
}

func runPostgresMigrations(db *sql.DB) error {
	driver, err := mpostgres.WithInstance(db, &mpostgres.Config{})
	if err != nil {
		return fmt.Errorf("creating postgres migration driver: %w", err)
	}

	source, err := iofs.New(pgmigrations.FS, ".")
	if err != nil {
		return fmt.Errorf("creating postgres migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating postgres migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running postgres migrations: %w", err)
	}

	log.Println("postgres migrations complete")
	return nil
}

func runSQLiteMigrations(db *sql.DB) error {
	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		return fmt.Errorf("creating sqlite migration driver: %w", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("creating sqlite migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("creating sqlite migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running sqlite migrations: %w", err)
	}

	log.Println("sqlite migrations complete")
	return nil
}
