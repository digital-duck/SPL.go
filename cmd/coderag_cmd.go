package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/digital-duck/spl20go/internal/config"
	"github.com/digital-duck/spl20go/internal/rag"
	"github.com/spf13/cobra"
)

var (
	codeRAGCookbookDir  string
	codeRAGTopK         int
	codeRAGShowSPL      bool
	codeRAGExportOutput string
)

var codeRAGCmd = &cobra.Command{
	Use:   "code-rag",
	Short: "Manage the Code-RAG store (description→SPL pairs via ChromaDB)",
}

var codeRAGImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Index cookbook *.spl recipes into the Code-RAG store",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		store := rag.NewCodeRAGStore(
			cfg.CodeRAG.ChromaURL,
			cfg.CodeRAG.OllamaURL,
			cfg.CodeRAG.EmbedModel,
		)
		n, err := store.IndexCookbook(context.Background(), codeRAGCookbookDir)
		if err != nil {
			return fmt.Errorf("code-rag import: %w", err)
		}
		fmt.Printf("code-rag: indexed %d recipe(s) from %s\n", n, codeRAGCookbookDir)
		return nil
	},
}

var codeRAGAddCmd = &cobra.Command{
	Use:   "add <description> <file.spl>",
	Short: "Add a (description, SPL source) pair to the Code-RAG store",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		description := args[0]
		splPath := args[1]

		data, err := os.ReadFile(splPath)
		if err != nil {
			return fmt.Errorf("code-rag add: read %s: %w", splPath, err)
		}

		cfg, _ := config.Load()
		store := rag.NewCodeRAGStore(
			cfg.CodeRAG.ChromaURL,
			cfg.CodeRAG.OllamaURL,
			cfg.CodeRAG.EmbedModel,
		)
		meta := map[string]string{"filename": splPath}
		if err := store.AddPair(context.Background(), description, string(data), meta); err != nil {
			return fmt.Errorf("code-rag add: %w", err)
		}
		fmt.Printf("code-rag: added pair for %q\n", description)
		return nil
	},
}

var codeRAGQueryCmd = &cobra.Command{
	Use:   "query <description>",
	Short: "Retrieve similar (description, SPL) pairs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		store := rag.NewCodeRAGStore(
			cfg.CodeRAG.ChromaURL,
			cfg.CodeRAG.OllamaURL,
			cfg.CodeRAG.EmbedModel,
		)
		results, err := store.Retrieve(context.Background(), args[0], codeRAGTopK)
		if err != nil {
			return fmt.Errorf("code-rag query: %w", err)
		}
		if len(results) == 0 {
			fmt.Println("(no results)")
			return nil
		}
		for i, r := range results {
			fmt.Printf("--- Result %d (score: %s) ---\n", i+1, r["score"])
			fmt.Printf("Description: %s\n", r["description"])
			if codeRAGShowSPL {
				fmt.Printf("SPL Source:\n%s\n", r["spl_source"])
			}
			fmt.Println()
		}
		return nil
	},
}

var codeRAGCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Count indexed (description, SPL) pairs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		store := rag.NewCodeRAGStore(
			cfg.CodeRAG.ChromaURL,
			cfg.CodeRAG.OllamaURL,
			cfg.CodeRAG.EmbedModel,
		)
		n, err := store.Count(context.Background())
		if err != nil {
			return fmt.Errorf("code-rag count: %w", err)
		}
		fmt.Printf("code-rag: %d pair(s) indexed\n", n)
		return nil
	},
}

var codeRAGExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all Code-RAG pairs as JSONL for fine-tuning",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		store := rag.NewCodeRAGStore(
			cfg.CodeRAG.ChromaURL,
			cfg.CodeRAG.OllamaURL,
			cfg.CodeRAG.EmbedModel,
		)

		// Retrieve a large number of pairs (use a very large top_k to get all)
		results, err := store.Retrieve(context.Background(), "", 100000)
		if err != nil {
			// If retrieve with empty query fails, try count to determine existence
			n, countErr := store.Count(context.Background())
			if countErr != nil {
				return fmt.Errorf("code-rag export: %w", err)
			}
			if n == 0 {
				fmt.Println("code-rag export: no pairs indexed")
				return nil
			}
			return fmt.Errorf("code-rag export: %w", err)
		}

		outPath := codeRAGExportOutput
		if outPath == "" {
			outPath = "code_rag_export.jsonl"
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("code-rag export: create output file: %w", err)
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		for _, r := range results {
			entry := map[string]string{
				"description": r["description"],
				"spl_source":  r["spl_source"],
			}
			if err := enc.Encode(entry); err != nil {
				return fmt.Errorf("code-rag export: encode: %w", err)
			}
		}

		fmt.Printf("Exported %d pairs to %s\n", len(results), outPath)
		return nil
	},
}

