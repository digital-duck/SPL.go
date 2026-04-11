package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/config"
	"github.com/digital-duck/spl20go/internal/executor"
	"github.com/digital-duck/spl20go/internal/lexer"
	"github.com/digital-duck/spl20go/internal/parser"
)

var runParams []string
var runWorkers int
var runPlan bool

var runCmd = &cobra.Command{
	Use:   "run <file.spl> [KEY=VALUE...]",
	Short: "Execute an SPL program",
	Long: `Execute an SPL file against the configured LLM adapter.

Parameters can be passed as KEY=VALUE arguments or via -p KEY=VALUE flags.

Examples:
  spl run my_workflow.spl topic="machine learning"
  spl run my_workflow.spl -p topic="machine learning" -m llama3.2
  spl run my_prompt.spl --adapter echo`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		// Parse params from positional args and -p flags
		params := make(map[string]string)
		for _, arg := range args[1:] {
			if idx := strings.Index(arg, "="); idx > 0 {
				key := arg[:idx]
				value := arg[idx+1:]
				params[key] = value
			}
		}
		for _, p := range runParams {
			if idx := strings.Index(p, "="); idx > 0 {
				key := p[:idx]
				value := p[idx+1:]
				params[key] = value
			}
		}

		// Read SPL file
		source, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("cannot read %q: %w", filename, err)
		}

		// Lex
		l := lexer.New(string(source))
		tokens, err := l.Tokenize()
		if err != nil {
			return fmt.Errorf("lexer error in %q: %w", filename, err)
		}

		// Parse
		p := parser.New(tokens)
		program, err := p.Parse()
		if err != nil {
			return fmt.Errorf("parse error in %q: %w", filename, err)
		}

		// Run Planner if requested
		if runPlan {
			planner := executor.NewPlanner(flagModel)
			planResult := planner.Plan(program)
			fmt.Println("PRE-EXECUTION PLAN")
			fmt.Println(separator)
			fmt.Print(planResult.Summary())
			fmt.Println(separator)
			fmt.Println()
		}

		// Select adapter
		cfg, _ := config.Load()
		adapterName := flagAdapter
		if adapterName == "" {
			adapterName = cfg.Adapter
		}
		if adapterName == "" {
			adapterName = "ollama"
		}

		adapterCfg := cfg.AdapterConfig(adapterName)
		if flagModel != "" {
			adapterCfg["model"] = flagModel
		} else if cfg.Model != "" {
			adapterCfg["model"] = cfg.Model
		}

		adp, err := adapter.New(adapterName, adapterCfg)
		if err != nil {
			return fmt.Errorf("adapter %q: %w", adapterName, err)
		}

		// Execute
		exec := executor.New(adp)
		exec.MaxWorkers = runWorkers
		exec.SourceDir = filepath.Dir(filename)
		ctx := context.Background()
		results, err := exec.ExecuteProgram(ctx, program, params)
		if err != nil {
			return fmt.Errorf("execution error: %w", err)
		}

		// Print results
		for _, res := range results {
			switch r := res.(type) {
			case *executor.SPLResult:
				printSPLResult(r)
			case *executor.WorkflowResult:
				printWorkflowResult(r)
			}
		}

		return nil
	},
}

func init() {
	runCmd.Flags().StringArrayVarP(&runParams, "param", "p", nil, "Parameter as KEY=VALUE (repeatable)")
	runCmd.Flags().IntVar(&runWorkers, "workers", 0, "Number of parallel workers for independent workflow steps (0 = sequential)")
	runCmd.Flags().BoolVar(&runPlan, "plan", false, "Show pre-execution plan and resource estimates")
}

const separator = "============================================================"
const dashedLine = "------------------------------------------------------------"

func printSPLResult(r *executor.SPLResult) {
	fmt.Println(separator)
	fmt.Printf("Model: %s\n", r.Model)
	fmt.Printf("Tokens: %d in / %d out\n", r.InputTokens, r.OutputTokens)
	fmt.Printf("Latency: %.0fms\n", r.LatencyMs)
	fmt.Printf("Cost: $%.6f\n", r.CostUSD)
	fmt.Println(dashedLine)
	fmt.Println(r.Content)
	fmt.Println(separator)
}

func printWorkflowResult(r *executor.WorkflowResult) {
	fmt.Println(separator)
	fmt.Printf("Workflow Status: %s\n", r.Status)
	fmt.Printf("LLM Calls: %d | Tokens: %d in / %d out\n", r.TotalLLMCalls, r.TotalInputToks, r.TotalOutputToks)
	fmt.Printf("Latency: %.0fms | Cost: $%.6f\n", r.TotalLatencyMs, r.TotalCostUSD)
	fmt.Println(dashedLine)

	if r.CommittedValue != "" {
		fmt.Println("Committed Output:")
		fmt.Println(r.CommittedValue)
		if len(r.CommittedOpts) > 0 {
			fmt.Println("\nCommit Options:")
			for k, v := range r.CommittedOpts {
				fmt.Printf("  %s = %s\n", k, v)
			}
		}
	} else if len(r.Output) > 0 {
		fmt.Println("Output Variables:")
		for k, v := range r.Output {
			if len(v) > 200 {
				fmt.Printf("  @%s = %s...\n", k, v[:200])
			} else {
				fmt.Printf("  @%s = %s\n", k, v)
			}
		}
	}
	fmt.Println(separator)
}
