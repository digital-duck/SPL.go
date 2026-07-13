// Package ir provides AST→JSON serialization for cross-language interop.
//
// The JSON format is compatible with the Python spl.ir module and is suitable
// for Momagrid execution, plan visualization, and cross-runtime exchange.
package ir

import (
	"encoding/json"

	"github.com/digital-duck/spl20go/internal/ast"
)

// ProgramToJSON serializes a full AST program to a JSON-compatible map.
func ProgramToJSON(prog *ast.Program) map[string]interface{} {
	stmts := make([]interface{}, 0, len(prog.Statements))
	for _, s := range prog.Statements {
		stmts = append(stmts, stmtToJSON(s))
	}
	return map[string]interface{}{
		"type":       "Program",
		"statements": stmts,
	}
}

// MarshalProgram serializes a program to a JSON byte slice.
func MarshalProgram(prog *ast.Program) ([]byte, error) {
	return json.Marshal(ProgramToJSON(prog))
}

// MarshalProgramIndent serializes with pretty-print indentation.
func MarshalProgramIndent(prog *ast.Program) ([]byte, error) {
	return json.MarshalIndent(ProgramToJSON(prog), "", "  ")
}

// =============================================================================
// Statement serialization
// =============================================================================

func stmtToJSON(stmt ast.Stmt) map[string]interface{} {
	switch s := stmt.(type) {
	case *ast.PromptStatement:
		return promptToJSON(s)
	case *ast.WorkflowStatement:
		return workflowToJSON(s)
	case *ast.ProcedureStatement:
		return procedureToJSON(s)
	case *ast.CreateFunctionStatement:
		params := make([]interface{}, 0, len(s.Parameters))
		for _, p := range s.Parameters {
			params = append(params, map[string]interface{}{"name": p.Name, "type": p.ParamType})
		}
		return map[string]interface{}{
			"type":        "CreateFunction",
			"name":        s.Name,
			"parameters":  params,
			"return_type": s.ReturnType,
			"body":        s.Body,
		}
	case *ast.ExplainStatement:
		return map[string]interface{}{"type": "Explain", "prompt_name": s.PromptName}
	case *ast.ExecuteStatement:
		params := make(map[string]interface{})
		for k, v := range s.Params {
			params[k] = exprToJSON(v)
		}
		return map[string]interface{}{"type": "Execute", "prompt_name": s.PromptName, "params": params}
	case *ast.ImportStatement:
		return map[string]interface{}{"type": "Import", "path": s.Path}
	default:
		return map[string]interface{}{"type": typeName(stmt)}
	}
}

func promptToJSON(s *ast.PromptStatement) map[string]interface{} {
	items := make([]interface{}, 0, len(s.SelectItems))
	for _, si := range s.SelectItems {
		items = append(items, selectItemToJSON(si))
	}
	result := map[string]interface{}{
		"type":         "Prompt",
		"name":         s.Name,
		"model":        s.Model,
		"budget":       s.Budget,
		"select_items": items,
	}
	if len(s.CTEs) > 0 {
		ctes := make([]interface{}, 0, len(s.CTEs))
		for _, c := range s.CTEs {
			var nested interface{}
			if c.NestedPrompt != nil {
				nested = promptToJSON(c.NestedPrompt)
			}
			ctes = append(ctes, map[string]interface{}{
				"name":          c.Name,
				"limit_tokens":  c.LimitTokens,
				"nested_prompt": nested,
			})
		}
		result["ctes"] = ctes
	}
	if s.GenerateClause != nil {
		gc := s.GenerateClause
		args := make([]interface{}, 0, len(gc.Arguments))
		for _, a := range gc.Arguments {
			args = append(args, exprToJSON(a))
		}
		result["generate"] = map[string]interface{}{
			"function":      gc.FunctionName,
			"arguments":     args,
			"output_budget": gc.OutputBudget,
			"temperature":   gc.Temperature,
			"format":        gc.OutputFormat,
		}
	}
	if s.StoreClause != nil {
		result["store"] = map[string]interface{}{"key": s.StoreClause.Key}
	}
	return result
}

