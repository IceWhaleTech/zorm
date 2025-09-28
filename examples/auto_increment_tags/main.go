package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

// 使用新的自增主键标签
type User struct {
	ID   int64  `zorm:"id,auto_incr"` // 自增主键
	Name string `zorm:"name"`
	Age  int    `zorm:"age"`
}

// 使用自增标签
type Product struct {
	ID    int64   `zorm:"id,auto_incr"` // 自增主键
	Name  string  `zorm:"name"`
	Price float64 `zorm:"price"`
}

// 使用自增标签
type Order struct {
	ID     int64  `zorm:"id,auto_incr"` // 自增主键
	UserID int64  `zorm:"user_id"`
	Amount int    `zorm:"amount"`
	Status string `zorm:"status"`
}

// 使用自增标签
type Category struct {
	ID   int64  `zorm:"id,auto_incr"` // 自增主键
	Name string `zorm:"name"`
}

// 向后兼容：使用旧的 ZormLastId 字段
type LegacyUser struct {
	ZormLastId int64  // 旧的字段名
	Name       string `zorm:"name"`
	Age        int    `zorm:"age"`
}

func main() {
	// 打开数据库连接
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 创建表
	createTables(db)

	// 测试自增主键功能
	testAutoIncrement(db)
}

func createTables(db *sql.DB) {
	// 创建用户表
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			age INTEGER
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建产品表
	_, err = db.Exec(`
		CREATE TABLE products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			price REAL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建订单表
	_, err = db.Exec(`
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			amount INTEGER,
			status TEXT
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建分类表
	_, err = db.Exec(`
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建旧格式表
	_, err = db.Exec(`
		CREATE TABLE legacy_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			age INTEGER
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func testAutoIncrement(db *sql.DB) {
	fmt.Println("=== 测试自增主键 struct tags 功能 ===")

	// 1. 测试新的自增主键标签
	fmt.Println("\n1. 测试新的自增主键标签:")
	user := User{Name: "Alice", Age: 25}
	tbl := zorm.Table(db, "users")
	n, err := tbl.Insert(&user)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入用户成功，影响行数: %d\n", n)
	fmt.Printf("用户 ID: %d, 姓名: %s, 年龄: %d\n", user.ID, user.Name, user.Age)

	// 2. 测试简化的自增标签
	fmt.Println("\n2. 测试简化的自增标签:")
	product := Product{Name: "iPhone", Price: 999.99}
	tbl = zorm.Table(db, "products")
	n, err = tbl.Insert(&product)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入产品成功，影响行数: %d\n", n)
	fmt.Printf("产品 ID: %d, 名称: %s, 价格: %.2f\n", product.ID, product.Name, product.Price)

	// 3. 测试主键+自增标签
	fmt.Println("\n3. 测试主键+自增标签:")
	order := Order{UserID: user.ID, Amount: 100, Status: "pending"}
	tbl = zorm.Table(db, "orders")
	n, err = tbl.Insert(&order)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入订单成功，影响行数: %d\n", n)
	fmt.Printf("订单 ID: %d, 用户ID: %d, 金额: %d, 状态: %s\n", order.ID, order.UserID, order.Amount, order.Status)

	// 4. 测试向后兼容性
	fmt.Println("\n4. 测试向后兼容性:")
	legacyUser := LegacyUser{Name: "Bob", Age: 30}
	tbl = zorm.Table(db, "legacy_users")
	n, err = tbl.Insert(&legacyUser)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入旧格式用户成功，影响行数: %d\n", n)
	fmt.Printf("用户 ID: %d, 姓名: %s, 年龄: %d\n", legacyUser.ZormLastId, legacyUser.Name, legacyUser.Age)

	// 5. 测试非指针类型插入
	fmt.Println("\n5. 测试非指针类型插入:")
	user2 := User{Name: "Charlie", Age: 35}
	n, err = tbl.Insert(user2) // 注意：这里传入的是值而不是指针
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入非指针用户成功，影响行数: %d\n", n)

	// 6. 测试批量插入
	fmt.Println("\n6. 测试批量插入:")
	users := []User{
		{Name: "David", Age: 28},
		{Name: "Eve", Age: 32},
		{Name: "Frank", Age: 29},
	}
	n, err = tbl.Insert(&users)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("批量插入用户成功，影响行数: %d\n", n)
	for i, u := range users {
		fmt.Printf("  用户 %d: ID=%d, 姓名=%s, 年龄=%d\n", i+1, u.ID, u.Name, u.Age)
	}

	// 7. 验证数据
	fmt.Println("\n7. 验证插入的数据:")
	var allUsers []User
	tbl = zorm.Table(db, "users") // 重新获取表对象
	n, err = tbl.Select(&allUsers, zorm.Fields("*"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("查询到 %d 个用户:\n", n)
	for _, u := range allUsers {
		fmt.Printf("  ID: %d, 姓名: %s, 年龄: %d\n", u.ID, u.Name, u.Age)
	}

	// 测试自增标签
	fmt.Println("\n--- 测试自增标签 ---")
	categoryTbl := zorm.Table(db, "categories")
	category := Category{Name: "电子产品"}
	n, err = categoryTbl.Insert(&category)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("插入分类成功，影响行数: %d，生成的ID: %d\n", n, category.ID)

	// 查询分类
	var allCategories []Category
	n, err = categoryTbl.Select(&allCategories, zorm.Fields("*"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("查询到 %d 个分类:\n", n)
	for _, c := range allCategories {
		fmt.Printf("  ID: %d, 名称: %s\n", c.ID, c.Name)
	}
}
