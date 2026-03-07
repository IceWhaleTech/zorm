package zorm_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/IceWhaleTech/zorm"

	_ "github.com/mattn/go-sqlite3"
)

// noop DB impl for Exec-only benchmarks
type benchDB struct{}

func (benchDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return &sql.Row{}
}

func (benchDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

type benchResult struct{}

func (benchResult) LastInsertId() (int64, error) { return 0, nil }
func (benchResult) RowsAffected() (int64, error) { return 1, nil }

func (benchDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return benchResult{}, nil
}

type benchUser struct {
	ID   int64  `zorm:"id"`
	Name string `zorm:"name"`
	Age  int    `zorm:"age"`
}

type benchInsertUser struct {
	ID   int64  `zorm:"id,auto_incr"`
	Name string `zorm:"name"`
	Age  int    `zorm:"age"`
}

type benchSelectUser struct {
	ID        int64     `zorm:"id,auto_incr"`
	Name      string    `zorm:"name"`
	Age       int       `zorm:"age"`
	CreatedAt time.Time `zorm:"created_at"`
}

func setupSelectBenchDB(bm *testing.B) *sql.DB {
	bm.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		bm.Fatalf("open sqlite: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		age INTEGER,
		created_at DATETIME
	)`)
	if err != nil {
		bm.Fatalf("create table: %v", err)
	}

	_, err = db.Exec(`INSERT INTO users(name, age, created_at) VALUES (?, ?, ?)`, "alice", 20, time.Now().UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		bm.Fatalf("insert seed: %v", err)
	}

	bm.Cleanup(func() { _ = db.Close() })
	return db
}

// shape: constant; default Reuse
func BenchmarkUpdate_Reuse_Default(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	u := benchUser{ID: 1, Name: "alice", Age: 20}
	where := zorm.Where(zorm.Eq("id", u.ID))

	// warm-up to populate cache
	_, _ = t.Update(&u, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		_, _ = t.Update(&u, where)
	}
}

// shape: constant; shape-aware Reuse
func BenchmarkUpdate_Reuse_SameShape(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	u := benchUser{ID: 1, Name: "alice", Age: 20}
	where := zorm.Where(zorm.Eq("id", u.ID))
	fields := zorm.Fields("name", "age")

	// warm-up
	_, _ = t.Update(&u, fields, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		_, _ = t.Update(&u, fields, where)
	}
}

// shape: alternates between two shapes under shape-aware Reuse
func BenchmarkUpdate_Reuse_ShapeChanges(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	u := benchUser{ID: 1, Name: "alice", Age: 20}
	where := zorm.Where(zorm.Eq("id", u.ID))
	fieldsA := zorm.Fields("name")
	fieldsB := zorm.Fields("name", "age")

	// warm-up both shapes
	_, _ = t.Update(&u, fieldsA, where)
	_, _ = t.Update(&u, fieldsB, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		if i&1 == 0 {
			_, _ = t.Update(&u, fieldsA, where)
		} else {
			_, _ = t.Update(&u, fieldsB, where)
		}
	}
}

func BenchmarkInsert_Reuse_Default(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	u := benchInsertUser{Name: "alice", Age: 20}

	_, _ = t.Insert(&u)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		_, _ = t.Insert(&u)
	}
}

func BenchmarkInsert_Reuse_WithFields(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	u := benchInsertUser{Name: "alice", Age: 20}
	fields := zorm.Fields("name", "age")

	_, _ = t.Insert(&u, fields)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		_, _ = t.Insert(&u, fields)
	}
}

func BenchmarkUpdateMap_Reuse_SameShape(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))
	update := zorm.V{"name": "alice", "age": 20}

	_, _ = t.Update(update, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		update["name"] = fmt.Sprintf("alice-%d", i)
		update["age"] = 20 + i&3
		_, _ = t.Update(update, where)
	}
}

func BenchmarkUpdateMap_Reuse_ShapeChanges(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))
	updateA := zorm.V{"name": "alice"}
	updateB := zorm.V{"age": 20}

	_, _ = t.Update(updateA, where)
	_, _ = t.Update(updateB, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		if i&1 == 0 {
			updateA["name"] = fmt.Sprintf("alice-%d", i)
			_, _ = t.Update(updateA, where)
		} else {
			updateB["age"] = 20 + i&3
			_, _ = t.Update(updateB, where)
		}
	}
}

func BenchmarkSelect_Reuse_SameShape(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm benchSelectUser
	_, _ = t.Select(&warm, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var user benchSelectUser
		_, _ = t.Select(&user, where)
	}
}

func BenchmarkSelect_Reuse_WithFields(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	fields := zorm.Fields("name", "age")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm benchSelectUser
	_, _ = t.Select(&warm, fields, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var user benchSelectUser
		_, _ = t.Select(&user, fields, where)
	}
}

func BenchmarkSelectMap_Reuse_Wildcard(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm map[string]interface{}
	_, _ = t.Select(&warm, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var row map[string]interface{}
		_, _ = t.Select(&row, where)
	}
}

func BenchmarkSelectMap_Reuse_WithFields(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	fields := zorm.Fields("name", "age")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm map[string]interface{}
	_, _ = t.Select(&warm, fields, where)

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var row map[string]interface{}
		_, _ = t.Select(&row, fields, where)
	}
}
