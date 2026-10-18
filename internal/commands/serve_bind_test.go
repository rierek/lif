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
	"runtime"
	"slices"
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
	out    *syncBuffer // everything the process printed so far
	log    string      // the startup lines
	port   int         // from the "Listening on" line: the port the listener really got
	host   string      // host:port under which this machine reaches the server
	token  string      // from the "Access token" line; empty when the server prints none
	scheme string      // "https" when the server was started with --tls-cert, else "http"
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

	p := &serveProcess{out: &out, log: out.String(), scheme: "http"}
	if slices.Contains(args, "--tls-cert") {
		p.scheme = "https"
	}
	m := listeningLine.FindStringSubmatch(p.log)
	if m == nil {
		t.Fatalf("no \"Listening on\" line in the startup log:\n%s", p.log)
	}
	host, portText, err := net.SplitHostPort(m[1])
	if err != nil {
		t.Fatalf("listening address %q: %v", m[1], err)
	}
	if p.port, err = strconv.Atoi(portText); err != nil || p.port == 0 {
		t.Fatalf("listening address %q has no real port", m[1])
	}
	// A listener on all interfaces answers on the loopback address too.
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	p.host = net.JoinHostPort(host, portText)
	if m := tokenLine.FindStringSubmatch(p.log); m != nil {
		p.token = m[1]
	}
	return p
}

