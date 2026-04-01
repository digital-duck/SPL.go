// Package executor implements the SPL 2.0 runtime execution engine and planner.
package executor

import (
	"fmt"
	"strings"

	"github.com/digital-duck/spl20go/internal/ast"
	"github.com/digital-duck/spl20go/internal/tokencount"
)

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
