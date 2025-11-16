package statistics

import (
	"sync"
	"time"
)

// StatisticsStore maintains statistics for all tables/collections in the database.
// It provides thread-safe access to table and column statistics used by the query optimizer.
type StatisticsStore struct {
	tables map[string]*TableStats
	mu     sync.RWMutex
}

// TableStats contains statistics for a single table/collection.
type TableStats struct {
	Tuples      int64              // Total number of tuples/objects
	Pages       int64              // Total number of pages/segments
	LastUpdated time.Time          // Timestamp of last statistics update
	Columns     map[string]*ColumnStats // Per-column statistics
}

// NewStatisticsStore creates a new statistics store.
func NewStatisticsStore() *StatisticsStore {
	return &StatisticsStore{
		tables: make(map[string]*TableStats),
	}
}

// GetTableStats retrieves statistics for a specific table.
// Returns nil if no statistics are available.
func (s *StatisticsStore) GetTableStats(tableName string) *TableStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tables[tableName]
}

// SetTableStats updates statistics for a specific table.
func (s *StatisticsStore) SetTableStats(tableName string, stats *TableStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats.LastUpdated = time.Now()
	s.tables[tableName] = stats
}

// GetColumnStats retrieves statistics for a specific column in a table.
// Returns nil if no statistics are available.
func (s *StatisticsStore) GetColumnStats(tableName, columnName string) *ColumnStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tableStats := s.tables[tableName]
	if tableStats == nil {
		return nil
	}

	return tableStats.Columns[columnName]
}

// UpdateColumnStats updates statistics for a specific column.
func (s *StatisticsStore) UpdateColumnStats(tableName, columnName string, stats *ColumnStats) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tableStats := s.tables[tableName]
	if tableStats == nil {
		tableStats = &TableStats{
			Columns: make(map[string]*ColumnStats),
		}
		s.tables[tableName] = tableStats
	}

	if tableStats.Columns == nil {
		tableStats.Columns = make(map[string]*ColumnStats)
	}

	tableStats.Columns[columnName] = stats
	tableStats.LastUpdated = time.Now()
}

// Clear removes all statistics.
func (s *StatisticsStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tables = make(map[string]*TableStats)
}

// TableCount returns the number of tables with statistics.
func (s *StatisticsStore) TableCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tables)
}
