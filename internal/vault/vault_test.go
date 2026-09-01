package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func fileStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("STEWARD_NO_KEYCHAIN", "1")
	return New(filepath.Join(t.TempDir(), "secrets"))
}

func TestRoundTripAndForget(t *testing.T) {
	s := fileStore(t)
	if err := s.Set("DB_PASSWORD", "hunter2"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("DB_PASSWORD")
	if err != nil || got != "hunter2" {
		t.Fatalf("Get = %q %v", got, err)
	}
	names, _ := s.Names()
	if len(names) != 1 || names[0] != "DB_PASSWORD" {
		t.Errorf("Names = %v", names)
	}
	if err := s.Remove("DB_PASSWORD"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("DB_PASSWORD"); err != ErrNotFound {
		t.Errorf("a forgotten secret should be gone, got %v", err)
	}
}

func TestStoredFileIsNotReadableByAnyoneElse(t *testing.T) {
	s := fileStore(t)
	if err := s.Set("TOKEN", "abc"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.path("TOKEN"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600 — a secret readable by the group is not stored, it is published", perm)
	}
}

func TestNameIsConstrained(t *testing.T) {
	s := fileStore(t)
	// A name becomes an environment variable and a filename, so anything that
	// could escape a directory or break a shell is refused outright.
	for _, bad := range []string{"", "../escape", "has space", "has/slash", "1leading", "dash-name", "dollar$"} {
		if err := s.Set(bad, "x"); err == nil {
			t.Errorf("Set(%q) should have been refused", bad)
		}
		if _, err := s.Get(bad); err == nil {
			t.Errorf("Get(%q) should have been refused", bad)
		}
	}
	for _, ok := range []string{"A", "DB_PASSWORD", "token2", "_private"} {
		if err := s.Set(ok, "x"); err != nil {
			t.Errorf("Set(%q) = %v, want accepted", ok, err)
		}
	}
}

func TestEmptyValueIsRefused(t *testing.T) {
	s := fileStore(t)
	if err := s.Set("TOKEN", ""); err == nil {
		t.Error("storing an empty value silently would look like it worked")
	}
}

func TestMissingSecretIsNamedNotGuessed(t *testing.T) {
	s := fileStore(t)
	if _, err := s.Get("NEVER_SET"); err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
