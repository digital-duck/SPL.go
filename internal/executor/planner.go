// Package executor implements the SPL 2.0 runtime execution engine and planner.
package executor

import (
	"fmt"
	"strings"

	"github.com/digital-duck/spl20go/internal/ast"
	"github.com/digital-duck/spl20go/internal/tokencount"
)

// =============================================================================
// Workflow Planning Types  (mirrors Python optimizer.WorkflowStep / WorkflowPlan)
// =============================================================================

// WorkflowStep is a single step in a workflow execution plan.
type WorkflowStep struct {
	StepType           string          // "assign", "generate", "evaluate", "while", "commit", "retry", "raise", "call", "select", "do_block", "exception_handler"
	Description        string
	EstimatedLLMCalls  int
	EstimatedTokens    int
	Substeps           []WorkflowStep
	Branches           []WorkflowBranch
	ExceptionHandlers  []WorkflowStep
}

// WorkflowBranch is a branch inside an EVALUATE step.
type WorkflowBranch struct {
	Condition     string // human-readable condition string
	ConditionType string // "semantic" or "deterministic"
	Steps         []WorkflowStep
}

// WorkflowPlan is the execution plan for a WORKFLOW or PROCEDURE statement.
type WorkflowPlan struct {
	WorkflowName              string
	Inputs                    []string
	Outputs                   []string
	Security                  map[string]string
	Accounting                map[string]string
	Steps                     []WorkflowStep
	ExceptionHandlers         []WorkflowStep
	TotalEstimatedLLMCalls    int
	TotalEstimatedTokens      int
}

// PlanResult holds pre-execution estimates for an SPL program.
type PlanResult struct {
	EstimatedLLMCalls    int
	EstimatedInputTokens int
	EstimatedCost        float64
	Warnings             []string
}

// Planner performs static analysis to estimate resource usage.
type Planner struct {
	DefaultModel string
}

// NewPlanner creates a new Planner with an optional default model.
func NewPlanner(model string) *Planner {
	return &Planner{DefaultModel: model}
}

// Plan walks the AST and returns estimated resource usage.
func (p *Planner) Plan(program *ast.Program) *PlanResult {
	res := &PlanResult{}
	for _, stmt := range program.Statements {
		p.planStatement(stmt, res)
	}
	return res
}

func (p *Planner) planStatement(stmt ast.Stmt, res *PlanResult) {
	switch s := stmt.(type) {
	case *ast.PromptStatement:
		res.EstimatedLLMCalls++
		model := s.Model
		if model == "" {
			model = p.DefaultModel
		}
		counter := tokencount.New(model)

		// Estimate input tokens from select items
		for _, item := range s.SelectItems {
			res.EstimatedInputTokens += p.estimateExprTokens(item.Expression, counter)
		}

		res.EstimatedCost += counter.EstimateCost(res.EstimatedInputTokens, 0)

	case *ast.WorkflowStatement:
		for _, bodyStmt := range s.Body {
			p.planStatement(bodyStmt, res)
		}

	case *ast.GenerateIntoStatement:
		res.EstimatedLLMCalls++
		model := s.GenerateClause.Model
		if model == "" {
			model = p.DefaultModel
		}
		counter := tokencount.New(model)

		for _, arg := range s.GenerateClause.Arguments {
			res.EstimatedInputTokens += p.estimateExprTokens(arg, counter)
		}
		res.EstimatedCost += counter.EstimateCost(res.EstimatedInputTokens, 0)

	case *ast.WhileStatement:
		// Pessimistic estimation: assume 5 iterations if not specified
		iters := s.MaxIterations
		if iters == 0 {
			iters = 5
			res.Warnings = append(res.Warnings, fmt.Sprintf("WHILE loop in %T has no MAX ITERATIONS; assuming 5 for planning", stmt))
		}
		
		subRes := &PlanResult{}
		for _, bodyStmt := range s.Body {
			p.planStatement(bodyStmt, subRes)
		}
		
		res.EstimatedLLMCalls += subRes.EstimatedLLMCalls * iters
		res.EstimatedInputTokens += subRes.EstimatedInputTokens * iters
		res.EstimatedCost += subRes.EstimatedCost * float64(iters)

	case *ast.EvaluateStatement:
		// Assume worst-case branch (most LLM calls)
		maxCalls := 0
		var worstBranchRes *PlanResult
		
		for _, when := range s.WhenClauses {
			branchRes := &PlanResult{}
			for _, bs := range when.Statements {
				p.planStatement(bs, branchRes)
			}
			if branchRes.EstimatedLLMCalls >= maxCalls {
				maxCalls = branchRes.EstimatedLLMCalls
				worstBranchRes = branchRes
			}
		}
		
		if worstBranchRes != nil {
			res.EstimatedLLMCalls += worstBranchRes.EstimatedLLMCalls
			res.EstimatedInputTokens += worstBranchRes.EstimatedInputTokens
			res.EstimatedCost += worstBranchRes.EstimatedCost
		}

	case *ast.DoBlock:
		for _, bodyStmt := range s.Statements {
			p.planStatement(bodyStmt, res)
		}

	case *ast.CallStatement:
		// Some built-ins might trigger LLM calls (e.g. semantic tools), but usually not.
		// For now, we only count explicit GENERATE and PROMPT.
	}
}

