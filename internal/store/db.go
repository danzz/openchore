package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Dialect represents the SQL dialect of the underlying database engine.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// DetectDialect queries the database to determine whether it is SQLite or PostgreSQL.
func DetectDialect(db *sql.DB) Dialect {
	var ver string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&ver); err == nil {
		return DialectSQLite
	}
	return DialectPostgres
}

// DB wraps *sql.DB to provide dialect-aware query execution and placeholder rewriting.
type DB struct {
	*sql.DB
	dialect Dialect
}

// Tx wraps *sql.Tx to provide dialect-aware query execution and placeholder rewriting within transactions.
type Tx struct {
	*sql.Tx
	dialect Dialect
}

func wrapDB(db *sql.DB, dialect Dialect) *DB {
	return &DB{
		DB:      db,
		dialect: dialect,
	}
}

// Dialect returns the DB's dialect.
func (db *DB) Dialect() Dialect {
	return db.dialect
}

func (db *DB) rebind(query string) string {
	if db.dialect != DialectPostgres {
		return query
	}
	return rebindPostgres(query)
}

func (tx *Tx) rebind(query string) string {
	if tx.dialect != DialectPostgres {
		return query
	}
	return rebindPostgres(query)
}

// ExecContext executes a query after dialect-specific placeholder rewriting.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.DB.ExecContext(ctx, db.rebind(query), args...)
}

// QueryContext runs a query after dialect-specific placeholder rewriting.
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, db.rebind(query), args...)
}

// QueryRowContext executes a query that is expected to return at most one row.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.DB.QueryRowContext(ctx, db.rebind(query), args...)
}

// BeginTx starts a transaction wrapped with dialect-aware rewriting.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{
		Tx:      tx,
		dialect: db.dialect,
	}, nil
}

// InsertReturningID executes an INSERT statement and returns the newly generated ID.
// For SQLite, it calls res.LastInsertId(). For PostgreSQL, it appends RETURNING <idCol>.
func (db *DB) InsertReturningID(ctx context.Context, query string, idCol string, args ...any) (int64, error) {
	if db.dialect == DialectPostgres {
		q := db.rebind(query) + " RETURNING " + idCol
		var id int64
		err := db.DB.QueryRowContext(ctx, q, args...).Scan(&id)
		return id, err
	}
	res, err := db.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ExecContext executes a query in the transaction with placeholder rewriting.
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.Tx.ExecContext(ctx, tx.rebind(query), args...)
}

// QueryContext runs a query in the transaction with placeholder rewriting.
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return tx.Tx.QueryContext(ctx, tx.rebind(query), args...)
}

// QueryRowContext executes a query in the transaction with placeholder rewriting.
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return tx.Tx.QueryRowContext(ctx, tx.rebind(query), args...)
}

// InsertReturningID executes an INSERT statement in the transaction and returns the new ID.
func (tx *Tx) InsertReturningID(ctx context.Context, query string, idCol string, args ...any) (int64, error) {
	if tx.dialect == DialectPostgres {
		q := tx.rebind(query) + " RETURNING " + idCol
		var id int64
		err := tx.Tx.QueryRowContext(ctx, q, args...).Scan(&id)
		return id, err
	}
	res, err := tx.Tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RebindPostgres converts standard '?' placeholders to PostgreSQL '$1, $2, ...' placeholders,
// safely skipping string literals, quotes, and SQL comments.
func RebindPostgres(query string) string {
	return rebindPostgres(query)
}

func rebindPostgres(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 32)
	paramIdx := 1
	n := len(query)

	for i := 0; i < n; i++ {
		ch := query[i]

		// Single-quoted string literal
		if ch == '\'' {
			b.WriteByte(ch)
			i++
			for i < n {
				b.WriteByte(query[i])
				if query[i] == '\'' {
					if i+1 < n && query[i+1] == '\'' {
						b.WriteByte(query[i+1])
						i += 2
						continue
					}
					break
				}
				i++
			}
			continue
		}

		// Double-quoted identifier
		if ch == '"' {
			b.WriteByte(ch)
			i++
			for i < n {
				b.WriteByte(query[i])
				if query[i] == '"' {
					if i+1 < n && query[i+1] == '"' {
						b.WriteByte(query[i+1])
						i += 2
						continue
					}
					break
				}
				i++
			}
			continue
		}

		// Single-line comment --
		if ch == '-' && i+1 < n && query[i+1] == '-' {
			b.WriteByte(ch)
			b.WriteByte(query[i+1])
			i += 2
			for i < n {
				b.WriteByte(query[i])
				if query[i] == '\n' {
					break
				}
				i++
			}
			continue
		}

		// Block comment /* ... */
		if ch == '/' && i+1 < n && query[i+1] == '*' {
			b.WriteByte(ch)
			b.WriteByte(query[i+1])
			i += 2
			for i < n {
				b.WriteByte(query[i])
				if query[i] == '*' && i+1 < n && query[i+1] == '/' {
					b.WriteByte(query[i+1])
					i++
					break
				}
				i++
			}
			continue
		}

		if ch == '?' {
			b.WriteString(fmt.Sprintf("$%d", paramIdx))
			paramIdx++
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}
