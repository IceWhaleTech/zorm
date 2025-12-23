/*
   zorm is a better orm library for Go.

  Copyright (c) 2019 <http://ez8.co> <orca.zhang@yahoo.com>

  This library is released under the MIT License.
  Please see LICENSE file or visit https://github.com/IceWhaleTech/zorm for details.
*/

// Package zorm provides atomic DDL operations and database schema management.
package zorm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/modern-go/reflect2"
)

// DDLCommand represents a single atomic DDL operation
type DDLCommand interface {
	Execute(ctx context.Context, db ZormDBIFace) error
	SQL() string
	Description() string
}

// AlterTableCommand represents an ALTER TABLE command
type AlterTableCommand struct {
	TableName string
	Operation string // ADD, DROP, MODIFY, RENAME
	Column    *ColumnDef
	OldName   string // for RENAME operations
	NewName   string // for RENAME operations
}

func (c *AlterTableCommand) Execute(ctx context.Context, db ZormDBIFace) error {
	_, err := db.ExecContext(ctx, c.SQL())
	return err
}

func (c *AlterTableCommand) SQL() string {
	sb := strings.Builder{}
	sb.WriteString("ALTER TABLE `")
	sb.WriteString(c.TableName)
	sb.WriteString("` ")
	sb.WriteString(c.Operation)
	sb.WriteString(" ")

	switch c.Operation {
	case "ADD COLUMN":
		sb.WriteString("`")
		sb.WriteString(c.Column.Name)
		sb.WriteString("` ")
		sb.WriteString(c.Column.Type)
		if !c.Column.Nullable {
			sb.WriteString(" NOT NULL")
		}
		if c.Column.DefaultValue != "" {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(c.Column.DefaultValue)
		}
	case "DROP COLUMN":
		sb.WriteString("`")
		sb.WriteString(c.Column.Name)
		sb.WriteString("`")
	case "MODIFY COLUMN":
		sb.WriteString("`")
		sb.WriteString(c.Column.Name)
		sb.WriteString("` ")
		sb.WriteString(c.Column.Type)
		if !c.Column.Nullable {
			sb.WriteString(" NOT NULL")
		}
		if c.Column.DefaultValue != "" {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(c.Column.DefaultValue)
		}
	case "RENAME COLUMN":
		sb.WriteString("`")
		sb.WriteString(c.OldName)
		sb.WriteString("` TO `")
		sb.WriteString(c.NewName)
		sb.WriteString("`")
	}

	return sb.String()
}

func (c *AlterTableCommand) Description() string {
	return fmt.Sprintf("ALTER TABLE %s %s", c.TableName, c.Operation)
}

// CreateIndexCommand represents a CREATE INDEX command
type CreateIndexCommand struct {
	IndexName string
	TableName string
	Columns   []string
	Unique    bool
}

func (c *CreateIndexCommand) Execute(ctx context.Context, db ZormDBIFace) error {
	_, err := db.ExecContext(ctx, c.SQL())
	return err
}

func (c *CreateIndexCommand) SQL() string {
	sb := strings.Builder{}
	sb.WriteString("CREATE ")
	if c.Unique {
		sb.WriteString("UNIQUE ")
	}
	sb.WriteString("INDEX `")
	sb.WriteString(c.IndexName)
	sb.WriteString("` ON `")
	sb.WriteString(c.TableName)
	sb.WriteString("` (")

	for i, col := range c.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("`")
		sb.WriteString(col)
		sb.WriteString("`")
	}
	sb.WriteString(")")

	return sb.String()
}

func (c *CreateIndexCommand) Description() string {
	return fmt.Sprintf("CREATE INDEX %s ON %s", c.IndexName, c.TableName)
}

// DropIndexCommand represents a DROP INDEX command
type DropIndexCommand struct {
	IndexName string
	TableName string
}

func (c *DropIndexCommand) Execute(ctx context.Context, db ZormDBIFace) error {
	_, err := db.ExecContext(ctx, c.SQL())
	return err
}

func (c *DropIndexCommand) SQL() string {
	return fmt.Sprintf("DROP INDEX `%s`", c.IndexName)
}

func (c *DropIndexCommand) Description() string {
	return fmt.Sprintf("DROP INDEX %s", c.IndexName)
}

// CreateTableCommand represents a CREATE TABLE command
type CreateTableCommand struct {
	TableName  string
	Columns    []*ColumnDef
	PrimaryKey []string
	Engine     string
	Charset    string
	Collate    string
}

func (c *CreateTableCommand) Execute(ctx context.Context, db ZormDBIFace) error {
	_, err := db.ExecContext(ctx, c.SQL())
	return err
}

