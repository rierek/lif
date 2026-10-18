package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rierek/lif/internal/filelock"
	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/pkg/config"
	"gopkg.in/yaml.v3"
)

// trustFileName is the file in ~/.lif that maps trust keys to fingerprints.
const trustFileName = "allowed-hooks.yml"

// Key names the project config at configPath in the trust file. It is the
// same in every worktree of a repository: the path that the config has in the
// main checkout. Symlinks are resolved, so the physical path that the TUI
// passes and the path that a shell reports give one key. The main checkout of
// a bare repository is the bare directory, so repositories that share a parent
// directory keep separate keys. Outside a git checkout, and in a directory
// whose .git file names a worktree that is not its own, the key is the
// absolute path of the config.
func Key(configPath string) string {
	abs, err := filepath.Abs(configPath)
	if err != nil {
		abs = filepath.Clean(configPath)
	}
	dir, name := resolve(filepath.Dir(abs)), filepath.Base(abs)
	top, ok := gitutil.Toplevel(dir)
	if !ok {
		return filepath.Join(dir, name)
	}
	root, ok := gitutil.MainCheckoutRoot(dir)
	if !ok { // a .git file that is no checkout of the repository it names
		return filepath.Join(dir, name)
	}
	rel, err := filepath.Rel(resolve(top), filepath.Join(dir, name))
	if err != nil {
		return filepath.Join(dir, name)
	}
	return filepath.Join(resolve(root), rel)
}

// resolve returns the absolute path with symlinks resolved; a path that
// cannot be resolved stays as it is.
func resolve(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

// Fingerprint identifies the hooks and tasks of cfg. Comments and formatting
// of the file do not change it. It covers the steps written in the config,
// not the scripts that they call, and an edit of any task changes it.
func Fingerprint(cfg *config.Config) string {
	data, _ := json.Marshal(struct { // cannot fail: strings, string lists, maps of those
		Hooks map[string]config.Hook
		Tasks map[string]config.Task
	}{cfg.Hooks, cfg.Tasks})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Allowed reports whether the trust file holds fp for key. A missing file and
// an unset HOME mean that nothing is allowed.
func Allowed(key, fp string) (bool, error) {
	trusted, err := readTrust()
	if err != nil {
		return false, err
	}
	return isAllowed(trusted, key, fp), nil
}

// Allow stores fp for key. A trust file that does not parse is replaced.
func Allow(key, fp string) error {
	if _, err := trustPath(); err != nil {
		return err
	}
	return updateTrust(func(trusted map[string]string) bool {
		trusted[key] = fp
		return true
	})
}

// Deny removes key from the trust file. A key that is not there is no error.
// A trust file that does not parse is replaced.
func Deny(key string) error {
	if _, err := trustPath(); err != nil {
		return nil // no HOME: nothing is allowed
	}
	return updateTrust(func(trusted map[string]string) bool {
		_, ok := trusted[key]
		delete(trusted, key)
		return ok
	})
}

// updateTrust applies change to the trust file under a lock, so that parallel
// allows and denies do not lose each other's update. change reports whether
// it changed the map; an unchanged map is not written, except to replace a
// corrupt file.
func updateTrust(change func(map[string]string) bool) error {
	path, err := trustPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	lock, err := filelock.Acquire(lockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("%s: %w", lockPath, err)
	}
	defer lock.Release()

	trusted, err := readTrust()
	var corrupt *CorruptTrustError
	switch {
	case errors.As(err, &corrupt):
		trusted = nil
	case err != nil:
		return err
	}
	if trusted == nil {
		trusted = make(map[string]string)
	}
	if !change(trusted) && corrupt == nil {
		return nil
	}
	return writeTrust(trusted)
}

// CorruptTrustError reports a trust file that cannot be parsed. `lif hooks
// allow` and `lif hooks deny` replace such a file.
type CorruptTrustError struct {
	Path string
	Err  error
}

func (e *CorruptTrustError) Error() string {
	return fmt.Sprintf("%s: %v (run 'lif hooks deny' or 'lif hooks allow' to replace the file; this drops all allows)", e.Path, e.Err)
}

func (e *CorruptTrustError) Unwrap() error { return e.Err }

// TrustFileProblem returns a *CorruptTrustError when the trust file does not
// parse, so that a command can say that it replaces the file. It is nil for a
// good file, a missing file and any other read error.
func TrustFileProblem() error {
	_, err := readTrust()
	var corrupt *CorruptTrustError
	if errors.As(err, &corrupt) {
		return err
	}
	return nil
}

func isAllowed(trusted map[string]string, key, fp string) bool {
	got, ok := trusted[key]
	return ok && got == fp
}

// trustPath returns ~/.lif/allowed-hooks.yml.
func trustPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, config.HomeDir, trustFileName), nil
}

// readTrust returns the trust file as a map from key to fingerprint. No HOME
// and no file mean an empty map; a file that cannot be read or parsed is an
// error that starts with its path.
func readTrust() (map[string]string, error) {
	path, err := trustPath()
	if err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var trusted map[string]string
	if err := yaml.Unmarshal(data, &trusted); err != nil {
		return nil, &CorruptTrustError{Path: path, Err: err}
	}
	return trusted, nil
}

// writeTrust replaces the trust file through a temp file in the same
// directory and a rename, so a reader never sees half a file. The file is
// private (0600), the directory is created private (0700).
func writeTrust(trusted map[string]string) error {
	path, err := trustPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(trusted)
	if err != nil {
		return err
	}
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target // a dotfile manager's link stays a link
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp") // created with mode 0600
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op after the rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
