package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sur1cat/steward/internal/audit"
)

// cmdStatus reports what steward is currently doing, in a form a user
// interface can read. Everything the panel needs to draw its switches and its
// recent activity comes from here, so that the panel never has to reason about
// settings files itself.
func cmdStatus(args []string) error {
	var asJSON bool
	var since time.Duration
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.BoolVar(&asJSON, "json", false, "machine-readable output")
	fs.DurationVar(&since, "since", 24*time.Hour, "window for the decision counts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := loadConfig()
	installed, where := hookInstalled()
	entries, _ := audit.Read(stewardDir(), time.Now().Add(-since))

	counts := map[string]int{"allow": 0, "deny": 0, "defer": 0}
	for _, e := range entries {
		counts[e.Decision]++
	}
	recent := entries
	if len(recent) > 20 {
		recent = recent[len(recent)-20:]
	}

	if asJSON {
		return emitJSON(map[string]any{
			"installed": installed, "settings": where,
			"auto_allow":  cfg.AutoAllow,
			"since_hours": since.Hours(),
			"counts":      counts, "total": len(entries),
			"recent": recent,
		})
	}

	state := "not installed"
	if installed {
		state = "installed in " + short(where)
	}
	fmt.Printf("hook        %s\n", state)
	if cfg.AutoAllow {
		fmt.Printf("approval    on — a call your rules cover is approved without asking\n")
	} else {
		fmt.Printf("approval    off — steward watches and records, you still see every prompt\n")
	}
	if cfg.GuardBashWrites {
		fmt.Printf("write guard on — a Bash command that writes a file directly asks first\n")
	} else {
		fmt.Printf("write guard off — direct file writes are recorded, not questioned\n")
	}
	fmt.Printf("decisions   %d in the last %s — %d allowed · %d denied · %d asked\n",
		len(entries), since, counts["allow"], counts["deny"], counts["defer"])
	return nil
}

// hookInstalled reports whether steward's entry is present in the settings
// file, and which file was examined.
func hookInstalled() (bool, string) {
	path := filepath.Join(claudeDir(), "settings.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, path
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		return false, path
	}
	hooks, _ := doc["hooks"].(map[string]any)
	list, _ := hooks["PermissionRequest"].([]any)
	for _, e := range list {
		if isStewardEntry(e) {
			return true, path
		}
	}
	return false, path
}
