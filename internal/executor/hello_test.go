package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/ast"
)

func TestHelloRecipe(t *testing.T) {
	// Replicate the hello.spl recipe AST
	stmt := &ast.PromptStatement{
		Name: "hello_world",
		SelectItems: []ast.SelectItem{
			{
				Expression: &ast.SystemRoleCall{
					Description: "You are a friendly assistant. Introduce yourself and SPL 2.0 in two sentences.",
				},
			},
		},
		GenerateClause: &ast.GenerateClause{
			FunctionName: "greeting",
			Arguments:    []ast.Expr{},
		},
	}

	adp := adapter.NewEchoAdapter()
	exec := New(adp)
	
	res, err := exec.ExecutePrompt(context.Background(), stmt, nil)
	if err != nil {
		t.Fatalf("ExecutePrompt failed: %v", err)
	}

	fmt.Printf("Prompt sent to adapter:\n%s\n", res.Content)
	
	// If the prompt is empty, llama3.2 will be confused.
	// In the user's case, it seems it was:
	// "\n## Task\nBased on the above context, generate: greeting()"
}
