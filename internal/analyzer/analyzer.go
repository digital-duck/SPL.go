// Package analyzer provides semantic analysis for SPL 2.0 AST programs.
// Mirrors spl/analyzer.py from the Python SPL runtime.
package analyzer

import (
	"fmt"
	"strings"

	"github.com/digital-duck/spl20go/internal/ast"
)

// AnalysisWarning is a non-fatal diagnostic from semantic analysis.
type AnalysisWarning struct {
	Message string
}

// AnalysisError is a fatal semantic error.
type AnalysisError struct {
	Message string
}

func (e *AnalysisError) Error() string { return e.Message }

// AnalysisResult contains all findings from semantic analysis.
type AnalysisResult struct {
	Warnings          []AnalysisWarning
	DefinedPrompts    map[string]bool
	DefinedWorkflows  map[string]bool
	DefinedProcedures map[string]bool
	DefinedFunctions  map[string]bool
}

// validExceptionTypes is the set of recognized exception type names.
var validExceptionTypes = map[string]bool{
	"HallucinationDetected":  true,
	"RefusalToAnswer":        true,
	"ContextLengthExceeded":  true,
	"ModelOverloaded":        true,
	"QualityBelowThreshold":  true,
	"MaxIterationsReached":   true,
	"BudgetExceeded":         true,
	"NodeUnavailable":        true,
	"GenerationError":        true,
	"OTHERS":                 true,
	"Others":                 true,
}

// Analyzer performs semantic analysis on an SPL 2.0 AST program.
type Analyzer struct{}

