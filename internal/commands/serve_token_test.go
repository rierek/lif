package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/rierek/lif/internal/cors"
)

// writeTokenFile writes content to a new token file with the given mode,
// whatever the umask says.
func writeTokenFile(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadToken(t *testing.T) {
	long := func(c string) string { return strings.Repeat(c, 43) }

	t.Run("a missing file is created and read again", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "lif", "serve-token") // the directory is missing too
		token, source, err := loadToken(path, "")
		if err != nil {
			t.Fatal(err)
		}
		if source != path || len(token) < minTokenLength {
			t.Fatalf("loadToken = %q from %q, want a usable token from %q", token, source, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Errorf("token file mode = %04o, want 0600", info.Mode().Perm())
		}

		again, source, err := loadToken(path, "")
		if err != nil || again != token || source != path {
			t.Errorf("second load = %q from %q (%v), want %q from %q", again, source, err, token, path)
		}
	})

	t.Run("a file other users can read is refused", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows has no file modes")
		}
		for _, mode := range []os.FileMode{0o644, 0o640, 0o604} {
			path := writeTokenFile(t, long("a")+"\n", mode)
			_, _, err := loadToken(path, "")
			want := fmt.Sprintf("token file %s is accessible by other users (mode %04o); run chmod 600 %s", path, mode, path)
			if err == nil || err.Error() != want {
				t.Errorf("mode %04o: err = %v, want %q", mode, err, want)
			}
		}
	})

	t.Run("a short token is refused", func(t *testing.T) {
		path := writeTokenFile(t, "short\n", 0o600)
		if _, _, err := loadToken(path, ""); err == nil || !strings.Contains(err.Error(), "token from "+path+" is shorter than 32 characters") {
			t.Errorf("short file token: err = %v", err)
		}
		if _, _, err := loadToken("", "short"); err == nil || !strings.Contains(err.Error(), "token from LIF_SERVE_TOKEN is shorter than 32 characters") {
			t.Errorf("short environment token: err = %v", err)
		}
	})

	t.Run("the flag beats the environment", func(t *testing.T) {
		path := writeTokenFile(t, long("a")+"\n", 0o600)
		token, source, err := loadToken(path, long("b"))
		if err != nil || token != long("a") || source != path {
			t.Errorf("loadToken = %q from %q (%v), want the token of the file", token, source, err)
		}
	})

	t.Run("the environment variable", func(t *testing.T) {
		token, source, err := loadToken("", " "+long("b")+"\n")
		if err != nil || token != long("b") || source != "LIF_SERVE_TOKEN" {
			t.Errorf("loadToken = %q from %q (%v), want the trimmed token from LIF_SERVE_TOKEN", token, source, err)
		}
	})

	t.Run("without a choice the token is random", func(t *testing.T) {
		first, source, err := loadToken("", "")
		if err != nil || source != "" || len(first) < minTokenLength {
			t.Fatalf("loadToken = %q from %q (%v), want a random token without a source", first, source, err)
		}
		if second, _, _ := loadToken("", ""); second == first {
			t.Error("two random tokens are equal")
		}
	})
}

func tokenRouter(a serveAccess) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(a.middleware(cors.NewChecker(a.withOwnAddress(nil)))...)
	router.GET("/page", func(c *gin.Context) { c.String(http.StatusOK, "page") })
	return router
}

func tokenGet(router http.Handler, target string, mod func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8080"+target, nil)
	if mod != nil {
		mod(req)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// A restart with the same token file gives the same token, so the cookie a
// browser got from the first start still opens the server.
func TestPersistentTokenKeepsTheBrowserSession(t *testing.T) {
	opts := serveOptions{requireToken: true, tokenFile: filepath.Join(t.TempDir(), "token")}

	first := mustAccessOpts(t, "127.0.0.1", opts)
	rec := tokenGet(tokenRouter(first), "/page?token="+first.token, nil)
	if rec.Code != http.StatusSeeOther || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("opening the printed address: status %d, %d cookies; want a 303 that sets one cookie", rec.Code, len(rec.Result().Cookies()))
	}
	cookie := rec.Result().Cookies()[0]

	second := mustAccessOpts(t, "127.0.0.1", opts) // the restart
	if second.token != first.token {
		t.Fatal("the restart changed the token")
	}
	if rec := tokenGet(tokenRouter(second), "/page", func(r *http.Request) { r.AddCookie(cookie) }); rec.Code != http.StatusOK {
		t.Errorf("the cookie of the first start after the restart: status %d, want 200", rec.Code)
	}
}

// On loopback without --require-token there is no token, and neither flag nor
// variable changes that.
func TestTokenSourcesAreIgnoredWithoutATokenRequirement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	a := mustAccessOpts(t, "127.0.0.1", serveOptions{tokenFile: path, envToken: strings.Repeat("e", 40)})
	if a.token != "" {
		t.Errorf("a plain loopback listener got the token %q", a.token)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the token file was created on a plain loopback listener: %v", err)
	}
}

