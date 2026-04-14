// Package lexer implements the SPL 2.0 tokenizer.
package lexer

import "fmt"

// TokenType enumerates every token kind in SPL 2.0.
type TokenType int

const (
	// SPL 1.0 keywords
	PROMPT TokenType = iota
	WITH
	BUDGET
	TOKENS
	USING
	MODEL
	SELECT
	AS
	LIMIT
	WHERE
	AND
	OR
	NOT
	IN
	ORDER
	BY
	ASC
	DESC
	GENERATE
	OUTPUT
	CREATE
	FUNCTION
	RETURNS
	EXPLAIN
	EXECUTE
	PARAMS
	STORE
	RESULT
	CACHE
	FOR
	FROM
	TEMPERATURE
	FORMAT
	BEGIN
	COMMIT
	ROLLBACK
	TRANSACTION
	ON
	ERROR
	AUTO_COMPRESS
	COMPRESSION_STRATEGY
	SCHEMA
	VERSION
	REFRESH
	EVERY
	MATERIALIZED
	GRID
	VRAM

	// SPL 2.0 keywords
	EVALUATE
	WHEN
	THEN
	WHILE
	DO
	END
	EXCEPTION
	WORKFLOW
	INPUT
	PROCEDURE
	RETRY
	RAISE
	ELSE
	INTO
	CALL
	DEFAULT
	SET
	LOGGING
	TO
	LEVEL
	TRUE
	FALSE
	FSTRING
	PIPE
	PIPE_PIPE
	SECURITY
	ACCOUNTING
	CLASSIFICATION
	LABELS
	HALLUCINATION
	REFUSAL
	OVERFLOW
	ITERATIONS
	OTHERS

	// SPL 3.0 keywords
	IMPORT
	PARALLEL
	IMAGE
	AUDIO
	VIDEO

	// Literals
	INTEGER
	FLOAT
	STRING
	IDENTIFIER

	// Operators
	DOT
	COMMA
	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACKET
	RBRACKET
	EQ
	NEQ
	GT
	LT
	GTE
	LTE
	STAR
	PLUS
	MINUS
	AT
	ASSIGN // :=
	COLON  // :
	PERCENT
	SEMICOLON
	DOLLAR_DOLLAR

	EOF
)

// keywords maps lowercase keyword strings to token types.
var keywords = map[string]TokenType{
	// SPL 1.0
	"prompt":               PROMPT,
	"with":                 WITH,
	"budget":               BUDGET,
	"tokens":               TOKENS,
	"using":                USING,
	"model":                MODEL,
	"select":               SELECT,
	"as":                   AS,
	"limit":                LIMIT,
	"where":                WHERE,
	"and":                  AND,
	"or":                   OR,
	"not":                  NOT,
	"in":                   IN,
	"order":                ORDER,
	"by":                   BY,
	"asc":                  ASC,
	"desc":                 DESC,
	"generate":             GENERATE,
	"output":               OUTPUT,
	"create":               CREATE,
	"function":             FUNCTION,
	"returns":              RETURNS,
	"explain":              EXPLAIN,
	"execute":              EXECUTE,
	"params":               PARAMS,
	"store":                STORE,
	"result":               RESULT,
	"cache":                CACHE,
	"for":                  FOR,
	"from":                 FROM,
	"temperature":          TEMPERATURE,
	"format":               FORMAT,
	"begin":                BEGIN,
	"commit":               COMMIT,
	"return":               COMMIT, // SPL 3.0 alias: RETURN @var = COMMIT @var in workflow body
	"rollback":             ROLLBACK,
	"transaction":          TRANSACTION,
	"on":                   ON,
	"error":                ERROR,
	"auto_compress":        AUTO_COMPRESS,
	"compression_strategy": COMPRESSION_STRATEGY,
	"schema":               SCHEMA,
	"version":              VERSION,
	"refresh":              REFRESH,
	"every":                EVERY,
	"materialized":         MATERIALIZED,
	"grid":                 GRID,
	"vram":                 VRAM,
	// SPL 2.0
	"evaluate":       EVALUATE,
	"when":           WHEN,
	"then":           THEN,
	"while":          WHILE,
	"do":             DO,
	"end":            END,
	"exception":      EXCEPTION,
	"workflow":       WORKFLOW,
	"input":          INPUT,
	"procedure":      PROCEDURE,
	"retry":          RETRY,
	"raise":          RAISE,
	"else":           ELSE,
	"otherwise":      ELSE, // backward-compat alias
	"into":           INTO,
	"call":           CALL,
	"default":        DEFAULT,
	"set":            SET,
	"logging":        LOGGING,
	"to":             TO,
	"level":          LEVEL,
	"true":           TRUE,
	"false":          FALSE,
	"security":       SECURITY,
	"accounting":     ACCOUNTING,
	"classification": CLASSIFICATION,
	"labels":         LABELS,
	"hallucination":  HALLUCINATION,
	"refusal":        REFUSAL,
	"overflow":       OVERFLOW,
	"iterations":     ITERATIONS,
	"others":         OTHERS,
	// SPL 3.0
	"import":   IMPORT,
	"parallel": PARALLEL,
	"image":    IMAGE,
	"audio":    AUDIO,
	"video":    VIDEO,
}

