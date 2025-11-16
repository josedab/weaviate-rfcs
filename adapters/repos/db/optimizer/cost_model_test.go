package optimizer

import (
	"testing"

	"github.com/weaviate/weaviate/adapters/repos/db/statistics"
)

func TestNewCostModel(t *testing.T) {
	stats := statistics.NewStatisticsStore()
	cm := NewCostModel(stats)

	if cm == nil {
		t.Fatal("NewCostModel returned nil")
	}

	// Check default cost factors are set
	if cm.cpuTupleProcessing == 0 {
		t.Error("CPU tuple processing cost not initialized")
	}

	if cm.ioSequentialRead == 0 {
		t.Error("IO sequential read cost not initialized")
	}
}

func TestCostModel_EstimateSeqScan(t *testing.T) {
	stats := statistics.NewStatisticsStore()
	stats.SetTableStats("users", &statistics.TableStats{
		Tuples: 1000,
		Pages:  100,
	})

	cm := NewCostModel(stats)

	op := &SeqScanOperator{
		Table: "users",
	}

	cost := cm.estimateSeqScan(op)

	// Cost should be > 0 and include both I/O and CPU costs
	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}

	// Cost should be I/O (100 pages * 100) + CPU (1000 tuples * 2)
	expectedMin := float64(100*100 + 1000*2)
	if cost < expectedMin*0.9 {
		t.Errorf("Cost too low: got %f, expected >= %f", cost, expectedMin)
	}
}

func TestCostModel_EstimateIndexScan(t *testing.T) {
	stats := statistics.NewStatisticsStore()
	stats.SetTableStats("users", &statistics.TableStats{
		Tuples: 1000,
		Pages:  100,
	})

	cm := NewCostModel(stats)

	op := &IndexScanOperator{
		Table:       "users",
		Index:       "idx_age",
		Selectivity: 0.1, // 10% selectivity
	}

	cost := cm.estimateIndexScan(op)

	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}

	// Index scan should be cheaper than seq scan for low selectivity
	seqScanOp := &SeqScanOperator{Table: "users"}
	seqScanCost := cm.estimateSeqScan(seqScanOp)

	if cost >= seqScanCost {
		t.Errorf("Index scan cost (%f) should be less than seq scan cost (%f)",
			cost, seqScanCost)
	}
}

func TestCostModel_EstimateVectorSearch(t *testing.T) {
	stats := statistics.NewStatisticsStore()
	stats.SetTableStats("documents", &statistics.TableStats{
		Tuples: 10000,
		Pages:  1000,
	})

	cm := NewCostModel(stats)

	op := &VectorSearchOperator{
		Table: "documents",
	}

	cost := cm.estimateVectorSearch(op)

	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}
}

func TestCostModel_EstimateHashJoin(t *testing.T) {
	stats := statistics.NewStatisticsStore()
	cm := NewCostModel(stats)

	left := &SeqScanOperator{
		Table:          "users",
		estimatedCards: 1000,
	}

	right := &SeqScanOperator{
		Table:          "orders",
		estimatedCards: 5000,
	}

	op := &HashJoinOperator{
		Left:  left,
		Right: right,
	}

	cost := cm.estimateHashJoin(op)

	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}
}

func TestCostModel_SetCPUCosts(t *testing.T) {
	stats := statistics.NewStatisticsStore()
	cm := NewCostModel(stats)

	cm.SetCPUCosts(5.0, 15.0, 8.0, 2.0)

	if cm.cpuTupleProcessing != 5.0 {
		t.Errorf("Expected cpuTupleProcessing=5.0, got %f", cm.cpuTupleProcessing)
	}

	if cm.cpuIndexLookup != 15.0 {
		t.Errorf("Expected cpuIndexLookup=15.0, got %f", cm.cpuIndexLookup)
	}
}

func TestCostModel_SetIOCosts(t *testing.T) {
	stats := statistics.NewStatisticsStore()
	cm := NewCostModel(stats)

	cm.SetIOCosts(150.0, 250.0, 120.0)

	if cm.ioSequentialRead != 150.0 {
		t.Errorf("Expected ioSequentialRead=150.0, got %f", cm.ioSequentialRead)
	}

	if cm.ioRandomRead != 250.0 {
		t.Errorf("Expected ioRandomRead=250.0, got %f", cm.ioRandomRead)
	}
}
