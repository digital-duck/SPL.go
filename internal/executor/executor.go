// Package executor implements the SPL 2.0 runtime execution engine.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/ast"
	"github.com/digital-duck/spl20go/internal/functions"
	"github.com/digital-duck/spl20go/internal/lexer"
	"github.com/digital-duck/spl20go/internal/parser"
	"github.com/digital-duck/spl20go/internal/stdlib"
	"github.com/digital-duck/spl20go/internal/storage"
)

// =============================================================================
// Result Types
// =============================================================================

// SPLResult holds the result of executing an SPL PROMPT statement.
type SPLResult struct {
	Content      string
	Prompt       string
	Model        string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	LatencyMs    float64
	CostUSD      float64
}

// WorkflowResult holds the result of executing an SPL WORKFLOW statement.
type WorkflowResult struct {
	Output          map[string]string
	Status          string
	TotalLLMCalls   int
	TotalInputToks  int
	TotalOutputToks int
	TotalLatencyMs  float64
	TotalCostUSD    float64
	CommittedValue  string
	CommittedOpts   map[string]string
}

// =============================================================================
// SPL Error Types
// =============================================================================

// SPLError is a runtime error that can be caught by EXCEPTION handlers.
type SPLError struct {
	Type    string
	Message string
}

func (e *SPLError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Type, e.Message)
	}
	return e.Type
}

const (
	ErrHallucination   = "HallucinationDetected"
	ErrRefusal         = "RefusalToAnswer"
	ErrContextLength   = "ContextLengthExceeded"
	ErrModelOverloaded = "ModelOverloaded"
	ErrQuality         = "QualityBelowThreshold"
	ErrMaxIterations   = "MaxIterationsReached"
	ErrBudget          = "BudgetExceeded"
	ErrNodeUnavailable = "NodeUnavailable"
)

// =============================================================================
// Workflow Execution State
// =============================================================================

// WorkflowState holds mutable state during workflow execution.
type WorkflowState struct {
	mu              sync.RWMutex
	Variables       map[string]string
	Memory          map[string]string    // in-process cache
	MemStore        *storage.MemoryStore // nil if not opened
	Committed       bool
	CommittedValue  string
	CommittedOpts   map[string]string
	TotalLLMCalls   int
	TotalInputToks  int
	TotalOutputToks int
	TotalLatencyMs  float64
	TotalCostUSD    float64
	MaxWorkers      int // 0 = sequential, >0 = parallel
}

func newWorkflowState(params map[string]string) *WorkflowState {
	s := &WorkflowState{
		Variables:     make(map[string]string),
		Memory:        make(map[string]string),
		CommittedOpts: make(map[string]string),
	}
	// Attempt to open the default SQLite memory store for persistence.
	if ms, err := storage.DefaultMemoryStore(); err == nil {
		s.MemStore = ms
		// Pre-populate in-process cache from DB
		if keys, err := ms.ListKeys(); err == nil {
			for _, k := range keys {
				if v, ok := ms.Get(k); ok {
					s.Memory[k] = v
				}
			}
		}
	}
	for k, v := range params {
		s.Variables[k] = v
	}
	return s
}

func (s *WorkflowState) setVar(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Variables[name] = value
}

func (s *WorkflowState) getVar(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Variables[name]
}

func (s *WorkflowState) recordLLMCall(res *adapter.GenerationResult) {
	s.TotalLLMCalls++
	s.TotalInputToks += res.InputTokens
	s.TotalOutputToks += res.OutputTokens
	s.TotalLatencyMs += res.LatencyMs
	s.TotalCostUSD += res.CostUSD
}

// =============================================================================
// Warning helpers
// =============================================================================

// warnNotImplemented prints a consistent warning to stderr when a feature is
// not fully supported in the Go runtime.
func warnNotImplemented(feature string) {
	fmt.Fprintf(os.Stderr, "WARNING: [%s] not fully supported in spl-go (Go runtime). Use 'spl' (Python) for this feature. See ROADMAP in docs/DESIGN.md\n", feature)
}

// =============================================================================
// Executor
// =============================================================================

const (
	defaultMaxLLMCalls    = 25
	defaultMaxTotalTokens = 100_000
	defaultMaxIterations  = 100
)

// Executor executes SPL 2.0/3.0 programs.
type Executor struct {
	Adapter        adapter.Adapter
	MaxLLMCalls    int
	MaxTotalTokens int
	MaxWorkers     int // 0 = sequential, >0 = parallel workflow steps
	// SourceDir is the directory of the top-level .spl file being executed.
	// Used to resolve IMPORT paths relative to the calling file.
	SourceDir      string
	Tools          map[string]func(args []string) string
	Functions      map[string]*ast.CreateFunctionStatement
	Procedures     map[string]*ast.ProcedureStatement
	Workflows      map[string]*ast.WorkflowStatement
	Registry       *functions.Registry
}

// New creates a new Executor with the given adapter.
func New(a adapter.Adapter) *Executor {
	return &Executor{
		Adapter:        a,
		MaxLLMCalls:    defaultMaxLLMCalls,
		MaxTotalTokens: defaultMaxTotalTokens,
		MaxWorkers:     0,
		Tools:          make(map[string]func(args []string) string),
		Functions:      make(map[string]*ast.CreateFunctionStatement),
		Procedures:     make(map[string]*ast.ProcedureStatement),
		Workflows:      make(map[string]*ast.WorkflowStatement),
		Registry:       functions.New(),
	}
}

// ExecuteProgram runs all statements in a program and returns results.
func (e *Executor) ExecuteProgram(ctx context.Context, program *ast.Program, params map[string]string) ([]interface{}, error) {
	if params == nil {
		params = make(map[string]string)
	}

	// Registration pass: process IMPORTs and register all definitions before execution.
	if err := e.registerProgram(ctx, program); err != nil {
		return nil, err
	}

	var results []interface{}
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.PromptStatement:
			res, err := e.ExecutePrompt(ctx, s, params)
			if err != nil {
				return results, err
			}
			results = append(results, res)
		case *ast.WorkflowStatement:
			res, err := e.ExecuteWorkflow(ctx, s, params)
			if err != nil {
				return results, err
			}
			results = append(results, res)
		case *ast.CreateFunctionStatement, *ast.ProcedureStatement, *ast.ImportStatement:
			// Already handled in registration pass; skip execution.
		}
	}
	return results, nil
}

