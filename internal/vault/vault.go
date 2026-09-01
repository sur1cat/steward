// Package vault keeps secrets out of a coding agent's context.
//
// The problem is narrow and real: you want the agent to put a token in a file
// or run a command that needs one, and the moment you type the value it is in
// the transcript forever — readable, backed up, and replayed into the model on
// every later turn. On the machine this was built for, 113 distinct
// credentials had reached permission rules that way, and a database password
// appears in the prompt history twenty-three times.
//
// The rule here is that a secret has exactly two safe places: the store, and
// the environment of the process that needs it. It is never an argument, never
// printed, never returned to a caller that might log it. The agent handles the
// name; only the child process sees the value.
package vault

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ErrNotFound is returned when a name has no stored value.
var ErrNotFound = errors.New("no secret stored under that name")

// service is the Keychain service secrets are filed under.
const service = "steward-secrets"

// Store is where secrets live. On macOS that is the login Keychain, which is
// encrypted at rest and unlocked with the account password. Elsewhere it is a
// file readable only by the owner, which is weaker and says so.
type Store struct{ dir string }

// New opens the store, creating its directory when a file store is used.
func New(dir string) *Store { return &Store{dir: dir} }

// Backend names where secrets are actually kept, so the user can judge it.
func (s *Store) Backend() string {
	if useKeychain() {
		return "the macOS login Keychain"
	}
	return "a 0600 file in " + s.dir + " — weaker than a Keychain, and not encrypted at rest"
}

func useKeychain() bool {
	if runtime.GOOS != "darwin" || os.Getenv("STEWARD_NO_KEYCHAIN") != "" {
		return false
	}
	_, err := exec.LookPath("security")
	return err == nil
}

// Set stores a value. The value is written to the helper's stdin rather than
// passed as an argument, because an argument is visible to anyone who can run
// ps for as long as the process lives.
func (s *Store) Set(name, value string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if value == "" {
		return errors.New("refusing to store an empty value")
	}
	if useKeychain() {
		cmd := exec.Command("security", "add-generic-password",
			"-a", name, "-s", service, "-U", "-w")
		cmd.Stdin = strings.NewReader(value)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("keychain refused it: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path(name), []byte(value), 0o600)
}

// Get retrieves a value. Callers must treat the result as radioactive: put it
// in an environment or a file and let it go.
func (s *Store) Get(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	if useKeychain() {
		out, err := exec.Command("security", "find-generic-password",
			"-a", name, "-s", service, "-w").Output()
		if err != nil {
			return "", ErrNotFound
		}
		return strings.TrimRight(string(out), "\n"), nil
	}
	raw, err := os.ReadFile(s.path(name))
	if err != nil {
		return "", ErrNotFound
	}
	return string(raw), nil
}

// Remove forgets a secret.
func (s *Store) Remove(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if useKeychain() {
		if err := exec.Command("security", "delete-generic-password",
			"-a", name, "-s", service).Run(); err != nil {
			return ErrNotFound
		}
		return nil
	}
	if err := os.Remove(s.path(name)); err != nil {
		return ErrNotFound
	}
	return nil
}

// Names lists what is stored, without touching any value.
func (s *Store) Names() ([]string, error) {
	if useKeychain() {
		// The Keychain dump is parsed for account names only; values are not
		// requested, so none can leak through this path.
		out, err := exec.Command("security", "dump-keychain").Output()
		if err != nil {
			return nil, err
		}
		var names []string
		var lastAccount string
		for _, line := range strings.Split(string(out), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, `"acct"<blob>=`) {
				lastAccount = unquote(t)
			}
			if strings.HasPrefix(t, `"svce"<blob>=`) && unquote(t) == service && lastAccount != "" {
				names = append(names, lastAccount)
			}
		}
		sort.Strings(names)
		return names, nil
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (s *Store) path(name string) string { return filepath.Join(s.dir, name) }

// checkName keeps a name to what can be an environment variable and cannot
// escape a directory.
func checkName(name string) error {
	if name == "" {
		return errors.New("a secret needs a name")
	}
	for i, r := range name {
		ok := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9' && i > 0)
		if !ok {
			return fmt.Errorf("%q is not a usable name: letters, digits and underscores only", name)
		}
	}
	return nil
}

// unquote pulls the value out of a dump-keychain line.
func unquote(line string) string {
	i := strings.Index(line, `="`)
	if i < 0 {
		return ""
	}
	rest := line[i+2:]
	if j := strings.LastIndex(rest, `"`); j >= 0 {
		return rest[:j]
	}
	return ""
}
