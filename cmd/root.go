// Package cmd implements the SPL 2.0 CLI using cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagAdapter string
	flagModel   string
	flagVerbose bool
)

var rootCmd = &cobra.Command{
	Use:   "spl",
	Short: "SPL 2.0 — Semantic Prompt Language runtime (Go)",
	Long: `SPL 2.0 is an agentic workflow orchestration language for LLMs.

Run SPL programs against local (Ollama) or distributed (Momagrid) models.`,
}

// Execute is the entry point for the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagAdapter, "adapter", "a", "", "LLM adapter (echo, ollama, momagrid)")
	rootCmd.PersistentFlags().StringVarP(&flagModel, "model", "m", "", "LLM model name")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(adaptersCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(docRAGCmd)
	rootCmd.AddCommand(codeRAGCmd)
	rootCmd.AddCommand(text2splCmd)
}