// registerProgram processes IMPORT statements and registers all definitions
// (CREATE FUNCTION, PROCEDURE, WORKFLOW) found in the program and its imports.
func (e *Executor) registerProgram(ctx context.Context, program *ast.Program) error {
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.ImportStatement:
			if err := e.execImport(ctx, s); err != nil {
				return err
			}
		case *ast.CreateFunctionStatement:
			e.Functions[s.Name] = s
			e.Registry.RegisterFunction(s)
		case *ast.ProcedureStatement:
			e.Procedures[s.Name] = s
			e.Registry.RegisterProcedure(s)
		case *ast.WorkflowStatement:
			e.Workflows[s.Name] = s
		}
	}
	return nil
}

// execImport loads an imported .spl file, parses it, and registers its
// definitions into the executor. Path is resolved relative to SourceDir.
func (e *Executor) execImport(_ context.Context, stmt *ast.ImportStatement) error {
	importPath := stmt.Path
	if !filepath.IsAbs(importPath) && e.SourceDir != "" {
		importPath = filepath.Join(e.SourceDir, importPath)
	}

	src, err := os.ReadFile(importPath)
	if err != nil {
		return fmt.Errorf("IMPORT %q: %w", stmt.Path, err)
	}

	l := lexer.New(string(src))
	tokens, err := l.Tokenize()
	if err != nil {
		return fmt.Errorf("IMPORT %q: lexer error: %w", stmt.Path, err)
	}

	p := parser.New(tokens)
	prog, err := p.Parse()
	if err != nil {
		return fmt.Errorf("IMPORT %q: parse error: %w", stmt.Path, err)
	}

	// Register definitions from the imported file (no top-level execution).
	for _, s := range prog.Statements {
		switch def := s.(type) {
		case *ast.CreateFunctionStatement:
			e.Functions[def.Name] = def
			e.Registry.RegisterFunction(def)
		case *ast.ProcedureStatement:
			e.Procedures[def.Name] = def
			e.Registry.RegisterProcedure(def)
		case *ast.WorkflowStatement:
			e.Workflows[def.Name] = def
		case *ast.ImportStatement:
			// Support transitive imports: resolve relative to the imported file's dir.
			savedDir := e.SourceDir
			e.SourceDir = filepath.Dir(importPath)
			if err := e.execImport(nil, def); err != nil { //nolint:staticcheck
				e.SourceDir = savedDir
				return err
			}
			e.SourceDir = savedDir
		}
	}
	return nil
}

// =============================================================================
// PROMPT Execution
// =============================================================================

// ExecutePrompt executes a PROMPT statement and returns the result.
func (e *Executor) ExecutePrompt(ctx context.Context, stmt *ast.PromptStatement, params map[string]string) (*SPLResult, error) {
	if params == nil {
		params = make(map[string]string)
	}
	start := time.Now()

	// Gather system prompt and context
	systemPrompt := ""
	contextParts := make(map[string]string)

	for _, item := range stmt.SelectItems {
		alias := item.Alias
		var value string

		switch expr := item.Expression.(type) {
		case *ast.SystemRoleCall:
			systemPrompt = expr.Description
			continue
		case *ast.ContextRef:
			// context.fieldName → params["fieldName"] or params["context.fieldName"]
			if v, ok := params[expr.FieldName]; ok {
				value = v
			} else if v, ok := params["context."+expr.FieldName]; ok {
				value = v
			} else {
				value = fmt.Sprintf("[Context: %s - not provided]", expr.FieldName)
			}
			if alias == "" {
				alias = expr.FieldName
			}
		case *ast.MemoryGet:
			dummyState := newWorkflowState(params)
			if v, ok := dummyState.Memory[expr.Key]; ok {
				value = v
			}
			if alias == "" {
				alias = expr.Key
			}
		case *ast.RagQuery:
			warnNotImplemented("RagQuery in PROMPT SELECT (rag.query(...) — requires ChromaDB + Ollama embeddings)")
			value = fmt.Sprintf("[RAG: %v]", expr.QueryText)
			if alias == "" {
				alias = "rag_results"
			}
		default:
			// Evaluate as expression with empty state
			dummyState := newWorkflowState(params)
			value = e.evalExpression(item.Expression, dummyState)
			if alias == "" {
				alias = "context"
			}
		}

		if alias != "" {
			contextParts[alias] = value
		}
	}

	// Assemble prompt
	prompt := e.assemblePrompt(contextParts, stmt.GenerateClause, params)

	// Call adapter
	model := stmt.Model
	maxTokens := 1000
	temperature := 0.7
	if stmt.Budget > 0 {
		maxTokens = stmt.Budget
	}
	if stmt.GenerateClause != nil {
		if stmt.GenerateClause.OutputBudget > 0 {
			maxTokens = stmt.GenerateClause.OutputBudget
		}
		if stmt.GenerateClause.Temperature != 0 {
			temperature = stmt.GenerateClause.Temperature
		}
		if stmt.GenerateClause.Model != "" {
			model = stmt.GenerateClause.Model
		}
	}

	genResult, err := e.Adapter.Generate(ctx, prompt, model, maxTokens, temperature, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("prompt %q: %w", stmt.Name, err)
	}

	latencyMs := float64(time.Since(start).Milliseconds())
	if genResult.LatencyMs > 0 {
		latencyMs = genResult.LatencyMs
	}

	return &SPLResult{
		Content:      genResult.Content,
		Prompt:       prompt,
		Model:        genResult.Model,
		InputTokens:  genResult.InputTokens,
		OutputTokens: genResult.OutputTokens,
		TotalTokens:  genResult.TotalTokens,
		LatencyMs:    latencyMs,
		CostUSD:      genResult.CostUSD,
	}, nil
}

