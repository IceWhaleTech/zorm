package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`create table users(id integer primary key, name text, age integer);`)

	t := zorm.Table(db, "users")

	// insert with map
	m := map[string]interface{}{"name": "alice", "age": 20}
	_, _ = t.Insert(m)

	// select to []map
	var rows []map[string]interface{}
	_, _ = t.Select(&rows, zorm.Fields("id", "name", "age"), zorm.Where(zorm.Gt("age", 18)))
	fmt.Printf("rows=%v\n", rows)

	// update with partial fields
	_, _ = t.Update(zorm.V{"name": "alice2", "age": 21}, zorm.Fields("name", "age"), zorm.Where(zorm.Eq("id", rows[0]["id"])))

	// delete
	_, _ = t.Delete(zorm.Where(zorm.Eq("id", rows[0]["id"])))
}