var codeRAGParseLogCmd = &cobra.Command{
	Use:   "parse-log <run_summary.md>",
	Short: "Extract (description, SPL) pairs from a run_all.py markdown log file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logFile := args[0]
		cfg, _ := config.Load()
		store := rag.NewCodeRAGStore(
			cfg.CodeRAG.ChromaURL,
			cfg.CodeRAG.OllamaURL,
			cfg.CodeRAG.EmbedModel,
		)

		f, err := os.Open(logFile)
		if err != nil {
			return fmt.Errorf("code-rag parse-log: open %s: %w", logFile, err)
		}
		defer f.Close()

		type pair struct {
			name        string
			description string
		}

		var pairs []pair
		var currentName string
		var descLines []string
		inDesc := false

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()

			// Match "## Recipe NN — <name>" or "## <name>"
			if strings.HasPrefix(line, "## ") {
				// Save previous pair if any
				if currentName != "" && len(descLines) > 0 {
					desc := strings.TrimSpace(strings.Join(descLines, " "))
					if desc != "" {
						pairs = append(pairs, pair{name: currentName, description: desc})
					}
				}
				// Parse new name
				name := strings.TrimPrefix(line, "## ")
				// Strip "Recipe NN — " prefix if present
				if idx := strings.Index(name, " — "); idx >= 0 {
					name = strings.TrimSpace(name[idx+3:])
				}
				currentName = strings.TrimSpace(name)
				descLines = nil
				inDesc = true
				continue
			}

			// Match "### <RecipeName>"
			if strings.HasPrefix(line, "### ") {
				if currentName != "" && len(descLines) > 0 {
					desc := strings.TrimSpace(strings.Join(descLines, " "))
					if desc != "" {
						pairs = append(pairs, pair{name: currentName, description: desc})
					}
				}
				currentName = strings.TrimSpace(strings.TrimPrefix(line, "### "))
				descLines = nil
				inDesc = true
				continue
			}

			// Accumulate description lines (until next heading or blank after content)
			if inDesc && currentName != "" {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					if len(descLines) > 0 {
						inDesc = false // stop at first blank line after content
					}
					continue
				}
				// Skip markdown code blocks, table rows, etc.
				if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "#") {
					inDesc = false
					continue
				}
				descLines = append(descLines, trimmed)
			}
		}

		// Save last pair
		if currentName != "" && len(descLines) > 0 {
			desc := strings.TrimSpace(strings.Join(descLines, " "))
			if desc != "" {
				pairs = append(pairs, pair{name: currentName, description: desc})
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("code-rag parse-log: scan: %w", err)
		}

		// Add pairs to store
		count := 0
		for _, p := range pairs {
			meta := map[string]string{
				"source": "parse-log",
				"file":   logFile,
			}
			// Use name as spl_source placeholder since we don't have actual SPL here
			if err := store.AddPair(context.Background(), p.description, p.name, meta); err != nil {
				fmt.Fprintf(os.Stderr, "code-rag parse-log: add %q: %v\n", p.name, err)
				continue
			}
			count++
		}

		fmt.Printf("Added %d pairs from %s\n", count, logFile)
		return nil
	},
}

func init() {
	codeRAGImportCmd.Flags().StringVar(&codeRAGCookbookDir, "cookbook-dir", "./cookbook", "Directory containing *.spl recipe files")
	codeRAGQueryCmd.Flags().IntVar(&codeRAGTopK, "top-k", 4, "Number of results to return")
	codeRAGQueryCmd.Flags().BoolVar(&codeRAGShowSPL, "show-spl", false, "Print full SPL source for each result")
	codeRAGExportCmd.Flags().StringVarP(&codeRAGExportOutput, "output", "o", "code_rag_export.jsonl", "Output JSONL file path")

	codeRAGCmd.AddCommand(codeRAGImportCmd)
	codeRAGCmd.AddCommand(codeRAGAddCmd)
	codeRAGCmd.AddCommand(codeRAGQueryCmd)
	codeRAGCmd.AddCommand(codeRAGCountCmd)
	codeRAGCmd.AddCommand(codeRAGExportCmd)
	codeRAGCmd.AddCommand(codeRAGParseLogCmd)
}
