// Package parser implements the SPL 2.0 recursive descent parser.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/digital-duck/spl20go/internal/ast"
	"github.com/digital-duck/spl20go/internal/lexer"
)

// ParseError is returned when the parser encounters unexpected tokens.
type ParseError struct {
	Message string
	Token   lexer.Token
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("Parse error at %d:%d: %s", e.Token.Line, e.Token.Column, e.Message)
}

// Parser is the SPL 2.0 recursive descent parser.
type Parser struct {
	tokens []lexer.Token
	pos    int
}

// New creates a new Parser from a token stream.
func New(tokens []lexer.Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// Parse parses the full program and returns the AST.
func (p *Parser) Parse() (*ast.Program, error) {
	var stmts []ast.Stmt
	for !p.check(lexer.EOF) {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
		// Optional semicolons between statements
		for p.check(lexer.SEMICOLON) {
			p.advance()
		}
	}
	return &ast.Program{Statements: stmts}, nil
}

// =============================================================================
// Statement Dispatch
// =============================================================================

func (p *Parser) parseStatement() (ast.Stmt, error) {
	switch p.current().Type {
	case lexer.PROMPT:
		return p.parsePromptStatement()
	case lexer.WITH:
		return p.parsePromptStatement()
	case lexer.CREATE:
		return p.parseCreateFunction()
	case lexer.EXPLAIN:
		return p.parseExplain()
	case lexer.EXECUTE:
		return p.parseExecute()
	case lexer.WORKFLOW:
		return p.parseWorkflowStatement()
	case lexer.PROCEDURE:
		return p.parseProcedureStatement()
	case lexer.EVALUATE:
		return p.parseEvaluateStatement()
	case lexer.WHILE:
		return p.parseWhileStatement()
	case lexer.DO:
		return p.parseDoBlock()
	case lexer.COMMIT:
		return p.parseCommitStatement()
	case lexer.RETRY:
		return p.parseRetryStatement()
	case lexer.RAISE:
		return p.parseRaiseStatement()
	case lexer.IMPORT:
		return p.parseImportStatement()
	case lexer.CALL:
		return p.parseCallStatement()
	case lexer.LOGGING:
		return p.parseLoggingStatement()
	case lexer.GENERATE:
		return p.parseGenerateIntoStatement()
	case lexer.SELECT:
		return p.parseSelectIntoStatement()
	case lexer.STORE:
		return p.parseStoreStatement()
	case lexer.AT:
		return p.parseAssignmentStatement()
	case lexer.SET:
		return p.parseSetStatement()
	}
	tok := p.current()
	return nil, &ParseError{
		Message: fmt.Sprintf("Expected statement keyword, got %d (%q)", tok.Type, tok.Value),
		Token:   tok,
	}
}

func (p *Parser) isBodyEnd() bool {
	return p.checkAny(lexer.END, lexer.EXCEPTION, lexer.EOF)
}

func (p *Parser) isWhenOrEnd() bool {
	return p.checkAny(lexer.WHEN, lexer.ELSE, lexer.END, lexer.EOF)
}

// =============================================================================
// SPL 1.0: PROMPT Statement
// =============================================================================

func (p *Parser) parseCTESelectInto(ctes []ast.CTEClause) (*ast.SelectIntoStatement, error) {
	selectItems, err := p.parseSelectClause()
	if err != nil {
		return nil, err
	}

	var fromClause *ast.FromClause
	if p.check(lexer.FROM) {
		fromClause, err = p.parseFromClause()
		if err != nil {
			return nil, err
		}
	}

	var whereClause *ast.WhereClause
	if p.check(lexer.WHERE) {
		whereClause, err = p.parseWhereClause()
		if err != nil {
			return nil, err
		}
	}

	var targetVariables []string
	if p.check(lexer.INTO) {
		p.advance()
		if _, err := p.expect(lexer.AT); err != nil {
			return nil, err
		}
		nameTok, err := p.expectIdentifierOrKeyword()
		if err != nil {
			return nil, err
		}
		targetVariables = append(targetVariables, nameTok.Value)
		for p.check(lexer.COMMA) {
			p.advance()
			if _, err := p.expect(lexer.AT); err != nil {
				return nil, err
			}
			nameTok, err = p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			targetVariables = append(targetVariables, nameTok.Value)
		}
	}

	return &ast.SelectIntoStatement{
		SelectItems:     selectItems,
		FromClause:      fromClause,
		WhereClause:     whereClause,
		TargetVariables: targetVariables,
		CTEs:            ctes,
	}, nil
}

func (p *Parser) parsePromptStatement() (ast.Stmt, error) {
	var ctes []ast.CTEClause
	var err error

	// Optional CTEs at the start: WITH <name> AS (...)
	if p.check(lexer.WITH) && !p.peekIs(lexer.BUDGET) && !p.peekIs(lexer.VRAM) {
		ctes, err = p.parseCTEBlock()
		if err != nil {
			return nil, err
		}
	}

	// CTEs followed by SELECT ... INTO (workflow fan-out pattern)
	if len(ctes) > 0 && p.check(lexer.SELECT) {
		return p.parseCTESelectInto(ctes)
	}

	if _, err = p.expect(lexer.PROMPT); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}
	name := nameTok.Value

	budget := 0
	model := ""
	cacheDuration := ""
	version := ""

	// WITH BUDGET <n> TOKENS
	if p.check(lexer.WITH) && p.peekIs(lexer.BUDGET) {
		p.advance() // WITH
		p.advance() // BUDGET
		intTok, err := p.expect(lexer.INTEGER)
		if err != nil {
			return nil, err
		}
		budget, _ = strconv.Atoi(intTok.Value)
		if _, err = p.expect(lexer.TOKENS); err != nil {
			return nil, err
		}
	}

	// USING MODEL <name>
	if p.check(lexer.USING) {
		p.advance()
		if _, err = p.expect(lexer.MODEL); err != nil {
			return nil, err
		}
		if p.check(lexer.STRING) {
			model = p.advance().Value
		} else {
			model, err = p.readModelName()
			if err != nil {
				return nil, err
			}
		}
	}

	// CACHE FOR <duration>
	if p.check(lexer.CACHE) {
		p.advance()
		if _, err = p.expect(lexer.FOR); err != nil {
			return nil, err
		}
		durVal, err := p.expect(lexer.INTEGER)
		if err != nil {
			return nil, err
		}
		durUnit, err := p.expect(lexer.IDENTIFIER)
		if err != nil {
			return nil, err
		}
		cacheDuration = durVal.Value + " " + durUnit.Value
	}

	// VERSION <version>
	if p.check(lexer.VERSION) {
		p.advance()
		if p.check(lexer.FLOAT) {
			version = p.advance().Value
		} else if p.check(lexer.INTEGER) {
			version = p.advance().Value
		} else {
			vTok, err := p.expect(lexer.STRING)
			if err != nil {
				return nil, err
			}
			version = vTok.Value
		}
	}

	// ON GRID [<url>]
	onGrid := ""
	hasOnGrid := false
	if p.check(lexer.ON) && p.peekIs(lexer.GRID) {
		p.advance() // ON
		p.advance() // GRID
		hasOnGrid = true
		if p.check(lexer.STRING) {
			onGrid = p.advance().Value
		}
	}
	_ = hasOnGrid

	// WITH VRAM <n>
	minVRAMGB := 0.0
	if p.check(lexer.WITH) && p.peekIs(lexer.VRAM) {
		p.advance() // WITH
		p.advance() // VRAM
		if p.check(lexer.FLOAT) {
			minVRAMGB, _ = strconv.ParseFloat(p.advance().Value, 64)
		} else {
			intTok, err := p.expect(lexer.INTEGER)
			if err != nil {
				return nil, err
			}
			v, _ := strconv.ParseFloat(intTok.Value, 64)
			minVRAMGB = v
		}
	}

	// Parse optional CTEs after header
	if len(ctes) == 0 && p.check(lexer.WITH) && !p.peekIs(lexer.BUDGET) && !p.peekIs(lexer.VRAM) {
		ctes, err = p.parseCTEBlock()
		if err != nil {
			return nil, err
		}
	}

	selectItems, err := p.parseSelectClause()
	if err != nil {
		return nil, err
	}

	var whereClause *ast.WhereClause
	if p.check(lexer.WHERE) {
		whereClause, err = p.parseWhereClause()
		if err != nil {
			return nil, err
		}
	}

	var orderBy []ast.OrderByItem
	if p.check(lexer.ORDER) {
		orderBy, err = p.parseOrderBy()
		if err != nil {
			return nil, err
		}
	}

	var generateClause *ast.GenerateClause
	if p.check(lexer.GENERATE) {
		generateClause, err = p.parseGenerateClause()
		if err != nil {
			return nil, err
		}
	}

	var storeClause *ast.StoreClause
	if p.check(lexer.STORE) {
		storeClause, err = p.parseStoreClause()
		if err != nil {
			return nil, err
		}
	}

	return &ast.PromptStatement{
		Name:           name,
		Budget:         budget,
		Model:          model,
		CacheDuration:  cacheDuration,
		Version:        version,
		OnGrid:         onGrid,
		MinVRAMGB:      minVRAMGB,
		CTEs:           ctes,
		SelectItems:    selectItems,
		WhereClause:    whereClause,
		OrderBy:        orderBy,
		GenerateClause: generateClause,
		StoreClause:    storeClause,
	}, nil
}

