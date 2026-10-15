package commands

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
)

var (
	serveBinaryOnce sync.Once
	serveBinaryPath string
	serveBinaryErr  error
)

// serveTestBinary builds cmd/lif-serve once per test run, next to the lif
// binary of TestMain, which removes the directory at the end.
func serveTestBinary(t *testing.T) string {
	t.Helper()
	serveBinaryOnce.Do(func() {
		repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			serveBinaryErr = err
			return
		}
		serveBinaryPath = filepath.Join(filepath.Dir(completionTestBinary), "lif-serve")
		build := exec.Command("go", "build", "-o", serveBinaryPath, "./cmd/lif-serve")
		build.Dir = repoRoot
		if out, err := build.CombinedOutput(); err != nil {
			serveBinaryErr = fmt.Errorf("building lif-serve: %v\n%s", err, out)
		}
	})
	if serveBinaryErr != nil {
		t.Fatal(serveBinaryErr)
	}
	return serveBinaryPath
}

// serveProcess is a running lif-serve binary and what it printed at startup.
type serveProcess struct {
	log   string // the startup lines
	port  int    // from the "Listening on" line: the port the listener really got
	token string // from the "Access token" line; empty when the server prints none
}

var (
	listeningLine = regexp.MustCompile(`\[lif\] Listening on (\S+) `)
	tokenLine     = regexp.MustCompile(`\[lif\] Access token: (\S+)`)
)

// startServeProcess runs the real lif-serve binary on an ephemeral port in a
// throwaway project and returns once it has printed its last startup line.
func startServeProcess(t *testing.T, args ...string) *serveProcess {
	t.Helper()
	bin := serveTestBinary(t)

	project := t.TempDir()
	store := filepath.Join(project, ".lif")
	if err := os.MkdirAll(store, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", project, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v\n%s", err, out)
	}

	var out syncBuffer
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, append([]string{"--lif-path", store, "--port", "0"}, args...)...)
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pw.Close()
	go func() { _, _ = io.Copy(&out, pr) }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(out.String(), "[lif] Terminal: ") {
		if time.Now().After(deadline) {
			t.Fatalf("lif-serve did not print its startup lines in time:\n%s", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	p := &serveProcess{log: out.String()}
	m := listeningLine.FindStringSubmatch(p.log)
	if m == nil {
		t.Fatalf("no \"Listening on\" line in the startup log:\n%s", p.log)
	}
	_, portText, err := net.SplitHostPort(m[1])
	if err != nil {
		t.Fatalf("listening address %q: %v", m[1], err)
	}
	if p.port, err = strconv.Atoi(portText); err != nil || p.port == 0 {
		t.Fatalf("listening address %q has no real port", m[1])
	}
	if m := tokenLine.FindStringSubmatch(p.log); m != nil {
		p.token = m[1]
	}
	return p
}

// url is the address of a path on this machine, where the server always
// answers: a loopback listener and a listener on all interfaces both do.
func (p *serveProcess) url(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", p.port, path)
}

// do sends one request without following redirects. A non-empty token goes in
// the Authorization header; an empty origin sends no Origin.
func (p *serveProcess) do(t *testing.T, method, path, token, origin, body string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, p.url(path), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data), resp.Header
}

// dialTerminal opens the terminal WebSocket with the headers of a browser page
// of the UI, plus the token when given.
func (p *serveProcess) dialTerminal(t *testing.T, token string) (*coderws.Conn, *http.Response, error) {
	t.Helper()
	header := http.Header{"Origin": {"http://localhost:5173"}}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return coderws.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/api/terminal", p.port), &coderws.DialOptions{HTTPHeader: header})
}

// The serve flags the safe default rests on.
func TestServeFlagsDefaultToLoopbackWithoutRemoteTerminal(t *testing.T) {
	root := NewRootCmd()
	if serveCmd.Flags().Lookup("port") == nil {
		RegisterServeCmd(root)
	} else {
		root.AddCommand(serveCmd)
	}
	for flag, want := range map[string]string{"host": "127.0.0.1", "allow-remote-terminal": "false"} {
		f := serveCmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("lif-serve has no --%s flag", flag)
		}
		if f.DefValue != want {
			t.Errorf("--%s defaults to %q, want %q", flag, f.DefValue, want)
		}
	}
}

