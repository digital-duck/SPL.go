// Package ast defines the Abstract Syntax Tree nodes for SPL 2.0.
package ast

// Node is the base interface for all AST nodes.
type Node interface {
	nodeTag()
}

// Expr is the base interface for all expression nodes.
type Expr interface {
	Node
	exprTag()
}

// Stmt is the base interface for all statement nodes.
type Stmt interface {
	Node
	stmtTag()
}

// =============================================================================
// Expression Nodes
// =============================================================================

// Literal represents a string, integer, float, or bool literal.
type Literal struct {
	Value   string // always stored as string representation
	LitType string // "string", "integer", "float", "bool"
}

func (*Literal) nodeTag() {}
func (*Literal) exprTag() {}

// Identifier is a simple identifier (e.g., "profile").
type Identifier struct {
	Name string
}

func (*Identifier) nodeTag() {}
func (*Identifier) exprTag() {}

// DottedName is a dotted identifier (e.g., "context.user_profile").
type DottedName struct {
	Parts []string
}

func (*DottedName) nodeTag() {}
func (*DottedName) exprTag() {}

func (d *DottedName) FullName() string {
	result := ""
	for i, p := range d.Parts {
		if i > 0 {
			result += "."
		}
		result += p
	}
	return result
}

// ParamRef is a parameter reference (e.g., "@user_data").
type ParamRef struct {
	Name string
}

func (*ParamRef) nodeTag() {}
func (*ParamRef) exprTag() {}

// FunctionCall is a function call (e.g., "summarize(text, 200)").
type FunctionCall struct {
	Name      string
	Arguments []Expr
}

func (*FunctionCall) nodeTag() {}
func (*FunctionCall) exprTag() {}

// BinaryOp is a binary operation (e.g., "a + b").
type BinaryOp struct {
	Left  Expr
	Op    string // "+", "-", "||"
	Right Expr
}

func (*BinaryOp) nodeTag() {}
func (*BinaryOp) exprTag() {}

// NamedArg is a named argument in function call (e.g., "top_k=5").
type NamedArg struct {
	Name  string
	Value Expr
}

func (*NamedArg) nodeTag() {}
func (*NamedArg) exprTag() {}

// SystemRoleCall represents system_role("description").
type SystemRoleCall struct {
	Description string
}

func (*SystemRoleCall) nodeTag() {}
func (*SystemRoleCall) exprTag() {}

// ContextRef represents context.<field> reference.
type ContextRef struct {
	FieldName string
}

func (*ContextRef) nodeTag() {}
func (*ContextRef) exprTag() {}

// RagQuery represents rag.query("search text", top_k=5).
type RagQuery struct {
	QueryText Expr
	TopK      int // 0 means not set
}

func (*RagQuery) nodeTag() {}
func (*RagQuery) exprTag() {}

// MemoryGet represents memory.get("key").
type MemoryGet struct {
	Key string
}

func (*MemoryGet) nodeTag() {}
func (*MemoryGet) exprTag() {}

// FStringLiteral represents f'text {@var} text'.
type FStringLiteral struct {
	Template string
}

func (*FStringLiteral) nodeTag() {}
func (*FStringLiteral) exprTag() {}

// ListLiteral represents [] or [expr, expr, ...].
type ListLiteral struct {
	Elements []Expr
}

func (*ListLiteral) nodeTag() {}
func (*ListLiteral) exprTag() {}

// MapPair is a single key-value pair inside a MapLiteral.
type MapPair struct {
	Key   Expr
	Value Expr
}

// MapLiteral represents {'key': value, ...} — a key/value map literal.
type MapLiteral struct {
	Pairs []MapPair
}

func (*MapLiteral) nodeTag() {}
func (*MapLiteral) exprTag() {}

// StorageSpec represents STORAGE(backend, path).
type StorageSpec struct {
	Backend string
	Path    string
}

func (*StorageSpec) nodeTag() {}
func (*StorageSpec) exprTag() {}

// StorageSubscript represents @memory['key'].
type StorageSubscript struct {
	StorageVar string
	Key        Expr
}

func (*StorageSubscript) nodeTag() {}
func (*StorageSubscript) exprTag() {}

// =============================================================================
// Clause nodes
// =============================================================================

// SelectItem is a single item in a SELECT clause.
type SelectItem struct {
	Expression  Expr
	Alias       string
	LimitTokens int // 0 means not set
}

// Condition is a single deterministic comparison condition.
type Condition struct {
	Left     Expr
	Operator string // "=", "!=", ">", "<", ">=", "<=", "IN"
	Right    Expr
}

// WhereClause holds WHERE clause conditions joined by AND/OR.
type WhereClause struct {
	Conditions   []Condition
	Conjunctions []string // "AND" or "OR" between conditions
}

