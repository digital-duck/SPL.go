package executor

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/go-zeromq/zmq4"
	"github.com/google/uuid"
)

// jupyterKernelSession is Option B: a real Jupyter wire-protocol client
// (message framing + HMAC signing over ZeroMQ DEALER/SUB sockets), talking
// to an actual `jupyter kernelspec`-launched kernel process (ipykernel,
// SageMath's kernel, etc.) — the same mechanism Python's spl3/kernel.py
// IPythonKernel uses via jupyter_client, reimplemented directly against
// zmq4 rather than shelling out. Implements the same execute(code) (string,
// error) contract as kernelSession (Option A, kernel.go) so the two are
// interchangeable and comparable side by side.
type jupyterKernelSession struct {
	cmd      *exec.Cmd
	shell    zmq4.Socket
	iopub    zmq4.Socket
	key      []byte
	session  string
	connFile string
	stderr   *syncBuffer

	mu      sync.Mutex
	pending map[string]*execCapture
}

// syncBuffer is a mutex-protected byte buffer used to capture the kernel
// subprocess's stderr concurrently with the main goroutine, so launch
// failures (missing deps, bad kernelspec, etc.) surface in error messages
// instead of vanishing silently.
type syncBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// execCapture accumulates one request's stdout/stderr stream text (via
// iopub "stream" messages) until either the shell channel's direct reply
// (kernel_info_reply / execute_reply — point-to-point, arrives reliably) or
// the iopub "status: idle" broadcast signals completion, whichever comes
// first. Two signals exist because iopub is PUB/SUB and can miss the very
// first broadcast after connecting (the ZMQ "slow joiner" problem); the
// shell reply has no such race and is the primary completion signal.
type execCapture struct {
	out       []byte
	err       string
	done      chan struct{}
	closeOnce sync.Once
}

func (c *execCapture) finish() {
	c.closeOnce.Do(func() { close(c.done) })
}

// jupyterConnInfo is the Jupyter connection-file schema.
type jupyterConnInfo struct {
	ShellPort       int    `json:"shell_port"`
	IOPubPort       int    `json:"iopub_port"`
	StdinPort       int    `json:"stdin_port"`
	ControlPort     int    `json:"control_port"`
	HBPort          int    `json:"hb_port"`
	IP              string `json:"ip"`
	Key             string `json:"key"`
	Transport       string `json:"transport"`
	SignatureScheme string `json:"signature_scheme"`
	KernelName      string `json:"kernel_name"`
}

// kernelspecEntry mirrors the relevant subset of `jupyter kernelspec list --json`.
type kernelspecEntry struct {
	ResourceDir string `json:"resource_dir"`
	Spec        struct {
		Argv []string `json:"argv"`
	} `json:"spec"`
}

