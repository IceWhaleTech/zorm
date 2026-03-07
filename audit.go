/*
   zorm is a better orm library for Go.

  Copyright (c) 2019 <http://ez8.co> <orca.zhang@yahoo.com>

  This library is released under the MIT License.
  Please see LICENSE file or visit https://github.com/IceWhaleTech/zorm for details.
*/

// Package zorm provides SQL audit and telemetry functionality for database operations.
package zorm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultTelemetryRetention = 1024

// SQLAuditEvent represents a SQL execution event for auditing
type SQLAuditEvent struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Operation    string                 `json:"operation"` // SELECT, INSERT, UPDATE, DELETE, DDL
	TableName    string                 `json:"table_name"`
	SQL          string                 `json:"sql"`
	Args         []interface{}          `json:"args"`
	Duration     time.Duration          `json:"duration_ms"`
	RowsAffected int64                  `json:"rows_affected"`
	Error        string                 `json:"error,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TelemetryData represents performance and usage telemetry data
type TelemetryData struct {
	ID              string                 `json:"id"`
	Timestamp       time.Time              `json:"timestamp"`
	Operation       string                 `json:"operation"`
	TableName       string                 `json:"table_name"`
	Duration        time.Duration          `json:"duration_ms"`
	RowsAffected    int64                  `json:"rows_affected"`
	CacheHit        bool                   `json:"cache_hit"`
	ReuseEnabled    bool                   `json:"reuse_enabled"`
	ConnectionPool  *ConnectionPoolStats   `json:"connection_pool,omitempty"`
	QueryComplexity int                    `json:"query_complexity"`
	Error           string                 `json:"error,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ConnectionPoolStats represents connection pool statistics
type ConnectionPoolStats struct {
	OpenConnections   int           `json:"open_connections"`
	InUseConnections  int           `json:"in_use_connections"`
	IdleConnections   int           `json:"idle_connections"`
	WaitCount         int64         `json:"wait_count"`
	WaitDuration      time.Duration `json:"wait_duration_ms"`
	MaxIdleClosed     int64         `json:"max_idle_closed"`
	MaxIdleTimeClosed int64         `json:"max_idle_time_closed"`
	MaxLifetimeClosed int64         `json:"max_lifetime_closed"`
}

// AuditLogger interface for logging SQL audit events
type AuditLogger interface {
	LogAuditEvent(ctx context.Context, event *SQLAuditEvent)
	LogTelemetryData(ctx context.Context, data *TelemetryData)
}

// TelemetryCollector interface for collecting telemetry data
type TelemetryCollector interface {
	CollectTelemetry(ctx context.Context, data *TelemetryData)
	GetMetrics() map[string]interface{}
}

// DefaultAuditLogger is a simple console audit logger
type DefaultAuditLogger struct {
	mu sync.Mutex
}

func (l *DefaultAuditLogger) LogAuditEvent(ctx context.Context, event *SQLAuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	fmt.Printf("[AUDIT] %s\n", eventJSON)
}

func (l *DefaultAuditLogger) LogTelemetryData(ctx context.Context, data *TelemetryData) {
	l.mu.Lock()
	defer l.mu.Unlock()

	dataJSON, _ := json.MarshalIndent(data, "", "  ")
	fmt.Printf("[TELEMETRY] %s\n", dataJSON)
}

// DefaultTelemetryCollector collects and stores telemetry data
type DefaultTelemetryCollector struct {
	mu      sync.RWMutex
	data    []*TelemetryData
	metrics map[string]interface{}
	maxData int
}

func NewDefaultTelemetryCollector() *DefaultTelemetryCollector {
	return NewDefaultTelemetryCollectorWithLimit(defaultTelemetryRetention)
}

func NewDefaultTelemetryCollectorWithLimit(limit int) *DefaultTelemetryCollector {
	if limit < 0 {
		limit = 0
	}
	return &DefaultTelemetryCollector{
		data:    make([]*TelemetryData, 0),
		metrics: make(map[string]interface{}),
		maxData: limit,
	}
}

func (c *DefaultTelemetryCollector) CollectTelemetry(ctx context.Context, data *TelemetryData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxData == 0 {
		c.data = c.data[:0]
	} else if len(c.data) < c.maxData {
		c.data = append(c.data, data)
	} else {
		copy(c.data, c.data[1:])
		c.data[len(c.data)-1] = data
	}

	// Update metrics
	c.updateMetrics(data)
}

func (c *DefaultTelemetryCollector) GetMetrics() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy of metrics
	result := make(map[string]interface{})
	for k, v := range c.metrics {
		result[k] = v
	}
	return result
}

func (c *DefaultTelemetryCollector) GetRecentTelemetry() []*TelemetryData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*TelemetryData, len(c.data))
	copy(result, c.data)
	return result
}

