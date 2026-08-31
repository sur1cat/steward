package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/sur1cat/pitwall/perms"
)

// cmdRules prints the rules that apply where you are, in the order they are
// consulted. Knowing which file a rule came from is most of debugging one.
func cmdRules(args []string) error {
	var dir, kind string
	var asJSON bool
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.StringVar(&dir, "cwd", "", "resolve as if run in this directory")
	fs.StringVar(&kind, "kind", "", "only allow, deny or ask")
	fs.BoolVar(&asJSON, "json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	sources := perms.Discover(claudeDir(), []string{dir, repoRoot(dir)})
	var rules []perms.Rule
	for _, s := range sources {
		rules = append(rules, perms.Read(s)...)
	}
	if kind != "" {
		var kept []perms.Rule
		for _, r := range rules {
			if r.Kind == kind {
				kept = append(kept, r)
			}
		}
		rules = kept
	}

	if asJSON {
		out := make([]map[string]string, 0, len(rules))
		for _, r := range rules {
			out = append(out, map[string]string{
				"kind": r.Kind, "rule": r.Raw, "tool": r.Tool,
				"source": r.Source.Short(), "scope": r.Source.Scope.String(),
			})
		}
		return emitJSON(map[string]any{"cwd": dir, "rules": out})
	}

	fmt.Printf("%d rules in force in %s\n\n", len(rules), short(dir))
	counts := map[string]int{}
	byFile := map[string]int{}
	for _, r := range rules {
		counts[r.Kind]++
		byFile[r.Source.Short()]++
	}
	fmt.Printf("  %d allow · %d deny · %d ask\n\n", counts["allow"], counts["deny"], counts["ask"])

	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		fmt.Printf("  %4d  %s\n", byFile[f], f)
	}
	fmt.Printf("\n  deny is checked first, then ask, then allow — a deny rule wins\n")
	fmt.Printf("  over a narrower allow, and steward cannot override it.\n")
	return nil
}
