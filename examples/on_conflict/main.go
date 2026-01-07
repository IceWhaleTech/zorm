package main

import (
	"database/sql"
	"log"

	"github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID   int64  `zorm:"id"`
	Name string `zorm:"name"`
	Age  int    `zorm:"age"`
}

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`create table users(id integer primary key, name text, age integer);`)

	t := zorm.Table(db, "users")

	u := User{ID: 1, Name: "alice", Age: 20}
	_, _ = t.Insert(&u, zorm.Fields("id", "name", "age"))

	u2 := User{ID: 1, Name: "alice2", Age: 21}
	_, _ = t.Insert(&u2, zorm.Fields("id", "name", "age"), zorm.OnConflictDoUpdateSet([]string{"id"}, []string{"name", "age"}))
}
