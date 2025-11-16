package optimizer

// Query represents a database query to be planned.
type Query struct {
	Table        string        // Table/collection name
	Filter       *FilterExpr   // Filter predicate
	Sort         *SortExpr     // Sort specification
	Limit        int64         // Result limit
	VectorSearch *VectorSearch // Vector search parameters
}

// FilterExpr represents a filter expression.
type FilterExpr struct {
	Column   string      // Column name
	Operator string      // Operator (=, <, >, !=, etc.)
	Value    interface{} // Filter value
	And      *FilterExpr // AND conjunction
	Or       *FilterExpr // OR conjunction
}

// SortExpr represents a sort specification.
type SortExpr struct {
	Column string // Column to sort by
	Order  string // ASC or DESC
}

// VectorSearch represents vector search parameters.
type VectorSearch struct {
	Vector []float32 // Query vector
	TopK   int       // Number of results
}

// QueryPlan represents a physical execution plan.
type QueryPlan struct {
	Root        Operator      // Root operator of the plan
	Type        PlanType      // Type of plan
	Cost        float64       // Estimated cost
	Cardinality int64         // Estimated output cardinality
	Runtime     *RuntimeStats // Runtime statistics (populated during execution)
}

// PlanType represents different types of query plans.
type PlanType int

const (
	PlanTypeSeqScan PlanType = iota
	PlanTypeIndexScan
	PlanTypeVectorSearch
	PlanTypeHashJoin
)

// RuntimeStats holds runtime statistics for a plan.
type RuntimeStats struct {
	ActualCardinality int64   // Actual rows processed
	ExecutionTimeMs   float64 // Execution time in milliseconds
}

// LogicalPlan represents a logical query plan.
type LogicalPlan struct {
	Query      *Query             // Original query
	Operations []LogicalOperation // Logical operations
}

// LogicalOperation is an interface for logical operations.
type LogicalOperation interface {
	Type() string
}

// LogicalScan represents a table scan operation.
type LogicalScan struct {
	Table string
}

func (ls *LogicalScan) Type() string {
	return "Scan"
}

// LogicalFilter represents a filter operation.
type LogicalFilter struct {
	Predicate *FilterExpr
}

func (lf *LogicalFilter) Type() string {
	return "Filter"
}

// LogicalSort represents a sort operation.
type LogicalSort struct {
	Column string
	Order  string
}

func (ls *LogicalSort) Type() string {
	return "Sort"
}

// LogicalLimit represents a limit operation.
type LogicalLimit struct {
	Count int64
}

func (ll *LogicalLimit) Type() string {
	return "Limit"
}

// Operator is an interface for physical operators.
type Operator interface {
	Execute(ctx interface{}) interface{}
	EstimatedCardinality() int64
	String() string
}

// SeqScanOperator represents a sequential scan operator.
type SeqScanOperator struct {
	Table          string
	estimatedCards int64
}

func (op *SeqScanOperator) Execute(ctx interface{}) interface{} {
	return nil // Placeholder
}

func (op *SeqScanOperator) EstimatedCardinality() int64 {
	return op.estimatedCards
}

func (op *SeqScanOperator) String() string {
	return "SeqScan(" + op.Table + ")"
}

// IndexScanOperator represents an index scan operator.
type IndexScanOperator struct {
	Table          string
	Index          string
	Selectivity    float64
	estimatedCards int64
}

func (op *IndexScanOperator) Execute(ctx interface{}) interface{} {
	return nil // Placeholder
}

func (op *IndexScanOperator) EstimatedCardinality() int64 {
	return op.estimatedCards
}

func (op *IndexScanOperator) String() string {
	return "IndexScan(" + op.Table + ", " + op.Index + ")"
}

// VectorSearchOperator represents a vector search operator.
type VectorSearchOperator struct {
	Table          string
	estimatedCards int64
}

func (op *VectorSearchOperator) Execute(ctx interface{}) interface{} {
	return nil // Placeholder
}

func (op *VectorSearchOperator) EstimatedCardinality() int64 {
	return op.estimatedCards
}

func (op *VectorSearchOperator) String() string {
	return "VectorSearch(" + op.Table + ")"
}

// FilterOperator represents a filter operator.
type FilterOperator struct {
	Input          Operator
	Predicate      *FilterExpr
	estimatedCards int64
}

func (op *FilterOperator) Execute(ctx interface{}) interface{} {
	return nil // Placeholder
}

func (op *FilterOperator) EstimatedCardinality() int64 {
	return op.estimatedCards
}

func (op *FilterOperator) String() string {
	return "Filter"
}

// HashJoinOperator represents a hash join operator.
type HashJoinOperator struct {
	Left           Operator
	Right          Operator
	JoinKey        string
	estimatedCards int64
}

func (op *HashJoinOperator) Execute(ctx interface{}) interface{} {
	return nil // Placeholder
}

func (op *HashJoinOperator) EstimatedCardinality() int64 {
	return op.estimatedCards
}

func (op *HashJoinOperator) String() string {
	return "HashJoin"
}
