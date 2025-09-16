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

	_, _ = db.Exec(`create table users(id integer primary key, name text, age integer);
	insert into users(id,name,age) values (1,'alice',20),(2,'bob',30);`)

	t := zorm.Table(db, "users")

	var u User
	n, err := t.Select(&u, zorm.Where(zorm.Eq("id", 1)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("rows=%d user=%+v\n", n, u)

	var ids []int64
	n, err = t.Select(&ids, zorm.Fields("id"), zorm.Where(zorm.Gt("age", 18)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("rows=%d ids=%v\n", n, ids)
}