func (c *CreateTableCommand) SQL() string {
	sb := strings.Builder{}
	sb.WriteString("CREATE TABLE IF NOT EXISTS `")
	sb.WriteString(c.TableName)
	sb.WriteString("` (")

	// Add columns
	for i, col := range c.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("\n  `")
		sb.WriteString(col.Name)
		sb.WriteString("` ")
		sb.WriteString(col.Type)

		if col.AutoIncrement {
			sb.WriteString(" PRIMARY KEY AUTOINCREMENT")
		} else if !col.Nullable {
			sb.WriteString(" NOT NULL")
		}

		if col.DefaultValue != "" && !col.AutoIncrement {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(col.DefaultValue)
		}
	}

	// Add primary key constraint if not auto-increment
	if len(c.PrimaryKey) > 0 && !c.hasAutoIncrementColumn() {
		sb.WriteString(",\n  PRIMARY KEY (")
		for i, col := range c.PrimaryKey {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("`")
			sb.WriteString(col)
			sb.WriteString("`")
		}
		sb.WriteString(")")
	}

	sb.WriteString("\n)")

	// Add table options
	if c.Engine != "" {
		sb.WriteString(" ENGINE=")
		sb.WriteString(c.Engine)
	}
	if c.Charset != "" {
		sb.WriteString(" DEFAULT CHARSET=")
		sb.WriteString(c.Charset)
	}
	if c.Collate != "" {
		sb.WriteString(" COLLATE=")
		sb.WriteString(c.Collate)
	}

	return sb.String()
}

func (c *CreateTableCommand) hasAutoIncrementColumn() bool {
	for _, col := range c.Columns {
		if col.AutoIncrement {
			return true
		}
	}
	return false
}

func (c *CreateTableCommand) Description() string {
	return fmt.Sprintf("CREATE TABLE %s", c.TableName)
}

// DropTableCommand represents a DROP TABLE command
type DropTableCommand struct {
	TableName string
	IfExists  bool
}

func (c *DropTableCommand) Execute(ctx context.Context, db ZormDBIFace) error {
	_, err := db.ExecContext(ctx, c.SQL())
	return err
}

func (c *DropTableCommand) SQL() string {
	sql := "DROP TABLE"
	if c.IfExists {
		sql += " IF EXISTS"
	}
	sql += " `" + c.TableName + "`"
	return sql
}

func (c *DropTableCommand) Description() string {
	return fmt.Sprintf("DROP TABLE %s", c.TableName)
}

// ColumnDef represents a column definition
type ColumnDef struct {
	Name          string
	Type          string
	Nullable      bool
	DefaultValue  string
	AutoIncrement bool
	Comment       string
}

// SchemaInfo represents current database schema information
type SchemaInfo struct {
	Tables map[string]*TableInfo
}

// TableInfo represents table schema information
type TableInfo struct {
	Name    string
	Columns map[string]*ColumnDef
	Indexes map[string]*IndexInfo
}

// IndexInfo represents index information
type IndexInfo struct {
	Name    string
	Columns []string
	Unique  bool
	Primary bool
}

// SchemaPlan represents a plan for database schema changes
type SchemaPlan struct {
	Commands []DDLCommand
	Summary  string
}

// DDLManager manages DDL operations with audit logging
type DDLManager struct {
	db     ZormDBIFace
	logger DDLLogger
}

// DDLLogger interface for logging DDL operations
type DDLLogger interface {
	LogCommand(ctx context.Context, cmd DDLCommand, err error)
	LogSchemaChange(ctx context.Context, plan *SchemaPlan, err error)
}

// DefaultDDLLogger is a simple console logger
type DefaultDDLLogger struct{}

func (l *DefaultDDLLogger) LogCommand(ctx context.Context, cmd DDLCommand, err error) {
	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}
	fmt.Printf("[DDL] %s - %s: %s\n", status, cmd.Description(), cmd.SQL())
	if err != nil {
		fmt.Printf("[DDL] Error: %v\n", err)
	}
}

func (l *DefaultDDLLogger) LogSchemaChange(ctx context.Context, plan *SchemaPlan, err error) {
	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}
	fmt.Printf("[SCHEMA] %s - %s\n", status, plan.Summary)
	fmt.Printf("[SCHEMA] Commands: %d\n", len(plan.Commands))
}

// NewDDLManager creates a new DDL manager
func NewDDLManager(db ZormDBIFace, logger DDLLogger) *DDLManager {
	if logger == nil {
		logger = &DefaultDDLLogger{}
	}
	return &DDLManager{
		db:     db,
		logger: logger,
	}
}

