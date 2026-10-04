// Package store owns the SQLite file: schema, migrations and row-level queries.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// TimeFormat is how instants are stored: UTC, second precision, so that
// string comparison in SQL matches chronological order.
const TimeFormat = "2006-01-02T15:04:05Z"

func FormatTime(t time.Time) string { return t.UTC().Format(TimeFormat) }

func ParseTime(s string) time.Time {
	t, _ := time.Parse(TimeFormat, s)
	return t
}

// Q is satisfied by both *sql.DB and *sql.Tx.
type Q interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type DB struct {
	*sql.DB
}

// Open opens (and migrates) <dir>/keduly.db.
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	v := url.Values{}
	for _, p := range []string{"journal_mode(WAL)", "foreign_keys(1)", "busy_timeout(10000)", "synchronous(NORMAL)"} {
		v.Add("_pragma", p)
	}
	// Write transactions take the lock up front so they never fail on upgrade.
	v.Set("_txlock", "immediate")
	dsn := "file:" + filepath.ToSlash(filepath.Join(dir, "keduly.db")) + "?" + v.Encode()
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(8)
	db := &DB{sqlDB}
	if err := db.migrate(context.Background()); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) migrate(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		var n int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version = ?`, version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Table maps a struct to a table whose first two columns are id and user_id.
// fields returns pointers to the struct fields in column order; the same
// pointers serve as scan targets and as statement arguments.
type Table[T any] struct {
	Name   string
	Cols   []string
	Order  string
	fields func(*T) []any
}

// SelectSQL is the SELECT prefix for custom queries passed to Query.
func (t Table[T]) SelectSQL() string {
	return "SELECT " + strings.Join(t.Cols, ", ") + " FROM " + t.Name
}

// Get returns nil when the row does not exist or belongs to another user.
func (t Table[T]) Get(ctx context.Context, q Q, userID, id string) (*T, error) {
	rows, err := t.List(ctx, q, userID, "id = ?", id)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// List returns the user's rows matching where (which may be empty).
func (t Table[T]) List(ctx context.Context, q Q, userID, where string, args ...any) ([]*T, error) {
	query := t.SelectSQL() + " WHERE user_id = ?"
	if where != "" {
		query += " AND (" + where + ")"
	}
	if t.Order != "" {
		query += " ORDER BY " + t.Order
	}
	return t.Query(ctx, q, query, append([]any{userID}, args...)...)
}

// Query runs a full SELECT that yields the table's columns.
func (t Table[T]) Query(ctx context.Context, q Q, query string, args ...any) ([]*T, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*T{}
	for rows.Next() {
		row := new(T)
		if err := rows.Scan(t.fields(row)...); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (t Table[T]) Count(ctx context.Context, q Q, userID, where string, args ...any) (int, error) {
	query := "SELECT count(*) FROM " + t.Name + " WHERE user_id = ?"
	if where != "" {
		query += " AND (" + where + ")"
	}
	var n int
	err := q.QueryRowContext(ctx, query, append([]any{userID}, args...)...).Scan(&n)
	return n, err
}

// Put inserts the row or updates it in place. It never moves a row between users.
func (t Table[T]) Put(ctx context.Context, q Q, row *T) error {
	sets := make([]string, 0, len(t.Cols))
	for _, c := range t.Cols[2:] {
		sets = append(sets, c+" = excluded."+c)
	}
	query := "INSERT INTO " + t.Name + " (" + strings.Join(t.Cols, ", ") + ") VALUES (" +
		strings.TrimSuffix(strings.Repeat("?, ", len(t.Cols)), ", ") +
		") ON CONFLICT(id) DO UPDATE SET " + strings.Join(sets, ", ") +
		" WHERE " + t.Name + ".user_id = excluded.user_id"
	res, err := q.ExecContext(ctx, query, t.fields(row)...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("store: row belongs to another user")
	}
	return nil
}

func (t Table[T]) Delete(ctx context.Context, q Q, userID, id string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM "+t.Name+" WHERE user_id = ? AND id = ?", userID, id)
	return err
}

// SetOwner overwrites the id and user_id of a row, used when restoring snapshots.
func (t Table[T]) SetOwner(row *T, userID string) {
	*(t.fields(row)[1].(*string)) = userID
}

func (t Table[T]) ID(row *T) string {
	return *(t.fields(row)[0].(*string))
}