func workflowToJSON(s *ast.WorkflowStatement) map[string]interface{} {
	inputs := paramSlice(s.Inputs)
	outputs := paramSlice(s.Outputs)
	body := bodySlice(s.Body)
	handlers := handlerSlice(s.ExceptionHandlers)
	return map[string]interface{}{
		"type":               "Workflow",
		"name":               s.Name,
		"inputs":             inputs,
		"outputs":            outputs,
		"security":           s.Security,
		"accounting":         s.Accounting,
		"labels":             s.Labels,
		"body":               body,
		"exception_handlers": handlers,
	}
}

func procedureToJSON(s *ast.ProcedureStatement) map[string]interface{} {
	params := paramSlice(s.Parameters)
	body := bodySlice(s.Body)
	handlers := handlerSlice(s.ExceptionHandlers)
	return map[string]interface{}{
		"type":               "Procedure",
		"name":               s.Name,
		"parameters":         params,
		"return_type":        s.ReturnType,
		"security":           s.Security,
		"accounting":         s.Accounting,
		"body":               body,
		"exception_handlers": handlers,
	}
}

// =============================================================================
// Body statement serialization
// =============================================================================

func bodyStmtToJSON(stmt ast.Stmt) map[string]interface{} {
	switch s := stmt.(type) {
	case *ast.AssignmentStatement:
		return map[string]interface{}{
			"type":       "Assignment",
			"variable":   s.Variable,
			"expression": exprToJSON(s.Expression),
		}
	case *ast.StorageAssignStatement:
		return map[string]interface{}{
			"type":        "StorageAssign",
			"storage_var": s.StorageVar,
			"key":         exprToJSON(s.Key),
			"value":       exprToJSON(s.Value),
		}
	case *ast.GenerateIntoStatement:
		gc := s.GenerateClause
		args := make([]interface{}, 0, len(gc.Arguments))
		for _, a := range gc.Arguments {
			args = append(args, exprToJSON(a))
		}
		return map[string]interface{}{
			"type":            "GenerateInto",
			"function":        gc.FunctionName,
			"arguments":       args,
			"output_budget":   gc.OutputBudget,
			"temperature":     gc.Temperature,
			"target_variable": s.TargetVariable,
		}
	case *ast.EvaluateStatement:
		whens := make([]interface{}, 0, len(s.WhenClauses))
		for _, wc := range s.WhenClauses {
			whens = append(whens, whenToJSON(wc))
		}
		var elseStmts interface{}
		if len(s.ElseStatements) > 0 {
			elseStmts = bodySlice(s.ElseStatements)
		}
		return map[string]interface{}{
			"type":        "Evaluate",
			"expression":  exprToJSON(s.Expression),
			"when_clauses": whens,
			"else":        elseStmts,
		}
	case *ast.WhileStatement:
		return map[string]interface{}{
			"type":      "While",
			"condition": conditionToJSON(s.Condition),
			"body":      bodySlice(s.Body),
		}
	case *ast.CommitStatement:
		opts := make(map[string]interface{})
		for k, v := range s.Options {
			opts[k] = exprToJSON(v)
		}
		return map[string]interface{}{
			"type":       "Commit",
			"expression": exprToJSON(s.Expression),
			"options":    opts,
		}
	case *ast.RetryStatement:
		opts := make(map[string]interface{})
		for k, v := range s.Options {
			opts[k] = exprToJSON(v)
		}
		return map[string]interface{}{"type": "Retry", "options": opts}
	case *ast.RaiseStatement:
		return map[string]interface{}{
			"type":           "Raise",
			"exception_type": s.ExceptionType,
			"message":        s.Message,
		}
	case *ast.CallStatement:
		args := make([]interface{}, 0, len(s.Arguments))
		for _, a := range s.Arguments {
			args = append(args, exprToJSON(a))
		}
		return map[string]interface{}{
			"type":            "Call",
			"procedure":       s.ProcedureName,
			"arguments":       args,
			"target_variable": s.TargetVariable,
		}
	case *ast.SelectIntoStatement:
		items := make([]interface{}, 0, len(s.SelectItems))
		for _, si := range s.SelectItems {
			items = append(items, selectItemToJSON(si))
		}
		return map[string]interface{}{
			"type":            "SelectInto",
			"items":           items,
			"target_variable": s.TargetVariable,
		}
	case *ast.DoBlock:
		return map[string]interface{}{
			"type":               "DoBlock",
			"statements":         bodySlice(s.Statements),
			"exception_handlers": handlerSlice(s.ExceptionHandlers),
		}
	case *ast.LoggingStatement:
		return map[string]interface{}{
			"type":        "Logging",
			"expression":  exprToJSON(s.Expression),
			"level":       s.Level,
			"destination": s.Destination,
		}
	case *ast.StoreStatement:
		return map[string]interface{}{
			"type":     "Store",
			"variable": s.Variable,
			"key":      s.Key,
		}
	case *ast.CallParallelStatement:
		branches := make([]interface{}, 0, len(s.Branches))
		for _, b := range s.Branches {
			args := make([]interface{}, 0, len(b.Arguments))
			for _, a := range b.Arguments {
				args = append(args, exprToJSON(a))
			}
			branches = append(branches, map[string]interface{}{
				"procedure":       b.ProcedureName,
				"arguments":       args,
				"target_variable": b.TargetVariable,
			})
		}
		return map[string]interface{}{"type": "CallParallel", "branches": branches}
	default:
		return map[string]interface{}{"type": typeName(stmt)}
	}
}