// =============================================================================
// CTE Block
// =============================================================================

func (p *Parser) parseCTEBlock() ([]ast.CTEClause, error) {
	if _, err := p.expect(lexer.WITH); err != nil {
		return nil, err
	}
	first, err := p.parseCTEDef()
	if err != nil {
		return nil, err
	}
	ctes := []ast.CTEClause{first}
	for p.check(lexer.COMMA) {
		p.advance()
		cte, err := p.parseCTEDef()
		if err != nil {
			return nil, err
		}
		ctes = append(ctes, cte)
	}
	return ctes, nil
}

func (p *Parser) parseCTEDef() (ast.CTEClause, error) {
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return ast.CTEClause{}, err
	}
	if _, err = p.expect(lexer.AS); err != nil {
		return ast.CTEClause{}, err
	}
	if _, err = p.expect(lexer.LPAREN); err != nil {
		return ast.CTEClause{}, err
	}

	if p.check(lexer.PROMPT) || p.check(lexer.WITH) {
		nestedPrompt, err := p.parseInnerPrompt()
		if err != nil {
			return ast.CTEClause{}, err
		}
		if _, err = p.expect(lexer.RPAREN); err != nil {
			return ast.CTEClause{}, err
		}
		return ast.CTEClause{Name: nameTok.Value, NestedPrompt: nestedPrompt}, nil
	}

	selectItems, err := p.parseSelectClause()
	if err != nil {
		return ast.CTEClause{}, err
	}

	var fromClause *ast.FromClause
	if p.check(lexer.FROM) {
		fromClause, err = p.parseFromClause()
		if err != nil {
			return ast.CTEClause{}, err
		}
	}

	var whereClause *ast.WhereClause
	if p.check(lexer.WHERE) {
		whereClause, err = p.parseWhereClause()
		if err != nil {
			return ast.CTEClause{}, err
		}
	}

	limitTokens := 0
	if p.check(lexer.LIMIT) {
		p.advance()
		ltTok, err := p.expect(lexer.INTEGER)
		if err != nil {
			return ast.CTEClause{}, err
		}
		limitTokens, _ = strconv.Atoi(ltTok.Value)
		if _, err = p.expect(lexer.TOKENS); err != nil {
			return ast.CTEClause{}, err
		}
	}

	if _, err = p.expect(lexer.RPAREN); err != nil {
		return ast.CTEClause{}, err
	}

	return ast.CTEClause{
		Name:        nameTok.Value,
		SelectItems: selectItems,
		FromClause:  fromClause,
		WhereClause: whereClause,
		LimitTokens: limitTokens,
	}, nil
}

func (p *Parser) parseInnerPrompt() (*ast.PromptStatement, error) {
	if _, err := p.expect(lexer.PROMPT); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}

	budget := 0
	model := ""

	if p.check(lexer.WITH) && p.peekIs(lexer.BUDGET) {
		p.advance()
		p.advance()
		intTok, err := p.expect(lexer.INTEGER)
		if err != nil {
			return nil, err
		}
		budget, _ = strconv.Atoi(intTok.Value)
		if _, err = p.expect(lexer.TOKENS); err != nil {
			return nil, err
		}
	}

	if p.check(lexer.USING) {
		p.advance()
		if _, err = p.expect(lexer.MODEL); err != nil {
			return nil, err
		}
		if p.check(lexer.STRING) {
			model = p.advance().Value
		} else {
			model, err = p.readModelName()
			if err != nil {
				return nil, err
			}
		}
	}

	onGrid := ""
	if p.check(lexer.ON) && p.peekIs(lexer.GRID) {
		p.advance()
		p.advance()
		if p.check(lexer.STRING) {
			onGrid = p.advance().Value
		}
	}

	minVRAMGB := 0.0
	if p.check(lexer.WITH) && p.peekIs(lexer.VRAM) {
		p.advance()
		p.advance()
		if p.check(lexer.FLOAT) {
			minVRAMGB, _ = strconv.ParseFloat(p.advance().Value, 64)
		} else {
			intTok, err := p.expect(lexer.INTEGER)
			if err != nil {
				return nil, err
			}
			v, _ := strconv.ParseFloat(intTok.Value, 64)
			minVRAMGB = v
		}
	}

	selectItems, err := p.parseSelectClause()
	if err != nil {
		return nil, err
	}

	var generateClause *ast.GenerateClause
	if p.check(lexer.GENERATE) {
		generateClause, err = p.parseGenerateClause()
		if err != nil {
			return nil, err
		}
	}

	return &ast.PromptStatement{
		Name:           nameTok.Value,
		Budget:         budget,
		Model:          model,
		OnGrid:         onGrid,
		MinVRAMGB:      minVRAMGB,
		SelectItems:    selectItems,
		GenerateClause: generateClause,
	}, nil
}

// =============================================================================
// SELECT, FROM, WHERE, ORDER BY, GENERATE, STORE
// =============================================================================

