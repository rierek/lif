package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// tailscaleHost is the value of --host that stands for this machine on the
// tailnet.
const tailscaleHost = "tailscale"

// macTailscaleCLI is the command line tool of the Tailscale app on macOS, which
// is not on the PATH unless the user installed it there.
const macTailscaleCLI = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"

// resolveTailscaleHost reads the output of `tailscale status --json` and
// returns the first tailnet IPv4 address of this machine, the address
// --host tailscale binds, and its MagicDNS name without the trailing dot,
// under which another tailnet device opens the server (empty when the tailnet
// has none).
func resolveTailscaleHost(statusJSON func() ([]byte, error)) (ip, dnsName string, err error) {
	out, err := statusJSON()
	if err != nil {
		return "", "", err
	}
	var status struct {
		BackendState string
		Self         struct {
			DNSName      string
			TailscaleIPs []string
		}
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return "", "", fmt.Errorf("--host tailscale: cannot read the output of tailscale status: %w", err)
	}
	if status.BackendState != "Running" {
		return "", "", fmt.Errorf("--host tailscale: Tailscale is not running (state %s)", status.BackendState)
	}
	for _, s := range status.Self.TailscaleIPs {
		if addr, err := netip.ParseAddr(s); err == nil && addr.Is4() {
			return s, strings.TrimSuffix(status.Self.DNSName, "."), nil
		}
	}
	return "", "", errors.New("--host tailscale: no tailnet IPv4 address")
}

// tailscaleStatusJSON runs `tailscale status --json`.
func tailscaleStatusJSON() ([]byte, error) {
	bin, err := exec.LookPath("tailscale")
	if err != nil {
		if _, statErr := os.Stat(macTailscaleCLI); runtime.GOOS != "darwin" || statErr != nil {
			return nil, errors.New("--host tailscale: tailscale CLI not found")
		}
		bin = macTailscaleCLI
	}
	out, err := exec.Command(bin, "status", "--json").Output()
	if err != nil {
		reason := err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(bytes.TrimSpace(exitErr.Stderr)) > 0 {
			reason = string(bytes.TrimSpace(exitErr.Stderr))
		}
		return nil, fmt.Errorf("--host tailscale: tailscale status failed: %s", reason)
	}
	return out, nil
}