// OrderByItem is a single item in ORDER BY.
type OrderByItem struct {
	Expression Expr
	Direction  string // "ASC" or "DESC"
}

// GenerateClause represents GENERATE function(args) WITH options.
type GenerateClause struct {
	FunctionName string
	Arguments    []Expr
	OutputBudget int     // 0 means not set
	Temperature  float64 // 0 means not set
	OutputFormat string
	Schema       string
	Model        string
}

// StoreClause represents STORE RESULT IN memory.<key>.
type StoreClause struct {
	Key string
}

// FromClause represents FROM source AS alias.
type FromClause struct {
	Source Expr
	Alias  string
}

// Parameter is a function/workflow parameter definition.
type Parameter struct {
	Name         string
	ParamType    string
	DefaultValue Expr // may be nil
}

// CTEClause is a WITH <name> AS (...) common table expression.
type CTEClause struct {
	Name         string
	SelectItems  []SelectItem
	FromClause   *FromClause
	WhereClause  *WhereClause
	LimitTokens  int // 0 means not set
	NestedPrompt *PromptStatement
}

// =============================================================================
// Evaluate / While condition types
// =============================================================================

// SemanticCondition is a semantic condition evaluated by LLM.
// SemanticValue examples: "coherent", "startswith:prefix", "contains:val1|val2"
type SemanticCondition struct {
	SemanticValue string
}

// ComparisonCondition is a comparison condition in EVALUATE context.
type ComparisonCondition struct {
	Operator string // ">", "<", ">=", "<=", "=", "!="
	Right    Expr
}

// WhenClause represents WHEN <condition> THEN <statements>.
// Condition is one of: *SemanticCondition, *ComparisonCondition, *Condition.
type WhenClause struct {
	Condition  interface{} // *SemanticCondition | *ComparisonCondition | *Condition
	Statements []Stmt
}

// ExceptionHandler represents WHEN <exception_type> THEN <statements>.
type ExceptionHandler struct {
	ExceptionType string
	Statements    []Stmt
}

// =============================================================================
// Top-level Statement nodes
// =============================================================================

// Program is the top-level program node.
type Program struct {
	Statements []Stmt
}

// PromptStatement represents PROMPT <name> WITH BUDGET ... SELECT ... GENERATE ...
type PromptStatement struct {
	Name         string
	Budget       int // 0 means not set
	Model        string
	CacheDuration string
	Version      string
	OnGrid       string
	MinVRAMGB    float64
	CTEs         []CTEClause
	SelectItems  []SelectItem
	WhereClause  *WhereClause
	OrderBy      []OrderByItem
	GenerateClause *GenerateClause
	StoreClause  *StoreClause
}

func (*PromptStatement) nodeTag() {}
func (*PromptStatement) stmtTag() {}

// CreateFunctionStatement represents CREATE FUNCTION <name>(...) RETURNS <type> AS $$ ... $$
type CreateFunctionStatement struct {
	Name       string
	Parameters []Parameter
	ReturnType string
	Body       string
}

func (*CreateFunctionStatement) nodeTag() {}
func (*CreateFunctionStatement) stmtTag() {}

// ExplainStatement represents EXPLAIN PROMPT <name>.
type ExplainStatement struct {
	PromptName string
}

func (*ExplainStatement) nodeTag() {}
func (*ExplainStatement) stmtTag() {}

// ExecuteStatement represents EXECUTE PROMPT <name> WITH PARAMS (...).
type ExecuteStatement struct {
	PromptName string
	Params     map[string]Expr
}

func (*ExecuteStatement) nodeTag() {}
func (*ExecuteStatement) stmtTag() {}

// WorkflowStatement represents WORKFLOW <name> INPUT ... OUTPUT ... DO ... END
type WorkflowStatement struct {
	Name              string
	Inputs            []Parameter
	Outputs           []Parameter
	Security          map[string]string
	Accounting        map[string]string
	Labels            map[string]string
	Body              []Stmt
	ExceptionHandlers []ExceptionHandler
}

func (*WorkflowStatement) nodeTag() {}
func (*WorkflowStatement) stmtTag() {}

// ProcedureStatement represents PROCEDURE <name>(...) RETURNS type DO ... END
type ProcedureStatement struct {
	Name              string
	Parameters        []Parameter
	ReturnType        string
	Security          map[string]string
	Accounting        map[string]string
	Body              []Stmt
	ExceptionHandlers []ExceptionHandler
}

func (*ProcedureStatement) nodeTag() {}
func (*ProcedureStatement) stmtTag() {}

// DoBlock represents DO <statements> [EXCEPTION ...] END
type DoBlock struct {
	Statements        []Stmt
	ExceptionHandlers []ExceptionHandler
}