// With no flags the server listens on the loopback address, as its log says,
// and nothing else of this machine reaches it. The check dials every other
// address of the machine: a listener on all interfaces would answer.
func TestServeListensOnLoopbackByDefault(t *testing.T) {
	p := startServeProcess(t)

	if want := fmt.Sprintf("[lif] Listening on 127.0.0.1:%d (loopback only", p.port); !strings.Contains(p.log, want) {
		t.Errorf("startup log lacks %q:\n%s", want, p.log)
	}
	if p.token != "" {
		t.Errorf("a loopback server printed an access token:\n%s", p.log)
	}
	if want := "[lif] Terminal: enabled"; !strings.Contains(p.log, want) {
		t.Errorf("startup log lacks %q:\n%s", want, p.log)
	}

	if status, _, _ := p.do(t, "GET", "/playground", "", "", ""); status != 200 {
		t.Errorf("GET /playground over loopback: status %d, want 200", status)
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	probed := 0
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || !ipnet.IP.IsGlobalUnicast() || ipnet.IP.IsLoopback() {
			continue
		}
		probed++
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipnet.IP.String(), strconv.Itoa(p.port)), 2*time.Second)
		if err == nil {
			conn.Close()
			t.Errorf("the server on its default address answers on %s", ipnet.IP)
		}
	}
	if probed == 0 {
		t.Log("no non-loopback address on this machine: only the log line was checked")
	}
}

// An address beyond loopback is an explicit decision. The server then
// demands the token from every client and has no terminal: not the WebSocket
// and not the GraphQL operations that write to a shell.
func TestServeOnTheNetworkNeedsTheTokenAndHasNoTerminal(t *testing.T) {
	p := startServeProcess(t, "--host", "0.0.0.0")

	// A wildcard listener reports itself as 0.0.0.0 or, dual-stack, as [::].
	if !regexp.MustCompile(fmt.Sprintf(`\[lif\] Listening on (0\.0\.0\.0|\[::\]):%d \(ALL network interfaces`, p.port)).MatchString(p.log) {
		t.Errorf("startup log lacks the wildcard \"Listening on\" line for port %d:\n%s", p.port, p.log)
	}
	for _, want := range []string{
		"[lif] WARNING:",
		"[lif] Terminal: disabled",
		"--allow-remote-terminal",
	} {
		if !strings.Contains(p.log, want) {
			t.Errorf("startup log lacks %q:\n%s", want, p.log)
		}
	}
	if len(p.token) < 32 {
		t.Fatalf("no usable access token in the startup log:\n%s", p.log)
	}

	if status, _, _ := p.do(t, "GET", "/playground", "", "", ""); status != 401 {
		t.Errorf("GET /playground without token: status %d, want 401", status)
	}
	if status, _, _ := p.do(t, "GET", "/playground", "wrong-token", "", ""); status != 401 {
		t.Errorf("GET /playground with a wrong token: status %d, want 401", status)
	}
	if status, body, _ := p.do(t, "GET", "/playground", p.token, "", ""); status != 200 || !strings.Contains(body, "<title>Lif GraphQL</title>") {
		t.Errorf("GET /playground with the token: status %d, body %.200q", status, body)
	}

	status, _, header := p.do(t, "GET", "/playground?token="+p.token, "", "", "")
	if status != http.StatusSeeOther || !strings.Contains(header.Get("Set-Cookie"), fmt.Sprintf("lif_token_%d=", p.port)) {
		t.Errorf("the printed address: status %d, Set-Cookie %q; want a 303 that sets the cookie", status, header.Get("Set-Cookie"))
	}

	status, body, _ := p.do(t, "GET", "/api/terminal", p.token, "http://localhost:5173", "")
	if status != http.StatusForbidden || !strings.Contains(body, "terminal is disabled") {
		t.Errorf("GET /api/terminal: status %d, body %q; want a 403 that says the terminal is disabled", status, body)
	}

	query := `{"query":"mutation { writeTerminalInput(sessionId: \"__central__\", data: \"echo x\\n\") }"}`
	if status, body, _ := p.do(t, "POST", "/api/graphql", "", "", query); status != 401 {
		t.Errorf("GraphQL mutation without token: status %d (%s), want 401", status, body)
	}
	if _, body, _ := p.do(t, "POST", "/api/graphql", p.token, "", query); !strings.Contains(body, "terminal support not available") {
		t.Errorf("writeTerminalInput with the terminal disabled: %s, want the error \"terminal support not available\"", body)
	}
}

// --allow-remote-terminal turns the terminal on for a listener on the network.
// It does not lift the token.
func TestServeRemoteTerminalOptIn(t *testing.T) {
	p := startServeProcess(t, "--host", "0.0.0.0", "--allow-remote-terminal")

	for _, want := range []string{"[lif] Terminal: enabled (--allow-remote-terminal)", "[lif] WARNING:"} {
		if !strings.Contains(p.log, want) {
			t.Errorf("startup log lacks %q:\n%s", want, p.log)
		}
	}
	if len(p.token) < 32 {
		t.Fatalf("no usable access token in the startup log:\n%s", p.log)
	}

	conn, resp, err := p.dialTerminal(t, "")
	if err == nil {
		conn.CloseNow()
		t.Fatal("the terminal upgrade was accepted without the token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("terminal upgrade without token: %v, want 401", resp)
	}

	conn, resp, err = p.dialTerminal(t, p.token)
	if err != nil {
		t.Fatalf("terminal upgrade with the token: %v (response %v)", err, resp)
	}
	conn.CloseNow()
}
