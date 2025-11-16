package optimizer

import (
	"math"

	"github.com/weaviate/weaviate/adapters/repos/db/statistics"
)

// IndexSelector selects the most appropriate index for a query.
type IndexSelector struct {
	indexes    map[string][]Index
	costModel  *CostModel
	statistics *statistics.StatisticsStore
}

// Index represents a database index.
type Index struct {
	Name       string    // Index name
	Type       IndexType // Index type (vector, inverted, btree)
	Table      string    // Table name
	Columns    []string  // Indexed columns
	Properties IndexProperties
}

// IndexType represents different types of indexes.
type IndexType int

const (
	IndexTypeVector IndexType = iota
	IndexTypeInverted
	IndexTypeBTree
	IndexTypeHash
)

// IndexProperties contains index-specific properties.
type IndexProperties struct {
	Selectivity float64 // Estimated selectivity
	Size        int64   // Index size in bytes
	Height      int     // Tree height (for B-tree indexes)
}

// IndexChoice represents a selected index with estimated costs.
type IndexChoice struct {
	Index       Index   // The selected index
	Selectivity float64 // Estimated selectivity
	Cost        float64 // Estimated cost of using this index
}

// NewIndexSelector creates a new index selector.
func NewIndexSelector(
	costModel *CostModel,
	statistics *statistics.StatisticsStore,
) *IndexSelector {
	return &IndexSelector{
		indexes:    make(map[string][]Index),
		costModel:  costModel,
		statistics: statistics,
	}
}

// RegisterIndex registers an index for selection.
func (s *IndexSelector) RegisterIndex(index Index) {
	if s.indexes[index.Table] == nil {
		s.indexes[index.Table] = make([]Index, 0)
	}
	s.indexes[index.Table] = append(s.indexes[index.Table], index)
}

// SelectIndex selects the best index for a query.
func (s *IndexSelector) SelectIndex(query *Query) *IndexChoice {
	candidates := s.findCandidates(query)

	if len(candidates) == 0 {
		return nil
	}

	best := &IndexChoice{
		Cost: math.MaxFloat64,
	}

	for _, index := range candidates {
		cost := s.estimateCost(index, query)
		selectivity := s.estimateSelectivity(index, query)

		if cost < best.Cost {
			best = &IndexChoice{
				Index:       index,
				Selectivity: selectivity,
				Cost:        cost,
			}
		}
	}

	return best
}

// findCandidates finds all indexes that could potentially be used for the query.
func (s *IndexSelector) findCandidates(query *Query) []Index {
	var candidates []Index

	// Get all indexes for this table
	tableIndexes := s.indexes[query.Table]
	if tableIndexes == nil {
		return candidates
	}

	// Check for vector indexes
	if query.VectorSearch != nil {
		for _, idx := range tableIndexes {
			if idx.Type == IndexTypeVector {
				candidates = append(candidates, idx)
			}
		}
	}

	// Check for inverted indexes (text search)
	if s.hasTextSearch(query) {
		for _, idx := range tableIndexes {
			if idx.Type == IndexTypeInverted {
				candidates = append(candidates, idx)
			}
		}
	}

	// Check for B-tree indexes on filter columns
	if query.Filter != nil {
		filterColumns := s.extractFilterColumns(query.Filter)
		for _, idx := range tableIndexes {
			if idx.Type == IndexTypeBTree && s.indexCoversColumns(idx, filterColumns) {
				candidates = append(candidates, idx)
			}
		}
	}

	return candidates
}

// hasTextSearch checks if the query contains text search operations.
func (s *IndexSelector) hasTextSearch(query *Query) bool {
	// In a real implementation, this would check for text search predicates
	return false
}

// extractFilterColumns extracts column names from filter expressions.
func (s *IndexSelector) extractFilterColumns(filter *FilterExpr) []string {
	columns := make([]string, 0)

	if filter.Column != "" {
		columns = append(columns, filter.Column)
	}

	if filter.And != nil {
		columns = append(columns, s.extractFilterColumns(filter.And)...)
	}

	if filter.Or != nil {
		columns = append(columns, s.extractFilterColumns(filter.Or)...)
	}

	return columns
}

// indexCoversColumns checks if an index covers the given columns.
func (s *IndexSelector) indexCoversColumns(index Index, columns []string) bool {
	if len(columns) == 0 {
		return false
	}

	// Check if the first index column matches any filter column
	// In a real implementation, this would be more sophisticated
	for _, col := range columns {
		for _, idxCol := range index.Columns {
			if col == idxCol {
				return true
			}
		}
	}

	return false
}

// estimateCost estimates the cost of using a specific index.
func (s *IndexSelector) estimateCost(index Index, query *Query) float64 {
	switch index.Type {
	case IndexTypeVector:
		return s.estimateVectorIndexCost(index, query)
	case IndexTypeInverted:
		return s.estimateInvertedIndexCost(index, query)
	case IndexTypeBTree:
		return s.estimateBTreeIndexCost(index, query)
	case IndexTypeHash:
		return s.estimateHashIndexCost(index, query)
	default:
		return math.MaxFloat64
	}
}

// estimateVectorIndexCost estimates the cost of a vector index scan.
func (s *IndexSelector) estimateVectorIndexCost(index Index, query *Query) float64 {
	tableStats := s.statistics.GetTableStats(index.Table)
	if tableStats == nil {
		return 1000.0
	}

	// HNSW search cost: O(log n * ef)
	ef := 100.0
	numTuples := float64(tableStats.Tuples)
	searchCost := math.Log2(numTuples) * ef * 10.0 // Distance calculations are expensive

	return searchCost
}

// estimateInvertedIndexCost estimates the cost of an inverted index scan.
func (s *IndexSelector) estimateInvertedIndexCost(index Index, query *Query) float64 {
	// Inverted index scans are generally very efficient
	return 50.0 * float64(len(index.Columns))
}

// estimateBTreeIndexCost estimates the cost of a B-tree index scan.
func (s *IndexSelector) estimateBTreeIndexCost(index Index, query *Query) float64 {
	tableStats := s.statistics.GetTableStats(index.Table)
	if tableStats == nil {
		return 500.0
	}

	numTuples := float64(tableStats.Tuples)
	selectivity := s.estimateSelectivity(index, query)

	// B-tree lookup cost: O(log n) + selectivity * n
	lookupCost := math.Log2(numTuples) * 10.0
	scanCost := numTuples * selectivity * 100.0

	return lookupCost + scanCost
}

// estimateHashIndexCost estimates the cost of a hash index scan.
func (s *IndexSelector) estimateHashIndexCost(index Index, query *Query) float64 {
	// Hash index has O(1) lookup cost
	return 10.0
}

// estimateSelectivity estimates the selectivity of using an index for a query.
func (s *IndexSelector) estimateSelectivity(index Index, query *Query) float64 {
	if query.Filter == nil {
		return 1.0
	}

	// Get selectivity estimate from statistics
	if len(index.Columns) > 0 {
		colStats := s.statistics.GetColumnStats(index.Table, index.Columns[0])
		if colStats != nil {
			// Use column statistics to estimate selectivity
			return 0.1 // Placeholder
		}
	}

	// Default selectivity
	return 0.1
}

// GetIndexCount returns the total number of registered indexes.
func (s *IndexSelector) GetIndexCount() int {
	count := 0
	for _, indexes := range s.indexes {
		count += len(indexes)
	}
	return count
}

// GetTableIndexes returns all indexes for a specific table.
func (s *IndexSelector) GetTableIndexes(table string) []Index {
	return s.indexes[table]
}