func (c *DefaultTelemetryCollector) updateMetrics(data *TelemetryData) {
	// Update operation counts
	opKey := fmt.Sprintf("operation_%s_count", data.Operation)
	totalCount := 1
	if count, ok := c.metrics[opKey].(int); ok {
		totalCount = count + 1
	}
	c.metrics[opKey] = totalCount

	// Update average duration
	durationKey := fmt.Sprintf("operation_%s_avg_duration_ms", data.Operation)
	if avg, ok := c.metrics[durationKey].(float64); ok {
		c.metrics[durationKey] = (avg*float64(totalCount-1) + float64(data.Duration.Milliseconds())) / float64(totalCount)
	} else {
		c.metrics[durationKey] = float64(data.Duration.Milliseconds())
	}

	// Update error rate
	errorCountKey := fmt.Sprintf("operation_%s_error_count", data.Operation)
	errorKey := fmt.Sprintf("operation_%s_error_rate", data.Operation)
	errorCount := 0
	if data.Error != "" {
		if currentErrorCount, ok := c.metrics[errorCountKey].(int); ok {
			errorCount = currentErrorCount + 1
		} else {
			errorCount = 1
		}
		c.metrics[errorCountKey] = errorCount
	} else if currentErrorCount, ok := c.metrics[errorCountKey].(int); ok {
		errorCount = currentErrorCount
	}
	c.metrics[errorKey] = float64(errorCount) / float64(totalCount)

	// Update cache hit rate
	cacheCountKey := fmt.Sprintf("operation_%s_cache_hit_count", data.Operation)
	cacheRateKey := fmt.Sprintf("operation_%s_cache_hit_rate", data.Operation)
	cacheCount := 0
	if data.CacheHit {
		if hitCount, ok := c.metrics[cacheCountKey].(int); ok {
			cacheCount = hitCount + 1
		} else {
			cacheCount = 1
		}
		c.metrics[cacheCountKey] = cacheCount
	} else if hitCount, ok := c.metrics[cacheCountKey].(int); ok {
		cacheCount = hitCount
	}
	c.metrics[cacheRateKey] = float64(cacheCount) / float64(totalCount)
}

// AuditableDB wraps a ZormDBIFace with audit logging
type AuditableDB struct {
	db                 ZormDBIFace
	auditLogger        AuditLogger
	telemetryCollector TelemetryCollector
	enabled            bool
}

// NewAuditableDB creates a new auditable database wrapper
func NewAuditableDB(db ZormDBIFace, auditLogger AuditLogger, telemetryCollector TelemetryCollector) *AuditableDB {
	if auditLogger == nil {
		auditLogger = &DefaultAuditLogger{}
	}
	if telemetryCollector == nil {
		telemetryCollector = NewDefaultTelemetryCollector()
	}

	return &AuditableDB{
		db:                 wrapDefaultStmtCacheDB(db),
		auditLogger:        auditLogger,
		telemetryCollector: telemetryCollector,
		enabled:            true,
	}
}

// Enable enables audit logging
func (adb *AuditableDB) Enable() {
	adb.enabled = true
}

// Disable disables audit logging
func (adb *AuditableDB) Disable() {
	adb.enabled = false
}

// QueryRowContext implements ZormDBIFace with audit logging
func (adb *AuditableDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if !adb.enabled {
		return adb.db.QueryRowContext(ctx, query, args...)
	}

	start := time.Now()
	event := &SQLAuditEvent{
		ID:        generateEventID(),
		Timestamp: start,
		Operation: "SELECT",
		SQL:       query,
		Args:      args,
		TableName: extractTableName(query),
	}

	row := adb.db.QueryRowContext(ctx, query, args...)

	event.Duration = time.Since(start)

	// Log audit event
	go adb.auditLogger.LogAuditEvent(ctx, event)

	// Collect telemetry
	telemetryData := &TelemetryData{
		ID:              generateEventID(),
		Timestamp:       start,
		Operation:       "SELECT",
		TableName:       event.TableName,
		Duration:        event.Duration,
		CacheHit:        false, // TODO: detect cache hits
		ReuseEnabled:    false, // TODO: detect reuse
		ConnectionPool:  extractConnectionPoolStats(adb.db),
		QueryComplexity: calculateQueryComplexity(query),
	}

	go adb.auditLogger.LogTelemetryData(ctx, telemetryData)
	go adb.telemetryCollector.CollectTelemetry(ctx, telemetryData)

	return row
}

