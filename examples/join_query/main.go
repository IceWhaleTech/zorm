package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID   int64  `zorm:"id,auto_incr"` // 自增主键
	Name string `zorm:"name"`
	Age  int    `zorm:"age"`
}

type Order struct {
	ID     int64  `zorm:"id,auto_incr"` // 自增主键
	UserID int64  `zorm:"user_id"`
	Amount int    `zorm:"amount"`
	Status string `zorm:"status"`
}

type UserOrder struct {
	UserID   int64  `zorm:"users.id"`
	UserName string `zorm:"users.name"`
	OrderID  int64  `zorm:"orders.id"`
	Amount   int    `zorm:"orders.amount"`
	Status   string `zorm:"orders.status"`
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

	// 插入测试数据
	insertTestData(db)

	// 测试联表查询
	testJoins(db)
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

	// 创建订单表
	_, err = db.Exec(`
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			amount INTEGER,
			status TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func insertTestData(db *sql.DB) {
	// 插入用户数据
	users := []User{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 35},
	}

	tbl := zorm.Table(db, "users")
	for _, user := range users {
		_, err := tbl.Insert(&user)
		if err != nil {
			log.Fatal(err)
		}
	}

	// 插入订单数据
	orders := []Order{
		{UserID: 1, Amount: 100, Status: "completed"},
		{UserID: 1, Amount: 200, Status: "pending"},
		{UserID: 2, Amount: 150, Status: "completed"},
		{UserID: 3, Amount: 300, Status: "cancelled"},
	}

	tbl = zorm.Table(db, "orders")
	for _, order := range orders {
		_, err := tbl.Insert(&order)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func testJoins(db *sql.DB) {
	fmt.Println("=== 测试联表查询功能 ===")

	// 1. 测试字符串格式的 ON 条件
	fmt.Println("\n1. 字符串格式的 ON 条件:")
	var results1 []UserOrder
	tbl := zorm.Table(db, "users")
	n, err := tbl.Select(&results1,
		zorm.Fields("users.id", "users.name", "orders.id", "orders.amount", "orders.status"),
		zorm.InnerJoin("orders", "users.id = orders.user_id"),
		zorm.Where("orders.status = ?", "completed"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("找到 %d 条记录:\n", n)
	for _, result := range results1 {
		fmt.Printf("  用户: %s (ID: %d), 订单: %d, 金额: %d, 状态: %s\n",
			result.UserName, result.UserID, result.OrderID, result.Amount, result.Status)
	}

	// 2. 测试条件对象格式的 ON 条件
	fmt.Println("\n2. 条件对象格式的 ON 条件:")
	var results2 []UserOrder
	n, err = tbl.Select(&results2,
		zorm.Fields("users.id", "users.name", "orders.id", "orders.amount", "orders.status"),
		zorm.LeftJoin("orders", zorm.Eq("users.id", zorm.U("orders.user_id"))),
		zorm.Where("users.age > ?", 25),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("找到 %d 条记录:\n", n)
	for _, result := range results2 {
		fmt.Printf("  用户: %s (ID: %d), 订单: %d, 金额: %d, 状态: %s\n",
			result.UserName, result.UserID, result.OrderID, result.Amount, result.Status)
	}

	// 3. 测试复杂的 ON 条件
	fmt.Println("\n3. 复杂的 ON 条件:")
	var results3 []UserOrder
	n, err = tbl.Select(&results3,
		zorm.Fields("users.id", "users.name", "orders.id", "orders.amount", "orders.status"),
		zorm.InnerJoin("orders",
			zorm.And(
				zorm.Eq("users.id", zorm.U("orders.user_id")),
				zorm.Neq("orders.status", "cancelled"),
			),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("找到 %d 条记录:\n", n)
	for _, result := range results3 {
		fmt.Printf("  用户: %s (ID: %d), 订单: %d, 金额: %d, 状态: %s\n",
			result.UserName, result.UserID, result.OrderID, result.Amount, result.Status)
	}

	// 4. 测试非指针类型支持
	fmt.Println("\n4. 测试非指针类型支持:")
	user := User{Name: "David", Age: 28}
	tbl = zorm.Table(db, "users")
	n, err = tbl.Insert(&user) // 注意：这里传入的是指针
	if err != nil {
		fmt.Printf("插入用户失败: %v\n", err)
	} else {
		fmt.Printf("成功插入非指针用户，影响行数: %d\n", n)
	}

	// 5. 测试事务支持
	fmt.Println("\n5. 测试事务支持:")
	tx, err := zorm.Begin(db)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback() // 测试回滚

	txTbl := zorm.Table(tx, "users")
	_, err = txTbl.Insert(&User{Name: "Eve", Age: 32})
	if err != nil {
		log.Fatal(err)
	}

	// 故意回滚事务
	err = tx.Rollback()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("事务已回滚")

	// 验证数据没有插入
	var count int64
	n, err = tbl.Select(&count, zorm.Fields("count(*)"), zorm.Where("name = ?", "Eve"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("回滚后，Eve 的记录数: %d\n", count)
}
