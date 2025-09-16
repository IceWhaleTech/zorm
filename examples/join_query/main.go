package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

type Info struct {
	ID   int64  `zorm:"t_usr.id"`
	Name string `zorm:"t_usr.name"`
	Tag  string `zorm:"t_tag.tag"`
}

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`create table t_usr(id integer primary key, name text);
	create table t_tag(id integer primary key, tag text);
	insert into t_usr(id,name) values(1,'alice');
	insert into t_tag(id,tag) values(1,'vip');`)

	t := zorm.Table(db, "t_usr")
	var o Info
	_, _ = t.Select(&o, zorm.Join("join t_tag on t_usr.id=t_tag.id"), zorm.Where(zorm.Eq("t_usr.id", 1)))
	fmt.Printf("info=%+v\n", o)
}
