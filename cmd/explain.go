package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/digital-duck/spl20go/internal/analyzer"
	"github.com/digital-duck/spl20go/internal/ast"
	"github.com/digital-duck/spl20go/internal/lexer"
	"github.com/digital-duck/spl20go/internal/parser"
)

var explainCmd = &cobra.Command{
	Use:   "explain <file.spl>",
	Short: "Parse an SPL file and print a human-readable summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		source, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("cannot read %q: %w", filename, err)
		}

		l := lexer.New(string(source))
		tokens, err := l.Tokenize()
		if err != nil {
			return fmt.Errorf("lexer error in %q: %w", filename, err)
		}

		p := parser.New(tokens)
		program, err := p.Parse()
		if err != nil {
			return fmt.Errorf("parse error in %q: %w", filename, err)
		}

		// Semantic analysis
		a := analyzer.New()
		result, analysisErr := a.Analyze(program)

		printProgramSummary(filename, program)

		// Print analysis section
		fmt.Println()
		fmt.Println("Analysis:")
		if analysisErr != nil {
			fmt.Printf("  ERROR: %v\n", analysisErr)
		} else {
			summary := result.Summary()
			if summary != "" {
				for _, line := range strings.Split(strings.TrimRight(summary, "\n"), "\n") {
					fmt.Printf("  %s\n", line)
				}
			} else {
				fmt.Println("  (no issues found)")
			}
		}

		return nil
	},
}

func printProgramSummary(filename string, program *ast.Program) {
	fmt.Printf("File: %s\n", filename)
	fmt.Printf("Total statements: %d\n", len(program.Statements))
	fmt.Println()

	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.WorkflowStatement:
			fmt.Printf("WORKFLOW: %s\n", s.Name)
			if len(s.Inputs) > 0 {
				inputNames := make([]string, 0, len(s.Inputs))
				for _, inp := range s.Inputs {
					p := inp.Name
					if inp.ParamType != "" {
						p += " " + inp.ParamType
					}
					inputNames = append(inputNames, p)
				}
				fmt.Printf("  Inputs:  %s\n", strings.Join(inputNames, ", "))
			}
			if len(s.Outputs) > 0 {
				outputNames := make([]string, 0, len(s.Outputs))
				for _, out := range s.Outputs {
					outputNames = append(outputNames, out.Name)
				}
				fmt.Printf("  Outputs: %s\n", strings.Join(outputNames, ", "))
			}
			fmt.Printf("  Body statements: %d\n", len(s.Body))
			if len(s.ExceptionHandlers) > 0 {
				fmt.Printf("  Exception handlers: %d\n", len(s.ExceptionHandlers))
			}

		case *ast.ProcedureStatement:
			fmt.Printf("PROCEDURE: %s\n", s.Name)
			if len(s.Parameters) > 0 {
				paramNames := make([]string, 0, len(s.Parameters))
				for _, param := range s.Parameters {
					p := param.Name
					if param.ParamType != "" {
						p += " " + param.ParamType
					}
					paramNames = append(paramNames, p)
				}
				fmt.Printf("  Parameters: %s\n", strings.Join(paramNames, ", "))
			}
			if s.ReturnType != "" {
				fmt.Printf("  Returns: %s\n", s.ReturnType)
			}
			fmt.Printf("  Body statements: %d\n", len(s.Body))

		case *ast.PromptStatement:
			fmt.Printf("PROMPT: %s\n", s.Name)
			if s.Budget > 0 {
				fmt.Printf("  Budget: %d tokens\n", s.Budget)
			}
			if s.Model != "" {
				fmt.Printf("  Model: %s\n", s.Model)
			}
			if s.GenerateClause != nil {
				fmt.Printf("  Generate: %s\n", s.GenerateClause.FunctionName)
			}

		case *ast.CreateFunctionStatement:
			fmt.Printf("FUNCTION: %s\n", s.Name)
			if len(s.Parameters) > 0 {
				paramNames := make([]string, 0, len(s.Parameters))
				for _, param := range s.Parameters {
					paramNames = append(paramNames, param.Name)
				}
				fmt.Printf("  Parameters: %s\n", strings.Join(paramNames, ", "))
			}
			if s.ReturnType != "" {
				fmt.Printf("  Returns: %s\n", s.ReturnType)
			}
		}
	}
}