// assemblePrompt builds the prompt string from context parts and generate clause.
func (e *Executor) assemblePrompt(contextParts map[string]string, gen *ast.GenerateClause, params map[string]string) string {
	var parts []string

	// Add non-placeholder context parts
	for alias, text := range contextParts {
		if text != "" && !strings.HasPrefix(text, "[") {
			parts = append(parts, fmt.Sprintf("## %s\n%s", alias, text))
		}
	}

	if gen != nil {
		funcDef, hasFuncDef := e.Functions[gen.FunctionName]
		if hasFuncDef {
			// Use function body as template
			dummyState := newWorkflowState(params)
			argValues := make(map[string]string)
			for i, param := range funcDef.Parameters {
				if i < len(gen.Arguments) {
					argValues[param.Name] = e.evalExpression(gen.Arguments[i], dummyState)
				}
			}
			taskText := funcDef.Body
			for k, v := range argValues {
				taskText = strings.ReplaceAll(taskText, "{"+k+"}", v)
			}
			parts = append(parts, "\n## Task\n"+taskText)
		} else {
			dummyState := newWorkflowState(params)
			argStrs := make([]string, 0, len(gen.Arguments))
			for _, arg := range gen.Arguments {
				argStrs = append(argStrs, e.evalExpression(arg, dummyState))
			}
			
			taskPrefix := "Based on the above context, generate: "
			if len(parts) == 0 {
				taskPrefix = "Generate: "
			}
			
			parts = append(parts, fmt.Sprintf("\n## Task\n%s%s(%s)",
				taskPrefix, gen.FunctionName, strings.Join(argStrs, ", ")))
		}
	}

	return strings.Join(parts, "\n\n")
}

// =============================================================================
// WORKFLOW Execution
// =============================================================================