func (p *Parser) parseSelectClause() ([]ast.SelectItem, error) {
	if _, err := p.expect(lexer.SELECT); err != nil {
		return nil, err
	}
	first, err := p.parseSelectItem()
	if err != nil {
		return nil, err
	}
	items := []ast.SelectItem{first}

	for p.check(lexer.COMMA) {
		p.advance()
		if p.checkAny(lexer.WHERE, lexer.ORDER, lexer.GENERATE, lexer.STORE, lexer.LIMIT, lexer.RPAREN, lexer.INTO) {
			break
		}
		item, err := p.parseSelectItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (p *Parser) parseSelectItem() (ast.SelectItem, error) {
	expr, err := p.parseSourceExpression()
	if err != nil {
		return ast.SelectItem{}, err
	}

	alias := ""
	if p.check(lexer.AS) {
		p.advance()
		aliasTok, err := p.expectIdentifierOrKeyword()
		if err != nil {
			return ast.SelectItem{}, err
		}
		alias = aliasTok.Value
	}

	limitTokens := 0
	if p.check(lexer.LIMIT) {
		p.advance()
		ltTok, err := p.expect(lexer.INTEGER)
		if err != nil {
			return ast.SelectItem{}, err
		}
		limitTokens, _ = strconv.Atoi(ltTok.Value)
		if _, err = p.expect(lexer.TOKENS); err != nil {
			return ast.SelectItem{}, err
		}
	}

	return ast.SelectItem{Expression: expr, Alias: alias, LimitTokens: limitTokens}, nil
}

func (p *Parser) parseSourceExpression() (ast.Expr, error) {
	tok := p.current()

	if tok.Type == lexer.IDENTIFIER {
		lower := strings.ToLower(tok.Value)

		if lower == "system_role" {
			p.advance()
			if _, err := p.expect(lexer.LPAREN); err != nil {
				return nil, err
			}
			descTok, err := p.expect(lexer.STRING)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(lexer.RPAREN); err != nil {
				return nil, err
			}
			return &ast.SystemRoleCall{Description: descTok.Value}, nil
		}

		if lower == "context" {
			p.advance()
			if _, err := p.expect(lexer.DOT); err != nil {
				return nil, err
			}
			fieldTok, err := p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			return &ast.ContextRef{FieldName: fieldTok.Value}, nil
		}

		if lower == "rag" {
			p.advance()
			if _, err := p.expect(lexer.DOT); err != nil {
				return nil, err
			}
			methodTok, err := p.expect(lexer.IDENTIFIER)
			if err != nil {
				return nil, err
			}
			if strings.ToLower(methodTok.Value) != "query" {
				return nil, &ParseError{
					Message: fmt.Sprintf("Expected 'query' after 'rag.', got %q", methodTok.Value),
					Token:   methodTok,
				}
			}
			if _, err = p.expect(lexer.LPAREN); err != nil {
				return nil, err
			}
			queryText, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			topK := 0
			if p.check(lexer.COMMA) {
				p.advance()
				argNameTok, err := p.expect(lexer.IDENTIFIER)
				if err != nil {
					return nil, err
				}
				if strings.ToLower(argNameTok.Value) != "top_k" {
					return nil, &ParseError{
						Message: fmt.Sprintf("Expected 'top_k', got %q", argNameTok.Value),
						Token:   argNameTok,
					}
				}
				if _, err = p.expect(lexer.EQ); err != nil {
					return nil, err
				}
				topKTok, err := p.expect(lexer.INTEGER)
				if err != nil {
					return nil, err
				}
				topK, _ = strconv.Atoi(topKTok.Value)
			}
			if _, err = p.expect(lexer.RPAREN); err != nil {
				return nil, err
			}
			return &ast.RagQuery{QueryText: queryText, TopK: topK}, nil
		}

		if lower == "memory" {
			p.advance()
			if _, err := p.expect(lexer.DOT); err != nil {
				return nil, err
			}
			methodTok, err := p.expect(lexer.IDENTIFIER)
			if err != nil {
				return nil, err
			}
			if strings.ToLower(methodTok.Value) != "get" {
				return nil, &ParseError{
					Message: fmt.Sprintf("Expected 'get' after 'memory.', got %q", methodTok.Value),
					Token:   methodTok,
				}
			}
			if _, err = p.expect(lexer.LPAREN); err != nil {
				return nil, err
			}
			keyTok, err := p.expect(lexer.STRING)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(lexer.RPAREN); err != nil {
				return nil, err
			}
			return &ast.MemoryGet{Key: keyTok.Value}, nil
		}
	}

	return p.parseExpression()
}

func (p *Parser) parseFromClause() (*ast.FromClause, error) {
	if _, err := p.expect(lexer.FROM); err != nil {
		return nil, err
	}
	source, err := p.parseSourceExpression()
	if err != nil {
		return nil, err
	}
	alias := ""
	if p.check(lexer.AS) {
		p.advance()
		aliasTok, err := p.expect(lexer.IDENTIFIER)
		if err != nil {
			return nil, err
		}
		alias = aliasTok.Value
	}
	return &ast.FromClause{Source: source, Alias: alias}, nil
}

func (p *Parser) parseWhereClause() (*ast.WhereClause, error) {
	if _, err := p.expect(lexer.WHERE); err != nil {
		return nil, err
	}
	first, err := p.parseComparisonCondition()
	if err != nil {
		return nil, err
	}
	conditions := []ast.Condition{first}
	var conjunctions []string

	for p.checkAny(lexer.AND, lexer.OR) {
		conj := p.advance().Value
		conjunctions = append(conjunctions, strings.ToUpper(conj))
		cond, err := p.parseComparisonCondition()
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, cond)
	}

	return &ast.WhereClause{Conditions: conditions, Conjunctions: conjunctions}, nil
}

func (p *Parser) parseComparisonCondition() (ast.Condition, error) {
	left, err := p.parseExpression()
	if err != nil {
		return ast.Condition{}, err
	}

	opMap := map[lexer.TokenType]string{
		lexer.EQ:  "=",
		lexer.NEQ: "!=",
		lexer.GT:  ">",
		lexer.LT:  "<",
		lexer.GTE: ">=",
		lexer.LTE: "<=",
		lexer.IN:  "IN",
	}

	tok := p.current()
	op, ok := opMap[tok.Type]
	if !ok {
		return ast.Condition{}, &ParseError{
			Message: fmt.Sprintf("Expected comparison operator, got %d (%q)", tok.Type, tok.Value),
			Token:   tok,
		}
	}
	p.advance()

	var right ast.Expr
	if op == "IN" {
		if _, err = p.expect(lexer.LPAREN); err != nil {
			return ast.Condition{}, err
		}
		firstVal, err := p.parseExpression()
		if err != nil {
			return ast.Condition{}, err
		}
		values := []ast.Expr{firstVal}
		for p.check(lexer.COMMA) {
			p.advance()
			val, err := p.parseExpression()
			if err != nil {
				return ast.Condition{}, err
			}
			values = append(values, val)
		}
		if _, err = p.expect(lexer.RPAREN); err != nil {
			return ast.Condition{}, err
		}
		right = &ast.FunctionCall{Name: "__in_list__", Arguments: values}
	} else {
		right, err = p.parseExpression()
		if err != nil {
			return ast.Condition{}, err
		}
	}

	return ast.Condition{Left: left, Operator: op, Right: right}, nil
}

func (p *Parser) parseOrderBy() ([]ast.OrderByItem, error) {
	if _, err := p.expect(lexer.ORDER); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.BY); err != nil {
		return nil, err
	}
	first, err := p.parseOrderItem()
	if err != nil {
		return nil, err
	}
	items := []ast.OrderByItem{first}
	for p.check(lexer.COMMA) {
		p.advance()
		item, err := p.parseOrderItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (p *Parser) parseOrderItem() (ast.OrderByItem, error) {
	expr, err := p.parseExpression()
	if err != nil {
		return ast.OrderByItem{}, err
	}
	direction := "ASC"
	if p.check(lexer.ASC) {
		p.advance()
	} else if p.check(lexer.DESC) {
		p.advance()
		direction = "DESC"
	}
	return ast.OrderByItem{Expression: expr, Direction: direction}, nil
}

func (p *Parser) parseGenerateClause() (*ast.GenerateClause, error) {
	if _, err := p.expect(lexer.GENERATE); err != nil {
		return nil, err
	}
	funcNameTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return nil, err
	}

	if _, err = p.expect(lexer.LPAREN); err != nil {
		return nil, err
	}
	var arguments []ast.Expr
	if !p.check(lexer.RPAREN) {
		arg, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, arg)
		for p.check(lexer.COMMA) {
			p.advance()
			arg, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, arg)
		}
	}
	if _, err = p.expect(lexer.RPAREN); err != nil {
		return nil, err
	}

	outputBudget := 0
	temperature := 0.0
	outputFormat := ""
	schema := ""

	generateWithTokens := map[lexer.TokenType]bool{
		lexer.OUTPUT:      true,
		lexer.TEMPERATURE: true,
		lexer.FORMAT:      true,
		lexer.SCHEMA:      true,
	}
	if p.check(lexer.WITH) && p.pos+1 < len(p.tokens) && generateWithTokens[p.tokens[p.pos+1].Type] {
		p.advance()
		for {
			if p.check(lexer.OUTPUT) {
				p.advance()
				if _, err = p.expect(lexer.BUDGET); err != nil {
					return nil, err
				}
				intTok, err := p.expect(lexer.INTEGER)
				if err != nil {
					return nil, err
				}
				outputBudget, _ = strconv.Atoi(intTok.Value)
				if _, err = p.expect(lexer.TOKENS); err != nil {
					return nil, err
				}
			} else if p.check(lexer.TEMPERATURE) {
				p.advance()
				if p.check(lexer.FLOAT) {
					temperature, _ = strconv.ParseFloat(p.advance().Value, 64)
				} else {
					intTok, err := p.expect(lexer.INTEGER)
					if err != nil {
						return nil, err
					}
					temperature, _ = strconv.ParseFloat(intTok.Value, 64)
				}
			} else if p.check(lexer.FORMAT) {
				p.advance()
				fmtTok, err := p.expect(lexer.IDENTIFIER)
				if err != nil {
					return nil, err
				}
				outputFormat = fmtTok.Value
			} else if p.check(lexer.SCHEMA) {
				p.advance()
				sTok, err := p.expect(lexer.IDENTIFIER)
				if err != nil {
					return nil, err
				}
				schema = sTok.Value
			} else {
				break
			}
			if p.check(lexer.COMMA) {
				p.advance()
			} else {
				break
			}
		}
	}

	model := ""
	if p.check(lexer.USING) {
		p.advance()
		if _, err = p.expect(lexer.MODEL); err != nil {
			return nil, err
		}
		if p.check(lexer.STRING) {
			model = p.advance().Value
		} else if p.check(lexer.AT) {
			p.advance()
			nameTok, err := p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			model = "@" + nameTok.Value
		} else {
			mTok, err := p.expect(lexer.IDENTIFIER)
			if err != nil {
				return nil, err
			}
			model = mTok.Value
		}
	}

	return &ast.GenerateClause{
		FunctionName: funcNameTok.Value,
		Arguments:    arguments,
		OutputBudget: outputBudget,
		Temperature:  temperature,
		OutputFormat: outputFormat,
		Schema:       schema,
		Model:        model,
	}, nil
}

