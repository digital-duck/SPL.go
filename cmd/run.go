package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/digital-duck/spl20go/internal/adapter"
	"github.com/digital-duck/spl20go/internal/config"
	"github.com/digital-duck/spl20go/internal/executor"
	"github.com/digital-duck/spl20go/internal/lexer"
	"github.com/digital-duck/spl20go/internal/parser"
	"github.com/digital-duck/spl20go/internal/tools"
)

var runParams []string
var runWorkers int
var runPlan bool
var runToolsFile string
var runKernel bool
var runKernelProtocol string
var runKernelName string
var runAllowedTools []string

var runCmd = &cobra.Command{
	Use:   "run <file.spl> [KEY=VALUE...]",
	Short: "Execute an SPL program",
	Long: `Execute an SPL file against the configured LLM adapter.

Parameters can be passed as KEY=VALUE arguments or via -p KEY=VALUE flags.

Examples:
  spl run my_workflow.spl topic="machine learning"
  spl run my_workflow.spl -p topic="machine learning" --model gemma3
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
		if len(runAllowedTools) > 0 {
			adapterCfg["allowed_tools"] = strings.Join(runAllowedTools, ",")
		}

		adp, err := adapter.New(adapterName, adapterCfg)
		if err != nil {
			return fmt.Errorf("adapter %q: %w", adapterName, err)
		}

		// Execute
		exec := executor.New(adp)
		exec.MaxWorkers = runWorkers
		exec.SourceDir = filepath.Dir(filename)
		exec.KernelEnabled = runKernel
		exec.KernelProtocol = runKernelProtocol
		exec.KernelName = runKernelName
		if runKernel {
			defer exec.CloseKernel()
		}

		// Load Python tools if --tools was specified
		if runToolsFile != "" {
			toolMap, err := tools.Load(runToolsFile)
			if err != nil {
				return fmt.Errorf("--tools: %w", err)
			}
			for name, fn := range toolMap {
				exec.Tools[name] = fn
			}
			fmt.Printf("Tools: loaded %d function(s) from %s\n", len(toolMap), runToolsFile)
		}

		ctx := context.Background()
		results, err := exec.ExecuteProgram(ctx, program, params)
		if err != nil {
			return fmt.Errorf("execution error: %w", err)
		}

		// Print and log results
		for _, res := range results {
			switch r := res.(type) {
			case *executor.SPLResult:
				printSPLResult(r)
				if logFile, err := logSPLResult(filename, adapterName, string(source), r); err == nil {
					fmt.Printf("Log: %s\n", logFile)
				}
			case *executor.WorkflowResult:
				printWorkflowResult(r)
				if logFile, err := logWorkflowResult(filename, adapterName, string(source), r); err == nil {
					fmt.Printf("Log: %s\n", logFile)
				}
			}
		}

		return nil
	},
}

func init() {
	runCmd.Flags().StringArrayVarP(&runParams, "param", "p", nil, "Parameter as KEY=VALUE (repeatable)")
	runCmd.Flags().IntVar(&runWorkers, "workers", 0, "Number of parallel workers for independent workflow steps (0 = sequential)")
	runCmd.Flags().BoolVar(&runPlan, "plan", false, "Show pre-execution plan and resource estimates")
	runCmd.Flags().StringVar(&runToolsFile, "tools", "", "Path to Python tools file (.py) — registers @spl_tool functions as CALL-able tools")
	runCmd.Flags().StringArrayVar(&runAllowedTools, "allowed-tools", nil, "Tools to allow for claude_cli adapter (e.g. --allowed-tools WebSearch Bash)")
	runCmd.Flags().BoolVar(&runKernel, "kernel", false, "Start a persistent python3 kernel session for SOLVE/ASSERT (deterministic-mode dispatch)")
	runCmd.Flags().StringVar(&runKernelProtocol, "kernel-protocol", "subprocess", "Kernel backend for --kernel: 'subprocess' (Option A, custom REPL protocol) or 'zmq' (Option B, real Jupyter wire protocol)")
	runCmd.Flags().StringVar(&runKernelName, "kernel-name", "python3", "Jupyter kernelspec name to launch when --kernel-protocol=zmq (e.g. python3, sagemath)")
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

func logSPLResult(recipePath, adapterName, source string, r *executor.SPLResult) (string, error) {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".spl", "logs")
	os.MkdirAll(logDir, 0755)

	recipeName := strings.TrimSuffix(filepath.Base(recipePath), ".spl")
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s-%s-%s-go.md", recipeName, adapterName, timestamp)
	logFilePath := filepath.Join(logDir, logFileName)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# SPL Run: %s\n\n", recipeName))
	sb.WriteString(fmt.Sprintf("- **Adapter:** %s\n", adapterName))
	sb.WriteString(fmt.Sprintf("- **Model:** %s\n", r.Model))
	sb.WriteString(fmt.Sprintf("- **Tokens:** %d in / %d out\n", r.InputTokens, r.OutputTokens))
	sb.WriteString(fmt.Sprintf("- **Latency:** %.0fms\n", r.LatencyMs))
	sb.WriteString(fmt.Sprintf("- **Timestamp:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## SPL Source\n\n```spl\n")
	sb.WriteString(source)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Final Prompt\n\n```prompt\n")
	sb.WriteString(r.Prompt)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Output\n\n```output\n")
	sb.WriteString(r.Content)
	sb.WriteString("\n```\n")

	err := os.WriteFile(logFilePath, []byte(sb.String()), 0644)
	return logFilePath, err
}

func logWorkflowResult(recipePath, adapterName, source string, r *executor.WorkflowResult) (string, error) {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".spl", "logs")
	os.MkdirAll(logDir, 0755)

	recipeName := strings.TrimSuffix(filepath.Base(recipePath), ".spl")
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s-%s-%s-go.md", recipeName, adapterName, timestamp)
	logFilePath := filepath.Join(logDir, logFileName)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# SPL Workflow Run: %s\n\n", recipeName))
	sb.WriteString(fmt.Sprintf("- **Status:** %s\n", r.Status))
	sb.WriteString(fmt.Sprintf("- **Adapter:** %s\n", adapterName))
	sb.WriteString(fmt.Sprintf("- **LLM Calls:** %d\n", r.TotalLLMCalls))
	sb.WriteString(fmt.Sprintf("- **Tokens:** %d in / %d out\n", r.TotalInputToks, r.TotalOutputToks))
	sb.WriteString(fmt.Sprintf("- **Latency:** %.0fms\n", r.TotalLatencyMs))
	sb.WriteString(fmt.Sprintf("- **Timestamp:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## SPL Source\n\n```spl\n")
	sb.WriteString(source)
	sb.WriteString("\n```\n\n")

	if r.CommittedValue != "" {
		sb.WriteString("## Committed Output\n\n```output\n")
		sb.WriteString(r.CommittedValue)
		sb.WriteString("\n```\n")
	}

	err := os.WriteFile(logFilePath, []byte(sb.String()), 0644)
	return logFilePath, err
}
