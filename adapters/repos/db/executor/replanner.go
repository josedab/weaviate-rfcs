package executor

import (
	"context"

	"github.com/weaviate/weaviate/adapters/repos/db/optimizer"
	"github.com/weaviate/weaviate/adapters/repos/db/statistics"
)

// Replanner handles mid-execution replanning when estimates are significantly off.
type Replanner struct {
	statistics *statistics.StatisticsStore
	costModel  *optimizer.CostModel
	threshold  float64 // Replanning threshold (cardinality error ratio)
}

// NewReplanner creates a new replanner instance.
func NewReplanner(
	stats *statistics.StatisticsStore,
	costModel *optimizer.CostModel,
	threshold float64,
) *Replanner {
	if threshold == 0.0 {
		threshold = 10.0 // Default: replan if estimate is off by 10x
	}

	return &Replanner{
		statistics: stats,
		costModel:  costModel,
		threshold:  threshold,
	}
}

// Replan generates a new query plan based on runtime statistics.
func (r *Replanner) Replan(
	ctx context.Context,
	originalPlan *optimizer.QueryPlan,
	runtimeStats *OperatorStats,
) *optimizer.QueryPlan {
	// Check if replanning is warranted
	if !r.shouldReplan(runtimeStats) {
		return nil
	}

	// Update statistics based on actual runtime data
	r.updateStatistics(originalPlan, runtimeStats)

	// Generate new plan alternatives
	alternatives := r.generateAlternatives(originalPlan)

	// Estimate costs with updated statistics
	for _, plan := range alternatives {
		plan.Cost = r.costModel.Estimate(plan)
	}

	// Select the best alternative
	best := r.selectBest(alternatives)

	// Only return new plan if it's significantly better
	if best != nil && best.Cost < originalPlan.Cost*0.9 {
		return best
	}

	return nil
}

// shouldReplan determines if replanning is warranted based on runtime statistics.
func (r *Replanner) shouldReplan(stats *OperatorStats) bool {
	if stats.EstimatedCardinality == 0 {
		return false
	}

	error := stats.CardinalityError()

	// Replan if error exceeds threshold in either direction
	return error > r.threshold || error < (1.0/r.threshold)
}

// updateStatistics updates statistics based on actual runtime observations.
func (r *Replanner) updateStatistics(plan *optimizer.QueryPlan, stats *OperatorStats) {
	// Extract table name from plan
	var tableName string
	switch op := plan.Root.(type) {
	case *optimizer.SeqScanOperator:
		tableName = op.Table
	case *optimizer.IndexScanOperator:
		tableName = op.Table
	case *optimizer.VectorSearchOperator:
		tableName = op.Table
	}

	if tableName == "" {
		return
	}

	// Update table statistics with actual cardinality
	tableStats := r.statistics.GetTableStats(tableName)
	if tableStats == nil {
		tableStats = &statistics.TableStats{
			Tuples:  stats.ActualCardinality,
			Columns: make(map[string]*statistics.ColumnStats),
		}
		r.statistics.SetTableStats(tableName, tableStats)
	}
}

// generateAlternatives generates alternative execution plans.
func (r *Replanner) generateAlternatives(originalPlan *optimizer.QueryPlan) []*optimizer.QueryPlan {
	alternatives := make([]*optimizer.QueryPlan, 0)

	// Add the original plan as baseline
	alternatives = append(alternatives, originalPlan)

	// Generate alternative with different scan strategy
	switch originalPlan.Type {
	case optimizer.PlanTypeSeqScan:
		// Try index scan instead
		indexPlan := r.createIndexScanAlternative(originalPlan)
		if indexPlan != nil {
			alternatives = append(alternatives, indexPlan)
		}

	case optimizer.PlanTypeIndexScan:
		// Try sequential scan instead
		seqPlan := r.createSeqScanAlternative(originalPlan)
		if seqPlan != nil {
			alternatives = append(alternatives, seqPlan)
		}
	}

	return alternatives
}

// createIndexScanAlternative creates an index scan alternative.
func (r *Replanner) createIndexScanAlternative(originalPlan *optimizer.QueryPlan) *optimizer.QueryPlan {
	var tableName string
	if seqOp, ok := originalPlan.Root.(*optimizer.SeqScanOperator); ok {
		tableName = seqOp.Table
	} else {
		return nil
	}

	return &optimizer.QueryPlan{
		Root: &optimizer.IndexScanOperator{
			Table:       tableName,
			Index:       "default_idx",
			Selectivity: 0.1,
		},
		Type:        optimizer.PlanTypeIndexScan,
		Cardinality: 0,
		Cost:        0,
	}
}

// createSeqScanAlternative creates a sequential scan alternative.
func (r *Replanner) createSeqScanAlternative(originalPlan *optimizer.QueryPlan) *optimizer.QueryPlan {
	var tableName string
	if idxOp, ok := originalPlan.Root.(*optimizer.IndexScanOperator); ok {
		tableName = idxOp.Table
	} else {
		return nil
	}

	return &optimizer.QueryPlan{
		Root: &optimizer.SeqScanOperator{
			Table: tableName,
		},
		Type:        optimizer.PlanTypeSeqScan,
		Cardinality: 0,
		Cost:        0,
	}
}

// selectBest selects the plan with the lowest estimated cost.
func (r *Replanner) selectBest(plans []*optimizer.QueryPlan) *optimizer.QueryPlan {
	if len(plans) == 0 {
		return nil
	}

	best := plans[0]
	for _, plan := range plans[1:] {
		if plan.Cost < best.Cost {
			best = plan
		}
	}

	return best
}

// SetThreshold updates the replanning threshold.
func (r *Replanner) SetThreshold(threshold float64) {
	r.threshold = threshold
}

// GetThreshold returns the current replanning threshold.
func (r *Replanner) GetThreshold() float64 {
	return r.threshold
}
