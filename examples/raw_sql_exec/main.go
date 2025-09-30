package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	z "github.com/IceWhaleTech/zorm"
)

func main() {
	// 打开数据库连接
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 创建表
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建 zorm table 实例
	t := z.Table(db, "users")

	fmt.Println("=== 原生SQL执行示例 ===")

	// 1. 插入数据
	fmt.Println("\n1. 插入数据:")
	affected, err := t.Exec("INSERT INTO users (name, email) VALUES (?, ?)", "Alice", "alice@example.com")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入了 %d 行数据\n", affected)

	affected, err = t.Exec("INSERT INTO users (name, email) VALUES (?, ?)", "Bob", "bob@example.com")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入了 %d 行数据\n", affected)

	// 2. 更新数据
	fmt.Println("\n2. 更新数据:")
	affected, err = t.Exec("UPDATE users SET status = ? WHERE name = ?", "inactive", "Alice")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("更新了 %d 行数据\n", affected)

	// 3. 批量更新
	fmt.Println("\n3. 批量更新:")
	affected, err = t.Exec("UPDATE users SET created_at = ? WHERE status = ?", time.Now(), "active")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("更新了 %d 行数据\n", affected)

	// 4. 删除数据
	fmt.Println("\n4. 删除数据:")
	affected, err = t.Exec("DELETE FROM users WHERE status = ?", "inactive")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("删除了 %d 行数据\n", affected)

	// 5. 创建索引
	fmt.Println("\n5. 创建索引:")
	_, err = t.Exec("CREATE INDEX idx_email ON users (email)")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("索引创建成功")

	// 6. 查询数据验证结果
	fmt.Println("\n6. 查询结果:")
	rows, err := db.Query("SELECT id, name, email, status FROM users")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, email, status string
		err := rows.Scan(&id, &name, &email, &status)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %d, Name: %s, Email: %s, Status: %s\n", id, name, email, status)
	}

	// 7. 使用 Debug 模式
	fmt.Println("\n7. Debug 模式示例:")
	debugT := t.Debug()
	affected, err = debugT.Exec("UPDATE users SET status = ? WHERE id = ?", "verified", 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Debug 模式更新了 %d 行数据\n", affected)

	fmt.Println("\n=== 示例完成 ===")
}