func (*DoBlock) nodeTag() {}
func (*DoBlock) stmtTag() {}

// EvaluateStatement represents EVALUATE <expr> WHEN <condition> THEN <statements> ... END
type EvaluateStatement struct {
	Expression    Expr
	WhenClauses   []WhenClause
	ElseStatements []Stmt
}

func (*EvaluateStatement) nodeTag() {}
func (*EvaluateStatement) stmtTag() {}

// WhileStatement represents WHILE <condition> DO <statements> END
// Condition is one of: *Condition, *SemanticCondition, Expr
type WhileStatement struct {
	Condition     interface{} // *Condition | *SemanticCondition | Expr
	Body          []Stmt
	MaxIterations int // 0 means use default
}

func (*WhileStatement) nodeTag() {}
func (*WhileStatement) stmtTag() {}

// CommitStatement represents COMMIT <expr> WITH status='...'
type CommitStatement struct {
	Expression Expr
	Options    map[string]Expr
}

func (*CommitStatement) nodeTag() {}
func (*CommitStatement) stmtTag() {}

// RetryStatement represents RETRY WITH fallback options
type RetryStatement struct {
	Options map[string]Expr
	Limit   int // 0 means not set
}

func (*RetryStatement) nodeTag() {}
func (*RetryStatement) stmtTag() {}

// RaiseStatement represents RAISE <exception_type> [message]
type RaiseStatement struct {
	ExceptionType string
	Message       string
}

func (*RaiseStatement) nodeTag() {}
func (*RaiseStatement) stmtTag() {}

// LoggingStatement represents LOGGING <expr> [LEVEL DEBUG|INFO|WARN|ERROR] [TO 'file_path']
type LoggingStatement struct {
	Expression  Expr
	Level       string // "DEBUG", "INFO", "WARN", "ERROR"
	Destination string // empty means console
}

func (*LoggingStatement) nodeTag() {}
func (*LoggingStatement) stmtTag() {}

// AssignmentStatement represents @var := expr
type AssignmentStatement struct {
	Variable   string
	Expression Expr
}

func (*AssignmentStatement) nodeTag() {}
func (*AssignmentStatement) stmtTag() {}

// StoreStatement represents STORE @var IN memory.<key>
type StoreStatement struct {
	Variable string
	Key      string
}

func (*StoreStatement) nodeTag() {}
func (*StoreStatement) stmtTag() {}

// GenerateIntoStatement represents GENERATE function(args) WITH options INTO @var
type GenerateIntoStatement struct {
	GenerateClause GenerateClause
	TargetVariable string
}

func (*GenerateIntoStatement) nodeTag() {}
func (*GenerateIntoStatement) stmtTag() {}

// CallStatement represents CALL procedure(args) INTO @var
type CallStatement struct {
	ProcedureName  string
	Arguments      []Expr
	TargetVariable string
}

func (*CallStatement) nodeTag() {}
func (*CallStatement) stmtTag() {}

// SelectIntoStatement represents SELECT ... FROM ... INTO @var (used inside workflows)
type SelectIntoStatement struct {
	SelectItems     []SelectItem
	FromClause      *FromClause
	WhereClause     *WhereClause
	TargetVariable  string   // single-var (backward compat)
	TargetVariables []string // multi-var INTO
	CTEs            []CTEClause
}

func (*SelectIntoStatement) nodeTag() {}
func (*SelectIntoStatement) stmtTag() {}

// StorageAssignStatement represents @memory['key'] := expr
type StorageAssignStatement struct {
	StorageVar string
	Key        Expr
	Value      Expr
}

func (*StorageAssignStatement) nodeTag() {}
func (*StorageAssignStatement) stmtTag() {}

// =============================================================================
// SPL 3.0 Statement nodes
// =============================================================================

// ImportStatement represents IMPORT 'path/to/file.spl'
// The imported file's CREATE FUNCTION and WORKFLOW/PROCEDURE definitions are
// merged into the calling program's registry before execution begins.
type ImportStatement struct {
	Path string // path relative to the calling .spl file's directory
}

func (*ImportStatement) nodeTag() {}
func (*ImportStatement) stmtTag() {}

// CallBranch is one branch inside a CALL PARALLEL block.
type CallBranch struct {
	ProcedureName  string
	Arguments      []Expr
	TargetVariable string // may be empty
}

// CallParallelStatement represents CALL PARALLEL ... END
// Each branch executes concurrently. Every branch receives a snapshot of the
// parent variable scope; only its TargetVariable is written back to the parent.
type CallParallelStatement struct {
	Branches []CallBranch
}

func (*CallParallelStatement) nodeTag() {}
func (*CallParallelStatement) stmtTag() {}
