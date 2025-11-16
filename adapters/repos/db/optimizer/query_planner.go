package optimizer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/weaviate/weaviate/adapters/repos/db/statistics"
)

// QueryPlanner is responsible for generating and selecting optimal query execution plans.
type QueryPlanner struct {
	statistics *statistics.StatisticsStore
	costModel  *CostModel
	planCache  *PlanCache
}

// NewQueryPlanner creates a new query planner instance.
func NewQueryPlanner(
	stats *statistics.StatisticsStore,
	costModel *CostModel,
	planCache *PlanCache,
) *QueryPlanner {
	return &QueryPlanner{
		statistics: stats,
		costModel:  costModel,
		planCache:  planCache,
	}
}

// Plan generates an optimal query execution plan.
func (p *QueryPlanner) Plan(ctx context.Context, query *Query) (*QueryPlan, error) {
	// Step 1: Check plan cache
	queryHash := p.hashQuery(query)
	if cachedPlan := p.planCache.Get(queryHash); cachedPlan != nil {
		return cachedPlan, nil
	}

	// Step 2: Generate logical plan
	logical := p.generateLogical(query)

	// Step 3: Generate alternative physical plans
	alternatives := p.generatePhysical(logical)

	// Step 4: Estimate costs for each alternative
	for _, plan := range alternatives {
		plan.Cost = p.costModel.Estimate(plan)
		plan.Cardinality = p.estimateCardinality(plan)
	}

	// Step 5: Select best plan
	best := p.selectBest(alternatives)

	// Step 6: Cache the best plan
	p.planCache.Store(queryHash, best)

	return best, nil
}

// generateLogical creates a logical plan from the query.
func (p *QueryPlanner) generateLogical(query *Query) *LogicalPlan {
	return &LogicalPlan{
		Query:      query,
		Operations: p.parseQueryOperations(query),
	}
}

// parseQueryOperations extracts logical operations from the query.
func (p *QueryPlanner) parseQueryOperations(query *Query) []LogicalOperation {
	ops := make([]LogicalOperation, 0)

	// Add scan operation
	ops = append(ops, &LogicalScan{
		Table: query.Table,
	})

	// Add filter operations
	if query.Filter != nil {
		ops = append(ops, &LogicalFilter{
			Predicate: query.Filter,
		})
	}

	// Add sort operation
	if query.Sort != nil {
		ops = append(ops, &LogicalSort{
			Column: query.Sort.Column,
			Order:  query.Sort.Order,
		})
	}

	// Add limit operation
	if query.Limit > 0 {
		ops = append(ops, &LogicalLimit{
			Count: query.Limit,
		})
	}

	return ops
}

// generatePhysical generates alternative physical execution plans.
func (p *QueryPlanner) generatePhysical(logical *LogicalPlan) []*QueryPlan {
	plans := make([]*QueryPlan, 0)

	// Generate plan with sequential scan
	seqScanPlan := p.createSeqScanPlan(logical)
	plans = append(plans, seqScanPlan)

	// Generate plan with index scan if applicable
	if p.canUseIndexScan(logical) {
		indexScanPlan := p.createIndexScanPlan(logical)
		plans = append(plans, indexScanPlan)
	}

	// Generate plan with vector search if applicable
	if p.canUseVectorSearch(logical) {
		vectorPlan := p.createVectorSearchPlan(logical)
		plans = append(plans, vectorPlan)
	}

	return plans
}

// createSeqScanPlan creates a plan using sequential scan.
func (p *QueryPlanner) createSeqScanPlan(logical *LogicalPlan) *QueryPlan {
	return &QueryPlan{
		Root: &SeqScanOperator{
			Table: logical.Query.Table,
		},
		Type:        PlanTypeSeqScan,
		Cardinality: 0, // Will be estimated later
		Cost:        0, // Will be estimated later
	}
}

// createIndexScanPlan creates a plan using index scan.
func (p *QueryPlanner) createIndexScanPlan(logical *LogicalPlan) *QueryPlan {
	return &QueryPlan{
		Root: &IndexScanOperator{
			Table: logical.Query.Table,
			Index: p.selectBestIndex(logical.Query),
		},
		Type:        PlanTypeIndexScan,
		Cardinality: 0,
		Cost:        0,
	}
}

// createVectorSearchPlan creates a plan using vector search.
func (p *QueryPlanner) createVectorSearchPlan(logical *LogicalPlan) *QueryPlan {
	return &QueryPlan{
		Root: &VectorSearchOperator{
			Table: logical.Query.Table,
		},
		Type:        PlanTypeVectorSearch,
		Cardinality: 0,
		Cost:        0,
	}
}

// canUseIndexScan checks if an index scan is applicable.
func (p *QueryPlanner) canUseIndexScan(logical *LogicalPlan) bool {
	return logical.Query.Filter != nil
}

// canUseVectorSearch checks if vector search is applicable.
func (p *QueryPlanner) canUseVectorSearch(logical *LogicalPlan) bool {
	return logical.Query.VectorSearch != nil
}

// selectBestIndex selects the best index for the query.
func (p *QueryPlanner) selectBestIndex(query *Query) string {
	// Simple heuristic: return first available index
	// In a full implementation, this would use cost-based selection
	if query.Filter != nil && query.Filter.Column != "" {
		return fmt.Sprintf("idx_%s", query.Filter.Column)
	}
	return ""
}

// estimateCardinality estimates the output cardinality of a plan.
func (p *QueryPlanner) estimateCardinality(plan *QueryPlan) int64 {
	// Get table statistics
	var tableName string
	switch op := plan.Root.(type) {
	case *SeqScanOperator:
		tableName = op.Table
	case *IndexScanOperator:
		tableName = op.Table
	case *VectorSearchOperator:
		tableName = op.Table
	}

	tableStats := p.statistics.GetTableStats(tableName)
	if tableStats == nil {
		return 1000 // Default estimate
	}

	// Start with total tuples
	cardinality := tableStats.Tuples

	// Apply selectivity based on plan type
	switch plan.Type {
	case PlanTypeIndexScan:
		// Index scans typically filter data
		cardinality = int64(float64(cardinality) * 0.1) // Assume 10% selectivity
	case PlanTypeVectorSearch:
		// Vector search returns top-k results
		if cardinality > 100 {
			cardinality = 100 // Typical vector search limit
		}
	}

	return cardinality
}

// selectBest selects the plan with the lowest cost.
func (p *QueryPlanner) selectBest(plans []*QueryPlan) *QueryPlan {
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

// hashQuery generates a hash of the query for caching.
func (p *QueryPlanner) hashQuery(query *Query) string {
	// Simple hash implementation
	data := fmt.Sprintf("%s:%v:%v:%d",
		query.Table,
		query.Filter,
		query.Sort,
		query.Limit,
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
