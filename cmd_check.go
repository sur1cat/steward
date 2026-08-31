package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sur1cat/steward/internal/gate"
)

// cmdCheck answers what would happen to a command without running anything.
// An enforcement tool you cannot interrogate is one nobody should trust, so
// this is the command to reach for whenever a decision looks wrong.
func cmdCheck(args []string) error {
	var tool, dir string
	var asJSON bool
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.StringVar(&tool, "tool", "Bash", "the tool the call would use")
	fs.StringVar(&dir, "cwd", "", "evaluate as if run in this directory")
	fs.BoolVar(&asJSON, "json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	subject := strings.Join(fs.Args(), " ")
	if subject == "" {
		return fmt.Errorf(`give the call to check, e.g. steward check "git push origin main"`)
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}

	rules, err := rulesFor(dir)
	if err != nil {
		return err
	}
	cfg := loadConfig()
	field := map[string]string{"Bash": "command", "PowerShell": "command",
		"Read": "file_path", "Edit": "file_path", "Write": "file_path", "WebFetch": "url"}[tool]
	if field == "" {
		field = "command"
	}
	res := gate.Evaluate(gate.Request{
		Tool: tool, Input: map[string]any{field: subject}, CWD: dir,
	}, rules, gate.Options{AutoAllow: cfg.AutoAllow})

	if asJSON {
		return emitJSON(map[string]any{
			"tool": tool, "subject": subject, "decision": string(res.Decision),
			"reason": res.Reason, "rule": res.Rule, "auto_allow": cfg.AutoAllow,
			"rules_considered": len(rules), "cwd": dir,
		})
	}

	fmt.Printf("%s %s\n", label(res.Decision), subject)
	fmt.Printf("  %s\n", res.Reason)
	if res.Rule != "" {
		fmt.Printf("  rule: %s\n", res.Rule)
	}
	fmt.Printf("  %d rules in force in %s\n", len(rules), short(dir))
	if res.Decision == gate.Defer && res.Rule != "" && !cfg.AutoAllow {
		fmt.Printf("  turn on approval with: steward install --auto-allow\n")
	}
	return nil
}

func label(d gate.Decision) string {
	switch d {
	case gate.Allow:
		return "ALLOW "
	case gate.Deny:
		return "DENY  "
	default:
		return "ASK   "
	}
}