// US-16: opting into a remote terminal or agent gives a new token, so a client
// that got the plain token before gets 401 now. The new token is stable for
// the same opt-ins.
func TestRemoteOptInsDeriveANewTokenFromAPersistentOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	with := func(opts serveOptions) serveAccess {
		opts.tokenFile = path
		return mustAccessOpts(t, "0.0.0.0", opts)
	}
	plain := with(serveOptions{})
	terminal := with(serveOptions{allowRemoteTerminal: true})
	agent := with(serveOptions{allowRemoteAgent: true})
	both := with(serveOptions{allowRemoteTerminal: true, allowRemoteAgent: true})

	if plain.tokenDerived {
		t.Error("the token of a server without opt-ins is derived")
	}
	seen := map[string]string{plain.token: "plain"}
	for name, a := range map[string]serveAccess{"terminal": terminal, "agent": agent, "both": both} {
		if !a.tokenDerived {
			t.Errorf("%s: the token is not derived", name)
		}
		if other, dup := seen[a.token]; dup {
			t.Errorf("%s and %s share a token", name, other)
		}
		seen[a.token] = name
	}
	if again := with(serveOptions{allowRemoteTerminal: true}); again.token != terminal.token {
		t.Error("the derived token changed with a restart")
	}

	router := tokenRouter(terminal)
	bearer := func(token string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
	}
	if rec := tokenGet(router, "/page", bearer(plain.token)); rec.Code != http.StatusUnauthorized {
		t.Errorf("the plain token after the opt-in: status %d, want 401", rec.Code)
	}
	if rec := tokenGet(router, "/page", bearer(terminal.token)); rec.Code != http.StatusOK {
		t.Errorf("the derived token: status %d, want 200", rec.Code)
	}

	// A token the server made up has nothing to derive from: it is new at every
	// start anyway.
	if a := mustAccessOpts(t, "0.0.0.0", serveOptions{allowRemoteTerminal: true}); a.tokenDerived {
		t.Error("a random token is marked as derived")
	}
}

func TestStartupLinesForAPersistentToken(t *testing.T) {
	lines := func(a serveAccess) []string { return a.startupLines(nil, nil) }
	path := filepath.Join(t.TempDir(), "token")

	t.Run("token file", func(t *testing.T) {
		a := mustAccessOpts(t, "10.0.0.9", serveOptions{tokenFile: path})
		got := lines(a)
		for _, want := range []string{
			"[lif] Access token: read from " + path,
			"[lif] Open the UI: http://10.0.0.9:8080/",
			"[lif] GraphQL Playground: http://10.0.0.9:8080/playground",
		} {
			if !slices.Contains(got, want) {
				t.Errorf("log lacks %q:\n%s", want, strings.Join(got, "\n"))
			}
		}
		if log := strings.Join(got, "\n"); strings.Contains(log, a.token) || strings.Contains(log, "?token=") {
			t.Errorf("the log prints the token of the file:\n%s", log)
		}
	})

	t.Run("environment variable", func(t *testing.T) {
		a := mustAccessOpts(t, "10.0.0.9", serveOptions{envToken: strings.Repeat("e", 40)})
		if want := "[lif] Access token: read from LIF_SERVE_TOKEN"; !slices.Contains(lines(a), want) {
			t.Errorf("log lacks %q:\n%s", want, strings.Join(lines(a), "\n"))
		}
	})

	t.Run("derived for the remote opt-ins", func(t *testing.T) {
		a := mustAccessOpts(t, "10.0.0.9", serveOptions{tokenFile: path, allowRemoteTerminal: true})
		got := lines(a)
		for _, want := range []string{
			"[lif] Access token: " + a.token + " (derived from " + path + " for the remote opt-ins)",
			"[lif] Open the UI: http://10.0.0.9:8080/?token=" + a.token,
			"[lif] GraphQL Playground: http://10.0.0.9:8080/playground?token=" + a.token,
		} {
			if !slices.Contains(got, want) {
				t.Errorf("log lacks %q:\n%s", want, strings.Join(got, "\n"))
			}
		}
	})
}