// GetCurrentSchema retrieves current database schema
func (dm *DDLManager) GetCurrentSchema(ctx context.Context) (*SchemaInfo, error) {
	schema := &SchemaInfo{
		Tables: make(map[string]*TableInfo),
	}

	// Get all tables
	tables, err := dm.getTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		tableInfo, err := dm.getTableInfo(ctx, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables[tableName] = tableInfo
	}

	return schema, nil
}

// getTables retrieves all table names
func (dm *DDLManager) getTables(ctx context.Context) ([]string, error) {
	var tables []string

	// Try SQLite first
	rows, err := dm.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
		return tables, nil
	}

	// Fallback to MySQL/PostgreSQL
	rows, err = dm.db.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()")
	if err != nil {
		// Try PostgreSQL
		rows, err = dm.db.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, nil
}

// getTableInfo retrieves table schema information
func (dm *DDLManager) getTableInfo(ctx context.Context, tableName string) (*TableInfo, error) {
	tableInfo := &TableInfo{
		Name:    tableName,
		Columns: make(map[string]*ColumnDef),
		Indexes: make(map[string]*IndexInfo),
	}

	// Get columns
	columns, err := dm.getColumns(ctx, tableName)
	if err != nil {
		return nil, err
	}
	tableInfo.Columns = columns

	// Get indexes
	indexes, err := dm.getIndexes(ctx, tableName)
	if err != nil {
		return nil, err
	}
	tableInfo.Indexes = indexes

	return tableInfo, nil
}

// getColumns retrieves column information for a table
func (dm *DDLManager) getColumns(ctx context.Context, tableName string) (map[string]*ColumnDef, error) {
	columns := make(map[string]*ColumnDef)

	// Try SQLite first
	rows, err := dm.db.QueryContext(ctx, "PRAGMA table_info(`"+tableName+"`)")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue sql.NullString

			err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
			if err != nil {
				return nil, err
			}

			// Check if column is auto increment (PRIMARY KEY with INTEGER type)
			isAutoIncr := pk == 1 && (dataType == "INTEGER" || strings.HasPrefix(strings.ToUpper(dataType), "INTEGER"))

			columns[name] = &ColumnDef{
				Name:          name,
				Type:          dataType,
				Nullable:      notNull == 0,
				DefaultValue:  defaultValue.String,
				AutoIncrement: isAutoIncr,
			}
		}
		return columns, nil
	}

	// Fallback to MySQL/PostgreSQL
	query := `
		SELECT 
			column_name, 
			data_type, 
			is_nullable, 
			column_default,
			extra
		FROM information_schema.columns 
		WHERE table_name = ? AND table_schema = DATABASE()
	`
	rows, err = dm.db.QueryContext(ctx, query, tableName)
	if err != nil {
		// Try PostgreSQL
		pgQuery := `
			SELECT 
				column_name, 
				data_type, 
				is_nullable, 
				column_default,
				''
			FROM information_schema.columns 
			WHERE table_name = $1 AND table_schema = 'public'
		`
		rows, err = dm.db.QueryContext(ctx, pgQuery, tableName)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, dataType, isNullable, defaultValue, extra string
		err := rows.Scan(&name, &dataType, &isNullable, &defaultValue, &extra)
		if err != nil {
			return nil, err
		}

		columns[name] = &ColumnDef{
			Name:          name,
			Type:          dataType,
			Nullable:      isNullable == "YES",
			DefaultValue:  defaultValue,
			AutoIncrement: strings.Contains(extra, "auto_increment"),
		}
	}

	return columns, nil
}

