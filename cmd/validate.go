package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/digital-duck/spl20go/internal/analyzer"
	"github.com/digital-duck/spl20go/internal/lexer"
	"github.com/digital-duck/spl20go/internal/parser"
)

var validateCmd = &cobra.Command{
	Use:   "validate <file.spl>",
	Short: "Validate an SPL file for syntax errors",
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
			fmt.Fprintf(os.Stderr, "✗ Lexer error in %s: %v\n", filename, err)
			os.Exit(1)
		}

		p := parser.New(tokens)
		program, err := p.Parse()
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Parse error in %s: %v\n", filename, err)
			os.Exit(1)
		}

		// Semantic analysis
		a := analyzer.New()
		result, analysisErr := a.Analyze(program)
		if analysisErr != nil {
			fmt.Fprintf(os.Stderr, "✗ Semantic error in %s: %v\n", filename, analysisErr)
			os.Exit(1)
		}

		// Print any warnings
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "WARNING: %s\n", w.Message)
		}

		fmt.Printf("✓ Valid: %s\n", filename)
		return nil
	},
}