// Token is a single lexical token.
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q, %d:%d)", t.Type, t.Value, t.Line, t.Column)
}

// LexerError is returned when the lexer encounters invalid input.
type LexerError struct {
	Message string
	Line    int
	Column  int
}

func (e *LexerError) Error() string {
	return fmt.Sprintf("Lexer error at %d:%d: %s", e.Line, e.Column, e.Message)
}

// Lexer tokenizes SPL 2.0 source code.
type Lexer struct {
	source []rune
	pos    int
	line   int
	column int
	tokens []Token
}

// New creates a new Lexer for the given source string.
func New(source string) *Lexer {
	return &Lexer{
		source: []rune(source),
		pos:    0,
		line:   1,
		column: 1,
	}
}

// Tokenize scans the entire source and returns the token stream.
func (l *Lexer) Tokenize() ([]Token, error) {
	for l.pos < len(l.source) {
		l.skipWhitespaceAndComments()
		if l.pos >= len(l.source) {
			break
		}

		ch := l.source[l.pos]

		switch {
		case ch == '$' && l.peek(1) == '$':
			if err := l.readDollarDollar(); err != nil {
				return nil, err
			}
		case ch == '"' || ch == '\'':
			if err := l.readString(ch); err != nil {
				return nil, err
			}
		case (ch == 'f' || ch == 'F') && (l.peek(1) == '"' || l.peek(1) == '\''):
			if err := l.readFString(); err != nil {
				return nil, err
			}
		case ch >= '0' && ch <= '9':
			l.readNumber()
		case isLetter(ch):
			l.readIdentifier()
		case ch == '.':
			l.emit(DOT, ".")
			l.advance()
		case ch == ',':
			l.emit(COMMA, ",")
			l.advance()
		case ch == '(':
			l.emit(LPAREN, "(")
			l.advance()
		case ch == ')':
			l.emit(RPAREN, ")")
			l.advance()
		case ch == '{':
			l.emit(LBRACE, "{")
			l.advance()
		case ch == '}':
			l.emit(RBRACE, "}")
			l.advance()
		case ch == '[':
			l.emit(LBRACKET, "[")
			l.advance()
		case ch == ']':
			l.emit(RBRACKET, "]")
			l.advance()
		case ch == ';':
			l.emit(SEMICOLON, ";")
			l.advance()
		case ch == '*':
			l.emit(STAR, "*")
			l.advance()
		case ch == '+':
			l.emit(PLUS, "+")
			l.advance()
		case ch == '-':
			l.emit(MINUS, "-")
			l.advance()
		case ch == '@':
			l.emit(AT, "@")
			l.advance()
		case ch == '%':
			l.emit(PERCENT, "%")
			l.advance()
		case ch == ':':
			if l.peek(1) == '=' {
				l.emit(ASSIGN, ":=")
				l.advance()
				l.advance()
			} else {
				l.emit(COLON, ":")
				l.advance()
			}
		case ch == '=':
			l.emit(EQ, "=")
			l.advance()
		case ch == '!':
			if l.peek(1) == '=' {
				l.emit(NEQ, "!=")
				l.advance()
				l.advance()
			} else {
				return nil, &LexerError{Message: "Unexpected character '!'", Line: l.line, Column: l.column}
			}
		case ch == '>':
			if l.peek(1) == '=' {
				l.emit(GTE, ">=")
				l.advance()
				l.advance()
			} else {
				l.emit(GT, ">")
				l.advance()
			}
		case ch == '<':
			if l.peek(1) == '=' {
				l.emit(LTE, "<=")
				l.advance()
				l.advance()
			} else {
				l.emit(LT, "<")
				l.advance()
			}
		case ch == '|':
			if l.peek(1) == '|' {
				l.emit(PIPE_PIPE, "||")
				l.advance()
				l.advance()
			} else {
				l.emit(PIPE, "|")
				l.advance()
			}
		default:
			return nil, &LexerError{
				Message: fmt.Sprintf("Unexpected character %q", ch),
				Line:    l.line,
				Column:  l.column,
			}
		}
	}

	l.emit(EOF, "")
	return l.tokens, nil
}

