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

// User represents a user model
type User struct {
	ID        int64 `zorm:"user_id,auto_incr"`
	Name      string
	Email     string
	CreatedAt time.Time
	IsActive  bool
}

func main() {
	// Open database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("=== Zorm Chain Audit Example ===")

	// Example 1: Basic table with default audit
	fmt.Println("\n1. Basic table with default audit...")
	userTable := zorm.Table(db, "users")

	// Enable audit with default loggers (chain style)
	userTable.Audit(nil, nil) // Uses default audit logger and telemetry collector

	// Create table
	createCmd := &zorm.CreateTableCommand{
		TableName: "users",
		Columns: []*zorm.ColumnDef{
			{Name: "user_id", Type: "INTEGER", AutoIncrement: true},
			{Name: "name", Type: "TEXT", Nullable: false},
			{Name: "email", Type: "TEXT", Nullable: false},
			{Name: "created_at", Type: "DATETIME", Nullable: false},
			{Name: "is_active", Type: "BOOLEAN", Nullable: false, DefaultValue: "true"},
		},
		PrimaryKey: []string{"user_id"},
	}

	err = createCmd.Execute(nil, db)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
	fmt.Println("✓ Table created with audit enabled")

	// Example 2: Insert with audit logging
	fmt.Println("\n2. Inserting data with audit logging...")
	users := []User{
		{Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now(), IsActive: true},
		{Name: "Bob", Email: "bob@example.com", CreatedAt: time.Now(), IsActive: true},
		{Name: "Charlie", Email: "charlie@example.com", CreatedAt: time.Now(), IsActive: false},
	}

	for _, user := range users {
		_, err := userTable.Insert(&user)
		if err != nil {
			log.Fatal("Failed to insert user:", err)
		}
	}
	fmt.Println("✓ Users inserted with audit logging")

	// Example 3: Query with audit logging
	fmt.Println("\n3. Querying data with audit logging...")
	var results []User
	count, err := userTable.Select(&results)
	if err != nil {
		log.Fatal("Failed to query users:", err)
	}
	fmt.Printf("✓ Found %d users with audit logging\n", count)

	// Example 4: Update with audit logging
	fmt.Println("\n4. Updating data with audit logging...")
	_, err = userTable.Update(zorm.V{"is_active": false}, zorm.Where(zorm.Eq("name", "Bob")))
	if err != nil {
		log.Fatal("Failed to update user:", err)
	}
	fmt.Println("✓ User updated with audit logging")

	// Example 5: Chain multiple options
	fmt.Println("\n5. Chaining multiple options...")
	advancedTable := zorm.Table(db, "users").
		Debug().        // Enable debug mode
		Audit(nil, nil) // Enable audit logging

	// Query with both debug and audit
	var debugResults []User
	_, err = advancedTable.Select(&debugResults, zorm.Where(zorm.Eq("is_active", true)))
	if err != nil {
		log.Fatal("Failed to query with debug and audit:", err)
	}
	fmt.Printf("✓ Found %d active users with debug and audit logging\n", len(debugResults))

	// Example 6: Custom audit logger
	fmt.Println("\n6. Using custom audit logger...")
	customLogger := &CustomAuditLogger{}
	customCollector := zorm.NewDefaultTelemetryCollector()

	customTable := zorm.Table(db, "users").Audit(customLogger, customCollector)

	// Query with custom audit
	var customResults []User
	_, err = customTable.Select(&customResults, zorm.Where(zorm.Eq("name", "Alice")))
	if err != nil {
		log.Fatal("Failed to query with custom audit:", err)
	}
	fmt.Printf("✓ Found %d users with custom audit logging\n", len(customResults))

	fmt.Println("\n=== Example completed successfully! ===")
	fmt.Println("\nKey points:")
	fmt.Println("- Use .Audit(nil, nil) for default audit logging")
	fmt.Println("- Chain .Debug().Audit() for multiple options")
	fmt.Println("- Pass custom loggers to .Audit(auditLogger, telemetryCollector)")
	fmt.Println("- All SQL operations are automatically audited")
}

// CustomAuditLogger implements custom audit logging
type CustomAuditLogger struct{}

func (l *CustomAuditLogger) LogAuditEvent(ctx context.Context, event *zorm.SQLAuditEvent) {
	fmt.Printf("[CUSTOM AUDIT] %s: %s (Duration: %v)\n",
		event.Operation, event.SQL, event.Duration)
}

func (l *CustomAuditLogger) LogTelemetryData(ctx context.Context, data *zorm.TelemetryData) {
	fmt.Printf("[CUSTOM TELEMETRY] %s: %v (Rows: %d)\n",
		data.Operation, data.Duration, data.RowsAffected)
}