// New creates a new Analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Analyze walks the AST program and returns an AnalysisResult.
// Returns an *AnalysisError if a fatal semantic error is found.
func (a *Analyzer) Analyze(program *ast.Program) (*AnalysisResult, error) {
	result := &AnalysisResult{
		DefinedPrompts:    make(map[string]bool),
		DefinedWorkflows:  make(map[string]bool),
		DefinedProcedures: make(map[string]bool),
		DefinedFunctions:  make(map[string]bool),
	}

	for _, stmt := range program.Statements {
		if err := a.analyzeStatement(stmt, result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (a *Analyzer) analyzeStatement(stmt ast.Stmt, result *AnalysisResult) error {
	switch s := stmt.(type) {
	case *ast.PromptStatement:
		if result.DefinedPrompts[s.Name] {
			return &AnalysisError{Message: fmt.Sprintf("duplicate PROMPT name: %q", s.Name)}
		}
		result.DefinedPrompts[s.Name] = true

		// Check temperature
		if s.GenerateClause != nil {
			t := s.GenerateClause.Temperature
			if t != 0 && (t < 0 || t > 2) {
				return &AnalysisError{Message: fmt.Sprintf("PROMPT %q: temperature %.2f out of range [0, 2]", s.Name, t)}
			}
			if s.GenerateClause.OutputBudget < 0 {
				return &AnalysisError{Message: fmt.Sprintf("PROMPT %q: output budget must be > 0", s.Name)}
			}
		}

	case *ast.WorkflowStatement:
		if result.DefinedWorkflows[s.Name] {
			return &AnalysisError{Message: fmt.Sprintf("duplicate WORKFLOW name: %q", s.Name)}
		}
		result.DefinedWorkflows[s.Name] = true

		// Check exception handlers
		for _, handler := range s.ExceptionHandlers {
			if !validExceptionTypes[handler.ExceptionType] {
				result.Warnings = append(result.Warnings, AnalysisWarning{
					Message: fmt.Sprintf("WORKFLOW %q: unknown exception type %q", s.Name, handler.ExceptionType),
				})
			}
		}

		// Recurse into body
		for _, bodyStmt := range s.Body {
			if err := a.analyzeBodyStatement(bodyStmt, s.Name, "WORKFLOW", result); err != nil {
				return err
			}
		}

	case *ast.ProcedureStatement:
		if result.DefinedProcedures[s.Name] {
			return &AnalysisError{Message: fmt.Sprintf("duplicate PROCEDURE name: %q", s.Name)}
		}
		result.DefinedProcedures[s.Name] = true

		// Check exception handlers
		for _, handler := range s.ExceptionHandlers {
			if !validExceptionTypes[handler.ExceptionType] {
				result.Warnings = append(result.Warnings, AnalysisWarning{
					Message: fmt.Sprintf("PROCEDURE %q: unknown exception type %q", s.Name, handler.ExceptionType),
				})
			}
		}

		// Recurse into body
		for _, bodyStmt := range s.Body {
			if err := a.analyzeBodyStatement(bodyStmt, s.Name, "PROCEDURE", result); err != nil {
				return err
			}
		}

	case *ast.CreateFunctionStatement:
		if result.DefinedFunctions[s.Name] {
			return &AnalysisError{Message: fmt.Sprintf("duplicate CREATE FUNCTION name: %q", s.Name)}
		}
		result.DefinedFunctions[s.Name] = true
	}

	return nil
}

func (a *Analyzer) analyzeBodyStatement(stmt ast.Stmt, parentName, parentType string, result *AnalysisResult) error {
	switch s := stmt.(type) {
	case *ast.GenerateIntoStatement:
		t := s.GenerateClause.Temperature
		if t != 0 && (t < 0 || t > 2) {
			return &AnalysisError{Message: fmt.Sprintf("%s %q: temperature %.2f out of range [0, 2]", parentType, parentName, t)}
		}
		if s.GenerateClause.OutputBudget < 0 {
			return &AnalysisError{Message: fmt.Sprintf("%s %q: output budget must be > 0", parentType, parentName)}
		}

	case *ast.DoBlock:
		for _, handler := range s.ExceptionHandlers {
			if !validExceptionTypes[handler.ExceptionType] {
				result.Warnings = append(result.Warnings, AnalysisWarning{
					Message: fmt.Sprintf("%s %q DO block: unknown exception type %q", parentType, parentName, handler.ExceptionType),
				})
			}
		}
		for _, inner := range s.Statements {
			if err := a.analyzeBodyStatement(inner, parentName, parentType, result); err != nil {
				return err
			}
		}

	case *ast.WhileStatement:
		for _, inner := range s.Body {
			if err := a.analyzeBodyStatement(inner, parentName, parentType, result); err != nil {
				return err
			}
		}

	case *ast.EvaluateStatement:
		for _, when := range s.WhenClauses {
			for _, inner := range when.Statements {
				if err := a.analyzeBodyStatement(inner, parentName, parentType, result); err != nil {
					return err
				}
			}
		}
		for _, inner := range s.ElseStatements {
			if err := a.analyzeBodyStatement(inner, parentName, parentType, result); err != nil {
				return err
			}
		}
	}

	return nil
}

// Summary returns a human-readable summary of the analysis result.
func (r *AnalysisResult) Summary() string {
	var sb strings.Builder

	if len(r.DefinedPrompts) > 0 {
		names := make([]string, 0, len(r.DefinedPrompts))
		for n := range r.DefinedPrompts {
			names = append(names, n)
		}
		sb.WriteString(fmt.Sprintf("Prompts: %s\n", strings.Join(names, ", ")))
	}
	if len(r.DefinedWorkflows) > 0 {
		names := make([]string, 0, len(r.DefinedWorkflows))
		for n := range r.DefinedWorkflows {
			names = append(names, n)
		}
		sb.WriteString(fmt.Sprintf("Workflows: %s\n", strings.Join(names, ", ")))
	}
	if len(r.DefinedProcedures) > 0 {
		names := make([]string, 0, len(r.DefinedProcedures))
		for n := range r.DefinedProcedures {
			names = append(names, n)
		}
		sb.WriteString(fmt.Sprintf("Procedures: %s\n", strings.Join(names, ", ")))
	}
	if len(r.DefinedFunctions) > 0 {
		names := make([]string, 0, len(r.DefinedFunctions))
		for n := range r.DefinedFunctions {
			names = append(names, n)
		}
		sb.WriteString(fmt.Sprintf("Functions: %s\n", strings.Join(names, ", ")))
	}
	if len(r.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("Warnings: %d\n", len(r.Warnings)))
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  WARNING: %s\n", w.Message))
		}
	}

	return sb.String()
}
