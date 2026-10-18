package commands

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/pkg/config"
)

// writeSelfSignedCert writes a self-signed ECDSA certificate for localhost and
// 127.0.0.1, and its key, as PEM files into a temporary directory.
func writeSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

// insecureTransport trusts any certificate: the one of the tests is self-signed.
func insecureTransport() *http.Transport {
	return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
}

func TestLoadTLSConfig(t *testing.T) {
	cert, key := writeSelfSignedCert(t)

	if cfg, err := loadTLSConfig("", ""); cfg != nil || err != nil {
		t.Errorf("neither flag: %v, %v; want no TLS and no error", cfg, err)
	}
	for _, args := range [][2]string{{cert, ""}, {"", key}} {
		if _, err := loadTLSConfig(args[0], args[1]); err == nil || err.Error() != "--tls-cert and --tls-key must be given together" {
			t.Errorf("cert %q, key %q: err = %v, want the must-be-given-together error", args[0], args[1], err)
		}
	}
	if _, err := loadTLSConfig(cert, filepath.Join(t.TempDir(), "missing.pem")); err == nil || !strings.HasPrefix(err.Error(), "failed to load the TLS certificate: ") {
		t.Errorf("missing key file: err = %v, want the load error", err)
	}
	if _, err := loadTLSConfig(key, cert); err == nil || !strings.HasPrefix(err.Error(), "failed to load the TLS certificate: ") {
		t.Errorf("swapped files: err = %v, want the load error", err)
	}

	cfg, err := loadTLSConfig(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) != 1 || cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("config has %d certificates and MinVersion %#x, want 1 and TLS 1.2", len(cfg.Certificates), cfg.MinVersion)
	}
}

// The token cookie is Secure when the token travels encrypted: the server
// speaks TLS itself, or the public URL of the proxy is https. Plain loopback
// and a plain http public URL keep it without.
func TestTokenCookieIsSecureWhenTheTokenTravelsEncrypted(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		opts serveOptions
		want bool
	}{
		{"network without TLS", "0.0.0.0", serveOptions{}, false},
		{"network with TLS", "0.0.0.0", serveOptions{tlsCert: "c.pem", tlsKey: "k.pem"}, true},
		{"loopback behind an https public URL", "127.0.0.1", serveOptions{publicURL: mustPublicURL(t, "https://box.example.ts.net/lif")}, true},
		{"loopback behind an http public URL", "127.0.0.1", serveOptions{publicURL: mustPublicURL(t, "http://box.example.ts.net/lif")}, false},
		{"loopback with --require-token", "127.0.0.1", serveOptions{requireToken: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mustAccessOpts(t, tt.ip, tt.opts)
			if got := a.secure(); got != tt.want {
				t.Errorf("secure() = %v, want %v", got, tt.want)
			}
			rec := tokenGet(tokenRouter(a), "/page?token="+a.token, nil)
			cookies := rec.Result().Cookies()
			if rec.Code != http.StatusSeeOther || len(cookies) != 1 {
				t.Fatalf("status %d, %d cookies; want a 303 that sets one cookie", rec.Code, len(cookies))
			}
			if cookies[0].Secure != tt.want {
				t.Errorf("cookie Secure = %v, want %v", cookies[0].Secure, tt.want)
			}
		})
	}
}

func TestStartupLinesNameTheSchemeAndDropTheClearTextWarningUnderTLS(t *testing.T) {
	lines := func(a serveAccess) string {
		return strings.Join(a.startupLines(config.DefaultCORSOrigins, nil), "\n")
	}

	plain := lines(mustAccess(t, "10.0.0.9", false))
	if !strings.Contains(plain, "There is no TLS, so the token travels in clear text.") || !strings.Contains(plain, "[lif] Open the UI: http://10.0.0.9:8080/?token=") {
		t.Errorf("a plain network listener lost its clear-text warning or its http link:\n%s", plain)
	}

	tlsLog := lines(mustAccessOpts(t, "10.0.0.9", serveOptions{tlsCert: "c.pem", tlsKey: "k.pem"}))
	if strings.Contains(tlsLog, "no TLS") || !strings.Contains(tlsLog, "[lif] Open the UI: https://10.0.0.9:8080/?token=") ||
		!strings.Contains(tlsLog, "[lif] GraphQL Playground: https://10.0.0.9:8080/playground?token=") {
		t.Errorf("a TLS listener prints a clear-text warning or no https links:\n%s", tlsLog)
	}

	loopback := lines(mustAccessOpts(t, "127.0.0.1", serveOptions{tlsCert: "c.pem", tlsKey: "k.pem"}))
	if !strings.Contains(loopback, "[lif] Starting server at https://127.0.0.1:8080/") {
		t.Errorf("a TLS loopback listener names no https address:\n%s", loopback)
	}

	behindProxy := lines(mustAccessOpts(t, "127.0.0.1", serveOptions{publicURL: mustPublicURL(t, "https://box.example.ts.net")}))
	if strings.Contains(behindProxy, "no TLS") {
		t.Errorf("an https public URL still warns about clear text:\n%s", behindProxy)
	}
	plainProxy := lines(mustAccessOpts(t, "127.0.0.1", serveOptions{publicURL: mustPublicURL(t, "http://box.example.ts.net")}))
	if !strings.Contains(plainProxy, "There is no TLS") {
		t.Errorf("an http public URL lost the clear-text warning:\n%s", plainProxy)
	}
}

