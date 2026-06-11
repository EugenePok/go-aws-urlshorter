//go:build integration

package stats

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(localhost:3306)/urlshortener?parseTime=true&loc=UTC"
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping : %v (is MySQL up?)", err)
	}
	if err := EnsureSchema(context.Background(), db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func countFor(t *testing.T, db *sql.DB, code string) int64 {
	t.Helper()
	var c int64
	err := db.QueryRow("SELECT click_count FROM click_stats WHERE code = ?", code).Scan(&c)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	return c
}

func TestMySQL_ApplyDeltas_InsertsThenIncrements(t *testing.T) {
	db := newTestDB(t)
	m := NewMySQL(db)
	ctx := context.Background()
	code := "stat" + time.Now().Format("150405.000")[:5]
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM click_stats WHERE code = ?", code) })

	now := time.Now().UTC()
	if err := m.ApplyDeltas(ctx, []Delta{{Code: code, Count: 3, LastClicked: now}}); err != nil {
		t.Fatalf("first ApplyDeltas: %v", err)
	}
	if got := countFor(t, db, code); got != 3 {
		t.Fatalf("after insert count = %d, want 3", got)
	}

	if err := m.ApplyDeltas(ctx, []Delta{{Code: code, Count: 2, LastClicked: now.Add(time.Minute)}}); err != nil {
		t.Fatalf("second ApplyDeltas: %v", err)
	}
	if got := countFor(t, db, code); got != 5 {
		t.Errorf("after increment count = %d, want 5", got)
	}
}

func TestMySQL_ApplyDeltas_Empty(t *testing.T) {
	m := NewMySQL(newTestDB(t))
	if err := m.ApplyDeltas(context.Background(), nil); err != nil {
		t.Errorf("empty ApplyDeltas: %v", err)
	}
}
