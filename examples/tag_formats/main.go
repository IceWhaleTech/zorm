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

// TestModel demonstrates different zorm tag formats
type TestModel struct {
	// Format 1: field_name,auto_incr - specify DB column name and mark as auto increment
	ID int64 `zorm:"test_id,auto_incr"`

	// Format 2: field_name - specify DB column name
	CustomName string `zorm:"custom_field"`

	// Format 3: auto_incr - use converted field name and mark as auto increment
	// This would be "another_id" in database
	AnotherID int64 `zorm:"auto_incr"`

	// Format 4: empty tag - auto convert camelCase to snake_case
	// This becomes "first_name" in database
	FirstName string

	// Format 5: ignore field
	Password string `zorm:"-"`

	// Format 6: more camelCase examples
	EmailAddress    string    // becomes "email_address"
	PhoneNumber     string    // becomes "phone_number"
	IsActive        bool      // becomes "is_active"
	CreatedAt       time.Time // becomes "created_at"
	LastLoginTime   time.Time // becomes "last_login_time"
	UserProfileData string    // becomes "user_profile_data"
}

func main() {
	// Open database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	fmt.Println("=== Zorm Tag Formats Example ===")

	// Example 1: Create table using DDL manager
	fmt.Println("\n1. Creating table with different tag formats...")

	// Create audit logger and telemetry collector
	auditLogger := zorm.NewJSONAuditLogger()
	telemetryCollector := zorm.NewDefaultTelemetryCollector()

	// Create auditable database using chain method
	auditableDB := zorm.Table(db, "users").Audit(auditLogger, telemetryCollector).DB
	ddlManager := zorm.NewDDLManager(auditableDB, auditLogger)

	// Create table using the new atomic system
	err = ddlManager.CreateTables(ctx, &TestModel{})
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
	fmt.Println("✓ Table created with different tag formats")

	// Example 2: Insert data
	fmt.Println("\n2. Inserting data...")

	table := zorm.Table(db, "test_models").Audit(auditLogger, telemetryCollector)

	testData := TestModel{
		CustomName:      "Test User",
		FirstName:       "John",
		EmailAddress:    "john@example.com",
		PhoneNumber:     "+1234567890",
		IsActive:        true,
		CreatedAt:       time.Now(),
		LastLoginTime:   time.Now(),
		UserProfileData: "Some profile data",
		// Password is ignored due to zorm:"-"
		// ID and AnotherID will be auto-generated
	}

	_, err = table.Insert(&testData)
	if err != nil {
		log.Fatal("Failed to insert data:", err)
	}
	fmt.Println("✓ Data inserted successfully")

	// Example 3: Query data
	fmt.Println("\n3. Querying data...")

	var results []TestModel
	count, err := table.Select(&results)
	if err != nil {
		log.Fatal("Failed to query data:", err)
	}
	fmt.Printf("✓ Found %d records\n", count)

	// Display the results
	for i, result := range results {
		fmt.Printf("Record %d:\n", i+1)
		fmt.Printf("  ID: %d (from test_id column)\n", result.ID)
		fmt.Printf("  CustomName: %s (from custom_field column)\n", result.CustomName)
		fmt.Printf("  AnotherID: %d (from another_id column)\n", result.AnotherID)
		fmt.Printf("  FirstName: %s (from first_name column)\n", result.FirstName)
		fmt.Printf("  EmailAddress: %s (from email_address column)\n", result.EmailAddress)
		fmt.Printf("  PhoneNumber: %s (from phone_number column)\n", result.PhoneNumber)
		fmt.Printf("  IsActive: %t (from is_active column)\n", result.IsActive)
		fmt.Printf("  CreatedAt: %s (from created_at column)\n", result.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  LastLoginTime: %s (from last_login_time column)\n", result.LastLoginTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("  UserProfileData: %s (from user_profile_data column)\n", result.UserProfileData)
		fmt.Printf("  Password: %s (ignored field)\n", result.Password)
		fmt.Println()
	}

	// Example 4: Show actual database schema
	fmt.Println("4. Database schema (showing actual column names):")

	rows, err := auditableDB.QueryContext(ctx, "PRAGMA table_info(test_models)")
	if err == nil {
		fmt.Println("Column mapping:")
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

	fmt.Println("\n=== Example completed successfully! ===")
	fmt.Println("\nTag format summary:")
	fmt.Println("✓ zorm:\"test_id,auto_incr\" → test_id column (auto increment)")
	fmt.Println("✓ zorm:\"custom_field\" → custom_field column")
	fmt.Println("✓ zorm:\"auto_incr\" → another_id column (auto increment, converted from AnotherID)")
	fmt.Println("✓ (empty tag) → snake_case conversion (FirstName → first_name)")
	fmt.Println("✓ zorm:\"-\" → ignored (Password field)")
}