func (l *Lexer) advance() rune {
	ch := l.source[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return ch
}

func (l *Lexer) peek(offset int) rune {
	idx := l.pos + offset
	if idx < len(l.source) {
		return l.source[idx]
	}
	return 0
}

func (l *Lexer) emit(t TokenType, value string) {
	l.tokens = append(l.tokens, Token{Type: t, Value: value, Line: l.line, Column: l.column})
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
		} else if ch == '-' && l.peek(1) == '-' {
			// Line comment: skip until end of line
			for l.pos < len(l.source) && l.source[l.pos] != '\n' {
				l.advance()
			}
		} else {
			break
		}
	}
}

func (l *Lexer) readString(quote rune) error {
	startLine := l.line
	startCol := l.column
	l.advance() // skip opening quote

	var chars []rune
	escapeMap := map[rune]rune{'n': '\n', 't': '\t', '\\': '\\', '\'': '\'', '"': '"'}

	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch == '\\' {
			l.advance()
			if l.pos < len(l.source) {
				escaped := l.source[l.pos]
				if r, ok := escapeMap[escaped]; ok {
					chars = append(chars, r)
				} else {
					chars = append(chars, escaped)
				}
				l.advance()
			}
		} else if ch == quote {
			l.advance() // skip closing quote
			l.tokens = append(l.tokens, Token{Type: STRING, Value: string(chars), Line: startLine, Column: startCol})
			return nil
		} else {
			chars = append(chars, ch)
			l.advance()
		}
	}
	return &LexerError{Message: "Unterminated string literal", Line: startLine, Column: startCol}
}

func (l *Lexer) readFString() error {
	startLine := l.line
	startCol := l.column
	l.advance() // skip 'f' prefix
	quote := l.source[l.pos]
	l.advance() // skip opening quote

	var chars []rune
	escapeMap := map[rune]rune{'n': '\n', 't': '\t', '\\': '\\', '\'': '\'', '"': '"'}

	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch == '\\' {
			l.advance()
			if l.pos < len(l.source) {
				escaped := l.source[l.pos]
				if r, ok := escapeMap[escaped]; ok {
					chars = append(chars, r)
				} else {
					chars = append(chars, escaped)
				}
				l.advance()
			}
		} else if ch == quote {
			l.advance()
			l.tokens = append(l.tokens, Token{Type: FSTRING, Value: string(chars), Line: startLine, Column: startCol})
			return nil
		} else {
			chars = append(chars, ch)
			l.advance()
		}
	}
	return &LexerError{Message: "Unterminated f-string literal", Line: startLine, Column: startCol}
}

func (l *Lexer) readNumber() {
	startLine := l.line
	startCol := l.column
	var chars []rune
	isFloat := false

	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch >= '0' && ch <= '9' {
			chars = append(chars, ch)
			l.advance()
		} else if ch == '.' && !isFloat {
			// Only treat as float if next char is a digit
			if l.peek(1) >= '0' && l.peek(1) <= '9' {
				isFloat = true
				chars = append(chars, ch)
				l.advance()
			} else {
				break
			}
		} else {
			break
		}
	}

	tt := INTEGER
	if isFloat {
		tt = FLOAT
	}
	l.tokens = append(l.tokens, Token{Type: tt, Value: string(chars), Line: startLine, Column: startCol})
}

func (l *Lexer) readIdentifier() {
	startLine := l.line
	startCol := l.column
	var chars []rune

	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if isLetter(ch) || (ch >= '0' && ch <= '9') {
			chars = append(chars, ch)
			l.advance()
		} else {
			break
		}
	}

	value := string(chars)
	lower := toLower(value)
	tt, ok := keywords[lower]
	if !ok {
		tt = IDENTIFIER
	}
	l.tokens = append(l.tokens, Token{Type: tt, Value: value, Line: startLine, Column: startCol})
}

func (l *Lexer) readDollarDollar() error {
	startLine := l.line
	startCol := l.column
	l.advance() // skip first $
	l.advance() // skip second $
	l.tokens = append(l.tokens, Token{Type: DOLLAR_DOLLAR, Value: "$$", Line: startLine, Column: startCol})

	bodyLine := l.line
	bodyCol := l.column
	var bodyChars []rune

	for l.pos < len(l.source) {
		if l.source[l.pos] == '$' && l.peek(1) == '$' {
			l.tokens = append(l.tokens, Token{Type: STRING, Value: string(bodyChars), Line: bodyLine, Column: bodyCol})
			l.advance() // skip first closing $
			l.advance() // skip second closing $
			return nil
		}
		bodyChars = append(bodyChars, l.source[l.pos])
		l.advance()
	}
	return &LexerError{Message: "Unterminated $$ string", Line: startLine, Column: startCol}
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
