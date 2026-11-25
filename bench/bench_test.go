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
	if n, err := t.Update(&u, where); err != nil || n != 1 {
		bm.Fatalf("Update: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		if n, err := t.Update(&u, where); err != nil || n != 1 {
			bm.Fatalf("Update: rows=%d err=%v", n, err)
		}
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
	if n, err := t.Update(&u, fields, where); err != nil || n != 1 {
		bm.Fatalf("Update: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		if n, err := t.Update(&u, fields, where); err != nil || n != 1 {
			bm.Fatalf("Update: rows=%d err=%v", n, err)
		}
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
	if n, err := t.Update(&u, fieldsA, where); err != nil || n != 1 {
		bm.Fatalf("Update: rows=%d err=%v", n, err)
	}
	if n, err := t.Update(&u, fieldsB, where); err != nil || n != 1 {
		bm.Fatalf("Update: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		if i&1 == 0 {
			if n, err := t.Update(&u, fieldsA, where); err != nil || n != 1 {
				bm.Fatalf("Update: rows=%d err=%v", n, err)
			}
		} else {
			if n, err := t.Update(&u, fieldsB, where); err != nil || n != 1 {
				bm.Fatalf("Update: rows=%d err=%v", n, err)
			}
		}
	}
}

func BenchmarkInsert_Reuse_Default(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	u := benchInsertUser{Name: "alice", Age: 20}

	if n, err := t.Insert(&u); err != nil || n != 1 {
		bm.Fatalf("Insert: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		if n, err := t.Insert(&u); err != nil || n != 1 {
			bm.Fatalf("Insert: rows=%d err=%v", n, err)
		}
	}
}

func BenchmarkInsert_Reuse_WithFields(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	u := benchInsertUser{Name: "alice", Age: 20}
	fields := zorm.Fields("name", "age")

	if n, err := t.Insert(&u, fields); err != nil || n != 1 {
		bm.Fatalf("Insert: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		u.Name = fmt.Sprintf("alice-%d", i)
		u.Age = 20 + i&3
		if n, err := t.Insert(&u, fields); err != nil || n != 1 {
			bm.Fatalf("Insert: rows=%d err=%v", n, err)
		}
	}
}

func BenchmarkUpdateMap_Reuse_SameShape(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))
	update := zorm.V{"name": "alice", "age": 20}

	if n, err := t.Update(update, where); err != nil || n != 1 {
		bm.Fatalf("Update: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		update["name"] = fmt.Sprintf("alice-%d", i)
		update["age"] = 20 + i&3
		if n, err := t.Update(update, where); err != nil || n != 1 {
			bm.Fatalf("Update: rows=%d err=%v", n, err)
		}
	}
}

func BenchmarkUpdateMap_Reuse_ShapeChanges(bm *testing.B) {
	bm.ReportAllocs()
	db := benchDB{}
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))
	updateA := zorm.V{"name": "alice"}
	updateB := zorm.V{"age": 20}

	if n, err := t.Update(updateA, where); err != nil || n != 1 {
		bm.Fatalf("Update: rows=%d err=%v", n, err)
	}
	if n, err := t.Update(updateB, where); err != nil || n != 1 {
		bm.Fatalf("Update: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		if i&1 == 0 {
			updateA["name"] = fmt.Sprintf("alice-%d", i)
			if n, err := t.Update(updateA, where); err != nil || n != 1 {
				bm.Fatalf("Update: rows=%d err=%v", n, err)
			}
		} else {
			updateB["age"] = 20 + i&3
			if n, err := t.Update(updateB, where); err != nil || n != 1 {
				bm.Fatalf("Update: rows=%d err=%v", n, err)
			}
		}
	}
}