// ExecuteWorkflow executes a WORKFLOW statement.
func (e *Executor) ExecuteWorkflow(ctx context.Context, stmt *ast.WorkflowStatement, params map[string]string) (*WorkflowResult, error) {
	if params == nil {
		params = make(map[string]string)
	}
	state := newWorkflowState(nil)
	state.MaxWorkers = e.MaxWorkers

	// Initialize input variables from params and defaults
	for _, inp := range stmt.Inputs {
		if inp.ParamType == "STORAGE" {
			warnNotImplemented(fmt.Sprintf("STORAGE parameter '@%s' in WORKFLOW INPUT — persistent storage params not supported", inp.Name))
			continue
		}
		if v, ok := params[inp.Name]; ok {
			state.setVar(inp.Name, v)
		} else if inp.DefaultValue != nil {
			state.setVar(inp.Name, e.evalExpression(inp.DefaultValue, state))
		}
	}

	// Also seed from params directly (positional/untyped params)
	for k, v := range params {
		if _, exists := state.Variables[k]; !exists {
			state.setVar(k, v)
		}
	}

	err := e.executeBody(ctx, stmt.Body, state)
	if err != nil {
		if splErr, ok := err.(*SPLError); ok {
			handled, handlerErr := e.handleException(ctx, splErr, stmt.ExceptionHandlers, state)
			if handlerErr != nil {
				return nil, handlerErr
			}
			if !handled {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	status := "complete"
	if !state.Committed {
		status = "no_commit"
	}

	return &WorkflowResult{
		Output:          state.Variables,
		Status:          status,
		TotalLLMCalls:   state.TotalLLMCalls,
		TotalInputToks:  state.TotalInputToks,
		TotalOutputToks: state.TotalOutputToks,
		TotalLatencyMs:  state.TotalLatencyMs,
		TotalCostUSD:    state.TotalCostUSD,
		CommittedValue:  state.CommittedValue,
		CommittedOpts:   state.CommittedOpts,
	}, nil
}

func (e *Executor) executeBody(ctx context.Context, stmts []ast.Stmt, state *WorkflowState) error {
	if state.MaxWorkers <= 0 {
		// Sequential execution
		for _, stmt := range stmts {
			if state.Committed {
				return nil
			}
			if err := e.executeStatement(ctx, stmt, state); err != nil {
				return err
			}
		}
		return nil
	}

	// Dependency-aware parallel execution
	i := 0
	for i < len(stmts) {
		if state.Committed {
			return nil
		}
		stmt := stmts[i]

		// If the current statement isn't parallelizable, run it sequentially
		if !isParallelizable(stmt) {
			if err := e.executeStatement(ctx, stmt, state); err != nil {
				return err
			}
			i++
			continue
		}

		// Look ahead to find a batch of independent parallelizable statements
		batch := []ast.Stmt{stmt}
		assignedInBatch := assignedVariables(stmt)
		
		j := i + 1
		for j < len(stmts) {
			nextStmt := stmts[j]
			
			// Stop if next statement is not parallelizable
			if !isParallelizable(nextStmt) {
				break
			}
			
			// Stop if next statement references any variable assigned in the current batch
			if referencesAny(nextStmt, assignedInBatch) {
				break
			}
			
			// Add to batch and update tracked assignments
			batch = append(batch, nextStmt)
			for v := range assignedVariables(nextStmt) {
				assignedInBatch[v] = true
			}
			j++
		}

		if len(batch) == 1 {
			if err := e.executeStatement(ctx, batch[0], state); err != nil {
				return err
			}
			i++
		} else {
			// Run batch in parallel
			tasks := make([]func(context.Context) error, len(batch))
			for k, s := range batch {
				s := s // capture
				tasks[k] = func(ctx context.Context) error {
					return e.executeStatement(ctx, s, state)
				}
			}
			if err := runParallel(ctx, tasks); err != nil {
				return err
			}
			i = j
		}
	}
	return nil
}

// isParallelizable returns true if the statement can safely run concurrently
// with other parallelizable statements (AssignmentStatement, GenerateIntoStatement).
func isParallelizable(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.AssignmentStatement, *ast.GenerateIntoStatement:
		return true
	}
	return false
}

// assignedVariables returns the set of variable names written by a statement.
func assignedVariables(stmt ast.Stmt) map[string]bool {
	result := make(map[string]bool)
	switch s := stmt.(type) {
	case *ast.AssignmentStatement:
		result[s.Variable] = true
	case *ast.GenerateIntoStatement:
		if s.TargetVariable != "" {
			result[s.TargetVariable] = true
		}
	}
	return result
}

// referencesAny returns true if stmt's expression references any variable in vars.
func referencesAny(stmt ast.Stmt, vars map[string]bool) bool {
	switch s := stmt.(type) {
	case *ast.AssignmentStatement:
		return exprReferencesAny(s.Expression, vars)
	case *ast.GenerateIntoStatement:
		for _, arg := range s.GenerateClause.Arguments {
			if exprReferencesAny(arg, vars) {
				return true
			}
		}
	}
	return false
}

// exprReferencesAny checks if an expression references any variable in vars.
func exprReferencesAny(expr ast.Expr, vars map[string]bool) bool {
	if expr == nil {
		return false
	}
	switch ex := expr.(type) {
	case *ast.ParamRef:
		return vars[ex.Name]
	case *ast.Identifier:
		return vars[ex.Name]
	case *ast.FStringLiteral:
		// Check {@ variable} references
		for v := range vars {
			if strings.Contains(ex.Template, "{@"+v+"}") {
				return true
			}
		}
	case *ast.BinaryOp:
		return exprReferencesAny(ex.Left, vars) || exprReferencesAny(ex.Right, vars)
	case *ast.FunctionCall:
		for _, arg := range ex.Arguments {
			if exprReferencesAny(arg, vars) {
				return true
			}
		}
	case *ast.DottedName:
		if len(ex.Parts) > 0 {
			return vars[ex.Parts[0]] || vars[ex.FullName()]
		}
	}
	return false
}

func (e *Executor) executeStatement(ctx context.Context, stmt ast.Stmt, state *WorkflowState) error {
	switch s := stmt.(type) {
	case *ast.AssignmentStatement:
		return e.execAssignment(ctx, s, state)
	case *ast.GenerateIntoStatement:
		return e.execGenerateInto(ctx, s, state)
	case *ast.EvaluateStatement:
		return e.execEvaluate(ctx, s, state)
	case *ast.WhileStatement:
		return e.execWhile(ctx, s, state)
	case *ast.CommitStatement:
		return e.execCommit(ctx, s, state)
	case *ast.RetryStatement:
		return nil // handled at exception level
	case *ast.RaiseStatement:
		return e.execRaise(s)
	case *ast.LoggingStatement:
		e.execLogging(s, state)
		return nil
	case *ast.StoreStatement:
		val := state.getVar(s.Variable)
		state.Memory[s.Key] = val
		if state.MemStore != nil {
			if err := state.MemStore.Set(s.Key, val); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: STORE @%s IN memory.%s: persist failed: %v\n", s.Variable, s.Key, err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "INFO: STORE @%s IN memory.%s = %q (in-process only; SQLite store unavailable)\n", s.Variable, s.Key, val)
		}
		return nil
	case *ast.StorageAssignStatement:
		key := e.evalExpression(s.Key, state)
		value := e.evalExpression(s.Value, state)
		varVal := state.getVar(s.StorageVar)
		// Try updating an existing JSON dict (MAP variable)
		var obj map[string]interface{}
		if json.Unmarshal([]byte(varVal), &obj) == nil {
			obj[key] = value
			b, _ := json.Marshal(obj)
			state.setVar(s.StorageVar, string(b))
			return nil
		}
		// Variable is not a MAP yet — initialize as a new MAP
		newObj := map[string]interface{}{key: value}
		b, _ := json.Marshal(newObj)
		state.setVar(s.StorageVar, string(b))
		return nil
	case *ast.CallStatement:
		return e.execCall(ctx, s, state)
	case *ast.CallParallelStatement:
		return e.execCallParallel(ctx, s, state)
	case *ast.DoBlock:
		return e.execDoBlock(ctx, s, state)
	case *ast.SelectIntoStatement:
		return e.execSelectInto(ctx, s, state)
	default:
		warnNotImplemented(fmt.Sprintf("unknown statement type %T", stmt))
		return nil
	}
}

// =============================================================================
// Statement Executors
// =============================================================================

func (e *Executor) execAssignment(_ context.Context, stmt *ast.AssignmentStatement, state *WorkflowState) error {
	value := e.evalExpression(stmt.Expression, state)
	state.setVar(stmt.Variable, value)
	return nil
}

func (e *Executor) execGenerateInto(ctx context.Context, stmt *ast.GenerateIntoStatement, state *WorkflowState) error {
	gen := &stmt.GenerateClause

	// Build prompt from function and arguments
	var argsText []string
	for _, arg := range gen.Arguments {
		argsText = append(argsText, e.evalExpression(arg, state))
	}

	prompt := fmt.Sprintf("Task: %s\n\n", gen.FunctionName)
	for i, argText := range argsText {
		prompt += fmt.Sprintf("Input %d:\n%s\n\n", i+1, argText)
	}

	// Check for user-defined function template
	if funcDef, ok := e.Functions[gen.FunctionName]; ok {
		prompt = funcDef.Body
		for i, param := range funcDef.Parameters {
			if i < len(argsText) {
				prompt = strings.ReplaceAll(prompt, "{"+param.Name+"}", argsText[i])
			}
		}
	}

	model := gen.Model
	if strings.HasPrefix(model, "@") {
		model = state.getVar(model[1:])
	}

	maxTokens := gen.OutputBudget
	if maxTokens == 0 {
		maxTokens = 1000
	}
	temperature := gen.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	if err := e.checkBudget(state); err != nil {
		return err
	}

	genResult, err := e.Adapter.Generate(ctx, prompt, model, maxTokens, temperature, "")
	if err != nil {
		return fmt.Errorf("GENERATE %s: %w", gen.FunctionName, err)
	}
	state.recordLLMCall(genResult)

	if stmt.TargetVariable != "" {
		state.setVar(stmt.TargetVariable, genResult.Content)
	}
	return nil
}

func (e *Executor) execEvaluate(ctx context.Context, stmt *ast.EvaluateStatement, state *WorkflowState) error {
	evalValue := e.evalExpression(stmt.Expression, state)

	for _, whenClause := range stmt.WhenClauses {
		matched, err := e.evalCondition(ctx, whenClause.Condition, evalValue, state)
		if err != nil {
			return err
		}
		if matched {
			return e.executeBody(ctx, whenClause.Statements, state)
		}
	}

	// Else clause
	if len(stmt.ElseStatements) > 0 {
		return e.executeBody(ctx, stmt.ElseStatements, state)
	}
	return nil
}

func (e *Executor) execWhile(ctx context.Context, stmt *ast.WhileStatement, state *WorkflowState) error {
	maxIter := stmt.MaxIterations
	if maxIter == 0 {
		maxIter = defaultMaxIterations
	}

	for iteration := 0; iteration < maxIter; iteration++ {
		if state.Committed {
			return nil
		}

		shouldContinue, err := e.evalWhileCondition(ctx, stmt.Condition, state)
		if err != nil {
			return err
		}
		if !shouldContinue {
			break
		}

		if err = e.executeBody(ctx, stmt.Body, state); err != nil {
			return err
		}

		if iteration == maxIter-1 {
			return &SPLError{Type: ErrMaxIterations, Message: fmt.Sprintf("WHILE loop exceeded %d iterations", maxIter)}
		}
	}
	return nil
}

func (e *Executor) evalWhileCondition(ctx context.Context, cond interface{}, state *WorkflowState) (bool, error) {
	switch c := cond.(type) {
	case *ast.CompoundCondition:
		leftVal, err := e.evalWhileCondition(ctx, c.Left, state)
		if err != nil {
			return false, err
		}
		rightVal, err := e.evalWhileCondition(ctx, c.Right, state)
		if err != nil {
			return false, err
		}
		if c.Operator == "AND" {
			return leftVal && rightVal, nil
		}
		return leftVal || rightVal, nil

	case *ast.UnaryOp:
		if c.Operator == "NOT" {
			val, err := e.evalWhileCondition(ctx, c.Operand, state)
			if err != nil {
				return false, err
			}
			return !val, nil
		}
		return false, nil

	case *ast.Condition:
		leftStr := e.evalExpression(c.Left, state)
		rightStr := e.evalExpression(c.Right, state)
		leftF, leftErr := strconv.ParseFloat(leftStr, 64)
		rightF, rightErr := strconv.ParseFloat(rightStr, 64)
		if leftErr == nil && rightErr == nil {
			return compareFloat(leftF, c.Operator, rightF), nil
		}
		return compareStrings(leftStr, c.Operator, rightStr), nil

	case *ast.SemanticCondition:
		// Build context string
		var contextLines []string
		for varName, varVal := range state.Variables {
			preview := varVal
			if len(preview) > 500 {
				preview = preview[:500]
			}
			contextLines = append(contextLines, fmt.Sprintf("  @%s = %s", varName, preview))
		}
		contextStr := strings.Join(contextLines, "\n")
		if contextStr == "" {
			contextStr = "(no variables)"
		}
		judgePrompt := fmt.Sprintf(
			"Given the current state:\n%s\n\nIs the condition '%s' still true?\nAnswer with only 'yes' or 'no'.",
			contextStr, c.SemanticValue,
		)
		if err := e.checkBudget(state); err != nil {
			return false, err
		}
		judgeResult, err := e.Adapter.Generate(ctx, judgePrompt, "", 10, 0.0, "")
		if err != nil {
			return false, err
		}
		state.recordLLMCall(judgeResult)
		return strings.Contains(strings.ToLower(judgeResult.Content), "yes"), nil

	case ast.Expr:
		val := e.evalExpression(c, state)
		return val != "" && val != "0" && strings.ToLower(val) != "false", nil

	default:
		return false, nil
	}
}

func (e *Executor) execCommit(_ context.Context, stmt *ast.CommitStatement, state *WorkflowState) error {
	value := e.evalExpression(stmt.Expression, state)
	opts := make(map[string]string)
	for k, v := range stmt.Options {
		opts[k] = e.evalExpression(v, state)
	}

	state.Committed = true
	state.CommittedValue = value
	state.CommittedOpts = opts
	return nil
}

func (e *Executor) execRaise(stmt *ast.RaiseStatement) error {
	return &SPLError{Type: stmt.ExceptionType, Message: stmt.Message}
}

var logLevels = map[string]int{"DEBUG": 0, "INFO": 1, "WARN": 2, "ERROR": 3}

func (e *Executor) execLogging(stmt *ast.LoggingStatement, state *WorkflowState) {
	level := strings.ToUpper(stmt.Level)
	if level == "" {
		level = "INFO"
	}
	message := e.evalExpression(stmt.Expression, state)

	if stmt.Destination == "" {
		fmt.Fprintf(os.Stdout, "[%s] %s\n", level, message)
	} else {
		f, err := os.OpenFile(stmt.Destination, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			ts := time.Now().Format(time.RFC3339)
			fmt.Fprintf(f, "[%s] [%s] %s\n", ts, level, message)
			f.Close()
		}
	}
}

func (e *Executor) execCall(ctx context.Context, stmt *ast.CallStatement, state *WorkflowState) error {
	// 0. Check functions.Registry built-ins
	{
		var argsText []string
		for _, arg := range stmt.Arguments {
			argsText = append(argsText, e.evalExpression(arg, state))
		}
		if result, ok := e.Registry.CallBuiltin(stmt.ProcedureName, argsText); ok {
			if stmt.TargetVariable != "" {
				state.setVar(stmt.TargetVariable, result)
			}
			return nil
		}
	}

	// 1. Check stdlib
	if stdlib.Has(stmt.ProcedureName) {
		var argsText []string
		for _, arg := range stmt.Arguments {
			argsText = append(argsText, e.evalExpression(arg, state))
		}
		result, _ := stdlib.Call(stmt.ProcedureName, argsText)
		if stmt.TargetVariable != "" {
			state.setVar(stmt.TargetVariable, result)
		}
		return nil
	}

	// 2. Check registered tools
	if fn, ok := e.Tools[stmt.ProcedureName]; ok {
		var argsText []string
		for _, arg := range stmt.Arguments {
			argsText = append(argsText, e.evalExpression(arg, state))
		}
		result := fn(argsText)
		if stmt.TargetVariable != "" {
			state.setVar(stmt.TargetVariable, result)
		}
		return nil
	}

	// 3. Check user-defined procedures
	if proc, ok := e.Procedures[stmt.ProcedureName]; ok {
		procState := newWorkflowState(nil)
		// Bind args to params
		var namedArgs = make(map[string]ast.Expr)
		var positionalArgs []ast.Expr
		for _, arg := range stmt.Arguments {
			if na, ok := arg.(*ast.NamedArg); ok {
				namedArgs[na.Name] = na.Value
			} else {
				positionalArgs = append(positionalArgs, arg)
			}
		}
		posIdx := 0
		for _, param := range proc.Parameters {
			if namedVal, ok := namedArgs[param.Name]; ok {
				procState.setVar(param.Name, e.evalExpression(namedVal, state))
			} else if posIdx < len(positionalArgs) {
				procState.setVar(param.Name, e.evalExpression(positionalArgs[posIdx], state))
				posIdx++
			} else if param.DefaultValue != nil {
				procState.setVar(param.Name, e.evalExpression(param.DefaultValue, state))
			}
		}

		if err := e.executeBody(ctx, proc.Body, procState); err != nil {
			if splErr, ok := err.(*SPLError); ok {
				handled, handlerErr := e.handleException(ctx, splErr, proc.ExceptionHandlers, procState)
				if handlerErr != nil {
					return handlerErr
				}
				if !handled {
					return err
				}
			} else {
				return err
			}
		}

		// Propagate metrics
		state.TotalLLMCalls += procState.TotalLLMCalls
		state.TotalInputToks += procState.TotalInputToks
		state.TotalOutputToks += procState.TotalOutputToks
		state.TotalLatencyMs += procState.TotalLatencyMs
		state.TotalCostUSD += procState.TotalCostUSD

		if stmt.TargetVariable != "" && procState.CommittedValue != "" {
			state.setVar(stmt.TargetVariable, procState.CommittedValue)
		}
		return nil
	}

	// 4. Check registered workflows (SPL 3.0: CALL can dispatch workflows)
	if wf, ok := e.Workflows[stmt.ProcedureName]; ok {
		wfParams := make(map[string]string)
		var namedArgs = make(map[string]ast.Expr)
		var positionalArgs []ast.Expr
		for _, arg := range stmt.Arguments {
			if na, ok := arg.(*ast.NamedArg); ok {
				namedArgs[na.Name] = na.Value
			} else {
				positionalArgs = append(positionalArgs, arg)
			}
		}
		posIdx := 0
		for _, param := range wf.Inputs {
			if namedVal, ok := namedArgs[param.Name]; ok {
				wfParams[param.Name] = e.evalExpression(namedVal, state)
			} else if posIdx < len(positionalArgs) {
				wfParams[param.Name] = e.evalExpression(positionalArgs[posIdx], state)
				posIdx++
			} else if param.DefaultValue != nil {
				wfParams[param.Name] = e.evalExpression(param.DefaultValue, state)
			}
		}
		res, err := e.ExecuteWorkflow(ctx, wf, wfParams)
		if err != nil {
			return err
		}
		state.TotalLLMCalls += res.TotalLLMCalls
		state.TotalInputToks += res.TotalInputToks
		state.TotalOutputToks += res.TotalOutputToks
		state.TotalLatencyMs += res.TotalLatencyMs
		state.TotalCostUSD += res.TotalCostUSD
		if stmt.TargetVariable != "" && res.CommittedValue != "" {
			state.setVar(stmt.TargetVariable, res.CommittedValue)
		}
		return nil
	}

	// 5. LLM fallback
	var argsText []string
	for _, arg := range stmt.Arguments {
		argsText = append(argsText, e.evalExpression(arg, state))
	}
	prompt := fmt.Sprintf("Execute procedure: %s(%s)", stmt.ProcedureName, strings.Join(argsText, ", "))
	if err := e.checkBudget(state); err != nil {
		return err
	}
	result, err := e.Adapter.Generate(ctx, prompt, "", 1000, 0.7, "")
	if err != nil {
		return err
	}
	state.recordLLMCall(result)
	if stmt.TargetVariable != "" {
		state.setVar(stmt.TargetVariable, result.Content)
	}
	return nil
}

func (e *Executor) execDoBlock(ctx context.Context, stmt *ast.DoBlock, state *WorkflowState) error {
	if err := e.executeBody(ctx, stmt.Statements, state); err != nil {
		if splErr, ok := err.(*SPLError); ok {
			handled, handlerErr := e.handleException(ctx, splErr, stmt.ExceptionHandlers, state)
			if handlerErr != nil {
				return handlerErr
			}
			if !handled {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// execCallParallel executes a CALL PARALLEL block (SPL 3.0).
//
// Each branch receives a read-only snapshot of the parent variable scope.
// Results are written back to the parent only via each branch's INTO @var.
// All branches run concurrently; the first failure cancels remaining branches.
func (e *Executor) execCallParallel(ctx context.Context, stmt *ast.CallParallelStatement, state *WorkflowState) error {
	// Snapshot the parent variable scope so branches read a consistent view.
	state.mu.RLock()
	snapshot := make(map[string]string, len(state.Variables))
	for k, v := range state.Variables {
		snapshot[k] = v
	}
	state.mu.RUnlock()

	type branchResult struct {
		targetVar string
		value     string
		calls     int
		inToks    int
		outToks   int
		latencyMs float64
		costUSD   float64
	}

	results := make([]branchResult, len(stmt.Branches))

	tasks := make([]func(context.Context) error, len(stmt.Branches))
	for i, branch := range stmt.Branches {
		i, branch := i, branch // capture for goroutine
		tasks[i] = func(ctx context.Context) error {
			// Each branch gets its own WorkflowState seeded from the parent snapshot.
			branchState := newWorkflowState(snapshot)

			callStmt := &ast.CallStatement{
				ProcedureName:  branch.ProcedureName,
				Arguments:      branch.Arguments,
				TargetVariable: branch.TargetVariable,
			}
			if err := e.execCall(ctx, callStmt, branchState); err != nil {
				return fmt.Errorf("CALL PARALLEL branch %q: %w", branch.ProcedureName, err)
			}

			val := ""
			if branch.TargetVariable != "" {
				val = branchState.getVar(branch.TargetVariable)
			}
			results[i] = branchResult{
				targetVar: branch.TargetVariable,
				value:     val,
				calls:     branchState.TotalLLMCalls,
				inToks:    branchState.TotalInputToks,
				outToks:   branchState.TotalOutputToks,
				latencyMs: branchState.TotalLatencyMs,
				costUSD:   branchState.TotalCostUSD,
			}
			return nil
		}
	}

	if err := runParallel(ctx, tasks); err != nil {
		return err
	}

	// Merge results back into parent state.
	for _, r := range results {
		if r.targetVar != "" {
			state.setVar(r.targetVar, r.value)
		}
		state.TotalLLMCalls += r.calls
		state.TotalInputToks += r.inToks
		state.TotalOutputToks += r.outToks
		state.TotalLatencyMs += r.latencyMs
		state.TotalCostUSD += r.costUSD
	}
	return nil
}

func (e *Executor) execSelectInto(ctx context.Context, stmt *ast.SelectIntoStatement, state *WorkflowState) error {
	// Run each CTE nested prompt
	cteResults := make(map[string]string)
	for _, cte := range stmt.CTEs {
		if cte.NestedPrompt != nil {
			result, err := e.execGenerateIntoPrompt(ctx, cte.NestedPrompt, state)
			if err != nil {
				return err
			}
			gen := cte.NestedPrompt.GenerateClause
			field := "result"
			if gen != nil {
				field = gen.FunctionName
			}
			cteResults[cte.Name+"."+field] = result
			cteResults[cte.Name] = result
		}
	}

	// Evaluate SELECT items
	var selected []string
	for _, item := range stmt.SelectItems {
		val := ""
		if item.Expression != nil {
			switch expr := item.Expression.(type) {
			case *ast.MemoryGet:
				if v, ok := state.Memory[expr.Key]; ok {
					val = v
				}
			case *ast.DottedName:
				fullName := expr.FullName()
				if v, ok := cteResults[fullName]; ok {
					val = v
				} else if v, ok := cteResults[strings.Split(fullName, ".")[0]]; ok {
					val = v
				}
			default:
				if len(cteResults) > 0 {
					exprStr := e.evalExpression(item.Expression, state)
					if v, ok := cteResults[exprStr]; ok {
						val = v
					}
				} else {
					val = e.evalExpression(item.Expression, state)
				}
			}
		}
		if val == "" && item.Alias != "" && len(cteResults) > 0 {
			val = cteResults[item.Alias]
		}
		selected = append(selected, val)
	}

	// Assign to target variables
	targets := stmt.TargetVariables
	if len(targets) == 0 && stmt.TargetVariable != "" {
		targets = []string{stmt.TargetVariable}
	}
	for i, varName := range targets {
		if i < len(selected) {
			state.setVar(varName, selected[i])
		}
	}
	return nil
}

func (e *Executor) execGenerateIntoPrompt(ctx context.Context, promptStmt *ast.PromptStatement, state *WorkflowState) (string, error) {
	gen := promptStmt.GenerateClause
	if gen == nil {
		return "", nil
	}

	var argsText []string
	for _, arg := range gen.Arguments {
		argsText = append(argsText, e.evalExpression(arg, state))
	}

	prompt := fmt.Sprintf("Task: %s\n\n", gen.FunctionName)
	for i, argText := range argsText {
		prompt += fmt.Sprintf("Input %d:\n%s\n\n", i+1, argText)
	}

	if funcDef, ok := e.Functions[gen.FunctionName]; ok {
		prompt = funcDef.Body
		for i, param := range funcDef.Parameters {
			if i < len(argsText) {
				prompt = strings.ReplaceAll(prompt, "{"+param.Name+"}", argsText[i])
			}
		}
	}

	model := gen.Model
	if strings.HasPrefix(model, "@") {
		model = state.getVar(model[1:])
	}
	maxTokens := gen.OutputBudget
	if maxTokens == 0 {
		maxTokens = 1000
	}
	temperature := gen.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	if err := e.checkBudget(state); err != nil {
		return "", err
	}
	genResult, err := e.Adapter.Generate(ctx, prompt, model, maxTokens, temperature, "")
	if err != nil {
		return "", err
	}
	state.recordLLMCall(genResult)
	return genResult.Content, nil
}

// =============================================================================
// Exception Handling
// =============================================================================

func (e *Executor) handleException(ctx context.Context, err *SPLError, handlers []ast.ExceptionHandler, state *WorkflowState) (bool, error) {
	for _, handler := range handlers {
		ht := handler.ExceptionType
		if ht == err.Type || strings.EqualFold(ht, "OTHERS") || strings.EqualFold(ht, "Others") {
			if execErr := e.executeBody(ctx, handler.Statements, state); execErr != nil {
				return true, execErr
			}
			return true, nil
		}
	}
	return false, nil
}

// =============================================================================
// Condition Evaluation
// =============================================================================

func (e *Executor) evalCondition(ctx context.Context, cond interface{}, evalValue string, state *WorkflowState) (bool, error) {
	switch c := cond.(type) {
	case *ast.SemanticCondition:
		sv := c.SemanticValue

		if strings.HasPrefix(sv, "startswith:") {
			prefix := sv[len("startswith:"):]
			return strings.HasPrefix(strings.ToLower(evalValue), strings.ToLower(prefix)), nil
		}

		if strings.HasPrefix(sv, "contains:") {
			needles := strings.Split(sv[len("contains:"):], "|")
			lv := strings.ToLower(evalValue)
			for _, needle := range needles {
				if strings.Contains(lv, strings.ToLower(needle)) {
					return true, nil
				}
			}
			return false, nil
		}

		// LLM judge
		judgePrompt := fmt.Sprintf(
			"Evaluate the following text and determine if it matches the condition '%s'.\n\nText:\n%s\n\nAnswer with only 'yes' or 'no'.",
			sv, evalValue,
		)
		if err := e.checkBudget(state); err != nil {
			return false, err
		}
		judgeResult, err := e.Adapter.Generate(ctx, judgePrompt, "", 10, 0.0, "")
		if err != nil {
			return false, err
		}
		state.recordLLMCall(judgeResult)
		return strings.Contains(strings.ToLower(judgeResult.Content), "yes"), nil

	case *ast.ComparisonCondition:
		rightStr := e.evalExpression(c.Right, state)
		// Boolean shorthand
		if (rightStr == "true" || rightStr == "false") && c.Operator == "=" {
			return strings.ToLower(evalValue) == rightStr, nil
		}
		// Numeric comparison
		leftF, leftErr := strconv.ParseFloat(evalValue, 64)
		rightF, rightErr := strconv.ParseFloat(rightStr, 64)
		if leftErr == nil && rightErr == nil {
			return compareFloat(leftF, c.Operator, rightF), nil
		}
		// String fallback
		return compareStrings(evalValue, c.Operator, rightStr), nil

	case *ast.Condition:
		leftStr := e.evalExpression(c.Left, state)
		rightStr := e.evalExpression(c.Right, state)
		leftF, leftErr := strconv.ParseFloat(leftStr, 64)
		rightF, rightErr := strconv.ParseFloat(rightStr, 64)
		if leftErr == nil && rightErr == nil {
			return compareFloat(leftF, c.Operator, rightF), nil
		}
		return compareStrings(leftStr, c.Operator, rightStr), nil

	default:
		return false, nil
	}
}

func compareFloat(left float64, op string, right float64) bool {
	switch op {
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	case "=", "==":
		return left == right
	case "!=":
		return left != right
	}
	return false
}

func compareStrings(left, op, right string) bool {
	switch op {
	case "=", "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	}
	return false
}

// =============================================================================
// Budget Check
// =============================================================================

func (e *Executor) checkBudget(state *WorkflowState) error {
	if state.TotalLLMCalls >= e.MaxLLMCalls {
		return &SPLError{
			Type:    ErrBudget,
			Message: fmt.Sprintf("LLM call limit reached (%d/%d)", state.TotalLLMCalls, e.MaxLLMCalls),
		}
	}
	total := state.TotalInputToks + state.TotalOutputToks
	if total >= e.MaxTotalTokens {
		return &SPLError{
			Type:    ErrBudget,
			Message: fmt.Sprintf("Token limit reached (%d/%d)", total, e.MaxTotalTokens),
		}
	}
	return nil
}

// =============================================================================
// Expression Evaluation
// =============================================================================

var fstringPattern = regexp.MustCompile(`\{@(\w+)\}`)

// evalExpression evaluates an AST expression to a string value.
func (e *Executor) evalExpression(expr ast.Expr, state *WorkflowState) string {
	if expr == nil {
		return ""
	}
	switch ex := expr.(type) {
	case *ast.Literal:
		if ex.LitType == "bool" {
			return ex.Value
		}
		return ex.Value

	case *ast.ParamRef:
		return state.getVar(ex.Name)

	case *ast.Identifier:
		return state.getVar(ex.Name)

	case *ast.FStringLiteral:
		return fstringPattern.ReplaceAllStringFunc(ex.Template, func(match string) string {
			varName := match[2 : len(match)-1] // strip {@ and }
			return state.getVar(varName)
		})

	case *ast.ListLiteral:
		elements := make([]string, 0, len(ex.Elements))
		for _, elem := range ex.Elements {
			elements = append(elements, e.evalExpression(elem, state))
		}
		b, _ := json.Marshal(elements)
		return string(b)

	case *ast.BinaryOp:
		left := e.evalExpression(ex.Left, state)
		right := e.evalExpression(ex.Right, state)
		switch ex.Op {
		case "||":
			return left + right
		case "+":
			lf, le := strconv.ParseFloat(left, 64)
			rf, re := strconv.ParseFloat(right, 64)
			if le == nil && re == nil {
				result := lf + rf
				if result == float64(int64(result)) {
					return strconv.FormatInt(int64(result), 10)
				}
				return strconv.FormatFloat(result, 'f', -1, 64)
			}
			return left + right
		case "-":
			lf, le := strconv.ParseFloat(left, 64)
			rf, re := strconv.ParseFloat(right, 64)
			if le == nil && re == nil {
				result := lf - rf
				if result == float64(int64(result)) {
					return strconv.FormatInt(int64(result), 10)
				}
				return strconv.FormatFloat(result, 'f', -1, 64)
			}
			return left
		}
		return left

	case *ast.FunctionCall:
		var argsText []string
		for _, arg := range ex.Arguments {
			argsText = append(argsText, e.evalExpression(arg, state))
		}
		// Check stdlib first
		if result, ok := stdlib.Call(ex.Name, argsText); ok {
			return result
		}
		// Check registered tools
		if fn, ok := e.Tools[ex.Name]; ok {
			return fn(argsText)
		}
		return fmt.Sprintf("[%s(...)]", ex.Name)

	case *ast.MemoryGet:
		if state != nil {
			if v, ok := state.Memory[ex.Key]; ok {
				return v
			}
		}
		return ""

	case *ast.MapLiteral:
		obj := make(map[string]string, len(ex.Pairs))
		for _, pair := range ex.Pairs {
			k := e.evalExpression(pair.Key, state)
			v := e.evalExpression(pair.Value, state)
			obj[k] = v
		}
		b, _ := json.Marshal(obj)
		return string(b)

	case *ast.StorageSubscript:
		key := e.evalExpression(ex.Key, state)
		varVal := state.getVar(ex.StorageVar)
		// Try JSON dict (MAP variable)
		var obj map[string]interface{}
		if json.Unmarshal([]byte(varVal), &obj) == nil {
			if v, ok := obj[key]; ok {
				switch vt := v.(type) {
				case string:
					return vt
				default:
					b, _ := json.Marshal(vt)
					return string(b)
				}
			}
			return ""
		}
		// Try JSON list (LIST variable — integer index)
		var arr []interface{}
		if json.Unmarshal([]byte(varVal), &arr) == nil {
			idx, err := strconv.Atoi(key)
			if err == nil && idx >= 0 && idx < len(arr) {
				switch vt := arr[idx].(type) {
				case string:
					return vt
				default:
					b, _ := json.Marshal(vt)
					return string(b)
				}
			}
			return ""
		}
		return ""

	case *ast.DottedName:
		return state.getVar(ex.FullName())

	case *ast.NamedArg:
		return e.evalExpression(ex.Value, state)

	case *ast.ContextRef:
		return state.getVar(ex.FieldName)

	case *ast.RagQuery:
		warnNotImplemented("RagQuery (rag.query(...) — requires ChromaDB + Ollama embeddings)")
		return fmt.Sprintf("[RAG: %s]", e.evalExpression(ex.QueryText, state))

	case *ast.SystemRoleCall:
		return ex.Description

	case *ast.StorageSpec:
		warnNotImplemented(fmt.Sprintf("StorageSpec (STORAGE(%s, %s) — persistent backend)", ex.Backend, ex.Path))
		return fmt.Sprintf("STORAGE(%s, %s)", ex.Backend, ex.Path)
	}

	return fmt.Sprintf("%v", expr)
}
