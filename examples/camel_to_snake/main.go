package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

// UserProfile represents a user profile with camelCase fields
type UserProfile struct {
	ID           int64     `zorm:"user_id,auto_incr"` // Will use "user_id" as DB column
	FirstName    string    // Will become "first_name"
	LastName     string    // Will become "last_name"
	EmailAddress string    // Will become "email_address"
	PhoneNumber  string    // Will become "phone_number"
	CreatedAt    time.Time // Will become "created_at"
	UpdatedAt    time.Time // Will become "updated_at"
	IsActive     bool      // Will become "is_active"
	Password     string    `zorm:"-"` // Will be ignored
}

// ProductInfo represents a product with camelCase fields
type ProductInfo struct {
	ID          int64     `zorm:"product_id,auto_incr"` // Will use "product_id" as DB column
	ProductName string    // Will become "product_name"
	UnitPrice   float64   // Will become "unit_price"
	StockCount  int       // Will become "stock_count"
	CategoryID  int64     // Will become "category_id"
	CreatedAt   time.Time // Will become "created_at"
	UpdatedAt   time.Time // Will become "updated_at"
	IsAvailable bool      // Will become "is_available"
}

// OrderDetail represents an order with camelCase fields
type OrderDetail struct {
	ID            int64      `zorm:"order_id,auto_incr"` // Will use "order_id" as DB column
	OrderNumber   string     // Will become "order_number"
	CustomerID    int64      // Will become "customer_id"
	ProductID     int64      // Will become "product_id"
	Quantity      int        // Will become "quantity"
	UnitPrice     float64    // Will become "unit_price"
	TotalAmount   float64    // Will become "total_amount"
	OrderStatus   string     // Will become "order_status"
	OrderDate     time.Time  // Will become "order_date"
	ShippedDate   *time.Time // Will become "shipped_date" (nullable)
	DeliveredDate *time.Time // Will become "delivered_date" (nullable)
}