func (p *Planner) estimateExprTokens(expr ast.Expr, counter *tokencount.Counter) int {
	if expr == nil {
		return 0
	}
	switch ex := expr.(type) {
	case *ast.Literal:
		return counter.Count(ex.Value)
	case *ast.FStringLiteral:
		return counter.Count(ex.Template)
	case *ast.BinaryOp:
		return p.estimateExprTokens(ex.Left, counter) + p.estimateExprTokens(ex.Right, counter)
	case *ast.FunctionCall:
		tokens := 0
		for _, arg := range ex.Arguments {
			tokens += p.estimateExprTokens(arg, counter)
		}
		return tokens
	case *ast.SystemRoleCall:
		return counter.Count(ex.Description)
	}
	return 0
}

// PlanWorkflow returns a WorkflowPlan for a WORKFLOW statement.
func (p *Planner) PlanWorkflow(stmt *ast.WorkflowStatement) *WorkflowPlan {
	inputs := make([]string, 0, len(stmt.Inputs))
	for _, inp := range stmt.Inputs {
		inputs = append(inputs, inp.Name)
	}
	outputs := make([]string, 0, len(stmt.Outputs))
	for _, out := range stmt.Outputs {
		outputs = append(outputs, out.Name)
	}
	plan := &WorkflowPlan{
		WorkflowName: stmt.Name,
		Inputs:       inputs,
		Outputs:      outputs,
		Security:     stmt.Security,
		Accounting:   stmt.Accounting,
	}
	plan.Steps = p.planBody(stmt.Body)
	plan.ExceptionHandlers = p.planHandlers(stmt.ExceptionHandlers)
	plan.TotalEstimatedLLMCalls = countLLMCalls(plan.Steps)
	plan.TotalEstimatedTokens = countTokens(plan.Steps)
	return plan
}

// PlanProcedure returns a WorkflowPlan for a PROCEDURE statement.
func (p *Planner) PlanProcedure(stmt *ast.ProcedureStatement) *WorkflowPlan {
	inputs := make([]string, 0, len(stmt.Parameters))
	for _, param := range stmt.Parameters {
		inputs = append(inputs, param.Name)
	}
	var outputs []string
	if stmt.ReturnType != "" {
		outputs = []string{stmt.ReturnType}
	}
	plan := &WorkflowPlan{
		WorkflowName: stmt.Name,
		Inputs:       inputs,
		Outputs:      outputs,
		Security:     stmt.Security,
		Accounting:   stmt.Accounting,
	}
	plan.Steps = p.planBody(stmt.Body)
	plan.ExceptionHandlers = p.planHandlers(stmt.ExceptionHandlers)
	plan.TotalEstimatedLLMCalls = countLLMCalls(plan.Steps)
	plan.TotalEstimatedTokens = countTokens(plan.Steps)
	return plan
}

func (p *Planner) planBody(stmts []ast.Stmt) []WorkflowStep {
	steps := make([]WorkflowStep, 0, len(stmts))
	for _, stmt := range stmts {
		if step, ok := p.planBodyStatement(stmt); ok {
			steps = append(steps, step)
		}
	}
	return steps
}