// QueryContext implements ZormDBIFace with audit logging
func (adb *AuditableDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if !adb.enabled {
		return adb.db.QueryContext(ctx, query, args...)
	}

	start := time.Now()
	event := &SQLAuditEvent{
		ID:        generateEventID(),
		Timestamp: start,
		Operation: "SELECT",
		SQL:       query,
		Args:      args,
		TableName: extractTableName(query),
	}

	rows, err := adb.db.QueryContext(ctx, query, args...)

	event.Duration = time.Since(start)
	if err != nil {
		event.Error = err.Error()
	}

	// Log audit event
	go adb.auditLogger.LogAuditEvent(ctx, event)

	// Collect telemetry
	telemetryData := &TelemetryData{
		ID:              generateEventID(),
		Timestamp:       start,
		Operation:       "SELECT",
		TableName:       event.TableName,
		Duration:        event.Duration,
		CacheHit:        false, // TODO: detect cache hits
		ReuseEnabled:    false, // TODO: detect reuse
		ConnectionPool:  extractConnectionPoolStats(adb.db),
		QueryComplexity: calculateQueryComplexity(query),
		Error:           event.Error,
	}

	go adb.auditLogger.LogTelemetryData(ctx, telemetryData)
	go adb.telemetryCollector.CollectTelemetry(ctx, telemetryData)

	return rows, err
}

// ExecContext implements ZormDBIFace with audit logging
func (adb *AuditableDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if !adb.enabled {
		return adb.db.ExecContext(ctx, query, args...)
	}

	start := time.Now()
	operation := extractOperation(query)
	event := &SQLAuditEvent{
		ID:        generateEventID(),
		Timestamp: start,
		Operation: operation,
		SQL:       query,
		Args:      args,
		TableName: extractTableName(query),
	}

	result, err := adb.db.ExecContext(ctx, query, args...)

	event.Duration = time.Since(start)
	if err != nil {
		event.Error = err.Error()
	} else {
		rowsAffected, _ := result.RowsAffected()
		event.RowsAffected = rowsAffected
	}

	// Log audit event
	go adb.auditLogger.LogAuditEvent(ctx, event)

	// Collect telemetry
	telemetryData := &TelemetryData{
		ID:              generateEventID(),
		Timestamp:       start,
		Operation:       operation,
		TableName:       event.TableName,
		Duration:        event.Duration,
		RowsAffected:    event.RowsAffected,
		CacheHit:        false, // TODO: detect cache hits
		ReuseEnabled:    false, // TODO: detect reuse
		ConnectionPool:  extractConnectionPoolStats(adb.db),
		QueryComplexity: calculateQueryComplexity(query),
		Error:           event.Error,
	}

	go adb.auditLogger.LogTelemetryData(ctx, telemetryData)
	go adb.telemetryCollector.CollectTelemetry(ctx, telemetryData)

	return result, err
}

// GetTelemetryMetrics returns current telemetry metrics
func (adb *AuditableDB) GetTelemetryMetrics() map[string]interface{} {
	return adb.telemetryCollector.GetMetrics()
}

// Utility functions

func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

func extractConnectionPoolStats(db ZormDBIFace) *ConnectionPoolStats {
	switch typed := db.(type) {
	case *sql.DB:
		stats := typed.Stats()
		return &ConnectionPoolStats{
			OpenConnections:   stats.OpenConnections,
			InUseConnections:  stats.InUse,
			IdleConnections:   stats.Idle,
			WaitCount:         stats.WaitCount,
			WaitDuration:      stats.WaitDuration,
			MaxIdleClosed:     stats.MaxIdleClosed,
			MaxIdleTimeClosed: stats.MaxIdleTimeClosed,
			MaxLifetimeClosed: stats.MaxLifetimeClosed,
		}
	case *AuditableDB:
		return extractConnectionPoolStats(typed.db)
	case *ReadWriteDB:
		return extractConnectionPoolStats(typed.Master)
	default:
		return nil
	}
}

func extractTableName(query string) string {
	// Simple table name extraction - can be improved
	query = strings.ToUpper(strings.TrimSpace(query))

	// Handle different SQL operations
	if strings.HasPrefix(query, "SELECT") {
		// Extract from FROM clause
		fromIndex := strings.Index(query, "FROM")
		if fromIndex != -1 {
			afterFrom := query[fromIndex+4:]
			words := strings.Fields(afterFrom)
			if len(words) > 0 {
				return strings.Trim(words[0], "`\"'")
			}
		}
	} else if strings.HasPrefix(query, "INSERT") {
		// Extract from INTO clause
		intoIndex := strings.Index(query, "INTO")
		if intoIndex != -1 {
			afterInto := query[intoIndex+4:]
			words := strings.Fields(afterInto)
			if len(words) > 0 {
				return strings.Trim(words[0], "`\"'")
			}
		}
	} else if strings.HasPrefix(query, "UPDATE") {
		// Extract table name after UPDATE
		words := strings.Fields(query)
		if len(words) > 1 {
			return strings.Trim(words[1], "`\"'")
		}
	} else if strings.HasPrefix(query, "DELETE") {
		// Extract from FROM clause
		fromIndex := strings.Index(query, "FROM")
		if fromIndex != -1 {
			afterFrom := query[fromIndex+4:]
			words := strings.Fields(afterFrom)
			if len(words) > 0 {
				return strings.Trim(words[0], "`\"'")
			}
		}
	}

	return "unknown"
}