func (p *Parser) parseStoreClause() (*ast.StoreClause, error) {
	if _, err := p.expect(lexer.STORE); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.RESULT); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.IN); err != nil {
		return nil, err
	}
	memTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(memTok.Value) != "memory" {
		return nil, &ParseError{
			Message: fmt.Sprintf("Expected 'memory' after STORE RESULT IN, got %q", memTok.Value),
			Token:   memTok,
		}
	}
	if _, err = p.expect(lexer.DOT); err != nil {
		return nil, err
	}
	keyTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return nil, err
	}
	return &ast.StoreClause{Key: keyTok.Value}, nil
}

// =============================================================================
// CREATE FUNCTION, EXPLAIN, EXECUTE
// =============================================================================

func (p *Parser) parseCreateFunction() (*ast.CreateFunctionStatement, error) {
	if _, err := p.expect(lexer.CREATE); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.FUNCTION); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}

	if _, err = p.expect(lexer.LPAREN); err != nil {
		return nil, err
	}
	var parameters []ast.Parameter
	if !p.check(lexer.RPAREN) {
		param, err := p.parseParameter()
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, param)
		for p.check(lexer.COMMA) {
			p.advance()
			param, err = p.parseParameter()
			if err != nil {
				return nil, err
			}
			parameters = append(parameters, param)
		}
	}
	if _, err = p.expect(lexer.RPAREN); err != nil {
		return nil, err
	}

	if _, err = p.expect(lexer.RETURNS); err != nil {
		return nil, err
	}
	rtTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}

	if _, err = p.expect(lexer.AS); err != nil {
		return nil, err
	}
	if _, err = p.expect(lexer.DOLLAR_DOLLAR); err != nil {
		return nil, err
	}
	bodyTok, err := p.expect(lexer.STRING)
	if err != nil {
		return nil, err
	}

	return &ast.CreateFunctionStatement{
		Name:       nameTok.Value,
		Parameters: parameters,
		ReturnType: rtTok.Value,
		Body:       bodyTok.Value,
	}, nil
}

func (p *Parser) parseParameter() (ast.Parameter, error) {
	nameTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return ast.Parameter{}, err
	}
	paramType := ""
	var defaultValue ast.Expr

	if p.check(lexer.IDENTIFIER) && !p.checkAny(lexer.COMMA, lexer.RPAREN, lexer.DEFAULT) {
		paramType = p.advance().Value
	}
	if p.check(lexer.DEFAULT) {
		p.advance()
		defaultValue, err = p.parseExpression()
		if err != nil {
			return ast.Parameter{}, err
		}
	}
	return ast.Parameter{Name: nameTok.Value, ParamType: paramType, DefaultValue: defaultValue}, nil
}

func (p *Parser) parseExplain() (*ast.ExplainStatement, error) {
	if _, err := p.expect(lexer.EXPLAIN); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.PROMPT); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}
	return &ast.ExplainStatement{PromptName: nameTok.Value}, nil
}

func (p *Parser) parseExecute() (*ast.ExecuteStatement, error) {
	if _, err := p.expect(lexer.EXECUTE); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.PROMPT); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}

	params := make(map[string]ast.Expr)
	if p.check(lexer.WITH) {
		p.advance()
		if _, err = p.expect(lexer.PARAMS); err != nil {
			return nil, err
		}
		if _, err = p.expect(lexer.LPAREN); err != nil {
			return nil, err
		}
		if !p.check(lexer.RPAREN) {
			key, val, err := p.parseParamAssignment()
			if err != nil {
				return nil, err
			}
			params[key] = val
			for p.check(lexer.COMMA) {
				p.advance()
				key, val, err = p.parseParamAssignment()
				if err != nil {
					return nil, err
				}
				params[key] = val
			}
		}
		if _, err = p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}
	}
	return &ast.ExecuteStatement{PromptName: nameTok.Value, Params: params}, nil
}

func (p *Parser) parseParamAssignment() (string, ast.Expr, error) {
	firstTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return "", nil, err
	}
	parts := []string{firstTok.Value}
	for p.check(lexer.DOT) {
		p.advance()
		partTok, err := p.expect(lexer.IDENTIFIER)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, partTok.Value)
	}
	key := strings.Join(parts, ".")
	if _, err = p.expect(lexer.EQ); err != nil {
		return "", nil, err
	}
	val, err := p.parseExpression()
	if err != nil {
		return "", nil, err
	}
	return key, val, nil
}

// =============================================================================
// WORKFLOW Statement
// =============================================================================

func (p *Parser) parseWorkflowStatement() (*ast.WorkflowStatement, error) {
	if _, err := p.expect(lexer.WORKFLOW); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}

	var inputs []ast.Parameter
	if p.check(lexer.INPUT) {
		p.advance()
		if p.check(lexer.COLON) {
			p.advance()
		}
		inputs, err = p.parseWorkflowParamList()
		if err != nil {
			return nil, err
		}
	}

	var outputs []ast.Parameter
	if p.check(lexer.OUTPUT) {
		p.advance()
		if p.check(lexer.COLON) {
			p.advance()
		}
		outputs, err = p.parseWorkflowParamList()
		if err != nil {
			return nil, err
		}
	}

	var security map[string]string
	if p.check(lexer.SECURITY) {
		security, err = p.parseSecurityBlock()
		if err != nil {
			return nil, err
		}
	}

	var accounting map[string]string
	if p.check(lexer.ACCOUNTING) {
		accounting, err = p.parseAccountingBlock()
		if err != nil {
			return nil, err
		}
	}

	var labels map[string]string
	if p.check(lexer.LABELS) {
		labels, err = p.parseLabelsBlock()
		if err != nil {
			return nil, err
		}
	}

	doBlock, err := p.parseDoBlock()
	if err != nil {
		return nil, err
	}

	return &ast.WorkflowStatement{
		Name:              nameTok.Value,
		Inputs:            inputs,
		Outputs:           outputs,
		Security:          security,
		Accounting:        accounting,
		Labels:            labels,
		Body:              doBlock.Statements,
		ExceptionHandlers: doBlock.ExceptionHandlers,
	}, nil
}

func (p *Parser) parseWorkflowParamList() ([]ast.Parameter, error) {
	first, err := p.parseWorkflowParam()
	if err != nil {
		return nil, err
	}
	params := []ast.Parameter{first}
	for p.check(lexer.COMMA) {
		p.advance()
		if p.checkAny(lexer.OUTPUT, lexer.INPUT, lexer.DO, lexer.SECURITY, lexer.ACCOUNTING, lexer.LABELS) {
			break
		}
		param, err := p.parseWorkflowParam()
		if err != nil {
			return nil, err
		}
		params = append(params, param)
	}
	return params, nil
}