func (p *Planner) planBodyStatement(stmt ast.Stmt) (WorkflowStep, bool) {
	switch s := stmt.(type) {
	case *ast.AssignmentStatement:
		return WorkflowStep{StepType: "assign", Description: fmt.Sprintf("@%s := ...", s.Variable)}, true
	case *ast.StorageAssignStatement:
		return WorkflowStep{StepType: "storage_assign", Description: fmt.Sprintf("@%s[...] := ...", s.StorageVar)}, true
	case *ast.GenerateIntoStatement:
		target := ""
		if s.TargetVariable != "" {
			target = " INTO @" + s.TargetVariable
		}
		budget := s.GenerateClause.OutputBudget
		if budget == 0 {
			budget = 1000
		}
		return WorkflowStep{
			StepType:          "generate",
			Description:       fmt.Sprintf("GENERATE %s(...)%s", s.GenerateClause.FunctionName, target),
			EstimatedLLMCalls: 1,
			EstimatedTokens:   budget,
		}, true
	case *ast.EvaluateStatement:
		branches := make([]WorkflowBranch, 0, len(s.WhenClauses))
		for _, wc := range s.WhenClauses {
			condStr, condType := describeCondition(wc.Condition)
			branches = append(branches, WorkflowBranch{
				Condition:     condStr,
				ConditionType: condType,
				Steps:         p.planBody(wc.Statements),
			})
		}
		hasSemantic := false
		for _, b := range branches {
			if b.ConditionType == "semantic" {
				hasSemantic = true
				break
			}
		}
		llmCalls := 0
		if hasSemantic {
			llmCalls = 1
		}
		return WorkflowStep{
			StepType:          "evaluate",
			Description:       "EVALUATE ...",
			EstimatedLLMCalls: llmCalls,
			Branches:          branches,
		}, true
	case *ast.WhileStatement:
		bodySteps := p.planBody(s.Body)
		genCalls := 0
		for _, bs := range bodySteps {
			genCalls += bs.EstimatedLLMCalls
		}
		iters := s.MaxIterations
		if iters == 0 {
			iters = 5
		}
		return WorkflowStep{
			StepType:          "while",
			Description:       "WHILE ... DO ... END",
			EstimatedLLMCalls: genCalls * iters,
			Substeps:          bodySteps,
		}, true
	case *ast.CommitStatement:
		return WorkflowStep{StepType: "commit", Description: "COMMIT ..."}, true
	case *ast.RetryStatement:
		return WorkflowStep{StepType: "retry", Description: "RETRY", EstimatedLLMCalls: 1}, true
	case *ast.RaiseStatement:
		return WorkflowStep{StepType: "raise", Description: fmt.Sprintf("RAISE %s", s.ExceptionType)}, true
	case *ast.CallStatement:
		return WorkflowStep{
			StepType:          "call",
			Description:       fmt.Sprintf("CALL %s(...)", s.ProcedureName),
			EstimatedLLMCalls: 1,
		}, true
	case *ast.SelectIntoStatement:
		return WorkflowStep{StepType: "select", Description: "SELECT ... INTO ..."}, true
	case *ast.LoggingStatement:
		return WorkflowStep{StepType: "logging", Description: "LOGGING ..."}, true
	case *ast.DoBlock:
		return WorkflowStep{
			StepType:          "do_block",
			Description:       "DO ... END",
			Substeps:          p.planBody(s.Statements),
			ExceptionHandlers: p.planHandlers(s.ExceptionHandlers),
		}, true
	case *ast.CallParallelStatement:
		return WorkflowStep{
			StepType:          "call_parallel",
			Description:       fmt.Sprintf("CALL PARALLEL (%d branches)", len(s.Branches)),
			EstimatedLLMCalls: len(s.Branches),
		}, true
	}
	return WorkflowStep{}, false
}

func (p *Planner) planHandlers(handlers []ast.ExceptionHandler) []WorkflowStep {
	steps := make([]WorkflowStep, 0, len(handlers))
	for _, h := range handlers {
		steps = append(steps, WorkflowStep{
			StepType:    "exception_handler",
			Description: fmt.Sprintf("WHEN %s THEN ...", h.ExceptionType),
			Substeps:    p.planBody(h.Statements),
		})
	}
	return steps
}

func describeCondition(cond interface{}) (string, string) {
	switch c := cond.(type) {
	case *ast.SemanticCondition:
		return fmt.Sprintf("'%s'", c.SemanticValue), "semantic"
	case *ast.ComparisonCondition:
		return fmt.Sprintf("%s ...", c.Operator), "deterministic"
	case *ast.Condition:
		return fmt.Sprintf("... %s ...", c.Operator), "deterministic"
	}
	return "...", "unknown"
}

func countLLMCalls(steps []WorkflowStep) int {
	total := 0
	for _, s := range steps {
		total += s.EstimatedLLMCalls
		total += countLLMCalls(s.Substeps)
		for _, b := range s.Branches {
			total += countLLMCalls(b.Steps)
		}
		total += countLLMCalls(s.ExceptionHandlers)
	}
	return total
}

func countTokens(steps []WorkflowStep) int {
	total := 0
	for _, s := range steps {
		total += s.EstimatedTokens
		total += countTokens(s.Substeps)
		for _, b := range s.Branches {
			total += countTokens(b.Steps)
		}
		total += countTokens(s.ExceptionHandlers)
	}
	return total
}

// Summary returns a human-readable summary of the plan.
func (r *PlanResult) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Estimated LLM Calls: %d\n", r.EstimatedLLMCalls))
	sb.WriteString(fmt.Sprintf("Estimated Input Tokens: %d\n", r.EstimatedInputTokens))
	sb.WriteString(fmt.Sprintf("Estimated Cost: $%.6f\n", r.EstimatedCost))
	if len(r.Warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}
	return sb.String()
}