// getIndexes retrieves index information for a table
func (dm *DDLManager) getIndexes(ctx context.Context, tableName string) (map[string]*IndexInfo, error) {
	indexes := make(map[string]*IndexInfo)

	// Try SQLite first
	rows, err := dm.db.QueryContext(ctx, "PRAGMA index_list(`"+tableName+"`)")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var seq int
			var name string
			var unique int
			var origin string
			var partial int

			err := rows.Scan(&seq, &name, &unique, &origin, &partial)
			if err != nil {
				return nil, err
			}

			// Skip auto-generated indexes
			if strings.HasPrefix(name, "sqlite_") {
				continue
			}

			// Get index columns
			indexColumns, err := dm.getIndexColumns(ctx, name)
			if err != nil {
				return nil, err
			}

			indexes[name] = &IndexInfo{
				Name:    name,
				Columns: indexColumns,
				Unique:  unique == 1,
				Primary: origin == "pk",
			}
		}
		return indexes, nil
	}

	// Fallback to MySQL/PostgreSQL
	query := `
		SELECT 
			index_name, 
			column_name, 
			non_unique,
			''
		FROM information_schema.statistics 
		WHERE table_name = ? AND table_schema = DATABASE()
		ORDER BY index_name, seq_in_index
	`
	rows, err = dm.db.QueryContext(ctx, query, tableName)
	if err != nil {
		// Try PostgreSQL
		pgQuery := `
			SELECT 
				indexname, 
				attname, 
				NOT indisunique,
				''
			FROM pg_indexes 
			JOIN pg_class ON pg_class.relname = pg_indexes.indexname
			JOIN pg_index ON pg_index.indexrelid = pg_class.oid
			JOIN pg_attribute ON pg_attribute.attrelid = pg_index.indrelid 
				AND pg_attribute.attnum = ANY(pg_index.indkey)
			WHERE pg_indexes.tablename = $1
			ORDER BY indexname, attnum
		`
		rows, err = dm.db.QueryContext(ctx, pgQuery, tableName)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	currentIndex := ""
	var currentColumns []string
	var currentUnique bool

	for rows.Next() {
		var indexName, columnName string
		var nonUnique int
		var extra string

		err := rows.Scan(&indexName, &columnName, &nonUnique, &extra)
		if err != nil {
			return nil, err
		}

		if currentIndex != indexName {
			// Save previous index
			if currentIndex != "" {
				indexes[currentIndex] = &IndexInfo{
					Name:    currentIndex,
					Columns: currentColumns,
					Unique:  currentUnique,
					Primary: false, // TODO: detect primary key
				}
			}

			// Start new index
			currentIndex = indexName
			currentColumns = []string{columnName}
			currentUnique = nonUnique == 0
		} else {
			currentColumns = append(currentColumns, columnName)
		}
	}

	// Save last index
	if currentIndex != "" {
		indexes[currentIndex] = &IndexInfo{
			Name:    currentIndex,
			Columns: currentColumns,
			Unique:  currentUnique,
			Primary: false, // TODO: detect primary key
		}
	}

	return indexes, nil
}

// getIndexColumns retrieves columns for a specific index
func (dm *DDLManager) getIndexColumns(ctx context.Context, indexName string) ([]string, error) {
	var columns []string

	rows, err := dm.db.QueryContext(ctx, "PRAGMA index_info(`"+indexName+"`)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var seqno int
		var cid int
		var name string

		err := rows.Scan(&seqno, &cid, &name)
		if err != nil {
			return nil, err
		}

		columns = append(columns, name)
	}

	return columns, nil
}

// GenerateSchemaPlan generates a schema plan to transform current schema to target schema
func (dm *DDLManager) GenerateSchemaPlan(ctx context.Context, targetModels []interface{}) (*SchemaPlan, error) {
	currentSchema, err := dm.GetCurrentSchema(ctx)
	if err != nil {
		return nil, err
	}

	var commands []DDLCommand
	var summary strings.Builder

	for _, model := range targetModels {
		tableName := getTableName(model)
		targetColumns, err := dm.getModelColumns(model)
		if err != nil {
			return nil, err
		}

		currentTable, exists := currentSchema.Tables[tableName]
		if !exists {
			// Table doesn't exist, create it
			createCmd, err := dm.createTableCommand(tableName, targetColumns)
			if err != nil {
				return nil, err
			}
			commands = append(commands, createCmd)
			summary.WriteString(fmt.Sprintf("Create table %s; ", tableName))
		} else {
			// Table exists, check for column differences
			tableCommands, err := dm.generateTableSchemaCommands(tableName, currentTable, targetColumns)
			if err != nil {
				return nil, err
			}
			commands = append(commands, tableCommands...)
			if len(tableCommands) > 0 {
				summary.WriteString(fmt.Sprintf("Update table %s; ", tableName))
			}
		}
	}

	return &SchemaPlan{
		Commands: commands,
		Summary:  strings.TrimSpace(summary.String()),
	}, nil
}

// ExecuteSchemaPlan executes a schema plan
func (dm *DDLManager) ExecuteSchemaPlan(ctx context.Context, plan *SchemaPlan) error {
	for _, cmd := range plan.Commands {
		err := cmd.Execute(ctx, dm.db)
		dm.logger.LogCommand(ctx, cmd, err)
		if err != nil {
			dm.logger.LogSchemaChange(ctx, plan, err)
			return fmt.Errorf("failed to execute command %s: %w", cmd.Description(), err)
		}
	}

	dm.logger.LogSchemaChange(ctx, plan, nil)
	return nil
}