// =============================================================================
// Expression serialization
// =============================================================================

func exprToJSON(expr ast.Expr) interface{} {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Literal:
		return map[string]interface{}{"type": "Literal", "value": e.Value, "lit_type": e.LitType}
	case *ast.Identifier:
		return map[string]interface{}{"type": "Identifier", "name": e.Name}
	case *ast.ParamRef:
		return map[string]interface{}{"type": "ParamRef", "name": e.Name}
	case *ast.DottedName:
		return map[string]interface{}{"type": "DottedName", "parts": e.Parts, "full_name": e.FullName()}
	case *ast.SystemRoleCall:
		return map[string]interface{}{"type": "SystemRole", "description": e.Description}
	case *ast.ContextRef:
		return map[string]interface{}{"type": "ContextRef", "field_name": e.FieldName}
	case *ast.RagQuery:
		return map[string]interface{}{"type": "RagQuery", "query": exprToJSON(e.QueryText), "top_k": e.TopK}
	case *ast.MemoryGet:
		return map[string]interface{}{"type": "MemoryGet", "key": e.Key}
	case *ast.FStringLiteral:
		return map[string]interface{}{"type": "FString", "template": e.Template}
	case *ast.FunctionCall:
		args := make([]interface{}, 0, len(e.Arguments))
		for _, a := range e.Arguments {
			args = append(args, exprToJSON(a))
		}
		return map[string]interface{}{"type": "FunctionCall", "name": e.Name, "arguments": args}
	case *ast.BinaryOp:
		return map[string]interface{}{
			"type":  "BinaryOp",
			"op":    e.Op,
			"left":  exprToJSON(e.Left),
			"right": exprToJSON(e.Right),
		}
	case *ast.StorageSubscript:
		return map[string]interface{}{
			"type":        "StorageSubscript",
			"storage_var": e.StorageVar,
			"key":         exprToJSON(e.Key),
		}
	case *ast.StorageSpec:
		return map[string]interface{}{"type": "StorageSpec", "backend": e.Backend, "path": e.Path}
	case *ast.ListLiteral:
		elems := make([]interface{}, 0, len(e.Elements))
		for _, el := range e.Elements {
			elems = append(elems, exprToJSON(el))
		}
		return map[string]interface{}{"type": "List", "elements": elems}
	case *ast.MapLiteral:
		pairs := make([]interface{}, 0, len(e.Pairs))
		for _, p := range e.Pairs {
			pairs = append(pairs, map[string]interface{}{"key": exprToJSON(p.Key), "value": exprToJSON(p.Value)})
		}
		return map[string]interface{}{"type": "Map", "pairs": pairs}
	case *ast.UnaryOp:
		return map[string]interface{}{"type": "UnaryOp", "op": e.Operator, "operand": conditionToJSON(e.Operand)}
	default:
		return map[string]interface{}{"type": typeName(expr)}
	}
}

