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

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		name TEXT,
		age INTEGER
	)`)

	t := zorm.Table(db, "users")

	// First insert
	u := User{ID: 1, Name: "Alice", Age: 18}
	_, _ = t.Insert(&u, zorm.Fields("id", "name", "age"), zorm.OnConflictDoUpdateSet([]string{"id"}, []string{"name", "age"}))

	// Second insert with same ID - will update using excluded values
	u2 := User{ID: 1, Name: "Bob", Age: 25}
	_, _ = t.Insert(&u2, zorm.Fields("id", "name", "age"), zorm.OnConflictDoUpdateSet([]string{"id"}, []string{"name", "age"}))

	// Verify the update
	var result User
	_, _ = t.Select(&result, zorm.Where("id = ?", 1))
	log.Printf("User after upsert: ID=%d, Name=%s, Age=%d", result.ID, result.Name, result.Age)
	// Output: User after upsert: ID=1, Name=Bob, Age=25
}