// resolveKernelspecArgv shells out to `jupyter kernelspec list --json` and
// returns the argv template for the named kernel (e.g. "python3",
// "sagemath"). Mirrors spl3/kernel.py installed_kernelspecs/ensure_kernelspec.
func resolveKernelspecArgv(name string) ([]string, error) {
	out, err := exec.Command("jupyter", "kernelspec", "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("jupyter kernelspec list: %w (is Jupyter installed?)", err)
	}
	var parsed struct {
		Kernelspecs map[string]kernelspecEntry `json:"kernelspecs"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("jupyter kernelspec list: parse error: %w", err)
	}
	entry, ok := parsed.Kernelspecs[name]
	if !ok {
		var available []string
		for k := range parsed.Kernelspecs {
			available = append(available, k)
		}
		return nil, fmt.Errorf("kernelspec %q not installed; available: %v", name, available)
	}
	return entry.Spec.Argv, nil
}

// freePort asks the OS for an unused TCP port by briefly binding to :0.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// startJupyterKernelSession launches a real Jupyter kernel subprocess and
// connects to its shell (DEALER) and iopub (SUB) channels.
func startJupyterKernelSession(ctx context.Context, kernelName string) (*jupyterKernelSession, error) {
	argv, err := resolveKernelspecArgv(kernelName)
	if err != nil {
		return nil, err
	}

	ports := make(map[string]int, 5)
	for _, name := range []string{"shell_port", "iopub_port", "stdin_port", "control_port", "hb_port"} {
		p, err := freePort()
		if err != nil {
			return nil, fmt.Errorf("kernel: allocate port: %w", err)
		}
		ports[name] = p
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("kernel: generate hmac key: %w", err)
	}
	key := hex.EncodeToString(keyBytes)

	conn := jupyterConnInfo{
		ShellPort: ports["shell_port"], IOPubPort: ports["iopub_port"],
		StdinPort: ports["stdin_port"], ControlPort: ports["control_port"], HBPort: ports["hb_port"],
		IP: "127.0.0.1", Key: key, Transport: "tcp",
		SignatureScheme: "hmac-sha256", KernelName: kernelName,
	}
	connFile, err := os.CreateTemp("", "spl-go-kernel-*.json")
	if err != nil {
		return nil, fmt.Errorf("kernel: create connection file: %w", err)
	}
	if err := json.NewEncoder(connFile).Encode(conn); err != nil {
		connFile.Close()
		return nil, fmt.Errorf("kernel: write connection file: %w", err)
	}
	connFile.Close()

	launchArgv := make([]string, len(argv))
	for i, a := range argv {
		if a == "{connection_file}" {
			launchArgv[i] = connFile.Name()
		} else {
			launchArgv[i] = a
		}
	}
	if len(launchArgv) == 0 {
		os.Remove(connFile.Name())
		return nil, fmt.Errorf("kernel: empty argv for kernelspec %q", kernelName)
	}
	cmd := exec.Command(launchArgv[0], launchArgv[1:]...)
	stderr := &syncBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		os.Remove(connFile.Name())
		return nil, fmt.Errorf("kernel: start %v: %w", launchArgv, err)
	}

	shell := zmq4.NewDealer(ctx)
	iopub := zmq4.NewSub(ctx)
	shellAddr := "tcp://" + conn.IP + ":" + strconv.Itoa(conn.ShellPort)
	iopubAddr := "tcp://" + conn.IP + ":" + strconv.Itoa(conn.IOPubPort)
	if err := shell.Dial(shellAddr); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("kernel: dial shell %s: %w", shellAddr, err)
	}
	if err := iopub.Dial(iopubAddr); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("kernel: dial iopub %s: %w", iopubAddr, err)
	}
	if err := iopub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("kernel: subscribe iopub: %w", err)
	}

	sess := &jupyterKernelSession{
		cmd: cmd, shell: shell, iopub: iopub,
		// HMAC key must be the UTF-8 bytes of the "key" string written to
		// the connection file (a hex string), not the raw bytes it was
		// derived from — jupyter_client signs with key.encode('utf-8').
		key: []byte(key), session: uuid.NewString(),
		connFile: connFile.Name(),
		stderr:   stderr,
		pending:  make(map[string]*execCapture),
	}
	go sess.iopubLoop()
	go sess.shellLoop()

	// Give the iopub SUB socket's subscription a moment to establish before
	// the first request goes out, to reduce (not eliminate — shellLoop is
	// the real fix) the window for the PUB/SUB slow-joiner race.
	time.Sleep(150 * time.Millisecond)

	if err := sess.waitReady(ctx, 30*time.Second); err != nil {
		sess.close()
		return nil, err
	}
	return sess, nil
}

// waitReady blocks until the kernel answers a kernel_info_request, i.e.
// until it has finished starting and is accepting execute_requests.
func (k *jupyterKernelSession) waitReady(_ context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msgID, err := k.sendShell("kernel_info_request", map[string]interface{}{})
		if err != nil {
			return fmt.Errorf("kernel: kernel_info_request: %w", err)
		}
		reply := make(chan struct{})
		k.registerCapture(msgID, reply)
		select {
		case <-reply:
			k.dropCapture(msgID)
			return nil
		case <-time.After(2 * time.Second):
			k.dropCapture(msgID)
			continue
		}
	}
	stderrTail := k.stderr.String()
	if stderrTail != "" {
		return fmt.Errorf("kernel: no response from kernel within %s; stderr: %s", timeout, stderrTail)
	}
	return fmt.Errorf("kernel: no response from kernel within %s (is %s reachable?)", timeout, k.connFile)
}

func (k *jupyterKernelSession) registerCapture(msgID string, done chan struct{}) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.pending[msgID] = &execCapture{done: done}
}

func (k *jupyterKernelSession) dropCapture(msgID string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.pending, msgID)
}

// jupyterMsg is the 6-part Jupyter wire message.
type jupyterMsg struct {
	Header       map[string]interface{}
	ParentHeader map[string]interface{}
	Metadata     map[string]interface{}
	Content      map[string]interface{}
}

const jupyterDelim = "<IDS|MSG>"

// sendShell builds, signs, and sends one request on the shell DEALER
// channel, returning the generated msg_id.
func (k *jupyterKernelSession) sendShell(msgType string, content map[string]interface{}) (string, error) {
	msgID := uuid.NewString()
	header := map[string]interface{}{
		"msg_id":   msgID,
		"username": "spl-go",
		"session":  k.session,
		"date":     time.Now().UTC().Format(time.RFC3339),
		"msg_type": msgType,
		"version":  "5.3",
	}
	headerJSON, _ := json.Marshal(header)
	parentJSON := []byte("{}")
	metaJSON := []byte("{}")
	contentJSON, _ := json.Marshal(content)

	mac := hmac.New(sha256.New, k.key)
	mac.Write(headerJSON)
	mac.Write(parentJSON)
	mac.Write(metaJSON)
	mac.Write(contentJSON)
	sig := hex.EncodeToString(mac.Sum(nil))

	msg := zmq4.NewMsgFrom(
		[]byte(jupyterDelim),
		[]byte(sig),
		headerJSON,
		parentJSON,
		metaJSON,
		contentJSON,
	)
	if err := k.shell.SendMulti(msg); err != nil {
		return "", err
	}
	return msgID, nil
}

// parsedMsg holds the decoded pieces of one Jupyter wire message that
// iopubLoop/shellLoop need.
type parsedMsg struct {
	msgType  string
	parentID string
	content  map[string]interface{}
}

// parseJupyterMsg locates the "<IDS|MSG>" delimiter in a raw multipart ZMQ
// message and decodes the header/parent_header/content JSON frames that
// follow it. Shared by iopubLoop (SUB) and shellLoop (DEALER) since both
// channels use the same 6-frame wire format.
func parseJupyterMsg(frames [][]byte) (parsedMsg, bool) {
	delimIdx := -1
	for i, frame := range frames {
		if string(frame) == jupyterDelim {
			delimIdx = i
			break
		}
	}
	if delimIdx < 0 || len(frames) < delimIdx+6 {
		return parsedMsg{}, false
	}
	// Frame layout after the delimiter: sig, header, parent_header,
	// metadata, content.
	var header, parent, content map[string]interface{}
	_ = json.Unmarshal(frames[delimIdx+2], &header)
	_ = json.Unmarshal(frames[delimIdx+3], &parent)
	_ = json.Unmarshal(frames[delimIdx+5], &content)

	parentID, _ := parent["msg_id"].(string)
	msgType, _ := header["msg_type"].(string)
	if parentID == "" {
		return parsedMsg{}, false
	}
	return parsedMsg{msgType: msgType, parentID: parentID, content: content}, true
}

// shellLoop continuously drains the shell DEALER socket, routing each
// *_reply message's error status and completion signal to the matching
// pending execCapture. This is the primary (race-free) completion signal —
// see execCapture's doc comment for why iopub alone isn't sufficient.
func (k *jupyterKernelSession) shellLoop() {
	for {
		msg, err := k.shell.Recv()
		if err != nil {
			return // socket closed
		}
		pm, ok := parseJupyterMsg(msg.Frames)
		if !ok {
			continue
		}
		k.mu.Lock()
		cap, ok := k.pending[pm.parentID]
		k.mu.Unlock()
		if !ok {
			continue
		}
		if status, _ := pm.content["status"].(string); status == "error" {
			ename, _ := pm.content["ename"].(string)
			evalue, _ := pm.content["evalue"].(string)
			cap.err = fmt.Sprintf("%s: %s", ename, evalue)
		}
		cap.finish()
	}
}

// iopubLoop continuously drains the iopub SUB socket, routing stream text,
// errors, and idle-completion signals to the matching pending execCapture
// by parent_header.msg_id. Runs for the lifetime of the session.
func (k *jupyterKernelSession) iopubLoop() {
	for {
		msg, err := k.iopub.Recv()
		if err != nil {
			return // socket closed
		}
		pm, ok := parseJupyterMsg(msg.Frames)
		if !ok {
			continue
		}
		k.mu.Lock()
		cap, ok := k.pending[pm.parentID]
		k.mu.Unlock()
		if !ok {
			continue
		}

		switch pm.msgType {
		case "stream":
			if text, ok := pm.content["text"].(string); ok {
				cap.out = append(cap.out, []byte(text)...)
			}
		case "error":
			ename, _ := pm.content["ename"].(string)
			evalue, _ := pm.content["evalue"].(string)
			cap.err = fmt.Sprintf("%s: %s", ename, evalue)
		case "execute_result":
			if data, ok := pm.content["data"].(map[string]interface{}); ok {
				if text, ok := data["text/plain"].(string); ok {
					cap.out = append(cap.out, []byte(text)...)
				}
			}
		case "status":
			if state, _ := pm.content["execution_state"].(string); state == "idle" {
				cap.finish()
			}
		}
	}
}

// execute runs code in the persistent kernel namespace and returns whatever
// it printed to stdout (trimmed), matching kernelSession.execute's contract.
func (k *jupyterKernelSession) execute(code string) (string, error) {
	msgID, err := k.sendShell("execute_request", map[string]interface{}{
		"code": code, "silent": false, "store_history": true,
		"user_expressions": map[string]interface{}{}, "allow_stdin": false, "stop_on_error": true,
	})
	if err != nil {
		return "", fmt.Errorf("kernel: send execute_request: %w", err)
	}

	done := make(chan struct{})
	k.mu.Lock()
	c := &execCapture{done: done}
	k.pending[msgID] = c
	k.mu.Unlock()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		k.dropCapture(msgID)
		return "", fmt.Errorf("kernel: execute timed out after 60s")
	}
	k.dropCapture(msgID)

	if c.err != "" {
		return "", fmt.Errorf("%s", c.err)
	}
	return string(c.out), nil
}

// close terminates the kernel subprocess and cleans up its connection file.
func (k *jupyterKernelSession) close() error {
	_ = k.shell.Close()
	_ = k.iopub.Close()
	if k.cmd != nil && k.cmd.Process != nil {
		_ = k.cmd.Process.Kill()
		_ = k.cmd.Wait()
	}
	if k.connFile != "" {
		_ = os.Remove(k.connFile)
	}
	return nil
}