func (p *Parser) parseWorkflowParam() (ast.Parameter, error) {
	if _, err := p.expect(lexer.AT); err != nil {
		return ast.Parameter{}, err
	}
	nameTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return ast.Parameter{}, err
	}
	paramType := ""
	var defaultValue ast.Expr

	if p.checkAny(lexer.IDENTIFIER, lexer.IMAGE, lexer.AUDIO, lexer.VIDEO) {
		if strings.ToUpper(p.current().Value) == "STORAGE" {
			p.advance()
			paramType = "STORAGE"
			if p.check(lexer.LPAREN) {
				p.advance()
				backendTok, err := p.expect(lexer.IDENTIFIER)
				if err != nil {
					return ast.Parameter{}, err
				}
				if _, err = p.expect(lexer.COMMA); err != nil {
					return ast.Parameter{}, err
				}
				pathTok, err := p.expect(lexer.STRING)
				if err != nil {
					return ast.Parameter{}, err
				}
				if _, err = p.expect(lexer.RPAREN); err != nil {
					return ast.Parameter{}, err
				}
				defaultValue = &ast.StorageSpec{Backend: backendTok.Value, Path: pathTok.Value}
			}
		} else {
			// Normalize SPL 3.0 media type keywords to uppercase strings
			tok := p.advance()
			switch tok.Type {
			case lexer.IMAGE:
				paramType = "IMAGE"
			case lexer.AUDIO:
				paramType = "AUDIO"
			case lexer.VIDEO:
				paramType = "VIDEO"
			default:
				paramType = tok.Value
			}
		}
	}

	if p.check(lexer.DEFAULT) {
		p.advance()
		defaultValue, err = p.parseExpression()
		if err != nil {
			return ast.Parameter{}, err
		}
	}

	return ast.Parameter{Name: nameTok.Value, ParamType: paramType, DefaultValue: defaultValue}, nil
}

// =============================================================================
// PROCEDURE Statement
// =============================================================================

func (p *Parser) parseProcedureStatement() (*ast.ProcedureStatement, error) {
	if _, err := p.expect(lexer.PROCEDURE); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}

	if _, err = p.expect(lexer.LPAREN); err != nil {
		return nil, err
	}
	var parameters []ast.Parameter
	if !p.check(lexer.RPAREN) {
		param, err := p.parseParameter()
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, param)
		for p.check(lexer.COMMA) {
			p.advance()
			param, err = p.parseParameter()
			if err != nil {
				return nil, err
			}
			parameters = append(parameters, param)
		}
	}
	if _, err = p.expect(lexer.RPAREN); err != nil {
		return nil, err
	}

	returnType := ""
	if p.check(lexer.RETURNS) {
		p.advance()
		rtTok, err := p.expect(lexer.IDENTIFIER)
		if err != nil {
			return nil, err
		}
		returnType = rtTok.Value
	}

	var security map[string]string
	if p.check(lexer.SECURITY) {
		security, err = p.parseSecurityBlock()
		if err != nil {
			return nil, err
		}
	}

	var accounting map[string]string
	if p.check(lexer.ACCOUNTING) {
		accounting, err = p.parseAccountingBlock()
		if err != nil {
			return nil, err
		}
	}

	doBlock, err := p.parseDoBlock()
	if err != nil {
		return nil, err
	}

	return &ast.ProcedureStatement{
		Name:              nameTok.Value,
		Parameters:        parameters,
		ReturnType:        returnType,
		Security:          security,
		Accounting:        accounting,
		Body:              doBlock.Statements,
		ExceptionHandlers: doBlock.ExceptionHandlers,
	}, nil
}

// =============================================================================
// DO Block
// =============================================================================

func (p *Parser) parseDoBlock() (*ast.DoBlock, error) {
	if _, err := p.expect(lexer.DO); err != nil {
		return nil, err
	}

	var statements []ast.Stmt
	for !p.isBodyEnd() {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
		for p.check(lexer.SEMICOLON) {
			p.advance()
		}
	}

	var exceptionHandlers []ast.ExceptionHandler
	if p.check(lexer.EXCEPTION) {
		var err error
		exceptionHandlers, err = p.parseExceptionBlock()
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(lexer.END); err != nil {
		return nil, err
	}

	return &ast.DoBlock{Statements: statements, ExceptionHandlers: exceptionHandlers}, nil
}

func (p *Parser) parseExceptionBlock() ([]ast.ExceptionHandler, error) {
	if _, err := p.expect(lexer.EXCEPTION); err != nil {
		return nil, err
	}

	var handlers []ast.ExceptionHandler
	for p.check(lexer.WHEN) {
		p.advance()
		var exceptionType string
		if p.check(lexer.OTHERS) {
			exceptionType = p.advance().Value
		} else {
			tok, err := p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			exceptionType = tok.Value
		}
		if _, err := p.expect(lexer.THEN); err != nil {
			return nil, err
		}

		var handlerStmts []ast.Stmt
		for !p.checkAny(lexer.WHEN, lexer.END, lexer.EOF) {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			handlerStmts = append(handlerStmts, stmt)
			for p.check(lexer.SEMICOLON) {
				p.advance()
			}
		}

		handlers = append(handlers, ast.ExceptionHandler{
			ExceptionType: exceptionType,
			Statements:    handlerStmts,
		})
	}
	return handlers, nil
}

// =============================================================================
// EVALUATE Statement
// =============================================================================

func (p *Parser) parseEvaluateStatement() (*ast.EvaluateStatement, error) {
	if _, err := p.expect(lexer.EVALUATE); err != nil {
		return nil, err
	}
	expression, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	var whenClauses []ast.WhenClause
	for p.check(lexer.WHEN) {
		p.advance()
		condition, err := p.parseEvaluateCondition()
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(lexer.THEN); err != nil {
			return nil, err
		}

		var thenStmts []ast.Stmt
		for !p.isWhenOrEnd() {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			thenStmts = append(thenStmts, stmt)
			for p.check(lexer.SEMICOLON) {
				p.advance()
			}
		}

		whenClauses = append(whenClauses, ast.WhenClause{
			Condition:  condition,
			Statements: thenStmts,
		})
	}

	var elseStmts []ast.Stmt
	if p.check(lexer.ELSE) {
		p.advance()
		for !p.checkAny(lexer.END, lexer.EOF) {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			elseStmts = append(elseStmts, stmt)
			for p.check(lexer.SEMICOLON) {
				p.advance()
			}
		}
	}

	if _, err := p.expect(lexer.END); err != nil {
		return nil, err
	}

	return &ast.EvaluateStatement{
		Expression:     expression,
		WhenClauses:    whenClauses,
		ElseStatements: elseStmts,
	}, nil
}

func (p *Parser) parseEvaluateCondition() (interface{}, error) {
	// Semantic condition: string literal
	if p.check(lexer.STRING) {
		value := p.advance().Value
		return &ast.SemanticCondition{SemanticValue: value}, nil
	}

	// Comparison condition: operator + expression
	opMap := map[lexer.TokenType]string{
		lexer.GT:  ">",
		lexer.LT:  "<",
		lexer.GTE: ">=",
		lexer.LTE: "<=",
		lexer.EQ:  "=",
		lexer.NEQ: "!=",
	}
	tok := p.current()
	if op, ok := opMap[tok.Type]; ok {
		p.advance()
		right, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &ast.ComparisonCondition{Operator: op, Right: right}, nil
	}

	// contains('value') [OR contains('value')]*
	if tok.Type == lexer.IDENTIFIER && strings.ToLower(tok.Value) == "contains" {
		var values []string
		for p.check(lexer.IDENTIFIER) && strings.ToLower(p.current().Value) == "contains" {
			p.advance()
			if _, err := p.expect(lexer.LPAREN); err != nil {
				return nil, err
			}
			valTok, err := p.expect(lexer.STRING)
			if err != nil {
				return nil, err
			}
			values = append(values, valTok.Value)
			if _, err = p.expect(lexer.RPAREN); err != nil {
				return nil, err
			}
			if !p.check(lexer.OR) {
				break
			}
			p.advance()
		}
		return &ast.SemanticCondition{SemanticValue: "contains:" + strings.Join(values, "|")}, nil
	}

	// STARTSWITH 'prefix'
	if tok.Type == lexer.IDENTIFIER && strings.ToLower(tok.Value) == "startswith" {
		p.advance()
		prefixTok, err := p.expect(lexer.STRING)
		if err != nil {
			return nil, err
		}
		return &ast.SemanticCondition{SemanticValue: "startswith:" + prefixTok.Value}, nil
	}

	return nil, &ParseError{
		Message: fmt.Sprintf("Expected condition (string literal or comparison operator), got %d (%q)", tok.Type, tok.Value),
		Token:   tok,
	}
}

// =============================================================================
// WHILE Statement
// =============================================================================

func (p *Parser) parseWhileStatement() (*ast.WhileStatement, error) {
	if _, err := p.expect(lexer.WHILE); err != nil {
		return nil, err
	}
	condition, err := p.parseWhileCondition()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(lexer.DO); err != nil {
		return nil, err
	}

	var body []ast.Stmt
	for !p.checkAny(lexer.END, lexer.EOF) {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, stmt)
		for p.check(lexer.SEMICOLON) {
			p.advance()
		}
	}
	if _, err = p.expect(lexer.END); err != nil {
		return nil, err
	}

	return &ast.WhileStatement{Condition: condition, Body: body}, nil
}