// waitForLog waits until the process has printed text and returns everything
// it printed so far.
func (p *serveProcess) waitForLog(t *testing.T, text string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(p.out.String(), text) {
		if time.Now().After(deadline) {
			t.Fatalf("the process never printed %q:\n%s", text, p.out.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
	return p.out.String()
}

// url is the address of a path on this machine, where the server answers: a
// loopback listener under its own address, a listener on all interfaces under
// 127.0.0.1.
func (p *serveProcess) url(path string) string {
	return p.scheme + "://" + p.host + path
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
	return send(t, req)
}

// doAs sends a GET whose Host header names host, as a browser does that
// reaches the server under that name.
func (p *serveProcess) doAs(t *testing.T, host, path string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest("GET", p.url(path), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	return send(t, req)
}

// getWithHeaders sends a GET with the given request headers, as a proxy in
// front of the server would add them, and the token in the Authorization
// header when it is not empty.
func (p *serveProcess) getWithHeaders(t *testing.T, path, token string, header http.Header) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest("GET", p.url(path), nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return send(t, req)
}

func send(t *testing.T, req *http.Request) (int, string, http.Header) {
	t.Helper()
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     insecureTransport(), // the test certificate is self-signed
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data), resp.Header
}

// dialWS opens a WebSocket on the server with the given Origin (none when
// empty) and, when not empty, the token.
func (p *serveProcess) dialWS(t *testing.T, path, origin, token string, subprotocols ...string) (*coderws.Conn, *http.Response, error) {
	t.Helper()
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	scheme := "ws"
	if p.scheme == "https" {
		scheme = "wss"
	}
	return coderws.Dial(ctx, scheme+"://"+p.host+path, &coderws.DialOptions{HTTPHeader: header, Subprotocols: subprotocols, HTTPClient: &http.Client{Transport: insecureTransport()}})
}

// The serve flags the safe default rests on.
func TestServeFlagsDefaultToLoopbackWithoutRemoteTerminal(t *testing.T) {
	root := NewRootCmd()
	if serveCmd.Flags().Lookup("port") == nil {
		RegisterServeCmd(root)
	} else {
		root.AddCommand(serveCmd)
	}
	for flag, want := range map[string]string{"host": "127.0.0.1", "allow-remote-terminal": "false", "allow-remote-agent": "false", "require-token": "false", "public-url": ""} {
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

// ::1 and 127.0.0.2 are loopback addresses too: the server keeps the safe
// defaults there and serves the URL it prints, to its own page.
func TestServeOnOtherLoopbackAddresses(t *testing.T) {
	for _, ip := range []string{"::1", "127.0.0.2"} {
		t.Run(ip, func(t *testing.T) {
			ln, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
			if err != nil {
				t.Skipf("this machine cannot listen on %s: %v", ip, err)
			}
			ln.Close()

			p := startServeProcess(t, "--host", ip)
			own := net.JoinHostPort(ip, strconv.Itoa(p.port))
			for _, want := range []string{
				"[lif] Listening on " + own + " (loopback only",
				"[lif] Starting server at http://" + own + "/",
				"[lif] Terminal: enabled",
			} {
				if !strings.Contains(p.log, want) {
					t.Errorf("startup log lacks %q:\n%s", want, p.log)
				}
			}
			if p.token != "" {
				t.Errorf("a loopback server printed an access token:\n%s", p.log)
			}

			if status, body, _ := p.do(t, "GET", "/playground", "", "", ""); status != 200 {
				t.Errorf("GET http://%s/playground: status %d (%.100s), want 200", own, status, body)
			}
			query := `{"query":"mutation { __typename }"}`
			if status, body, _ := p.do(t, "POST", "/api/graphql", "", "http://"+own, query); status != 200 {
				t.Errorf("mutation from the page at %s: status %d (%.100s), want 200", own, status, body)
			}
			conn, resp, err := p.dialWS(t, "/api/terminal", "http://"+own, "")
			if err != nil {
				t.Fatalf("terminal upgrade from the UI at %s: %v (response %v)", own, err, resp)
			}
			conn.CloseNow()

			if status, _, _ := p.do(t, "POST", "/api/graphql", "", "http://evil.example", query); status != 403 {
				t.Errorf("mutation from a page of another site: status %d, want 403", status)
			}
		})
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
	// The printed address opens the UI as it is: it names a real host and the
	// token. The address the server answers 401 under is not printed.
	open := regexp.MustCompile(`\[lif\] Open the UI: http://([^/\s]+):(\d+)/\?token=(\S+)`).FindStringSubmatch(p.log)
	if open == nil || open[2] != strconv.Itoa(p.port) || open[3] != p.token || strings.ContainsAny(open[1], "<>") {
		t.Errorf("startup log has no usable \"Open the UI\" line:\n%s", p.log)
	}
	if strings.Contains(p.log, "Starting server at") {
		t.Errorf("startup log prints an address that answers 401 without the token:\n%s", p.log)
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

	// The UI the server serves is of its own origin: its WebSocket needs no
	// entry in the origin list. A page of another site gets none.
	conn, resp, err := p.dialWS(t, "/api/graphql", "http://"+p.host, p.token, "graphql-transport-ws")
	if err != nil {
		t.Fatalf("GraphQL WebSocket from the UI served at %s: %v (response %v)", p.host, err, resp)
	}
	conn.CloseNow()
	if conn, _, err := p.dialWS(t, "/api/graphql", "http://evil.example", p.token, "graphql-transport-ws"); err == nil {
		conn.CloseNow()
		t.Error("GraphQL WebSocket from a page of another site was accepted")
	}

	// The access log carries no token: it travels in ?token=. A request for a
	// marker path makes sure the lines of the requests above are in the log.
	p.do(t, "GET", "/log-marker", p.token, "", "")
	live := p.waitForLog(t, `"/log-marker"`)
	if !strings.Contains(live, `"/playground"`) {
		t.Errorf("the access log has no line for /playground:\n%s", live)
	}
	for _, line := range strings.Split(live, "\n") {
		if strings.HasPrefix(line, "[GIN]") && strings.Contains(line, p.token) {
			t.Errorf("an access log line carries the token: %s", line)
		}
	}
}

// --require-token gives a loopback server the rules of a network one, for use
// behind a proxy: the token for every client, and no terminal.
func TestServeRequireTokenOnLoopback(t *testing.T) {
	p := startServeProcess(t, "--require-token")

	for _, want := range []string{
		fmt.Sprintf("[lif] Listening on 127.0.0.1:%d (loopback only", p.port),
		"[lif] Access: token required (--require-token or --public-url)",
		"[lif] Terminal: disabled",
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
	if status, body, _ := p.do(t, "GET", "/playground", p.token, "", ""); status != 200 || !strings.Contains(body, "<title>Lif GraphQL</title>") {
		t.Errorf("GET /playground with the token: status %d, body %.200q", status, body)
	}
	status, body, _ := p.do(t, "GET", "/api/terminal", p.token, "http://localhost:5173", "")
	if status != http.StatusForbidden || !strings.Contains(body, "terminal is disabled") {
		t.Errorf("GET /api/terminal: status %d, body %q; want a 403 that says the terminal is disabled", status, body)
	}
	// A proxy adds its headers; with the token they are no reason to refuse.
	if status, _, _ := p.getWithHeaders(t, "/playground", p.token, http.Header{"X-Forwarded-For": {"203.0.113.9"}}); status != 200 {
		t.Errorf("proxied GET /playground with the token: status %d, want 200", status)
	}
}

// Without a token the server cannot tell a proxied request from a local one,
// so it refuses the requests that carry proxy headers and says what to do.
func TestServeRefusesProxiedRequestsWithoutToken(t *testing.T) {
	p := startServeProcess(t)

	if status, _, _ := p.do(t, "GET", "/playground", "", "", ""); status != 200 {
		t.Errorf("GET /playground without proxy headers: status %d, want 200", status)
	}
	for _, h := range proxyHeaders {
		status, body, _ := p.getWithHeaders(t, "/playground", "", http.Header{h: {"203.0.113.9"}})
		if status != http.StatusForbidden || !strings.Contains(body, "came through a proxy") {
			t.Errorf("%s: status %d, body %q; want a 403 that names the proxy", h, status, body)
		}
	}
}

// An agent session in act mode runs commands as the server user, a shell like
// the terminal. Beyond loopback, and behind a proxy, the server refuses it
// entirely and tells the UI to hide it, unless --allow-remote-agent asks for it.
func TestServeAgentGate(t *testing.T) {
	const agentEnabled = `{"query":"{ agentEnabled }"}`
	const sendMessage = `{"query":"mutation { sendAgentMessage(itemId: \"__central__\", message: \"run ls\") }"}`

	tests := []struct {
		name    string
		args    []string
		wantLog string
		want    bool
	}{
		{"loopback", nil, "[lif] Agent: enabled\n", true},
		{"network", []string{"--host", "0.0.0.0"}, "[lif] Agent: disabled (the server is reachable from the network). Add --allow-remote-agent to enable it.", false},
		{"network, opted in", []string{"--host", "0.0.0.0", "--allow-remote-agent"}, "[lif] Agent: enabled (--allow-remote-agent). WARNING:", true},
		{"behind a proxy", []string{"--require-token"}, "[lif] Agent: disabled", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := startServeProcess(t, tt.args...)
			if !strings.Contains(p.log, tt.wantLog) {
				t.Errorf("startup log lacks %q:\n%s", tt.wantLog, p.log)
			}

			status, body, _ := p.do(t, "POST", "/api/graphql", p.token, "", agentEnabled)
			if want := fmt.Sprintf(`{"data":{"agentEnabled":%v}}`, tt.want); status != 200 || strings.TrimSpace(body) != want {
				t.Errorf("{ agentEnabled }: status %d, body %q; want %s", status, body, want)
			}
			if !tt.want {
				if _, body, _ := p.do(t, "POST", "/api/graphql", p.token, "", sendMessage); !strings.Contains(body, "agent manager not available") {
					t.Errorf("sendAgentMessage with the agent refused: %s, want the error \"agent manager not available\"", body)
				}
			}
		})
	}
}

// --token-file keeps the access token across restarts: the first start
// creates the file with mode 0600, the log names the file and does not print
// the token, and a restart accepts the same token.
func TestServeTokenFileKeepsTheTokenAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve-token")

	first := startServeProcess(t, "--require-token", "--token-file", path)
	if want := "[lif] Access token: read from " + path; !strings.Contains(first.log, want) {
		t.Errorf("startup log lacks %q:\n%s", want, first.log)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the token file was not created: %v", err)
	}
	token := strings.TrimSpace(string(data))
	if len(token) < 32 {
		t.Fatalf("token file holds %q, want a usable token", token)
	}
	if strings.Contains(first.log, token) {
		t.Errorf("the startup log prints the token of the file:\n%s", first.log)
	}
	if info, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Errorf("token file: %v, %v; want mode 0600", info, err)
	}
	if status, _, _ := first.do(t, "GET", "/playground", "", "", ""); status != 401 {
		t.Errorf("GET /playground without token: status %d, want 401", status)
	}
	if status, _, _ := first.do(t, "GET", "/playground", token, "", ""); status != 200 {
		t.Errorf("GET /playground with the token of the file: status %d, want 200", status)
	}

	second := startServeProcess(t, "--require-token", "--token-file", path)
	if status, _, _ := second.do(t, "GET", "/playground", token, "", ""); status != 200 {
		t.Errorf("after a restart, GET /playground with the same token: status %d, want 200", status)
	}
}

// A token file other users can read, and a token that is too short, stop the
// server at startup and say why.
func TestServeRefusesAnUnsafeToken(t *testing.T) {
	bin := serveTestBinary(t)
	run := func(t *testing.T, env string, args ...string) string {
		t.Helper()
		project := t.TempDir()
		store := filepath.Join(project, ".lif")
		if err := os.MkdirAll(store, 0755); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, append([]string{"--lif-path", store, "--port", "0", "--require-token"}, args...)...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), env)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("lif-serve started although the token is unsafe:\n%s", out)
		}
		return string(out)
	}

	t.Run("file other users can read", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows has no file modes")
		}
		path := writeTokenFile(t, strings.Repeat("a", 43)+"\n", 0o644)
		if out := run(t, "LIF_SERVE_TOKEN=", "--token-file", path); !strings.Contains(out, "accessible by other users") || !strings.Contains(out, "chmod 600 "+path) {
			t.Errorf("output lacks the refusal with the chmod hint:\n%s", out)
		}
	})

	t.Run("short token", func(t *testing.T) {
		if out := run(t, "LIF_SERVE_TOKEN=short"); !strings.Contains(out, "token from LIF_SERVE_TOKEN is shorter than 32 characters") {
			t.Errorf("output lacks the length refusal:\n%s", out)
		}
	})
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

	conn, resp, err := p.dialWS(t, "/api/terminal", "http://localhost:5173", "")
	if err == nil {
		conn.CloseNow()
		t.Fatal("the terminal upgrade was accepted without the token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("terminal upgrade without token: %v, want 401", resp)
	}

	// The UI the server serves, and the dev server of the UI, get the shell.
	for _, origin := range []string{"http://localhost:5173", "http://" + p.host} {
		conn, resp, err = p.dialWS(t, "/api/terminal", origin, p.token)
		if err != nil {
			t.Fatalf("terminal upgrade with the token from %s: %v (response %v)", origin, err, resp)
		}
		conn.CloseNow()
	}
}

// --cors-origin '*' hands out the CORS header to every page and nothing else:
// no page of another site gets the terminal or a change through it, and the
// host check stays on. The log says so, and names the shell.
func TestServeWildcardOriginAllowsNoShell(t *testing.T) {
	p := startServeProcess(t, "--cors-origin", "*")

	for _, want := range []string{"[lif] WARNING: allowed origins include *", "shell", "[lif] Terminal: enabled"} {
		if !strings.Contains(p.log, want) {
			t.Errorf("startup log lacks %q:\n%s", want, p.log)
		}
	}

	status, _, header := p.do(t, "GET", "/playground", "", "http://evil.example", "")
	if status != 200 || header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("a read from another site: status %d, Access-Control-Allow-Origin %q; want 200 and *", status, header.Get("Access-Control-Allow-Origin"))
	}

	if conn, resp, err := p.dialWS(t, "/api/terminal", "http://evil.example", ""); err == nil {
		conn.CloseNow()
		t.Error("the terminal upgrade from another site was accepted under *")
	} else if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Errorf("terminal upgrade from another site: %v, want 403", resp)
	}

	query := `{"query":"mutation { writeTerminalInput(sessionId: \"__central__\", data: \"echo x\\n\") }"}`
	if status, body, _ := p.do(t, "POST", "/api/graphql", "", "http://evil.example", query); status != 403 {
		t.Errorf("a mutation from another site under *: status %d (%.100s), want 403", status, body)
	}

	if status, _, _ := p.doAs(t, fmt.Sprintf("evil.example:%d", p.port), "/playground"); status != 403 {
		t.Errorf("a request under a rebound host name under *: status %d, want 403", status)
	}
	if status, _, _ := p.doAs(t, fmt.Sprintf("localhost:%d", p.port), "/playground"); status != 200 {
		t.Errorf("a request under localhost under *: status %d, want 200", status)
	}
}
