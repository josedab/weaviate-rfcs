//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// AuditLog represents a single audit log entry
type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`

	// User context
	UserID    string `json:"user_id"`
	UserEmail string `json:"user_email"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`

	// Action
	Action   string   `json:"action"`
	Resource Resource `json:"resource"`

	// Result
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	Query        string        `json:"query,omitempty"`
	Duration     time.Duration `json:"duration"`
	RowsAffected int64         `json:"rows_affected"`

	// Data access
	ObjectsAccessed []string `json:"objects_accessed,omitempty"`
	FieldsAccessed  []string `json:"fields_accessed,omitempty"`
}

// Resource represents a resource being accessed
type Resource struct {
	Type       ResourceType `json:"type"`
	Identifier string       `json:"identifier"`
}

// ResourceType defines the type of resource
type ResourceType string

const (
	ResourceTypeClass  ResourceType = "class"
	ResourceTypeSchema ResourceType = "schema"
	ResourceTypeSystem ResourceType = "system"
	ResourceTypeData   ResourceType = "data"
)

// AuditLogger handles audit logging
type AuditLogger struct {
	writer    AuditWriter
	retention time.Duration
	logger    logrus.FieldLogger
	mu        sync.RWMutex
	enabled   bool
}

// AuditWriter defines the interface for writing audit logs
type AuditWriter interface {
	Write(log *AuditLog) error
	Query(startTime, endTime time.Time) ([]*AuditLog, error)
	Close() error
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(writer AuditWriter, retention time.Duration, logger logrus.FieldLogger) *AuditLogger {
	return &AuditLogger{
		writer:    writer,
		retention: retention,
		logger:    logger,
		enabled:   true,
	}
}

// LogQuery logs a query execution
func (a *AuditLogger) LogQuery(ctx context.Context, query string, result QueryResult) error {
	if !a.enabled {
		return nil
	}

	user := GetUserFromContext(ctx)
	if user == nil {
		return fmt.Errorf("no user in context")
	}

	log := &AuditLog{
		ID:           uuid.New().String(),
		Timestamp:    time.Now(),
		RequestID:    GetRequestID(ctx),
		UserID:       user.ID,
		UserEmail:    user.Email,
		IPAddress:    GetClientIP(ctx),
		UserAgent:    GetUserAgent(ctx),
		Action:       "query",
		Query:        query,
		Success:      result.Error == nil,
		Duration:     result.Duration,
		RowsAffected: int64(len(result.Objects)),
	}

	if result.Error != nil {
		log.Error = result.Error.Error()
	}

	return a.writer.Write(log)
}

// LogAccess logs a resource access attempt
func (a *AuditLogger) LogAccess(ctx context.Context, user *User, resource Resource, action string, success bool) error {
	if !a.enabled {
		return nil
	}

	log := &AuditLog{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		RequestID: GetRequestID(ctx),
		UserID:    user.ID,
		UserEmail: user.Email,
		IPAddress: GetClientIP(ctx),
		UserAgent: GetUserAgent(ctx),
		Action:    action,
		Resource:  resource,
		Success:   success,
	}

	return a.writer.Write(log)
}

// LogDataAccess logs access to specific data objects
func (a *AuditLogger) LogDataAccess(ctx context.Context, user *User, objectIDs []string, fields []string) error {
	if !a.enabled {
		return nil
	}

	log := &AuditLog{
		ID:              uuid.New().String(),
		Timestamp:       time.Now(),
		RequestID:       GetRequestID(ctx),
		UserID:          user.ID,
		UserEmail:       user.Email,
		IPAddress:       GetClientIP(ctx),
		UserAgent:       GetUserAgent(ctx),
		Action:          "data_access",
		Success:         true,
		ObjectsAccessed: objectIDs,
		FieldsAccessed:  fields,
	}

	return a.writer.Write(log)
}

// GenerateComplianceReport generates a compliance report for a time period
func (a *AuditLogger) GenerateComplianceReport(startTime, endTime time.Time) (*ComplianceReport, error) {
	logs, err := a.writer.Query(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	report := &ComplianceReport{
		Period:         Period{Start: startTime, End: endTime},
		TotalAccesses:  len(logs),
		UniqueUsers:    countUniqueUsers(logs),
		FailedAccesses: countFailedAccesses(logs),
		TopUsers:       topUsers(logs, 10),
		Anomalies:      detectAnomalies(logs),
	}

	return report, nil
}

// QueryResult represents the result of a query
type QueryResult struct {
	Objects  []interface{}
	Error    error
	Duration time.Duration
}

// User represents a user for audit purposes
type User struct {
	ID    string
	Email string
}

// ComplianceReport represents a compliance report
type ComplianceReport struct {
	Period         Period            `json:"period"`
	TotalAccesses  int               `json:"total_accesses"`
	UniqueUsers    int               `json:"unique_users"`
	FailedAccesses int               `json:"failed_accesses"`
	TopUsers       []UserAccessCount `json:"top_users"`
	Anomalies      []Anomaly         `json:"anomalies"`
}

// Period represents a time period
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// UserAccessCount represents user access statistics
type UserAccessCount struct {
	UserEmail string `json:"user_email"`
	Count     int    `json:"count"`
}

// Anomaly represents a detected anomaly
type Anomaly struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	UserEmail   string    `json:"user_email,omitempty"`
}

// Helper functions

func countUniqueUsers(logs []*AuditLog) int {
	users := make(map[string]bool)
	for _, log := range logs {
		users[log.UserID] = true
	}
	return len(users)
}

func countFailedAccesses(logs []*AuditLog) int {
	count := 0
	for _, log := range logs {
		if !log.Success {
			count++
		}
	}
	return count
}

func topUsers(logs []*AuditLog, n int) []UserAccessCount {
	counts := make(map[string]int)
	emails := make(map[string]string)

	for _, log := range logs {
		counts[log.UserID]++
		emails[log.UserID] = log.UserEmail
	}

	// Convert to slice and sort
	var results []UserAccessCount
	for userID, count := range counts {
		results = append(results, UserAccessCount{
			UserEmail: emails[userID],
			Count:     count,
		})
	}

	// Simple sort by count (descending)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Count > results[i].Count {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > n {
		results = results[:n]
	}

	return results
}

func detectAnomalies(logs []*AuditLog) []Anomaly {
	var anomalies []Anomaly

	// Detect multiple failed access attempts
	failedAttempts := make(map[string]int)
	failedTimes := make(map[string]time.Time)

	for _, log := range logs {
		if !log.Success {
			failedAttempts[log.UserEmail]++
			failedTimes[log.UserEmail] = log.Timestamp
		}
	}

	for email, count := range failedAttempts {
		if count > 5 {
			anomalies = append(anomalies, Anomaly{
				Type:        "multiple_failed_access",
				Description: fmt.Sprintf("User had %d failed access attempts", count),
				Timestamp:   failedTimes[email],
				UserEmail:   email,
			})
		}
	}

	return anomalies
}

// Context helper functions

func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value("user").(*User); ok {
		return user
	}
	return nil
}

func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value("request_id").(string); ok {
		return reqID
	}
	return ""
}

func GetClientIP(ctx context.Context) string {
	if ip, ok := ctx.Value("client_ip").(string); ok {
		return ip
	}
	return ""
}

func GetUserAgent(ctx context.Context) string {
	if ua, ok := ctx.Value("user_agent").(string); ok {
		return ua
	}
	return ""
}

// FileAuditWriter implements AuditWriter using file storage
type FileAuditWriter struct {
	filePath string
	mu       sync.Mutex
	logs     []*AuditLog
}

// NewFileAuditWriter creates a new file-based audit writer
func NewFileAuditWriter(filePath string) *FileAuditWriter {
	return &FileAuditWriter{
		filePath: filePath,
		logs:     make([]*AuditLog, 0),
	}
}

// Write writes an audit log
func (w *FileAuditWriter) Write(log *AuditLog) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.logs = append(w.logs, log)

	// In a real implementation, this would write to a file
	data, err := json.Marshal(log)
	if err != nil {
		return err
	}

	_ = data // TODO: Write to file
	return nil
}

// Query queries audit logs
func (w *FileAuditWriter) Query(startTime, endTime time.Time) ([]*AuditLog, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var results []*AuditLog
	for _, log := range w.logs {
		if log.Timestamp.After(startTime) && log.Timestamp.Before(endTime) {
			results = append(results, log)
		}
	}

	return results, nil
}

// Close closes the audit writer
func (w *FileAuditWriter) Close() error {
	return nil
}
