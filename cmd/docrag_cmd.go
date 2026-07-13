package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/digital-duck/spl20go/internal/config"
	"github.com/digital-duck/spl20go/internal/rag"
	"github.com/spf13/cobra"
)

var docRAGTopK int

var docRAGCmd = &cobra.Command{
	Use:   "doc-rag",
	Short: "Manage the Doc-RAG store (ChromaDB + Ollama embeddings)",
}

var docRAGAddCmd = &cobra.Command{
	Use:   "add <text_or_file>",
	Short: "Add text (or file contents) to the Doc-RAG store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		dr := rag.NewDocRAGStore(
			cfg.DocRAG.ChromaURL,
			cfg.DocRAG.OllamaURL,
			cfg.DocRAG.EmbedModel,
		)

		text := args[0]
		// If it looks like a readable file path, read its contents.
		if _, err := os.Stat(text); err == nil {
			extracted, err := readDocFile(text)
			if err != nil {
				return fmt.Errorf("doc-rag add: read file: %w", err)
			}
			text = extracted
		}

		n, err := dr.Add(context.Background(), text, map[string]string{})
		if err != nil {
			return fmt.Errorf("doc-rag add: %w", err)
		}
		fmt.Printf("doc-rag: added %d chunk(s)\n", n)
		return nil
	},
}

var docRAGQueryCmd = &cobra.Command{
	Use:   "query <search>",
	Short: "Query the Doc-RAG store for similar chunks",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		dr := rag.NewDocRAGStore(
			cfg.DocRAG.ChromaURL,
			cfg.DocRAG.OllamaURL,
			cfg.DocRAG.EmbedModel,
		)

		results, err := dr.Query(context.Background(), args[0], docRAGTopK)
		if err != nil {
			return fmt.Errorf("doc-rag query: %w", err)
		}
		if len(results) == 0 {
			fmt.Println("(no results)")
			return nil
		}
		for i, r := range results {
			fmt.Printf("--- Result %d (score: %s) ---\n%s\n\n", i+1, r["score"], r["text"])
		}
		return nil
	},
}

var docRAGCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Count indexed chunks in the Doc-RAG store",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		dr := rag.NewDocRAGStore(
			cfg.DocRAG.ChromaURL,
			cfg.DocRAG.OllamaURL,
			cfg.DocRAG.EmbedModel,
		)

		n, err := dr.Count(context.Background())
		if err != nil {
			return fmt.Errorf("doc-rag count: %w", err)
		}
		fmt.Printf("doc-rag: %d chunk(s) indexed\n", n)
		return nil
	},
}

// readDocFile reads a file, extracting text from PDFs via pdftotext (poppler).
// Falls back to raw bytes for all other file types.
func readDocFile(path string) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		// Try pdftotext (from poppler-utils) first
		if out, err := extractPDFText(path); err == nil {
			return out, nil
		}
		// Fall back: warn and read raw bytes (will contain binary noise)
		fmt.Fprintf(os.Stderr, "WARN: pdftotext not found — reading PDF as raw bytes. Install poppler-utils for proper extraction.\n")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// extractPDFText calls pdftotext -layout - <path> and returns stdout.
func extractPDFText(path string) (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func init() {
	docRAGQueryCmd.Flags().IntVar(&docRAGTopK, "top-k", 5, "Number of results to return")

	docRAGCmd.AddCommand(docRAGAddCmd)
	docRAGCmd.AddCommand(docRAGQueryCmd)
	docRAGCmd.AddCommand(docRAGCountCmd)
}
