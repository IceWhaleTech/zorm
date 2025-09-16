package zorm_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/IceWhaleTech/zorm"
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
