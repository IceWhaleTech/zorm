package zorm_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/IceWhaleTech/zorm"
)

func reviewUpdateTable(tbl *zorm.ZormTable, name string) (int, error) {
	return tbl.Update(zorm.V{"name": name}, zorm.Where(zorm.Eq("id", 1)))
}

func TestReviewReuseSeparatesTables(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE review_a(id INTEGER PRIMARY KEY,name TEXT); CREATE TABLE review_b(id INTEGER PRIMARY KEY,name TEXT); INSERT INTO review_a VALUES(1,'a'); INSERT INTO review_b VALUES(1,'b');"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"review_a", "review_b"} {
		if n, err := reviewUpdateTable(zorm.Table(db, name), name+"-updated"); err != nil || n != 1 {
			t.Fatalf("update %s: rows=%d err=%v", name, n, err)
		}
	}
	for _, name := range []string{"review_a", "review_b"} {
		var got string
		if err := db.QueryRow("SELECT name FROM " + name + " WHERE id=1").Scan(&got); err != nil {
			t.Fatal(err)
		}
		if want := name + "-updated"; got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

type reviewBatchUser struct {
	Name string `zorm:"name"`
}

func reviewInsertBatch(tbl *zorm.ZormTable, users []reviewBatchUser) (int, error) {
	return tbl.Insert(&users)
}
func TestReviewReuseSeparatesBatchSizes(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE review_batch(name TEXT)"); err != nil {
		t.Fatal(err)
	}
	tbl := zorm.Table(db, "review_batch")
	for _, users := range [][]reviewBatchUser{{{Name: "a"}}, {{Name: "b"}, {Name: "c"}}, {{Name: "d"}}} {
		if n, err := reviewInsertBatch(tbl, users); err != nil || n != len(users) {
			t.Fatalf("batch size=%d: rows=%d err=%v", len(users), n, err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM review_batch").Scan(&count); err != nil || count != 4 {
		t.Fatalf("inserted rows=%d err=%v", count, err)
	}
}

type reviewUpdateDB struct{ marker byte }
type reviewUpdateResult struct{}

func (reviewUpdateResult) LastInsertId() (int64, error) { return 0, nil }
func (reviewUpdateResult) RowsAffected() (int64, error) { return 1, nil }
func (reviewUpdateDB) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return &sql.Row{}
}
func (reviewUpdateDB) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, nil
}
func (reviewUpdateDB) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return reviewUpdateResult{}, nil
}

type reviewUpdateUser struct {
	Name string `zorm:"name"`
	Age  int    `zorm:"age"`
}

func reviewDefaultUpdate(tbl *zorm.ZormTable, user *reviewUpdateUser) (int, error) {
	return tbl.Update(user, zorm.Where(zorm.Eq("id", 1)))
}
func TestReviewConcurrentDefaultUpdate(t *testing.T) {
	tbl := zorm.Table(&reviewUpdateDB{marker: 1}, "users")
	warm := reviewUpdateUser{Name: "warm", Age: 1}
	if _, err := reviewDefaultUpdate(tbl, &warm); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			user := reviewUpdateUser{Name: "worker", Age: 2}
			<-start
			if _, err := reviewDefaultUpdate(tbl, &user); err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()
}