func conditionToJSON(cond interface{}) interface{} {
	if cond == nil {
		return nil
	}
	switch c := cond.(type) {
	case *ast.SemanticCondition:
		return map[string]interface{}{"type": "SemanticCondition", "value": c.SemanticValue}
	case *ast.ComparisonCondition:
		return map[string]interface{}{"type": "ComparisonCondition", "operator": c.Operator, "right": exprToJSON(c.Right)}
	case *ast.Condition:
		return map[string]interface{}{
			"type":     "Condition",
			"left":     exprToJSON(c.Left),
			"operator": c.Operator,
			"right":    exprToJSON(c.Right),
		}
	case *ast.CompoundCondition:
		return map[string]interface{}{
			"type":     "CompoundCondition",
			"operator": c.Operator,
			"left":     conditionToJSON(c.Left),
			"right":    conditionToJSON(c.Right),
		}
	default:
		if e, ok := cond.(ast.Expr); ok {
			return exprToJSON(e)
		}
		return map[string]interface{}{"type": "Unknown"}
	}
}

func whenToJSON(wc ast.WhenClause) map[string]interface{} {
	return map[string]interface{}{
		"condition":  conditionToJSON(wc.Condition),
		"statements": bodySlice(wc.Statements),
	}
}

func handlerToJSON(h ast.ExceptionHandler) map[string]interface{} {
	return map[string]interface{}{
		"exception_type": h.ExceptionType,
		"statements":     bodySlice(h.Statements),
	}
}

func selectItemToJSON(si ast.SelectItem) map[string]interface{} {
	return map[string]interface{}{
		"expression":   exprToJSON(si.Expression),
		"alias":        si.Alias,
		"limit_tokens": si.LimitTokens,
	}
}

// =============================================================================
// Helpers
// =============================================================================

func paramSlice(params []ast.Parameter) []interface{} {
	result := make([]interface{}, 0, len(params))
	for _, p := range params {
		var def interface{}
		if p.DefaultValue != nil {
			def = exprToJSON(p.DefaultValue)
		}
		result = append(result, map[string]interface{}{"name": p.Name, "type": p.ParamType, "default": def})
	}
	return result
}

func bodySlice(stmts []ast.Stmt) []interface{} {
	result := make([]interface{}, 0, len(stmts))
	for _, s := range stmts {
		result = append(result, bodyStmtToJSON(s))
	}
	return result
}

func handlerSlice(handlers []ast.ExceptionHandler) []interface{} {
	result := make([]interface{}, 0, len(handlers))
	for _, h := range handlers {
		result = append(result, handlerToJSON(h))
	}
	return result
}

func typeName(v interface{}) string {
	if v == nil {
		return "nil"
	}
	t := json.RawMessage{}
	_ = t
	switch v.(type) {
	case *ast.PromptStatement:
		return "PromptStatement"
	case *ast.WorkflowStatement:
		return "WorkflowStatement"
	case *ast.ProcedureStatement:
		return "ProcedureStatement"
	default:
		return "Unknown"
	}
}