// getModelColumns extracts column definitions from a model struct
// Only supports zorm:"auto_incr" tag, ignores all other tags
func (dm *DDLManager) getModelColumns(model interface{}) (map[string]*ColumnDef, error) {
	rt := reflect2.TypeOf(model)
	if rt.Kind() == reflect.Ptr {
		rt = rt.(reflect2.PtrType).Elem()
	}

	if rt.Kind() != reflect.Struct {
		return nil, errors.New("model must be a struct")
	}

	s := rt.(reflect2.StructType)
	columns := make(map[string]*ColumnDef)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("zorm")

		// Skip fields with zorm tag "-"
		if ft == "-" {
			continue
		}

		// Use getFieldName to get database field name (auto convert camelCase to snake_case)
		fieldName := getFieldName(f)
		if fieldName == "" {
			continue // Skip ignored fields
		}

		column := &ColumnDef{
			Name:          fieldName,
			Type:          getSQLType(f.Type()),
			Nullable:      isNullable(f),
			DefaultValue:  getDefaultValue(f),
			AutoIncrement: isAutoIncrementField(f),
		}

		columns[fieldName] = column
	}

	return columns, nil
}

// createTableCommand creates a CREATE TABLE command
func (dm *DDLManager) createTableCommand(tableName string, columns map[string]*ColumnDef) (*CreateTableCommand, error) {
	var columnList []*ColumnDef
	var primaryKey []string

	for _, col := range columns {
		columnList = append(columnList, col)
		if col.AutoIncrement {
			primaryKey = append(primaryKey, col.Name)
		}
	}

	return &CreateTableCommand{
		TableName:  tableName,
		Columns:    columnList,
		PrimaryKey: primaryKey,
		Engine:     "InnoDB",
		Charset:    "utf8mb4",
		Collate:    "utf8mb4_unicode_ci",
	}, nil
}

// generateTableSchemaCommands generates commands to modify a table schema
func (dm *DDLManager) generateTableSchemaCommands(tableName string, currentTable *TableInfo, targetColumns map[string]*ColumnDef) ([]DDLCommand, error) {
	var commands []DDLCommand

	// Check for new columns
	for colName, targetCol := range targetColumns {
		if _, exists := currentTable.Columns[colName]; !exists {
			cmd := &AlterTableCommand{
				TableName: tableName,
				Operation: "ADD COLUMN",
				Column:    targetCol,
			}
			commands = append(commands, cmd)
		}
	}

	// Check for modified columns
	for colName, targetCol := range targetColumns {
		if currentCol, exists := currentTable.Columns[colName]; exists {
			if dm.columnChanged(currentCol, targetCol) {
				cmd := &AlterTableCommand{
					TableName: tableName,
					Operation: "MODIFY COLUMN",
					Column:    targetCol,
				}
				commands = append(commands, cmd)
			}
		}
	}

	// Check for dropped columns (optional - might want to be more careful)
	// This is commented out for safety - dropping columns can cause data loss
	/*
		for colName := range currentTable.Columns {
			if _, exists := targetColumns[colName]; !exists {
				cmd := &AlterTableCommand{
					TableName: tableName,
					Operation: "DROP COLUMN",
					Column:    &ColumnDef{Name: colName},
				}
				commands = append(commands, cmd)
			}
		}
	*/

	return commands, nil
}

// columnChanged checks if a column definition has changed
func (dm *DDLManager) columnChanged(current, target *ColumnDef) bool {
	return current.Type != target.Type ||
		current.Nullable != target.Nullable ||
		current.DefaultValue != target.DefaultValue ||
		current.AutoIncrement != target.AutoIncrement
}

// CreateTables performs atomic table creation using the new DDL system
func (dm *DDLManager) CreateTables(ctx context.Context, models ...interface{}) error {
	plan, err := dm.GenerateSchemaPlan(ctx, models)
	if err != nil {
		return err
	}

	if len(plan.Commands) == 0 {
		// No schema changes needed
		return nil
	}

	return dm.ExecuteSchemaPlan(ctx, plan)
}

// Convenience functions for backward compatibility

// AtomicCreateTables performs atomic table creation with audit logging
func AtomicCreateTables(db ZormDBIFace, logger DDLLogger, models ...interface{}) error {
	manager := NewDDLManager(db, logger)
	ctx := context.Background()
	return manager.CreateTables(ctx, models...)
}

// AtomicCreateTablesWithContext performs atomic table creation with context and audit logging
func AtomicCreateTablesWithContext(ctx context.Context, db ZormDBIFace, logger DDLLogger, models ...interface{}) error {
	manager := NewDDLManager(db, logger)
	return manager.CreateTables(ctx, models...)
}