func extractOperation(query string) string {
	query = strings.ToUpper(strings.TrimSpace(query))

	if strings.HasPrefix(query, "SELECT") {
		return "SELECT"
	} else if strings.HasPrefix(query, "INSERT") {
		return "INSERT"
	} else if strings.HasPrefix(query, "UPDATE") {
		return "UPDATE"
	} else if strings.HasPrefix(query, "DELETE") {
		return "DELETE"
	} else if strings.HasPrefix(query, "CREATE") {
		return "DDL"
	} else if strings.HasPrefix(query, "ALTER") {
		return "DDL"
	} else if strings.HasPrefix(query, "DROP") {
		return "DDL"
	}

	return "UNKNOWN"
}

func calculateQueryComplexity(query string) int {
	// Simple query complexity calculation
	complexity := 1

	// Count JOINs
	complexity += strings.Count(strings.ToUpper(query), "JOIN")

	// Count subqueries
	complexity += strings.Count(strings.ToUpper(query), "SELECT") - 1

	// Count WHERE conditions
	complexity += strings.Count(strings.ToUpper(query), "WHERE")

	// Count GROUP BY
	complexity += strings.Count(strings.ToUpper(query), "GROUP BY")

	// Count ORDER BY
	complexity += strings.Count(strings.ToUpper(query), "ORDER BY")

	// Count HAVING
	complexity += strings.Count(strings.ToUpper(query), "HAVING")

	return complexity
}

// FileAuditLogger logs audit events to a file
type FileAuditLogger struct {
	filename string
	mu       sync.Mutex
}

func NewFileAuditLogger(filename string) *FileAuditLogger {
	return &FileAuditLogger{
		filename: filename,
	}
}

func (l *FileAuditLogger) LogAuditEvent(ctx context.Context, event *SQLAuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.writeJSONLine("audit", event)
}

func (l *FileAuditLogger) LogTelemetryData(ctx context.Context, data *TelemetryData) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.writeJSONLine("telemetry", data)
}

func (l *FileAuditLogger) writeJSONLine(kind string, payload interface{}) {
	file, err := os.OpenFile(l.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("[FILE_%s_ERROR] %v\n", strings.ToUpper(kind), err)
		return
	}
	defer file.Close()

	record := map[string]interface{}{
		"kind":      kind,
		"timestamp": time.Now().UTC(),
		"payload":   payload,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		fmt.Printf("[FILE_%s_ERROR] %v\n", strings.ToUpper(kind), err)
		return
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		fmt.Printf("[FILE_%s_ERROR] %v\n", strings.ToUpper(kind), err)
	}
}

// JSONAuditLogger logs audit events as JSON
type JSONAuditLogger struct {
	mu sync.Mutex
}

func NewJSONAuditLogger() *JSONAuditLogger {
	return &JSONAuditLogger{}
}

func (l *JSONAuditLogger) LogAuditEvent(ctx context.Context, event *SQLAuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	eventJSON, _ := json.Marshal(event)
	fmt.Printf("[JSON_AUDIT] %s\n", eventJSON)
}

func (l *JSONAuditLogger) LogTelemetryData(ctx context.Context, data *TelemetryData) {
	l.mu.Lock()
	defer l.mu.Unlock()

	dataJSON, _ := json.Marshal(data)
	fmt.Printf("[JSON_TELEMETRY] %s\n", dataJSON)
}

// LogCommand implements DDLLogger interface
func (l *JSONAuditLogger) LogCommand(ctx context.Context, cmd DDLCommand, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}

	event := &SQLAuditEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Operation: "DDL",
		SQL:       cmd.SQL(),
		Error:     fmt.Sprintf("%v", err),
	}

	eventJSON, _ := json.Marshal(event)
	fmt.Printf("[JSON_DDL] %s - %s: %s\n", status, cmd.Description(), eventJSON)
}

// LogSchemaChange implements DDLLogger interface
func (l *JSONAuditLogger) LogSchemaChange(ctx context.Context, plan *SchemaPlan, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}

	schemaData := map[string]interface{}{
		"status":    status,
		"summary":   plan.Summary,
		"commands":  len(plan.Commands),
		"error":     fmt.Sprintf("%v", err),
		"timestamp": time.Now(),
	}

	dataJSON, _ := json.Marshal(schemaData)
	fmt.Printf("[JSON_SCHEMA] %s\n", dataJSON)
}
