package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestResolveTailscaleHost(t *testing.T) {
	status := func(json string) func() ([]byte, error) {
		return func() ([]byte, error) { return []byte(json), nil }
	}

	t.Run("running", func(t *testing.T) {
		ip, name, err := resolveTailscaleHost(status(`{"BackendState":"Running","Self":{"DNSName":"macm.banteng-cardassian.ts.net.","TailscaleIPs":["100.85.223.39","fd7a:115c:a1e0::e201:df8e"]}}`))
		if err != nil || ip != "100.85.223.39" || name != "macm.banteng-cardassian.ts.net" {
			t.Errorf("got (%q, %q, %v), want the first IPv4 and the name without its trailing dot", ip, name, err)
		}
	})

	t.Run("IPv4 after an IPv6 address", func(t *testing.T) {
		ip, _, err := resolveTailscaleHost(status(`{"BackendState":"Running","Self":{"DNSName":"a.ts.net.","TailscaleIPs":["fd7a:115c:a1e0::1","100.64.0.7"]}}`))
		if err != nil || ip != "100.64.0.7" {
			t.Errorf("got (%q, %v), want 100.64.0.7", ip, err)
		}
	})

	t.Run("a tailnet without a MagicDNS name", func(t *testing.T) {
		ip, name, err := resolveTailscaleHost(status(`{"BackendState":"Running","Self":{"TailscaleIPs":["100.64.0.7"]}}`))
		if err != nil || ip != "100.64.0.7" || name != "" {
			t.Errorf("got (%q, %q, %v), want the address and no name", ip, name, err)
		}
	})

	failures := []struct {
		name   string
		status func() ([]byte, error)
		want   string
	}{
		{"stopped", status(`{"BackendState":"Stopped","Self":{"DNSName":"a.ts.net.","TailscaleIPs":["100.64.0.7"]}}`), "--host tailscale: Tailscale is not running (state Stopped)"},
		{"needs login", status(`{"BackendState":"NeedsLogin"}`), "--host tailscale: Tailscale is not running (state NeedsLogin)"},
		{"IPv6 only", status(`{"BackendState":"Running","Self":{"DNSName":"a.ts.net.","TailscaleIPs":["fd7a:115c:a1e0::1"]}}`), "--host tailscale: no tailnet IPv4 address"},
		{"no address", status(`{"BackendState":"Running","Self":{}}`), "--host tailscale: no tailnet IPv4 address"},
		{"not JSON", status(`Tailscale is stopped.`), "--host tailscale: cannot read the output of tailscale status"},
		{"the CLI fails", func() ([]byte, error) { return nil, errors.New("--host tailscale: tailscale CLI not found") }, "--host tailscale: tailscale CLI not found"},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			if ip, name, err := resolveTailscaleHost(tt.status); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got (%q, %q, %v), want the error %q", ip, name, err, tt.want)
			}
		})
	}
}

// Under --host tailscale the links name the MagicDNS name, which another
// device of the tailnet resolves, and not the address the listener is bound to.
func TestStartupLinesNameTheTailnetHost(t *testing.T) {
	machine := []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.1.50"), Mask: net.CIDRMask(24, 32)}}
	opts := serveOptions{tailnetName: "macm.banteng-cardassian.ts.net"}

	a := mustAccessOpts(t, "100.85.223.39", opts)
	log := strings.Join(a.startupLines(nil, machine), "\n")
	for _, want := range []string{
		"[lif] Listening on 100.85.223.39:8080 (non-loopback address",
		"[lif] Open the UI: http://macm.banteng-cardassian.ts.net:8080/?token=" + a.token,
		"[lif] GraphQL Playground: http://macm.banteng-cardassian.ts.net:8080/playground?token=" + a.token,
		"[lif] Terminal: disabled",
		"[lif] Agent: disabled",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}

	opts.publicURL = mustPublicURL(t, "https://box.example.ts.net/lif")
	a = mustAccessOpts(t, "100.85.223.39", opts)
	if log := strings.Join(a.startupLines(nil, machine), "\n"); !strings.Contains(log, "[lif] Open the UI: https://box.example.ts.net/lif/?token="+a.token) || strings.Contains(log, "macm.banteng") {
		t.Errorf("a public URL wins over the tailnet name:\n%s", log)
	}
}

// fakeTailscale puts a tailscale CLI on the PATH that prints statusJSON for
// `tailscale status --json`.
func fakeTailscale(t *testing.T, statusJSON string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake tailscale CLI is a shell script")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\ncat <<'EOF'\n" + statusJSON + "\nEOF\n"
	if err := os.WriteFile(filepath.Join(dir, "tailscale"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// --host tailscale binds the address the tailscale CLI names. The tests give
// it a LAN address of this machine in place of a tailnet one.
func TestServeHostTailscale(t *testing.T) {
	addrs, _ := net.InterfaceAddrs()
	ip := firstGlobalUnicastIPv4(addrs)
	if ip == "" {
		t.Skip("this machine has no non-loopback IPv4 address")
	}
	fakeTailscale(t, fmt.Sprintf(`{"BackendState":"Running","Self":{"DNSName":"box.example.ts.net.","TailscaleIPs":["%s"]}}`, ip))
	p := startServeProcess(t, "--host", "tailscale")

	for _, want := range []string{
		fmt.Sprintf("[lif] Listening on %s:%d (non-loopback address", ip, p.port),
		fmt.Sprintf("[lif] Open the UI: http://box.example.ts.net:%d/?token=%s", p.port, p.token),
		"[lif] Terminal: disabled",
		"[lif] Agent: disabled",
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
	if status, _, _ := p.do(t, "GET", "/api/terminal", p.token, "", ""); status != 403 {
		t.Errorf("GET /api/terminal: status %d, want 403", status)
	}
}

func TestServeHostTailscaleNotRunning(t *testing.T) {
	fakeTailscale(t, `{"BackendState":"Stopped","Self":{"DNSName":"box.example.ts.net.","TailscaleIPs":["100.64.0.7"]}}`)
	bin := serveTestBinary(t)
	project := t.TempDir()
	store := filepath.Join(project, ".lif")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--lif-path", store, "--port", "0", "--host", "tailscale")
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("lif-serve started although Tailscale is stopped:\n%s", out)
	}
	if !strings.Contains(string(out), "--host tailscale: Tailscale is not running (state Stopped)") {
		t.Errorf("output lacks the cause:\n%s", out)
	}
}
