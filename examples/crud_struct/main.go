package main

import (
	"database/sql"
	"fmt"
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

	u := User{Name: "alice", Age: 20}
	_, _ = t.Insert(&u)

	u2 := User{Name: "bob", Age: 30}
	_, _ = t.Insert(&u2)

	// select with reuse
	var list []User
	_, _ = t.Reuse().Select(&list, zorm.Where(zorm.Gt("age", 18)))
	fmt.Printf("users=%+v\n", list)

	// update
	u.Age = 21
	_, _ = t.Update(&u, zorm.Where(zorm.Eq("id", u.ID)))

	// delete
	_, _ = t.Delete(zorm.Where(zorm.Eq("id", u2.ID)))
}
