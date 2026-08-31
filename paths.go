package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// claudeDir is where Claude Code keeps its configuration.
func claudeDir() string {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// repoRoot resolves a directory to the root of its git repository, following
// worktrees back to the main checkout. Claude Code loads settings.local.json
// from there rather than from the working directory, so a worktree session
// reads the main checkout's local rules.
func repoRoot(dir string) string {
	if dir == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return dir
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return dir
	}
	return filepath.Dir(common)
}

// short collapses the home directory for display.
func short(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
