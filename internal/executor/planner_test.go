package executor

import (
	"testing"

	"github.com/digital-duck/spl20go/internal/ast"
)

func TestPlannerSimple(t *testing.T) {
	// Simple program with one PROMPT and one WORKFLOW with 2 parallelizable steps
	program := &ast.Program{
		Statements: []ast.Stmt{
			&ast.PromptStatement{
				Name:  "test_prompt",
				Model: "llama3.2",
				SelectItems: []ast.SelectItem{
					{Expression: &ast.Literal{Value: "hello"}},
				},
			},
			&ast.WorkflowStatement{
				Name: "test_workflow",
				Body: []ast.Stmt{
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{
							FunctionName: "task1",
							Model:        "llama3.2",
							Arguments:    []ast.Expr{&ast.Literal{Value: "input1"}},
						},
						TargetVariable: "res1",
					},
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{
							FunctionName: "task2",
							Model:        "llama3.2",
							Arguments:    []ast.Expr{&ast.Literal{Value: "input2"}},
						},
						TargetVariable: "res2",
					},
				},
			},
		},
	}

	planner := NewPlanner("llama3.2")
	plan := planner.Plan(program)

	if plan.EstimatedLLMCalls != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", plan.EstimatedLLMCalls)
	}

	// 1 prompt + 2 generate into
	// "hello" (5 chars) + "input1" (6 chars) + "input2" (6 chars) = 17 chars
	// llama3.2 has CPT=3.5. 17 / 3.5 = 4.85 -> 4 (int)
	// Wait, estimateExprTokens rounds? No, it uses counter.Count
	// "hello" (5/3.5 = 1.4) -> 1
	// "input1" (6/3.5 = 1.7) -> 1
	// "input2" (6/3.5 = 1.7) -> 1
	// Total = 3
	if plan.EstimatedInputTokens < 3 {
		t.Errorf("Expected at least 3 input tokens, got %d", plan.EstimatedInputTokens)
	}
}

func TestPlannerWithWhile(t *testing.T) {
	program := &ast.Program{
		Statements: []ast.Stmt{
			&ast.WorkflowStatement{
				Name: "loop_workflow",
				Body: []ast.Stmt{
					&ast.WhileStatement{
						MaxIterations: 10,
						Body: []ast.Stmt{
							&ast.GenerateIntoStatement{
								GenerateClause: ast.GenerateClause{
									FunctionName: "loop_task",
								},
							},
						},
					},
				},
			},
		},
	}

	planner := NewPlanner("llama3.2")
	plan := planner.Plan(program)

	if plan.EstimatedLLMCalls != 10 {
		t.Errorf("Expected 10 LLM calls from while loop, got %d", plan.EstimatedLLMCalls)
	}
}
