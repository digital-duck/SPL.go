package executor

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// kernelBackend is the common surface both kernel implementations satisfy:
// Option A (kernelSession, this file — a custom stdin/stdout REPL protocol)
// and Option B (jupyterKernelSession, kernel_zmq.go — a real Jupyter wire
// protocol client over ZeroMQ). Executor.Kernel holds whichever one
// --kernel-protocol selected, so SOLVE/ASSERT dispatch is identical either way.
type kernelBackend interface {
	execute(code string) (string, error)
	close() error
}

// kernelSession is a persistent python3 subprocess used by SOLVE and
// ASSERT's kernel path (Option A: a stdin/stdout REPL protocol over a
// long-lived process, not a full Jupyter/ZMQ client — see kernel_zmq.go for
// Option B). State (imported modules, variables, TOOL_API function defs)
// persists across calls within a workflow, matching the semantics of
// Python's spl3/kernel.py KernelSession/IPythonKernel.
type kernelSession struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// kernelEndSentinel marks the end of one exec's output on the driver's
// stdout stream. The NUL bytes make accidental collision with normal
// program output effectively impossible.
const kernelEndSentinel = "\x00SPL_KERNEL_END\x00"

// kernelErrPrefix marks a line as a Python exception rather than output.
const kernelErrPrefix = "SPL_KERNEL_ERROR: "

// kernelDriver is a minimal persistent REPL: it reads one code block
// (terminated by kernelEndSentinel on its own line) from stdin, execs it
// into a namespace that survives across calls, and writes whatever the code
// printed to stdout, followed by the sentinel so the Go side knows where
// the response ends.
const kernelDriver = `
import sys
_ns = {}
_END = "\x00SPL_KERNEL_END\x00"
_lines = []
while True:
    _line = sys.stdin.readline()
    if not _line:
        break
    if _line.rstrip("\n") == _END:
        _code = "".join(_lines)
        _lines = []
        try:
            exec(_code, _ns)
        except Exception as _e:
            sys.stdout.write("SPL_KERNEL_ERROR: " + repr(_e) + "\n")
        sys.stdout.write(_END + "\n")
        sys.stdout.flush()
    else:
        _lines.append(_line)
`

// startKernelSession launches the persistent python3 driver subprocess.
func startKernelSession() (*kernelSession, error) {
	cmd := exec.Command("python3", "-u", "-c", kernelDriver)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("kernel: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("kernel: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("kernel: start python3: %w", err)
	}
	return &kernelSession{cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout)}, nil
}

// execute runs one code block in the persistent namespace and returns
// whatever it wrote to stdout (trimmed). Returns an error if the code
// raised a Python exception.
func (k *kernelSession) execute(code string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if _, err := io.WriteString(k.stdin, code); err != nil {
		return "", fmt.Errorf("kernel: write code: %w", err)
	}
	if !strings.HasSuffix(code, "\n") {
		if _, err := io.WriteString(k.stdin, "\n"); err != nil {
			return "", fmt.Errorf("kernel: write newline: %w", err)
		}
	}
	if _, err := io.WriteString(k.stdin, kernelEndSentinel+"\n"); err != nil {
		return "", fmt.Errorf("kernel: write sentinel: %w", err)
	}

	var out strings.Builder
	var runtimeErr string
	for {
		line, err := k.stdout.ReadString('\n')
		if line != "" {
			trimmed := strings.TrimRight(line, "\n")
			if trimmed == kernelEndSentinel {
				break
			}
			if strings.HasPrefix(trimmed, kernelErrPrefix) {
				runtimeErr = strings.TrimPrefix(trimmed, kernelErrPrefix)
				continue
			}
			out.WriteString(line)
		}
		if err != nil {
			if err == io.EOF {
				return "", fmt.Errorf("kernel: subprocess exited unexpectedly")
			}
			return "", fmt.Errorf("kernel: read output: %w", err)
		}
	}
	if runtimeErr != "" {
		return "", fmt.Errorf("%s", runtimeErr)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// close terminates the kernel subprocess.
func (k *kernelSession) close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	_ = k.stdin.Close()
	return k.cmd.Wait()
}

// evalWithTools evaluates a single Python boolean expression via a one-shot
// python3 subprocess, with every registered CREATE TOOL_API function def'd
// into the eval namespace first. This is ASSERT's kernel-free path — it
// covers TOOL_API predicates (e.g. ASSERT is_optimal(@solution)) without
// requiring --kernel, mirroring spl3/executor.py's tools-eval branch.
// Returns "True", "False", or "assert_error: <message>" on exception.
func evalWithTools(code string, toolBodies map[string]string) string {
	var defs strings.Builder
	for _, body := range toolBodies {
		defs.WriteString(body)
		defs.WriteString("\n")
	}
	snippet := fmt.Sprintf(
		"import sys\n_ns = {}\nexec(%q, _ns)\n"+
			"try:\n    print(bool(eval(%q, _ns)))\n"+
			"except Exception as _e:\n    print('assert_error: ' + repr(_e))\n",
		defs.String(), code,
	)
	cmd := exec.Command("python3", "-c", snippet)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "assert_error: " + strings.TrimSpace(string(ee.Stderr))
		}
		return "assert_error: " + err.Error()
	}
	return strings.TrimSpace(string(out))
}