func main() {
	// Open database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	fmt.Println("=== Zorm CamelCase to Snake_case Example ===")

	// Example 1: Create tables using DDL manager (automatic camelCase to snake_case conversion)
	fmt.Println("\n1. Creating tables with automatic camelCase to snake_case conversion...")

	// Create audit logger and telemetry collector
	auditLogger := zorm.NewJSONAuditLogger()
	telemetryCollector := zorm.NewDefaultTelemetryCollector()

	// Create auditable database using chain method
	auditableDB := zorm.Table(db, "users").Audit(auditLogger, telemetryCollector).DB
	ddlManager := zorm.NewDDLManager(auditableDB, auditLogger)

	// Create tables using the new atomic system - field names will be automatically converted
	err = ddlManager.CreateTables(ctx, &UserProfile{}, &ProductInfo{}, &OrderDetail{})
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}
	fmt.Println("✓ Tables created with automatic field name conversion")

	// Example 2: Insert data using camelCase struct fields
	fmt.Println("\n2. Inserting data with camelCase struct fields...")

	// Insert user profiles with audit logging
	userTable := zorm.Table(db, "user_profiles").Audit(auditLogger, telemetryCollector)

	users := []UserProfile{
		{
			FirstName:    "John",
			LastName:     "Doe",
			EmailAddress: "john.doe@example.com",
			PhoneNumber:  "+1234567890",
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			FirstName:    "Jane",
			LastName:     "Smith",
			EmailAddress: "jane.smith@example.com",
			PhoneNumber:  "+0987654321",
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, user := range users {
		_, insertErr := userTable.Insert(&user)
		if insertErr != nil {
			log.Fatal("Failed to insert user:", insertErr)
		}
	}
	fmt.Println("✓ User profiles inserted")

	// Insert products with audit logging
	productTable := zorm.Table(db, "product_infos").Audit(auditLogger, telemetryCollector)

	products := []ProductInfo{
		{
			ProductName: "Laptop Computer",
			UnitPrice:   999.99,
			StockCount:  50,
			CategoryID:  1,
			IsAvailable: true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ProductName: "Wireless Mouse",
			UnitPrice:   29.99,
			StockCount:  100,
			CategoryID:  2,
			IsAvailable: true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, product := range products {
		_, insertErr := productTable.Insert(&product)
		if insertErr != nil {
			log.Fatal("Failed to insert product:", insertErr)
		}
	}
	fmt.Println("✓ Products inserted")

	// Insert orders with audit logging
	orderTable := zorm.Table(db, "order_details").Audit(auditLogger, telemetryCollector)

	now := time.Now()
	orders := []OrderDetail{
		{
			OrderNumber:   "ORD-001",
			CustomerID:    1,
			ProductID:     1,
			Quantity:      2,
			UnitPrice:     999.99,
			TotalAmount:   1999.98,
			OrderStatus:   "pending",
			OrderDate:     now,
			ShippedDate:   nil,
			DeliveredDate: nil,
		},
		{
			OrderNumber:   "ORD-002",
			CustomerID:    2,
			ProductID:     2,
			Quantity:      5,
			UnitPrice:     29.99,
			TotalAmount:   149.95,
			OrderStatus:   "shipped",
			OrderDate:     now.Add(-24 * time.Hour),
			ShippedDate:   &now,
			DeliveredDate: nil,
		},
	}

	for _, order := range orders {
		_, insertErr := orderTable.Insert(&order)
		if insertErr != nil {
			log.Fatal("Failed to insert order:", insertErr)
		}
	}
	fmt.Println("✓ Orders inserted")

	// Example 3: Query data using camelCase field names in Go struct
	fmt.Println("\n3. Querying data with camelCase field names...")

	// Query active users
	var activeUsers []UserProfile
	count, err := userTable.Select(&activeUsers, zorm.Where(zorm.Eq("is_active", true)))
	if err != nil {
		log.Fatal("Failed to query users:", err)
	}
	fmt.Printf("✓ Found %d active users\n", count)

	// Query available products
	var availableProducts []ProductInfo
	count, err = productTable.Select(&availableProducts, zorm.Where(zorm.Eq("is_available", true)))
	if err != nil {
		log.Fatal("Failed to query products:", err)
	}
	fmt.Printf("✓ Found %d available products\n", count)

	// Query pending orders
	var pendingOrders []OrderDetail
	count, err = orderTable.Select(&pendingOrders, zorm.Where(zorm.Eq("order_status", "pending")))
	if err != nil {
		log.Fatal("Failed to query orders:", err)
	}
	fmt.Printf("✓ Found %d pending orders\n", count)

	// Example 4: Update data using camelCase field names
	fmt.Println("\n4. Updating data with camelCase field names...")

	// Update user profile
	updateUser := UserProfile{
		FirstName:    "John",
		LastName:     "Doe",
		EmailAddress: "john.doe.updated@example.com",
		PhoneNumber:  "+1234567890",
		IsActive:     true,
		UpdatedAt:    time.Now(),
	}

	rowsAffected, err := userTable.Update(&updateUser, zorm.Where(zorm.Eq("id", 1)))
	if err != nil {
		log.Fatal("Failed to update user:", err)
	}
	fmt.Printf("✓ Updated %d user records\n", rowsAffected)

	// Example 5: Show the actual database schema (snake_case)
	fmt.Println("\n5. Database schema (showing snake_case field names):")

	// Query to show table structure
	rows, err := auditableDB.QueryContext(ctx, "PRAGMA table_info(user_profiles)")
	if err == nil {
		fmt.Println("User profiles table structure:")
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue sql.NullString
			var autoIncrement int

			err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk, &autoIncrement)
			if err != nil {
				log.Fatal("Failed to scan table info:", err)
			}

			fmt.Printf("  %s: %s (nullable: %t, pk: %t, auto_incr: %t)\n",
				name, dataType, notNull == 0, pk == 1, autoIncrement == 1)
		}
		rows.Close()
	}

	// Example 6: Show telemetry metrics
	fmt.Println("\n6. Telemetry metrics:")
	metrics := auditableDB.GetTelemetryMetrics()
	for key, value := range metrics {
		fmt.Printf("  %s: %v\n", key, value)
	}

	fmt.Println("\n=== Example completed successfully! ===")
	fmt.Println("\nKey points:")
	fmt.Println("- Go struct fields use camelCase (FirstName, LastName, etc.)")
	fmt.Println("- Database columns are automatically converted to snake_case (first_name, last_name, etc.)")
	fmt.Println("- Supported zorm tag formats:")
	fmt.Println("  * zorm:\"field_name,auto_incr\" - specify DB column name and mark as auto increment")
	fmt.Println("  * zorm:\"field_name\" - specify DB column name")
	fmt.Println("  * zorm:\"auto_incr\" - use converted field name and mark as auto increment")
	fmt.Println("  * zorm:\"-\" - ignore field")
	fmt.Println("  * empty tag - auto convert camelCase to snake_case")
}
