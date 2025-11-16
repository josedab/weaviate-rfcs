package optimizer

import (
	"math"

	"github.com/weaviate/weaviate/adapters/repos/db/statistics"
)

// CostModel estimates the cost of query execution plans.
type CostModel struct {
	// CPU cost factors (relative units)
	cpuTupleProcessing float64 // Per tuple processing cost
	cpuIndexLookup     float64 // Per index lookup cost
	cpuHashJoin        float64 // Per hash join probe cost
	cpuComparison      float64 // Per comparison cost

	// I/O cost factors
	ioSequentialRead float64 // Per page sequential read
	ioRandomRead     float64 // Per page random read
	ioIndexScan      float64 // Per index page scan

	// Network cost factors
	networkTransfer float64 // Per byte network transfer

	// Statistics for cost estimation
	statistics *statistics.StatisticsStore
}

// NewCostModel creates a new cost model with default cost factors.
// These values are based on the existing Weaviate query planner costs.
func NewCostModel(stats *statistics.StatisticsStore) *CostModel {
	return &CostModel{
		// CPU costs (calibrated from sorter/query_planner.go)
		cpuTupleProcessing: 2.0,
		cpuIndexLookup:     10.0,
		cpuHashJoin:        5.0,
		cpuComparison:      1.0,

		// I/O costs (calibrated from sorter/query_planner.go)
		ioSequentialRead: 100.0,  // FixedCostRowRead
		ioRandomRead:     200.0,  // FixedCostIndexSeek
		ioIndexScan:      100.0,  // FixedCostInvertedBucketRow

		// Network costs
		networkTransfer: 0.01,

		statistics: stats,
	}
}

// Estimate estimates the total cost of a query plan.
func (cm *CostModel) Estimate(plan *QueryPlan) float64 {
	if plan == nil || plan.Root == nil {
		return 0.0
	}

	return cm.estimateOperator(plan.Root)
}

// estimateOperator estimates the cost of a specific operator.
func (cm *CostModel) estimateOperator(op Operator) float64 {
	switch operator := op.(type) {
	case *SeqScanOperator:
		return cm.estimateSeqScan(operator)
	case *IndexScanOperator:
		return cm.estimateIndexScan(operator)
	case *VectorSearchOperator:
		return cm.estimateVectorSearch(operator)
	case *FilterOperator:
		return cm.estimateFilter(operator)
	case *HashJoinOperator:
		return cm.estimateHashJoin(operator)
	default:
		return 0.0
	}
}

// estimateSeqScan estimates the cost of a sequential scan.
func (cm *CostModel) estimateSeqScan(op *SeqScanOperator) float64 {
	tableStats := cm.statistics.GetTableStats(op.Table)
	if tableStats == nil {
		// Default estimate
		return 1000.0 * cm.ioSequentialRead
	}

	numPages := tableStats.Pages
	numTuples := tableStats.Tuples

	// I/O cost: read all pages sequentially
	ioCost := float64(numPages) * cm.ioSequentialRead

	// CPU cost: process all tuples
	cpuCost := float64(numTuples) * cm.cpuTupleProcessing

	return ioCost + cpuCost
}

// estimateIndexScan estimates the cost of an index scan.
func (cm *CostModel) estimateIndexScan(op *IndexScanOperator) float64 {
	tableStats := cm.statistics.GetTableStats(op.Table)
	if tableStats == nil {
		// Default estimate
		return 100.0 * cm.ioRandomRead
	}

	numTuples := tableStats.Tuples
	selectivity := op.Selectivity
	if selectivity == 0.0 {
		selectivity = 0.1 // Default selectivity
	}

	// Expected tuples to retrieve
	expectedTuples := int64(float64(numTuples) * selectivity)

	// Index lookup cost (B-tree traversal)
	indexCost := math.Log2(float64(numTuples)) * cm.cpuIndexLookup

	// Random I/O for each tuple (scattered reads)
	ioCost := float64(expectedTuples) * cm.ioRandomRead

	// CPU cost to process retrieved tuples
	cpuCost := float64(expectedTuples) * cm.cpuTupleProcessing

	return indexCost + ioCost + cpuCost
}

// estimateVectorSearch estimates the cost of vector search.
func (cm *CostModel) estimateVectorSearch(op *VectorSearchOperator) float64 {
	tableStats := cm.statistics.GetTableStats(op.Table)
	if tableStats == nil {
		// Default estimate for vector search
		return 500.0
	}

	numTuples := tableStats.Tuples

	// HNSW search complexity: O(log n * ef)
	// Using ef=100 as default
	ef := 100.0
	searchCost := math.Log2(float64(numTuples)) * ef * cm.cpuComparison

	// Random I/O for visited nodes
	ioVisits := math.Log2(float64(numTuples)) * ef
	ioCost := ioVisits * cm.ioRandomRead

	// CPU cost for distance calculations
	distanceCost := ef * 10.0 // Vector distance is more expensive

	return searchCost + ioCost + distanceCost
}

// estimateFilter estimates the cost of a filter operator.
func (cm *CostModel) estimateFilter(op *FilterOperator) float64 {
	// Cost of input operator
	inputCost := cm.estimateOperator(op.Input)

	// Cost of applying filter to each tuple
	inputCardinality := op.Input.EstimatedCardinality()
	filterCost := float64(inputCardinality) * cm.cpuComparison

	return inputCost + filterCost
}

// estimateHashJoin estimates the cost of a hash join.
func (cm *CostModel) estimateHashJoin(op *HashJoinOperator) float64 {
	// Cost of left input (build side)
	leftCost := cm.estimateOperator(op.Left)
	leftCardinality := op.Left.EstimatedCardinality()

	// Cost of right input (probe side)
	rightCost := cm.estimateOperator(op.Right)
	rightCardinality := op.Right.EstimatedCardinality()

	// Build hash table cost
	buildCost := float64(leftCardinality) * cm.cpuHashJoin

	// Probe hash table cost
	probeCost := float64(rightCardinality) * cm.cpuHashJoin

	return leftCost + rightCost + buildCost + probeCost
}

// SetCPUCosts allows calibration of CPU cost factors.
func (cm *CostModel) SetCPUCosts(tupleProcessing, indexLookup, hashJoin, comparison float64) {
	cm.cpuTupleProcessing = tupleProcessing
	cm.cpuIndexLookup = indexLookup
	cm.cpuHashJoin = hashJoin
	cm.cpuComparison = comparison
}

// SetIOCosts allows calibration of I/O cost factors.
func (cm *CostModel) SetIOCosts(sequentialRead, randomRead, indexScan float64) {
	cm.ioSequentialRead = sequentialRead
	cm.ioRandomRead = randomRead
	cm.ioIndexScan = indexScan
}

// SetNetworkCosts allows calibration of network cost factors.
func (cm *CostModel) SetNetworkCosts(transfer float64) {
	cm.networkTransfer = transfer
}