func BenchmarkSelect_Reuse_SameShape(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm benchSelectUser
	if n, err := t.Select(&warm, where); err != nil || n != 1 {
		bm.Fatalf("Select: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var user benchSelectUser
		if n, err := t.Select(&user, where); err != nil || n != 1 {
			bm.Fatalf("Select: rows=%d err=%v", n, err)
		}
	}
}

func BenchmarkSelect_Reuse_WithFields(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	fields := zorm.Fields("name", "age")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm benchSelectUser
	if n, err := t.Select(&warm, fields, where); err != nil || n != 1 {
		bm.Fatalf("Select: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var user benchSelectUser
		if n, err := t.Select(&user, fields, where); err != nil || n != 1 {
			bm.Fatalf("Select: rows=%d err=%v", n, err)
		}
	}
}

func BenchmarkSelectMap_Reuse_Wildcard(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm map[string]interface{}
	if n, err := t.Select(&warm, where); err != nil || n != 1 {
		bm.Fatalf("Select: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var row map[string]interface{}
		if n, err := t.Select(&row, where); err != nil || n != 1 {
			bm.Fatalf("Select: rows=%d err=%v", n, err)
		}
	}
}

func BenchmarkSelectMap_Reuse_WithFields(bm *testing.B) {
	bm.ReportAllocs()
	db := setupSelectBenchDB(bm)
	t := zorm.Table(db, "users")
	fields := zorm.Fields("name", "age")
	where := zorm.Where(zorm.Eq("id", 1))

	var warm map[string]interface{}
	if n, err := t.Select(&warm, fields, where); err != nil || n != 1 {
		bm.Fatalf("Select: rows=%d err=%v", n, err)
	}

	bm.ResetTimer()
	for i := 0; i < bm.N; i++ {
		var row map[string]interface{}
		if n, err := t.Select(&row, fields, where); err != nil || n != 1 {
			bm.Fatalf("Select: rows=%d err=%v", n, err)
		}
	}
}

// Keep formatting and database I/O out of the cache-layout comparison.
func BenchmarkUpdate_Reuse_Parallel(bm *testing.B) {
	for _, shared := range []bool{true, false} {
		name := "IndependentTables"
		if shared {
			name = "SharedTable"
		}
		bm.Run(name, func(bm *testing.B) {
			tbl := zorm.Table(benchDB{}, "users")
			bm.ReportAllocs()
			bm.RunParallel(func(pb *testing.PB) {
				workerTable := tbl
				if !shared {
					workerTable = zorm.Table(benchDB{}, "users")
				}
				u := benchUser{ID: 1, Name: "alice", Age: 20}
				args := []zorm.ZormItem{zorm.Fields("name", "age"), zorm.Where(zorm.Eq("id", u.ID))}
				for pb.Next() {
					if n, err := workerTable.Update(&u, args...); err != nil || n != 1 {
						bm.Fatalf("Update: rows=%d err=%v", n, err)
					}
				}
			})
		})
	}
}

func BenchmarkUpdate_Reuse_NewTable(bm *testing.B) {
	db := benchDB{}
	u := benchUser{ID: 1, Name: "alice", Age: 20}
	args := []zorm.ZormItem{zorm.Fields("name", "age"), zorm.Where(zorm.Eq("id", u.ID))}
	bm.ReportAllocs()
	for i := 0; i < bm.N; i++ {
		if n, err := zorm.Table(db, "users").Update(&u, args...); err != nil || n != 1 {
			bm.Fatalf("Update: rows=%d err=%v", n, err)
		}
	}
}

func BenchmarkUpdate_Reuse_NewTable_AlternatingTables(bm *testing.B) {
	db := benchDB{}
	u := benchUser{ID: 1, Name: "alice", Age: 20}
	args := []zorm.ZormItem{zorm.Fields("name", "age"), zorm.Where(zorm.Eq("id", u.ID))}
	names := [...]string{"users_a", "users_b"}
	bm.ReportAllocs()
	for i := 0; i < bm.N; i++ {
		if n, err := zorm.Table(db, names[i&1]).Update(&u, args...); err != nil || n != 1 {
			bm.Fatalf("Update: rows=%d err=%v", n, err)
		}
	}
}