func (p *Parser) parseWhileCondition() (interface{}, error) {
	opMap := map[lexer.TokenType]string{
		lexer.GT:  ">",
		lexer.LT:  "<",
		lexer.GTE: ">=",
		lexer.LTE: "<=",
		lexer.EQ:  "=",
		lexer.NEQ: "!=",
	}

	// NOT <expr> — boolean negation at condition level
	if p.check(lexer.NOT) {
		p.advance()
		operand, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		left := &ast.UnaryOp{Operator: "NOT", Operand: operand}
		// Check for AND/OR compound after NOT <expr>
		if p.checkAny(lexer.AND, lexer.OR) {
			logicalOp := "AND"
			if p.current().Type == lexer.OR {
				logicalOp = "OR"
			}
			p.advance()
			right, err := p.parseWhileCondition()
			if err != nil {
				return nil, err
			}
			return &ast.CompoundCondition{Operator: logicalOp, Left: left, Right: right}, nil
		}
		return left, nil
	}

	left, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	tok := p.current()
	if op, ok := opMap[tok.Type]; ok {
		p.advance()
		right, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		condition := &ast.Condition{Left: left, Operator: op, Right: right}

		// Check for AND/OR compound after a comparison condition
		if p.checkAny(lexer.AND, lexer.OR) {
			logicalOp := "AND"
			if p.current().Type == lexer.OR {
				logicalOp = "OR"
			}
			p.advance()
			rightCond, err := p.parseWhileCondition()
			if err != nil {
				return nil, err
			}
			return &ast.CompoundCondition{Operator: logicalOp, Left: condition, Right: rightCond}, nil
		}
		return condition, nil
	}

	// No operator — expression-based condition (truthy string check)
	return left, nil
}

// =============================================================================
// COMMIT Statement
// =============================================================================

func (p *Parser) parseCommitStatement() (*ast.CommitStatement, error) {
	if _, err := p.expect(lexer.COMMIT); err != nil {
		return nil, err
	}
	expression, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	options := make(map[string]ast.Expr)
	if p.check(lexer.WITH) {
		p.advance()
		for {
			keyTok, err := p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(lexer.EQ); err != nil {
				return nil, err
			}
			val, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			options[keyTok.Value] = val
			if !p.check(lexer.COMMA) {
				break
			}
			p.advance()
		}
	}
	return &ast.CommitStatement{Expression: expression, Options: options}, nil
}

// =============================================================================
// RETRY Statement
// =============================================================================

func (p *Parser) parseRetryStatement() (*ast.RetryStatement, error) {
	if _, err := p.expect(lexer.RETRY); err != nil {
		return nil, err
	}

	options := make(map[string]ast.Expr)
	if p.check(lexer.WITH) {
		p.advance()
		for {
			keyTok, err := p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(lexer.EQ); err != nil {
				return nil, err
			}
			val, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			options[keyTok.Value] = val
			if !p.check(lexer.COMMA) {
				break
			}
			p.advance()
		}
	}

	limit := 0
	if p.check(lexer.LIMIT) {
		p.advance()
		limitTok, err := p.expect(lexer.INTEGER)
		if err != nil {
			return nil, err
		}
		limit, _ = strconv.Atoi(limitTok.Value)
	}

	return &ast.RetryStatement{Options: options, Limit: limit}, nil
}

// =============================================================================
// RAISE Statement
// =============================================================================

func (p *Parser) parseRaiseStatement() (*ast.RaiseStatement, error) {
	if _, err := p.expect(lexer.RAISE); err != nil {
		return nil, err
	}
	exTypeTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return nil, err
	}

	message := ""
	if p.check(lexer.STRING) {
		message = p.advance().Value
	}

	return &ast.RaiseStatement{ExceptionType: exTypeTok.Value, Message: message}, nil
}

// =============================================================================
// LOGGING Statement
// =============================================================================

func (p *Parser) parseLoggingStatement() (*ast.LoggingStatement, error) {
	if _, err := p.expect(lexer.LOGGING); err != nil {
		return nil, err
	}
	expression, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	level := "INFO"
	if p.check(lexer.LEVEL) {
		p.advance()
		levelTok, err := p.expectIdentifierOrKeyword()
		if err != nil {
			return nil, err
		}
		level = strings.ToUpper(levelTok.Value)
	}

	destination := ""
	if p.check(lexer.TO) {
		p.advance()
		destTok, err := p.expect(lexer.STRING)
		if err != nil {
			return nil, err
		}
		destination = destTok.Value
	}

	return &ast.LoggingStatement{Expression: expression, Level: level, Destination: destination}, nil
}

// =============================================================================
// Assignment Statement
// =============================================================================

func (p *Parser) parseSetStatement() (*ast.AssignmentStatement, error) {
	if _, err := p.expect(lexer.SET); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.AT); err != nil {
		return nil, err
	}
	nameTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(lexer.EQ); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.AssignmentStatement{Variable: nameTok.Value, Expression: value}, nil
}

func (p *Parser) parseStoreStatement() (*ast.StoreStatement, error) {
	if _, err := p.expect(lexer.STORE); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.AT); err != nil {
		return nil, err
	}
	varNameTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(lexer.IN); err != nil {
		return nil, err
	}
	memTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(memTok.Value) != "memory" {
		return nil, &ParseError{
			Message: fmt.Sprintf("Expected 'memory' after STORE @var IN, got %q", memTok.Value),
			Token:   memTok,
		}
	}
	if _, err = p.expect(lexer.DOT); err != nil {
		return nil, err
	}
	keyTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return nil, err
	}
	return &ast.StoreStatement{Variable: varNameTok.Value, Key: keyTok.Value}, nil
}

func (p *Parser) parseAssignmentStatement() (ast.Stmt, error) {
	if _, err := p.expect(lexer.AT); err != nil {
		return nil, err
	}
	varNameTok, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return nil, err
	}

	// Storage subscript assignment: @memory['key'] := expr
	if p.check(lexer.LBRACKET) {
		p.advance()
		keyExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(lexer.RBRACKET); err != nil {
			return nil, err
		}
		if _, err = p.expect(lexer.ASSIGN); err != nil {
			return nil, err
		}
		valueExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &ast.StorageAssignStatement{
			StorageVar: varNameTok.Value,
			Key:        keyExpr,
			Value:      valueExpr,
		}, nil
	}

	if _, err = p.expect(lexer.ASSIGN); err != nil {
		return nil, err
	}
	expression, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.AssignmentStatement{Variable: varNameTok.Value, Expression: expression}, nil
}

// =============================================================================
// GENERATE ... INTO @var
// =============================================================================

func (p *Parser) parseGenerateIntoStatement() (*ast.GenerateIntoStatement, error) {
	genClause, err := p.parseGenerateClause()
	if err != nil {
		return nil, err
	}

	target := ""
	if p.check(lexer.INTO) {
		p.advance()
		if _, err = p.expect(lexer.AT); err != nil {
			return nil, err
		}
		targetTok, err := p.expectIdentifierOrKeyword()
		if err != nil {
			return nil, err
		}
		target = targetTok.Value
	}

	return &ast.GenerateIntoStatement{
		GenerateClause: *genClause,
		TargetVariable: target,
	}, nil
}

// =============================================================================
// CALL Statement
// =============================================================================

func (p *Parser) parseCallStatement() (ast.Stmt, error) {
	if _, err := p.expect(lexer.CALL); err != nil {
		return nil, err
	}
	// SPL 3.0: CALL PARALLEL ... END
	if p.check(lexer.PARALLEL) {
		return p.parseCallParallelBody()
	}
	procNameTok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return nil, err
	}

	if _, err = p.expect(lexer.LPAREN); err != nil {
		return nil, err
	}
	var arguments []ast.Expr
	if !p.check(lexer.RPAREN) {
		arg, err := p.parseCallArgument()
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, arg)
		for p.check(lexer.COMMA) {
			p.advance()
			arg, err = p.parseCallArgument()
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, arg)
		}
	}
	if _, err = p.expect(lexer.RPAREN); err != nil {
		return nil, err
	}

	target := ""
	if p.check(lexer.INTO) {
		p.advance()
		if _, err = p.expect(lexer.AT); err != nil {
			return nil, err
		}
		targetTok, err := p.expectIdentifierOrKeyword()
		if err != nil {
			return nil, err
		}
		target = targetTok.Value
	}

	return &ast.CallStatement{
		ProcedureName:  procNameTok.Value,
		Arguments:      arguments,
		TargetVariable: target,
	}, nil
}

