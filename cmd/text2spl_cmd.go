package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/config"
	"github.com/digital-duck/spl20go/internal/rag"
	"github.com/digital-duck/spl20go/internal/text2spl"
	"github.com/spf13/cobra"
)

var (
	text2splMode       string
	text2splOutput     string
	text2splExecute    bool
	text2splAdapter    string
	text2splModel      string
	text2splNoCodeRAG  bool
	text2splNoValidate bool
)

var text2splCmd = &cobra.Command{
	Use:   "text2spl <description>",
	Short: "Compile a natural language description to SPL 2.0 source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		description := args[0]

		cfg, _ := config.Load()

		// Determine adapter
		adapterName := text2splAdapter
		if adapterName == "" {
			adapterName = cfg.Text2SPL.Adapter
		}
		model := text2splModel
		if model == "" {
			model = cfg.Text2SPL.Model
		}

		adpCfg := cfg.AdapterConfig(adapterName)
		if model != "" {
			adpCfg["default_model"] = model
		}
		adp, err := adapter.New(adapterName, adpCfg)
		if err != nil {
			return fmt.Errorf("text2spl: %w", err)
		}

		// Determine mode
		mode := text2splMode
		if mode == "" {
			mode = cfg.Text2SPL.Mode
		}

		// Set up CodeRAG if enabled
		var codeStore *rag.CodeRAGStore
		if !text2splNoCodeRAG && cfg.CodeRAG.Enabled {
			codeStore = rag.NewCodeRAGStore(
				cfg.CodeRAG.ChromaURL,
				cfg.CodeRAG.OllamaURL,
				cfg.CodeRAG.EmbedModel,
			)
		}

		maxRetries := cfg.Text2SPL.MaxRetries
		if text2splNoValidate {
			maxRetries = 0
		}

		compiler := text2spl.New(adp, codeStore, maxRetries)
		splSource, err := compiler.Compile(context.Background(), description, mode)
		if err != nil {
			return fmt.Errorf("text2spl: %w", err)
		}

		// Output
		if text2splOutput != "" {
			if err := os.WriteFile(text2splOutput, []byte(splSource+"\n"), 0644); err != nil {
				return fmt.Errorf("text2spl: write output: %w", err)
			}
			fmt.Fprintf(os.Stderr, "text2spl: wrote %s\n", text2splOutput)
		} else {
			fmt.Println(splSource)
		}

		// Optionally execute
		if text2splExecute {
			fmt.Fprintln(os.Stderr, "text2spl: --execute not yet implemented; save to file and run: spl run <file.spl>")
		}

		return nil
	},
}

func init() {
	text2splCmd.Flags().StringVar(&text2splMode, "mode", "", "Generation mode: auto, prompt, workflow (default: auto)")
	text2splCmd.Flags().StringVarP(&text2splOutput, "output", "o", "", "Write generated SPL to file")
	text2splCmd.Flags().BoolVar(&text2splExecute, "execute", false, "Execute the generated SPL immediately")
	text2splCmd.Flags().StringVar(&text2splAdapter, "adapter", "", "LLM adapter to use for generation")
	text2splCmd.Flags().StringVarP(&text2splModel, "model", "m", "", "LLM model to use for generation")
	text2splCmd.Flags().BoolVar(&text2splNoCodeRAG, "no-code-rag", false, "Disable Code-RAG example injection")
	text2splCmd.Flags().BoolVar(&text2splNoValidate, "no-validate", false, "Skip SPL validation (no retry on parse error)")
}
