package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is what the user has agreed to let steward do. It lives beside the
// audit log rather than in Claude Code's settings, so that turning steward off
// never risks corrupting the file Claude Code needs to start.
type Config struct {
	// AutoAllow lets steward approve a call its rules cover instead of
	// deferring to the prompt. Off unless explicitly turned on.
	AutoAllow bool `json:"auto_allow"`

	// GuardBashWrites turns a Bash command that writes a file directly into a
	// prompt. Such a write is invisible to /rewind and to the Read cache, but
	// it is also sometimes exactly what you meant, so this asks rather than
	// refuses, and it is off until asked for.
	GuardBashWrites bool `json:"guard_bash_writes"`
}

// stewardDir is where the config and the audit log live.
func stewardDir() string {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return filepath.Join(v, "steward")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".steward"
	}
	return filepath.Join(home, ".claude", "steward")
}

func configPath() string { return filepath.Join(stewardDir(), "config.json") }

// loadConfig reads the config, treating anything unreadable as the safe
// default. A corrupt config must not silently grant permissions.
func loadConfig() Config {
	raw, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}
	}
	var c Config
	if json.Unmarshal(raw, &c) != nil {
		return Config{}
	}
	return c
}

func saveConfig(c Config) error {
	if err := os.MkdirAll(stewardDir(), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), append(raw, '\n'), 0o600)
}