// =============================================================================
// SELECT ... INTO @var
// =============================================================================

func (p *Parser) parseSelectIntoStatement() (*ast.SelectIntoStatement, error) {
	selectItems, err := p.parseSelectClause()
	if err != nil {
		return nil, err
	}

	var fromClause *ast.FromClause
	if p.check(lexer.FROM) {
		fromClause, err = p.parseFromClause()
		if err != nil {
			return nil, err
		}
	}

	var whereClause *ast.WhereClause
	if p.check(lexer.WHERE) {
		whereClause, err = p.parseWhereClause()
		if err != nil {
			return nil, err
		}
	}

	target := ""
	if p.check(lexer.INTO) {
		p.advance()
		if _, err = p.expect(lexer.AT); err != nil {
			return nil, err
		}
		targetTok, err := p.expectIdentifierOrKeyword()
		if err != nil {
			return nil, err
		}
		target = targetTok.Value
	}

	return &ast.SelectIntoStatement{
		SelectItems:    selectItems,
		FromClause:     fromClause,
		WhereClause:    whereClause,
		TargetVariable: target,
	}, nil
}

// =============================================================================
// Metadata Blocks
// =============================================================================

func (p *Parser) parseSecurityBlock() (map[string]string, error) {
	if _, err := p.expect(lexer.SECURITY); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}
	security := make(map[string]string)

	for p.check(lexer.CLASSIFICATION) {
		p.advance()
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		valTok, err := p.expect(lexer.IDENTIFIER)
		if err != nil {
			return nil, err
		}
		security["classification"] = valTok.Value
	}
	return security, nil
}

func (p *Parser) parseAccountingBlock() (map[string]string, error) {
	if _, err := p.expect(lexer.ACCOUNTING); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}
	accounting := make(map[string]string)

	for p.check(lexer.IDENTIFIER) || p.check(lexer.STRING) {
		if p.current().Type == lexer.IDENTIFIER {
			key := p.advance().Value
			if _, err := p.expect(lexer.COLON); err != nil {
				return nil, err
			}
			if p.check(lexer.STRING) {
				accounting[strings.ToLower(key)] = p.advance().Value
			} else if p.check(lexer.INTEGER) {
				accounting[strings.ToLower(key)] = p.advance().Value
			} else if p.check(lexer.FLOAT) {
				accounting[strings.ToLower(key)] = p.advance().Value
			} else {
				break
			}
		} else {
			break
		}
	}
	return accounting, nil
}

func (p *Parser) parseLabelsBlock() (map[string]string, error) {
	if _, err := p.expect(lexer.LABELS); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.LBRACE); err != nil {
		return nil, err
	}

	labels := make(map[string]string)
	if !p.check(lexer.RBRACE) {
		keyTok, err := p.expect(lexer.STRING)
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		valTok, err := p.expect(lexer.STRING)
		if err != nil {
			return nil, err
		}
		labels[keyTok.Value] = valTok.Value

		for p.check(lexer.COMMA) {
			p.advance()
			if p.check(lexer.RBRACE) {
				break
			}
			keyTok, err = p.expect(lexer.STRING)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(lexer.COLON); err != nil {
				return nil, err
			}
			valTok, err = p.expect(lexer.STRING)
			if err != nil {
				return nil, err
			}
			labels[keyTok.Value] = valTok.Value
		}
	}

	if _, err := p.expect(lexer.RBRACE); err != nil {
		return nil, err
	}
	return labels, nil
}

// =============================================================================
// Expression Parsing
// =============================================================================

func (p *Parser) parseExpression() (ast.Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for p.checkAny(lexer.PLUS, lexer.MINUS, lexer.PIPE_PIPE) {
		op := p.advance().Value
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Left: left, Op: op, Right: right}
	}

	return left, nil
}

func (p *Parser) parsePrimary() (ast.Expr, error) {
	tok := p.current()

	// NOT <expr> — boolean negation
	if tok.Type == lexer.NOT {
		p.advance()
		operand, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryOp{Operator: "NOT", Operand: operand}, nil
	}

	// @param reference or storage subscript
	if tok.Type == lexer.AT {
		p.advance()
		nameTok, err := p.expectIdentifierOrKeyword()
		if err != nil {
			return nil, err
		}
		if p.check(lexer.LBRACKET) {
			p.advance()
			keyExpr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(lexer.RBRACKET); err != nil {
				return nil, err
			}
			return &ast.StorageSubscript{StorageVar: nameTok.Value, Key: keyExpr}, nil
		}
		return &ast.ParamRef{Name: nameTok.Value}, nil
	}

	// F-string literal
	if tok.Type == lexer.FSTRING {
		p.advance()
		return &ast.FStringLiteral{Template: tok.Value}, nil
	}

	// List literal
	if tok.Type == lexer.LBRACKET {
		p.advance()
		var elements []ast.Expr
		if !p.check(lexer.RBRACKET) {
			elem, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			elements = append(elements, elem)
			for p.check(lexer.COMMA) {
				p.advance()
				elem, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
				elements = append(elements, elem)
			}
		}
		if _, err := p.expect(lexer.RBRACKET); err != nil {
			return nil, err
		}
		return &ast.ListLiteral{Elements: elements}, nil
	}

	// Map literal: {} or {'key': value, ...}
	if tok.Type == lexer.LBRACE {
		p.advance() // consume {
		var pairs []ast.MapPair
		if !p.check(lexer.RBRACE) {
			key, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(lexer.COLON); err != nil {
				return nil, err
			}
			val, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, ast.MapPair{Key: key, Value: val})
			for p.check(lexer.COMMA) {
				p.advance()
				if p.check(lexer.RBRACE) {
					break // trailing comma
				}
				key, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
				if _, err = p.expect(lexer.COLON); err != nil {
					return nil, err
				}
				val, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
				pairs = append(pairs, ast.MapPair{Key: key, Value: val})
			}
		}
		if _, err := p.expect(lexer.RBRACE); err != nil {
			return nil, err
		}
		return &ast.MapLiteral{Pairs: pairs}, nil
	}

	// Boolean literals
	if tok.Type == lexer.TRUE {
		p.advance()
		return &ast.Literal{Value: "true", LitType: "bool"}, nil
	}
	if tok.Type == lexer.FALSE {
		p.advance()
		return &ast.Literal{Value: "false", LitType: "bool"}, nil
	}

	// String literal
	if tok.Type == lexer.STRING {
		p.advance()
		return &ast.Literal{Value: tok.Value, LitType: "string"}, nil
	}

	// Integer literal
	if tok.Type == lexer.INTEGER {
		p.advance()
		return &ast.Literal{Value: tok.Value, LitType: "integer"}, nil
	}

	// Float literal
	if tok.Type == lexer.FLOAT {
		p.advance()
		return &ast.Literal{Value: tok.Value, LitType: "float"}, nil
	}

	// Identifier (possibly dotted, possibly function call)
	if tok.Type == lexer.IDENTIFIER {
		return p.parseIdentifierExpression()
	}

	// Keywords used as identifiers in expression context
	keywordAsExpr := map[lexer.TokenType]bool{
		lexer.FORMAT:         true,
		lexer.MODEL:          true,
		lexer.RESULT:         true,
		lexer.VERSION:        true,
		lexer.SCHEMA:         true,
		lexer.ERROR:          true,
		lexer.OUTPUT:         true,
		lexer.INPUT:          true,
		lexer.TEMPERATURE:    true,
		lexer.PROMPT:         true,
		lexer.BUDGET:         true,
		lexer.TOKENS:         true,
		lexer.ORDER:          true,
		lexer.FROM:           true,
		lexer.WHERE:          true,
		lexer.WORKFLOW:       true,
		lexer.PROCEDURE:      true,
		lexer.DEFAULT:        true,
		lexer.SECURITY:       true,
		lexer.ACCOUNTING:     true,
		lexer.CLASSIFICATION: true,
	}
	if keywordAsExpr[tok.Type] {
		p.advance()
		return &ast.Identifier{Name: tok.Value}, nil
	}

	// Parenthesized expression
	if tok.Type == lexer.LPAREN {
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}
		return expr, nil
	}

	return nil, &ParseError{
		Message: fmt.Sprintf("Expected expression, got %d (%q)", tok.Type, tok.Value),
		Token:   tok,
	}
}

