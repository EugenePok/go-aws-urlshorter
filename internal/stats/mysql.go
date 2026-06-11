package stats

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // register the "mysql" driver
)

//go:embed schema.sql
var schemaSQL string

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	return db, nil
}

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}

type MySQL struct {
	db *sql.DB
}

func NewMySQL(db *sql.DB) *MySQL { return &MySQL{db: db} }

const upsert = `INSERT INTO click_stats (code, click_count, last_clicked_at)
VALUES (?, ?, ?) AS new
ON DUPLICATE KEY UPDATE
	click_count = click_stats.click_count + new.click_count,
	last_clicked_at = GREATEST(click_stats.last_clicked_at, new.last_clicked_at)`

func (m *MySQL) ApplyDeltas(ctx context.Context, deltas []Delta) error {
	if len(deltas) == 0 {
		return nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, upsert)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, d := range deltas {
		if _, err := stmt.ExecContext(ctx, d.Code, d.Count, d.LastClicked.UTC()); err != nil {
			return fmt.Errorf("upsert %q: %w", d.Code, err)
		}
	}
	return tx.Commit()
}
