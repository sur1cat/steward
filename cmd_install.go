package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// cmdInstall wires the hook into Claude Code's settings, and is also where the
// one consequential setting is turned on. It edits only the hooks block and
// backs up the file first: a settings file that fails to parse stops Claude
// Code from starting, and that is a far worse outcome than an unwired hook.
func cmdInstall(args []string) error {
	var remove, autoAllow, noAutoAllow, printOnly bool
	var guardWrites, noGuardWrites bool
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.BoolVar(&remove, "remove", false, "take the hook back out")
	fs.BoolVar(&autoAllow, "auto-allow", false, "let steward approve calls your rules already cover")
	fs.BoolVar(&noAutoAllow, "no-auto-allow", false, "go back to only watching and recording")
	fs.BoolVar(&guardWrites, "guard-writes", false, "ask before a Bash command writes a file directly")
	fs.BoolVar(&noGuardWrites, "no-guard-writes", false, "stop asking about direct file writes")
	fs.BoolVar(&printOnly, "print", false, "show the change without writing it")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if guardWrites || noGuardWrites {
		cfg := loadConfig()
		cfg.GuardBashWrites = guardWrites && !noGuardWrites
		if !printOnly {
			if err := saveConfig(cfg); err != nil {
				return err
			}
		}
		if cfg.GuardBashWrites {
			fmt.Println("A Bash command that writes a file directly will now ask first.")
			fmt.Println("Such a write is invisible to /rewind and to the Read cache.")
		} else {
			fmt.Println("Direct file writes will be recorded but not questioned.")
		}
		if !cfg.GuardBashWrites && !remove && !autoAllow {
			return nil
		}
	}

	if autoAllow || noAutoAllow {
		cfg := loadConfig()
		cfg.AutoAllow = autoAllow && !noAutoAllow
		if !printOnly {
			if err := saveConfig(cfg); err != nil {
				return err
			}
		}
		// Turning approval off is not a reason to wire the hook in. Without
		// this, switching the second toggle off in a user interface silently
		// switched the first one on.
		if !cfg.AutoAllow && !remove {
			fmt.Println("steward will watch and record; every prompt you would have seen, you see.")
			return nil
		}
		if cfg.AutoAllow {
			fmt.Println("steward will now approve a call when your own rules already cover it.")
			fmt.Println("It still cannot override a deny rule, and every decision is recorded:")
			fmt.Println("  steward log")
		} else {
			fmt.Println("steward will watch and record; every prompt you would have seen, you see.")
		}
	}

	path := filepath.Join(claudeDir(), "settings.json")
	doc := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%s does not parse, so it will not be edited: %w", short(path), err)
		}
	}

	bin, err := os.Executable()
	if err != nil || bin == "" {
		if bin, err = exec.LookPath("steward"); err != nil {
			return fmt.Errorf("cannot find the steward binary to reference")
		}
	}
	changed := setHook(doc, bin, remove)
	// The write guard rides on a second event. Both go in together: a guard
	// that is installed but never consulted is worse than one that is absent,
	// because the settings file says it is watching.
	if setEventHook(doc, "PreToolUse", bin+" hook pre-tool-use", remove) {
		changed = true
	}
	if !changed {
		if remove {
			fmt.Println("The hook was not installed; nothing to remove.")
		} else {
			fmt.Println("The hook is already installed.")
		}
		return nil
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if printOnly {
		fmt.Printf("%s\n\n%s", short(path), out)
		return nil
	}
	if err := backup(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return err
	}
	if remove {
		fmt.Printf("Removed the hook from %s\n", short(path))
		return nil
	}
	fmt.Printf("Wired into %s\n\n", short(path))
	fmt.Println("steward now sees every call Claude Code was going to ask you about.")
	fmt.Println("It records them and changes nothing until you say so:")
	fmt.Println()
	fmt.Println("  steward log                     what it has seen")
	fmt.Println("  steward check \"git push\"        what would happen to a command")
	fmt.Println("  steward install --auto-allow    let it answer for you")
	return nil
}

// setHook adds or removes steward's PermissionRequest entry, leaving every
// other hook in the file untouched. It reports whether anything changed.
func setHook(doc map[string]any, bin string, remove bool) bool {
	return setEventHook(doc, "PermissionRequest", bin+" hook permission-request", remove)
}

// setEventHook adds or removes one steward entry under one event name.
func setEventHook(doc map[string]any, event, command string, remove bool) bool {
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		if remove {
			return false
		}
		hooks = map[string]any{}
	}
	list, _ := hooks[event].([]any)

	var kept []any
	found := false
	for _, entry := range list {
		if isStewardEntry(entry) {
			found = true
			continue
		}
		kept = append(kept, entry)
	}
	if remove {
		if !found {
			return false
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
		if len(hooks) == 0 {
			delete(doc, "hooks")
		} else {
			doc["hooks"] = hooks
		}
		return true
	}
	if found {
		return false
	}
	kept = append(kept, map[string]any{
		"hooks": []any{map[string]any{
			"type":    "command",
			"command": command,
		}},
	})
	hooks[event] = kept
	doc["hooks"] = hooks
	return true
}

// isStewardEntry recognises an entry this command wrote, by the command it
// runs rather than by position, so a hand-edited file is still understood.
func isStewardEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	inner, _ := m["hooks"].([]any)
	for _, h := range inner {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, _ := hm["command"].(string); ownsCommand(cmd) {
			return true
		}
	}
	return false
}

// ownsCommand reports whether a hook command is one steward wrote. It looks at
// both halves: the subcommand only steward defines, and the name of the binary
// being run. Matching the subcommand alone would risk removing another tool's
// entry; matching a fixed path would miss our own after a move or a rename,
// and a missed entry means the next install silently adds a duplicate.
func ownsCommand(cmd string) bool {
	if !strings.Contains(cmd, "hook permission-request") &&
		!strings.Contains(cmd, "hook pre-tool-use") {
		return false
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(filepath.Base(fields[0])), "steward")
}

// backup copies the settings file next to itself before it is rewritten.
func backup(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := fmt.Sprintf("%s.steward-backup-%s", path, time.Now().Format("20060102-150405"))
	return os.WriteFile(dst, raw, 0o600)
}