// The own address of a loopback listener is an https origin under TLS.
func TestWithOwnAddressFollowsTheScheme(t *testing.T) {
	a := mustAccessOpts(t, "::1", serveOptions{tlsCert: "c.pem", tlsKey: "k.pem"})
	got := a.withOwnAddress([]string{"http://localhost:*"})
	if want := []string{"http://localhost:*", "https://[::1]:*"}; !slices.Equal(got, want) {
		t.Errorf("withOwnAddress = %v, want %v", got, want)
	}
}

// --tls-cert and --tls-key serve the UI, GraphQL and its WebSocket over HTTPS.
// The token cookie is Secure, the log has no clear-text warning, a page of the
// http origin of the same address is not the server's own, and plain HTTP to
// the port gets no page.
func TestServeOverTLS(t *testing.T) {
	cert, key := writeSelfSignedCert(t)
	p := startServeProcess(t, "--host", "0.0.0.0", "--tls-cert", cert, "--tls-key", key)

	if len(p.token) < 32 {
		t.Fatalf("no usable access token in the startup log:\n%s", p.log)
	}
	if regexp.MustCompile(`\[lif\] Open the UI: https://[^/\s]+:\d+/\?token=`).FindString(p.log) == "" {
		t.Errorf("startup log has no https \"Open the UI\" line:\n%s", p.log)
	}
	if strings.Contains(p.log, "There is no TLS") {
		t.Errorf("startup log warns about clear text under TLS:\n%s", p.log)
	}

	if status, _, _ := p.do(t, "GET", "/playground", "", "", ""); status != 401 {
		t.Errorf("GET /playground over HTTPS without token: status %d, want 401", status)
	}
	if status, body, _ := p.do(t, "GET", "/playground", p.token, "", ""); status != 200 || !strings.Contains(body, "<title>Lif GraphQL</title>") {
		t.Errorf("GET /playground over HTTPS with the token: status %d, body %.200q", status, body)
	}
	status, _, header := p.do(t, "GET", "/playground?token="+p.token, "", "", "")
	if cookie := header.Get("Set-Cookie"); status != http.StatusSeeOther || !strings.Contains(cookie, "Secure") {
		t.Errorf("the printed address: status %d, Set-Cookie %q; want a 303 with a Secure cookie", status, cookie)
	}

	// The page the server served is of the origin https://<the Host it was sent
	// to>. Under a name outside the origin list that is the server's own origin
	// over https only.
	query := `{"query":"{ __typename }"}`
	own := fmt.Sprintf("nas.example:%d", p.port)
	post := func(origin string) int {
		req, err := http.NewRequest("POST", p.url("/api/graphql"), strings.NewReader(query))
		if err != nil {
			t.Fatal(err)
		}
		req.Host = own
		req.Header.Set("Authorization", "Bearer "+p.token)
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		status, _, _ := send(t, req)
		return status
	}
	if status := post("https://" + own); status != 200 {
		t.Errorf("GraphQL query from the https UI: status %d, want 200", status)
	}
	if status := post("http://" + own); status != 403 {
		t.Errorf("GraphQL query from the http origin of the same address: status %d, want 403", status)
	}

	// The UI at https://127.0.0.1:<port> is of the server's own origin as well:
	// its WebSocket needs no entry in the origin list.
	conn, resp, err := p.dialWS(t, "/api/graphql", "https://"+p.host, p.token, "graphql-transport-ws")
	if err != nil {
		t.Fatalf("GraphQL WebSocket from the https UI: %v (response %v)", err, resp)
	}
	conn.CloseNow()

	plain, err := http.Get("http://" + p.host + "/playground")
	if err == nil {
		defer plain.Body.Close()
		if plain.StatusCode == http.StatusOK {
			t.Errorf("plain HTTP to the TLS port got a page: status %d", plain.StatusCode)
		}
	}
}
