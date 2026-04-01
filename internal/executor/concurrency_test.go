package executor

import (
	"context"
	"testing"
	"time"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/ast"
)

// slowEchoAdapter records calls and adds a small delay to test parallelism
type slowEchoAdapter struct {
	*adapter.EchoAdapter
	callCount int
	calls     []string
}

func (a *slowEchoAdapter) Generate(ctx context.Context, prompt, model string, maxTokens int, temp float64, system string) (*adapter.GenerationResult, error) {
	a.callCount++
	a.calls = append(a.calls, prompt)
	time.Sleep(50 * time.Millisecond)
	return a.EchoAdapter.Generate(ctx, prompt, model, maxTokens, temp, system)
}

func (a *slowEchoAdapter) Name() string { return "slow_echo" }

func TestParallelExecutionGrouping(t *testing.T) {
	// 3 independent generate steps. With MaxWorkers > 0, they should run in parallel.
	program := &ast.Program{
		Statements: []ast.Stmt{
			&ast.WorkflowStatement{
				Name: "parallel_test",
				Body: []ast.Stmt{
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{FunctionName: "task1"},
						TargetVariable: "a",
					},
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{FunctionName: "task2"},
						TargetVariable: "b",
					},
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{FunctionName: "task3"},
						TargetVariable: "c",
					},
				},
			},
		},
	}

	slow := &slowEchoAdapter{EchoAdapter: adapter.NewEchoAdapter()}
	exec := New(slow)
	exec.MaxWorkers = 3

	start := time.Now()
	_, err := exec.ExecuteProgram(context.Background(), program, nil)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if slow.callCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", slow.callCount)
	}

	// Each call takes 50ms. 
	// Sequential = ~150ms
	// Parallel = ~50ms + overhead
	if duration > 120*time.Millisecond {
		t.Errorf("Execution took too long (%v), parallelization might not be working", duration)
	}
}

func TestParallelExecutionWithDependency(t *testing.T) {
	// @a := task1
	// @b := task2(@a) -- dependency
	// @c := task3     -- independent of @b but after it
	program := &ast.Program{
		Statements: []ast.Stmt{
			&ast.WorkflowStatement{
				Name: "dependency_test",
				Body: []ast.Stmt{
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{FunctionName: "task1"},
						TargetVariable: "a",
					},
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{
							FunctionName: "task2",
							Arguments:    []ast.Expr{&ast.Identifier{Name: "a"}},
						},
						TargetVariable: "b",
					},
					&ast.GenerateIntoStatement{
						GenerateClause: ast.GenerateClause{FunctionName: "task3"},
						TargetVariable: "c",
					},
				},
			},
		},
	}

	slow := &slowEchoAdapter{EchoAdapter: adapter.NewEchoAdapter()}
	exec := New(slow)
	exec.MaxWorkers = 3

	start := time.Now()
	_, err := exec.ExecuteProgram(context.Background(), program, nil)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Task 1 runs.
	// Task 2 depends on Task 1, so it must wait.
	// Task 3 is parallelizable but the loop currently groups adjacent ones.
	// Wait, task 3 is independent of task 1 and 2 in terms of input, 
	// but it comes after a dependency break (task 2 depends on task 1).
	
	// My refined logic:
	// i=0: stmt=task1. parallelizable=true. j=1: next=task2. referencesAny(task2, {a:true}) -> true. Break.
	// Batch=[task1]. Run task1.
	// i=1: stmt=task2. parallelizable=true. j=2: next=task3. referencesAny(task3, {b:true}) -> false. 
	// Batch=[task2, task3]. Run in parallel.
	
	// Total time should be roughly 100ms (50ms for task1, then 50ms for task2+task3)
	if duration > 130*time.Millisecond {
		t.Errorf("Execution took too long (%v)", duration)
	}
	if duration < 80*time.Millisecond {
		t.Errorf("Execution was too fast (%v)", duration)
	}
}
