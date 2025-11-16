package statistics

import (
	"testing"
)

func TestNewStatisticsStore(t *testing.T) {
	store := NewStatisticsStore()
	if store == nil {
		t.Fatal("NewStatisticsStore returned nil")
	}

	if store.TableCount() != 0 {
		t.Errorf("Expected empty store, got %d tables", store.TableCount())
	}
}

func TestStatisticsStore_SetAndGetTableStats(t *testing.T) {
	store := NewStatisticsStore()

	stats := &TableStats{
		Tuples:  1000,
		Pages:   100,
		Columns: make(map[string]*ColumnStats),
	}

	store.SetTableStats("users", stats)

	retrieved := store.GetTableStats("users")
	if retrieved == nil {
		t.Fatal("GetTableStats returned nil")
	}

	if retrieved.Tuples != 1000 {
		t.Errorf("Expected 1000 tuples, got %d", retrieved.Tuples)
	}

	if retrieved.Pages != 100 {
		t.Errorf("Expected 100 pages, got %d", retrieved.Pages)
	}
}

func TestStatisticsStore_GetNonExistentTable(t *testing.T) {
	store := NewStatisticsStore()

	stats := store.GetTableStats("nonexistent")
	if stats != nil {
		t.Error("Expected nil for nonexistent table")
	}
}

func TestStatisticsStore_UpdateColumnStats(t *testing.T) {
	store := NewStatisticsStore()

	colStats := &ColumnStats{
		NDV:      100,
		NullFrac: 0.05,
		AvgWidth: 20,
	}

	store.UpdateColumnStats("users", "age", colStats)

	retrieved := store.GetColumnStats("users", "age")
	if retrieved == nil {
		t.Fatal("GetColumnStats returned nil")
	}

	if retrieved.NDV != 100 {
		t.Errorf("Expected NDV=100, got %d", retrieved.NDV)
	}

	if retrieved.NullFrac != 0.05 {
		t.Errorf("Expected NullFrac=0.05, got %f", retrieved.NullFrac)
	}
}

func TestStatisticsStore_Clear(t *testing.T) {
	store := NewStatisticsStore()

	store.SetTableStats("table1", &TableStats{Tuples: 1000})
	store.SetTableStats("table2", &TableStats{Tuples: 2000})

	if store.TableCount() != 2 {
		t.Errorf("Expected 2 tables, got %d", store.TableCount())
	}

	store.Clear()

	if store.TableCount() != 0 {
		t.Errorf("Expected 0 tables after clear, got %d", store.TableCount())
	}
}

func TestStatisticsStore_Concurrent(t *testing.T) {
	store := NewStatisticsStore()

	// Run concurrent updates and reads
	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			stats := &TableStats{
				Tuples: int64(id * 1000),
			}
			store.SetTableStats("table", stats)
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			_ = store.GetTableStats("table")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
