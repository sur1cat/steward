package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cmdAdopt takes someone else's ruleset and shows what accepting it would
// change before anything is written.
//
// Importing rules is granting permissions on somebody else's judgement, so the
// default is a preview: which rules are new, which you already have, and — the
// part that matters — which of theirs would widen what your agent may do. A
// deny rule of yours is never removed by an import.
func cmdAdopt(args []string) error {
	var from, project string
	var write, asJSON bool
	fs := flag.NewFlagSet("adopt", flag.ContinueOnError)
	fs.Usage = func() { fmt.Print(adoptUsage) }
	fs.StringVar(&from, "from", "", "the bundle to read")
	fs.StringVar(&project, "cwd", "", "adopt into the project in this directory")
	fs.BoolVar(&write, "write", false, "actually apply it")
	fs.BoolVar(&asJSON, "json", false, "machine-readable output")
	if err := fs.Parse(hoistFlags(fs, args)); err != nil {
		return err
	}
	if from == "" && fs.NArg() > 0 {
		from = fs.Arg(0)
	}
	if from == "" {
		return errors.New("give the bundle to adopt, e.g. steward adopt team-rules.json")
	}
	if project == "" {
		project, _ = os.Getwd()
	}

	raw, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	var b bundle
	if err := json.Unmarshal(raw, &b); err != nil {
		return fmt.Errorf("%s is not a steward bundle: %w", from, err)
	}
	if b.Version != bundleVersion {
		return fmt.Errorf("bundle version %d, this steward understands %d", b.Version, bundleVersion)
	}

	mine, err := rulesFor(project)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, r := range mine {
		have[r.Kind+"\x00"+r.Raw] = true
	}

	var add []incoming
	var already int
	for _, pair := range []struct {
		kind  string
		rules []string
	}{{"deny", b.Deny}, {"ask", b.Ask}, {"allow", b.Allow}} {
		for _, r := range pair.rules {
			if have[pair.kind+"\x00"+r] {
				already++
				continue
			}
			add = append(add, incoming{pair.kind, r, pair.kind == "allow"})
		}
	}

	if asJSON {
		return emitJSON(map[string]any{
			"from": from, "note": b.Note, "new": len(add), "already_have": already,
			"widening": countWidening(add), "written": write,
		})
	}

	fmt.Printf("%s  %s\n", bold("steward adopt"), from)
	if b.Note != "" {
		fmt.Printf("  %s\n", b.Note)
	}
	fmt.Printf("  made %s\n\n", b.CreatedAt.Format("2 Jan 2006"))
	if len(add) == 0 {
		fmt.Printf("  Nothing new — you already have all %d of these.\n", already)
		return nil
	}

	widening := countWidening(add)
	fmt.Printf("  %d rules are new, %d you already have\n", len(add), already)
	if widening > 0 {
		fmt.Printf("  %s %d of them widen what your agent may do\n\n", "!", widening)
	} else {
		fmt.Println()
	}
	for i, in := range add {
		if i >= 12 {
			fmt.Printf("  … and %d more\n", len(add)-12)
			break
		}
		mark := " "
		if in.Widens {
			mark = "!"
		}
		fmt.Printf("  %s %-5s %s\n", mark, in.Kind, in.Rule)
	}

	if !write {
		fmt.Printf("\n  Nothing written. %s\n",
			"steward adopt "+from+" --write applies it to .claude/settings.local.json")
		fmt.Printf("  %s\n", "Your own deny rules are never removed by an import.")
		return nil
	}

	target := filepath.Join(project, ".claude", "settings.local.json")
	if err := applyBundle(target, add2rules(add)); err != nil {
		return err
	}
	fmt.Printf("\n  added %d rules to %s\n", len(add), target)
	fmt.Printf("  %s\n", "run 'steward rules' to see the result in order")
	return nil
}

// incoming is one rule a bundle would add.
type incoming struct {
	Kind, Rule string
	// Widens marks a rule that grants something rather than restricting it.
	// Those are the ones worth reading before accepting somebody else's
	// judgement about what an agent may do.
	Widens bool
}

func countWidening(add []incoming) int {
	n := 0
	for _, a := range add {
		if a.Widens {
			n++
		}
	}
	return n
}

func add2rules(add []incoming) map[string][]string {
	out := map[string][]string{}
	for _, a := range add {
		out[a.Kind] = append(out[a.Kind], a.Rule)
	}
	return out
}

// applyBundle merges rules into a settings file, keeping every other key and
// every rule already there. An import adds; it never takes away.
func applyBundle(path string, add map[string][]string) error {
	doc := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%s does not parse, so it will not be edited: %w", path, err)
		}
	}
	perm, _ := doc["permissions"].(map[string]any)
	if perm == nil {
		perm = map[string]any{}
	}
	for kind, rules := range add {
		existing, _ := perm[kind].([]any)
		for _, r := range rules {
			existing = append(existing, r)
		}
		perm[kind] = existing
	}
	doc["permissions"] = perm

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	var probe map[string]any
	if json.Unmarshal(out, &probe) != nil {
		return errors.New("refusing to write settings that do not parse")
	}
	if raw, err := os.ReadFile(path); err == nil {
		backup := path + ".steward-backup-" + time.Now().Format("20060102-150405")
		if err := os.WriteFile(backup, raw, 0o600); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

const adoptUsage = `steward adopt — take someone else's permission rules

Importing rules is granting permissions on somebody else's judgement, so this
shows what would change and writes nothing until asked. Rules that widen what
your agent may do are marked, because those are the ones worth reading.

Usage:
  steward adopt team-rules.json           what it would change
  steward adopt team-rules.json --write   apply it

Flags:
      --from FILE   the bundle to read
      --cwd DIR     adopt into the project in this directory
      --write       apply it to .claude/settings.local.json, backing it up
      --json        machine-readable output

An import only adds. Your own deny rules are never removed by one.
`