func (p *Parser) parseIdentifierExpression() (ast.Expr, error) {
	name := p.advance().Value

	// Function call: name(...)
	if p.check(lexer.LPAREN) {
		p.advance()
		var args []ast.Expr
		if !p.check(lexer.RPAREN) {
			arg, err := p.parseCallArgument()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			for p.check(lexer.COMMA) {
				p.advance()
				arg, err = p.parseCallArgument()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}
		}
		if _, err := p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}
		return &ast.FunctionCall{Name: name, Arguments: args}, nil
	}

	// Dotted name: name.field.subfield
	if p.check(lexer.DOT) {
		parts := []string{name}
		for p.check(lexer.DOT) {
			p.advance()
			partTok, err := p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			parts = append(parts, partTok.Value)
		}
		return &ast.DottedName{Parts: parts}, nil
	}

	return &ast.Identifier{Name: name}, nil
}

func (p *Parser) parseCallArgument() (ast.Expr, error) {
	if p.current().Type == lexer.IDENTIFIER &&
		p.pos+1 < len(p.tokens) &&
		p.tokens[p.pos+1].Type == lexer.EQ {
		name := p.advance().Value
		p.advance() // =
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &ast.NamedArg{Name: name, Value: value}, nil
	}
	return p.parseExpression()
}

// =============================================================================
// Token Helpers
// =============================================================================

func (p *Parser) current() lexer.Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return lexer.Token{Type: lexer.EOF, Value: ""}
}

func (p *Parser) peek(offset int) lexer.Token {
	idx := p.pos + offset
	if idx < len(p.tokens) {
		return p.tokens[idx]
	}
	return lexer.Token{Type: lexer.EOF, Value: ""}
}

func (p *Parser) advance() lexer.Token {
	tok := p.current()
	p.pos++
	return tok
}

func (p *Parser) check(t lexer.TokenType) bool {
	return p.current().Type == t
}

func (p *Parser) checkAny(types ...lexer.TokenType) bool {
	ct := p.current().Type
	for _, t := range types {
		if ct == t {
			return true
		}
	}
	return false
}

func (p *Parser) peekIs(t lexer.TokenType) bool {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1].Type == t
	}
	return false
}

func (p *Parser) expect(t lexer.TokenType) (lexer.Token, error) {
	tok := p.current()
	if tok.Type != t {
		return lexer.Token{}, &ParseError{
			Message: fmt.Sprintf("Expected %d, got %d (%q)", t, tok.Type, tok.Value),
			Token:   tok,
		}
	}
	return p.advance(), nil
}

func (p *Parser) expectIdentifierOrKeyword() (lexer.Token, error) {
	tok := p.current()
	if tok.Type == lexer.IDENTIFIER {
		return p.advance(), nil
	}
	// Accept keywords as identifiers in certain contexts
	keywordAsIdent := map[lexer.TokenType]bool{
		lexer.OUTPUT:      true,
		lexer.RESULT:      true,
		lexer.FORMAT:      true,
		lexer.MODEL:       true,
		lexer.VERSION:     true,
		lexer.SCHEMA:      true,
		lexer.ERROR:       true,
		lexer.STORE:       true,
		lexer.CACHE:       true,
		lexer.BUDGET:      true,
		lexer.LIMIT:       true,
		lexer.INPUT:       true,
		lexer.COMMIT:      true,
		lexer.GENERATE:    true,
		lexer.SELECT:      true,
		lexer.PROMPT:      true,
		lexer.ORDER:       true,
		lexer.FROM:        true,
		lexer.WHERE:       true,
		lexer.EXECUTE:     true,
		lexer.EXPLAIN:     true,
		lexer.HALLUCINATION: true,
		lexer.REFUSAL:     true,
		lexer.OVERFLOW:    true,
		lexer.ITERATIONS:  true,
		lexer.OTHERS:      true,
		lexer.TEMPERATURE: true,
		lexer.WORKFLOW:    true,
		lexer.PROCEDURE:   true,
		lexer.EVALUATE:    true,
		lexer.RETRY:       true,
		lexer.RAISE:       true,
		lexer.CALL:        true,
		lexer.DEFAULT:     true,
		lexer.INTO:        true,
		// SPL 3.0
		lexer.IMPORT:   true,
		lexer.PARALLEL: true,
		lexer.IMAGE:    true,
		lexer.AUDIO:    true,
		lexer.VIDEO:    true,
	}
	if keywordAsIdent[tok.Type] {
		return p.advance(), nil
	}
	return lexer.Token{}, &ParseError{
		Message: fmt.Sprintf("Expected identifier, got %d (%q)", tok.Type, tok.Value),
		Token:   tok,
	}
}

// =============================================================================
// SPL 3.0: IMPORT Statement
// =============================================================================

func (p *Parser) parseImportStatement() (*ast.ImportStatement, error) {
	if _, err := p.expect(lexer.IMPORT); err != nil {
		return nil, err
	}
	pathTok, err := p.expect(lexer.STRING)
	if err != nil {
		return nil, err
	}
	return &ast.ImportStatement{Path: pathTok.Value}, nil
}

// =============================================================================
// SPL 3.0: CALL PARALLEL Statement
// =============================================================================

// parseCallParallelBody parses the body of a CALL PARALLEL block.
// Called after CALL has been consumed; PARALLEL is consumed here.
//
// Syntax:
//
//	CALL PARALLEL
//	  workflow_a(@x, @y) INTO @result_a,
//	  workflow_b(@z)     INTO @result_b
//	END
//
// Commas between branches always appear AFTER the INTO clause (or after the
// closing paren when INTO is absent), so argument list parsing is identical to
// a regular CALL — no ambiguity.
func (p *Parser) parseCallParallelBody() (*ast.CallParallelStatement, error) {
	if _, err := p.expect(lexer.PARALLEL); err != nil {
		return nil, err
	}

	var branches []ast.CallBranch
	for !p.checkAny(lexer.END, lexer.EOF) {
		procNameTok, err := p.expect(lexer.IDENTIFIER)
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(lexer.LPAREN); err != nil {
			return nil, err
		}

		// Parse argument list — normal comma-separated args until ')'
		var arguments []ast.Expr
		if !p.check(lexer.RPAREN) {
			arg, err := p.parseCallArgument()
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, arg)
			for p.check(lexer.COMMA) {
				p.advance()
				arg, err = p.parseCallArgument()
				if err != nil {
					return nil, err
				}
				arguments = append(arguments, arg)
			}
		}
		if _, err = p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}

		// Optional INTO @var
		target := ""
		if p.check(lexer.INTO) {
			p.advance()
			if _, err = p.expect(lexer.AT); err != nil {
				return nil, err
			}
			targetTok, err := p.expectIdentifierOrKeyword()
			if err != nil {
				return nil, err
			}
			target = targetTok.Value
		}

		branches = append(branches, ast.CallBranch{
			ProcedureName:  procNameTok.Value,
			Arguments:      arguments,
			TargetVariable: target,
		})

		// Optional comma separating branches
		if p.check(lexer.COMMA) {
			p.advance()
		}
	}

	if _, err := p.expect(lexer.END); err != nil {
		return nil, err
	}
	return &ast.CallParallelStatement{Branches: branches}, nil
}

func (p *Parser) readModelName() (string, error) {
	parts := []string{}
	tok, err := p.expect(lexer.IDENTIFIER)
	if err != nil {
		return "", err
	}
	parts = append(parts, tok.Value)
	for p.check(lexer.MINUS) {
		p.advance()
		if p.check(lexer.IDENTIFIER) {
			parts = append(parts, "-"+p.advance().Value)
		} else if p.check(lexer.INTEGER) {
			parts = append(parts, "-"+p.advance().Value)
		} else if p.check(lexer.FLOAT) {
			parts = append(parts, "-"+p.advance().Value)
		} else {
			break
		}
	}
	return strings.Join(parts, ""), nil
}