func reuseTestDB(t *testing.T, schema string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestReuseCacheTablesForSelectInsertDelete(t *testing.T) {
	for _, op := range []string{"Select", "Insert", "Delete"} {
		t.Run(op, func(t *testing.T) {
			db := reuseTestDB(t, "CREATE TABLE a(name TEXT); CREATE TABLE b(name TEXT); INSERT INTO a VALUES('a'); INSERT INTO b VALUES('b');")
			run := func(tbl *zorm.ZormTable, name string) error {
				switch op {
				case "Select":
					var row reviewBatchUser
					n, err := tbl.Select(&row)
					if err == nil && (n != 1 || row.Name != name) {
						return fmt.Errorf("select %s: rows=%d name=%q", name, n, row.Name)
					}
					return err
				case "Insert":
					_, err := tbl.Insert(&reviewBatchUser{Name: name})
					return err
				default:
					_, err := tbl.Delete(zorm.Where("1 = 1"))
					return err
				}
			}
			for _, name := range []string{"a", "b"} {
				if err := run(zorm.Table(db, name), name); err != nil {
					t.Fatal(err)
				}
				want := 1
				if op == "Insert" {
					want = 2
				} else if op == "Delete" {
					want = 0
				}
				var got int
				if err := db.QueryRow("SELECT COUNT(*) FROM " + name).Scan(&got); err != nil || got != want {
					t.Fatalf("%s %s: rows=%d want=%d err=%v", op, name, got, want, err)
				}
			}
		})
	}
}

func TestReuseCacheModelIdentity(t *testing.T) {
	db := reuseTestDB(t, "CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT, age INTEGER); INSERT INTO users VALUES(1,'old',0);")
	tbl := zorm.Table(db, "users")
	run := func(user interface{}) (int, error) {
		return tbl.Update(user, zorm.Fields("name", "age"), zorm.Where("id = ?", 1))
	}
	type reorderedUser struct {
		Age  int    `zorm:"age"`
		Name string `zorm:"name"`
	}
	for _, user := range []interface{}{&reviewUpdateUser{Name: "first", Age: 1}, &reorderedUser{Name: "second", Age: 2}} {
		if n, err := run(user); err != nil || n != 1 {
			t.Fatalf("update %T: rows=%d err=%v", user, n, err)
		}
	}
	var name string
	var age int
	if err := db.QueryRow("SELECT name, age FROM users").Scan(&name, &age); err != nil || name != "second" || age != 2 {
		t.Fatalf("name=%q age=%d err=%v", name, age, err)
	}
}

func TestReuseCacheConfigIdentity(t *testing.T) {
	db := reuseTestDB(t, "CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT, age INTEGER); INSERT INTO users VALUES(1,'old',0);")
	tbl := zorm.Table(db, "users")
	user := struct {
		Name string `zorm:"name"`
		Age  int
	}{Name: "new", Age: 42}
	run := func() (int, error) { return tbl.Update(&user, zorm.Where("id = ?", 1)) }
	for _, useNames := range []bool{false, true} {
		tbl.Cfg.UseNameWhenTagEmpty = useNames
		if n, err := run(); err != nil || n != 1 {
			t.Fatalf("UseNameWhenTagEmpty=%t: rows=%d err=%v", useNames, n, err)
		}
		var age int
		want := 0
		if useNames {
			want = user.Age
		}
		if err := db.QueryRow("SELECT age FROM users").Scan(&age); err != nil || age != want {
			t.Fatalf("age=%d want=%d err=%v", age, want, err)
		}
	}
}

func TestReuseCacheDatabaseIdentity(t *testing.T) {
	run := func(db *sql.DB) (map[string]interface{}, error) {
		var row map[string]interface{}
		_, err := zorm.Table(db, "users").Select(&row)
		return row, err
	}
	for _, column := range []string{"name", "email"} {
		db := reuseTestDB(t, "CREATE TABLE users("+column+" TEXT); INSERT INTO users VALUES('value');")
		row, err := run(db)
		if err != nil || row[column] != "value" {
			t.Fatalf("column=%s row=%v err=%v", column, row, err)
		}
	}
}

func TestReuseCacheInsertModes(t *testing.T) {
	db := reuseTestDB(t, "CREATE TABLE users(name TEXT PRIMARY KEY, age INTEGER);")
	tbl := zorm.Table(db, "users")
	user := reviewUpdateUser{Name: "key", Age: 1}
	run := func(insert func(interface{}, ...zorm.ZormItem) (int, error)) (int, error) {
		return insert(&user)
	}
	for i, insert := range []func(interface{}, ...zorm.ZormItem) (int, error){tbl.Insert, tbl.InsertIgnore, tbl.ReplaceInto} {
		user.Age = i + 1
		want := 1
		if i == 1 {
			want = 0
		}
		if n, err := run(insert); err != nil || n != want {
			t.Fatalf("mode=%d rows=%d want=%d err=%v", i, n, want, err)
		}
	}
	var age int
	if err := db.QueryRow("SELECT age FROM users").Scan(&age); err != nil || age != 3 {
		t.Fatalf("age=%d err=%v", age, err)
	}
}

func TestReuseCachePointerBatchSizes(t *testing.T) {
	db := reuseTestDB(t, "CREATE TABLE users(name TEXT, age INTEGER);")
	tbl := zorm.Table(db, "users")
	run := func(users []*reviewUpdateUser) (int, error) { return tbl.Insert(&users) }
	for _, size := range []int{1, 2, 1} {
		users := make([]*reviewUpdateUser, size)
		for i := range users {
			users[i] = &reviewUpdateUser{Name: "user", Age: i}
		}
		if n, err := run(users); err != nil || n != size {
			t.Fatalf("size=%d rows=%d err=%v", size, n, err)
		}
	}
}

type nonComparableCacheDB struct {
	reviewUpdateDB
	values []int
}

func TestReuseCacheNonComparableDatabase(t *testing.T) {
	tbl := zorm.Table(nonComparableCacheDB{values: []int{1}}, "users")
	user := reviewUpdateUser{Name: "user", Age: 1}
	for i := 0; i < 2; i++ {
		if n, err := reviewDefaultUpdate(tbl, &user); err != nil || n != 1 {
			t.Fatalf("rows=%d err=%v", n, err)
		}
	}
}

func TestReuseCacheAcrossTableInstances(t *testing.T) {
	db := reuseTestDB(t, "CREATE TABLE users(name TEXT); INSERT INTO users VALUES('value');")
	where := &countingWhereItem{sql: " where 1 = ?", args: []interface{}{1}}
	run := func(row *reviewBatchUser) (int, error) {
		return zorm.Table(db, "users").Select(row, where)
	}
	for i := 0; i < 2; i++ {
		var row reviewBatchUser
		if n, err := run(&row); err != nil || n != 1 || row.Name != "value" {
			t.Fatalf("rows=%d row=%v err=%v", n, row, err)
		}
	}
	if where.buildCalls != 1 {
		t.Fatalf("expected one SQL build across table instances, got %d", where.buildCalls)
	}
}

func TestReuseCacheConcurrentTableInstances(t *testing.T) {
	db := reuseTestDB(t, "CREATE TABLE a(id INTEGER PRIMARY KEY, name TEXT); CREATE TABLE b(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO a VALUES(1,'old'); INSERT INTO b VALUES(1,'old');")
	run := func(name string) (int, error) {
		return zorm.Table(db, name).Update(zorm.V{"name": name + "-updated"}, zorm.Where("id = ?", 1))
	}
	if n, err := run("a"); err != nil || n != 1 {
		t.Fatalf("warm-up: rows=%d err=%v", n, err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		name := []string{"a", "b"}[i%2]
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 20; j++ {
				if n, err := run(name); err != nil || n != 1 {
					t.Errorf("update %s: rows=%d err=%v", name, n, err)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	for _, name := range []string{"a", "b"} {
		var got string
		if err := db.QueryRow("SELECT name FROM " + name + " WHERE id = 1").Scan(&got); err != nil || got != name+"-updated" {
			t.Fatalf("table=%s name=%q err=%v", name, got, err)
		}
	}
}
