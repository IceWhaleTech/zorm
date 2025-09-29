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

// User represents a user model with simplified tags
type User struct {
	ID        int64  `zorm:"auto_incr"` // Only auto_incr tag supported
	Name      string // No tags needed
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
	IsActive  bool
}

// Product represents a product model
type Product struct {
	ID          int64 `zorm:"auto_incr"`
	Name        string
	Price       float64
	Description string
	Category    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Order represents an order model
type Order struct {
	ID        int64 `zorm:"auto_incr"`
	UserID    int64
	ProductID int64
	Quantity  int
	Total     float64
	Status    string
	CreatedAt time.Time
}

func main() {
	// Open database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	fmt.Println("=== Zorm DDL and Schema Management Example ===")

	// Example 1: Create tables using atomic DDL commands
	fmt.Println("\n1. Creating tables with atomic DDL commands...")

	// Create User table
	userTableCmd := &zorm.CreateTableCommand{
		TableName: "users",
		Columns: []*zorm.ColumnDef{
			{Name: "id", Type: "INTEGER", AutoIncrement: true},
			{Name: "name", Type: "TEXT", Nullable: false},
			{Name: "email", Type: "TEXT", Nullable: false},
			{Name: "created_at", Type: "DATETIME", Nullable: false, DefaultValue: "CURRENT_TIMESTAMP"},
			{Name: "updated_at", Type: "DATETIME", Nullable: false, DefaultValue: "CURRENT_TIMESTAMP"},
			{Name: "is_active", Type: "BOOLEAN", Nullable: false, DefaultValue: "1"},
		},
		PrimaryKey: []string{"id"},
	}

	err = userTableCmd.Execute(ctx, db)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}
	fmt.Println("✓ Users table created")

	// Create Product table
	productTableCmd := &zorm.CreateTableCommand{
		TableName: "products",
		Columns: []*zorm.ColumnDef{
			{Name: "id", Type: "INTEGER", AutoIncrement: true},
			{Name: "name", Type: "TEXT", Nullable: false},
			{Name: "price", Type: "REAL", Nullable: false},
			{Name: "description", Type: "TEXT", Nullable: true},
			{Name: "category", Type: "TEXT", Nullable: false},
			{Name: "created_at", Type: "DATETIME", Nullable: false, DefaultValue: "CURRENT_TIMESTAMP"},
			{Name: "updated_at", Type: "DATETIME", Nullable: false, DefaultValue: "CURRENT_TIMESTAMP"},
		},
		PrimaryKey: []string{"id"},
	}

	err = productTableCmd.Execute(ctx, db)
	if err != nil {
		log.Fatal("Failed to create products table:", err)
	}
	fmt.Println("✓ Products table created")

	// Example 2: Create indexes using atomic commands
	fmt.Println("\n2. Creating indexes with atomic commands...")

	userEmailIndex := &zorm.CreateIndexCommand{
		IndexName: "idx_users_email",
		TableName: "users",
		Columns:   []string{"email"},
		Unique:    true,
	}

	err = userEmailIndex.Execute(ctx, db)
	if err != nil {
		log.Fatal("Failed to create email index:", err)
	}
	fmt.Println("✓ Email index created")

	productCategoryIndex := &zorm.CreateIndexCommand{
		IndexName: "idx_products_category",
		TableName: "products",
		Columns:   []string{"category"},
		Unique:    false,
	}

	err = productCategoryIndex.Execute(ctx, db)
	if err != nil {
		log.Fatal("Failed to create category index:", err)
	}
	fmt.Println("✓ Category index created")

	// Example 3: Use atomic table creation with DDL manager
	fmt.Println("\n3. Using atomic table creation with DDL manager...")

	// Create audit logger and telemetry collector
	auditLogger := zorm.NewJSONAuditLogger()
	telemetryCollector := zorm.NewDefaultTelemetryCollector()

	// Create auditable database using chain method
	auditableDB := zorm.Table(db, "users").Audit(auditLogger, telemetryCollector).DB
	ddlManager := zorm.NewDDLManager(auditableDB, auditLogger)

	// Create tables using the new atomic system
	err = ddlManager.CreateTables(ctx, &User{}, &Product{}, &Order{})
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}
	fmt.Println("✓ Atomic table creation completed")

	// Example 4: Perform CRUD operations with audit logging
	fmt.Println("\n4. Performing CRUD operations with audit logging...")

	// Insert users with audit logging
	userTable := zorm.Table(db, "users").Audit(auditLogger, telemetryCollector)

	users := []User{
		{Name: "Alice", Email: "alice@example.com", IsActive: true},
		{Name: "Bob", Email: "bob@example.com", IsActive: true},
		{Name: "Charlie", Email: "charlie@example.com", IsActive: false},
	}

	for _, user := range users {
		user.CreatedAt = time.Now()
		user.UpdatedAt = time.Now()

		_, err := userTable.Insert(&user)
		if err != nil {
			log.Fatal("Failed to insert user:", err)
		}
	}
	fmt.Println("✓ Users inserted")

	// Insert products with audit logging
	productTable := zorm.Table(db, "products").Audit(auditLogger, telemetryCollector)

	products := []Product{
		{Name: "Laptop", Price: 999.99, Description: "High-performance laptop", Category: "Electronics"},
		{Name: "Mouse", Price: 29.99, Description: "Wireless mouse", Category: "Electronics"},
		{Name: "Book", Price: 19.99, Description: "Programming book", Category: "Books"},
	}

	for _, product := range products {
		product.CreatedAt = time.Now()
		product.UpdatedAt = time.Now()

		_, err := productTable.Insert(&product)
		if err != nil {
			log.Fatal("Failed to insert product:", err)
		}
	}
	fmt.Println("✓ Products inserted")

	// Query with audit logging
	fmt.Println("\n5. Querying data with audit logging...")

	var allUsers []User
	count, err := userTable.Select(&allUsers, zorm.Where(zorm.Eq("is_active", true)))
	if err != nil {
		log.Fatal("Failed to query users:", err)
	}
	fmt.Printf("✓ Found %d active users\n", count)

	var allProducts []Product
	count, err = productTable.Select(&allProducts, zorm.Where(zorm.Eq("category", "Electronics")))
	if err != nil {
		log.Fatal("Failed to query products:", err)
	}
	fmt.Printf("✓ Found %d electronics products\n", count)

	// Example 6: Show telemetry metrics
	fmt.Println("\n6. Telemetry metrics:")
	// Create auditableDB instance to get telemetry metrics
	auditableDB := zorm.NewAuditableDB(db, auditLogger, telemetryCollector)
	metrics := auditableDB.GetTelemetryMetrics()
	for key, value := range metrics {
		fmt.Printf("  %s: %v\n", key, value)
	}

	// Example 7: Alter table operations
	fmt.Println("\n7. Performing ALTER TABLE operations...")

	// Add a new column to users table
	addColumnCmd := &zorm.AlterTableCommand{
		TableName: "users",
		Operation: "ADD COLUMN",
		Column: &zorm.ColumnDef{
			Name:         "phone",
			Type:         "TEXT",
			Nullable:     true,
			DefaultValue: "",
		},
	}

	err = addColumnCmd.Execute(ctx, auditableDB)
	if err != nil {
		log.Fatal("Failed to add phone column:", err)
	}
	fmt.Println("✓ Phone column added to users table")

	// Update a user with the new column
	updateCmd := &zorm.AlterTableCommand{
		TableName: "users",
		Operation: "MODIFY COLUMN",
		Column: &zorm.ColumnDef{
			Name:     "email",
			Type:     "TEXT",
			Nullable: false,
		},
	}

	err = updateCmd.Execute(ctx, auditableDB)
	if err != nil {
		log.Fatal("Failed to modify email column:", err)
	}
	fmt.Println("✓ Email column modified")

	fmt.Println("\n=== Example completed successfully! ===")
}
