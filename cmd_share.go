package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sur1cat/pitwall/perms"
)

// bundle is a permission ruleset prepared for someone else to use.
//
// Sharing rules is the obvious way for a team to stop each person rebuilding
// the same allow list by clicking "Yes, and don't ask again" a thousand times.
// It is also the obvious way to hand someone your credentials by accident:
// 134 of the rules on the machine this was built for had one in the text. So
// a bundle is filtered, and what was left out is stated rather than silently
// dropped.
type bundle struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	From      string    `json:"from,omitempty"`
	Note      string    `json:"note,omitempty"`
	Allow     []string  `json:"allow,omitempty"`
	Deny      []string  `json:"deny,omitempty"`
	Ask       []string  `json:"ask,omitempty"`
	// Withheld counts what was removed on the way out, by reason, so the
	// person sharing can see that filtering happened.
	Withheld map[string]int `json:"withheld,omitempty"`
}

const bundleVersion = 1

// cmdShare exports the rules of a project in a form fit to hand to someone.
func cmdShare(args []string) error {
	var out, note, project string
	var asJSON bool
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	fs.Usage = func() { fmt.Print(shareUsage) }
	fs.StringVar(&out, "out", "", "write the bundle to this file instead of stdout")
	fs.StringVar(&note, "note", "", "a line describing what this ruleset is for")
	fs.StringVar(&project, "cwd", "", "export the rules in force in this directory")
	fs.BoolVar(&asJSON, "json", false, "machine-readable summary of what was exported")
	if err := fs.Parse(hoistFlags(fs, args)); err != nil {
		return err
	}
	if project == "" {
		project, _ = os.Getwd()
	}
	rules, err := rulesFor(project)
	if err != nil {
		return err
	}

	b := bundle{Version: bundleVersion, CreatedAt: time.Now(), Note: note, Withheld: map[string]int{}}
	seen := map[string]bool{}
	for _, r := range rules {
		// Everything the audit calls broken is left behind. A shared ruleset
		// should be the rules that work, not a copy of someone's clutter.
		if reason, drop := withhold(r); drop {
			b.Withheld[reason]++
			continue
		}
		key := r.Kind + "\x00" + r.Raw
		if seen[key] {
			b.Withheld["duplicate"]++
			continue
		}
		seen[key] = true
		switch r.Kind {
		case "allow":
			b.Allow = append(b.Allow, r.Raw)
		case "deny":
			b.Deny = append(b.Deny, r.Raw)
		case "ask":
			b.Ask = append(b.Ask, r.Raw)
		}
	}
	sort.Strings(b.Allow)
	sort.Strings(b.Deny)
	sort.Strings(b.Ask)

	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	if asJSON {
		return emitJSON(map[string]any{
			"allow": len(b.Allow), "deny": len(b.Deny), "ask": len(b.Ask),
			"withheld": b.Withheld,
		})
	}
	if out != "" {
		if err := os.WriteFile(out, raw, 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %d rules to %s\n", len(b.Allow)+len(b.Deny)+len(b.Ask), out)
	} else {
		os.Stdout.Write(raw)
	}
	if len(b.Withheld) > 0 {
		fmt.Fprintf(os.Stderr, "\nleft out: %s\n", describeWithheld(b.Withheld))
		if b.Withheld["secret"] > 0 {
			fmt.Fprintf(os.Stderr, "  %d carried a credential and were never going to be shared\n",
				b.Withheld["secret"])
		}
	}
	return nil
}

// withhold decides whether a rule may leave this machine, and why not.
func withhold(r perms.Rule) (string, bool) {
	for _, f := range perms.Lint([]perms.Rule{r}) {
		switch f.Category {
		case "secret":
			return "secret", true
		case "fragment", "unmatchable", "ignored", "never-consulted":
			return f.Category, true
		case "one-off":
			// A literal command from someone else's afternoon is noise in your
			// ruleset; it can only ever match that exact string again.
			return "one-off", true
		}
	}
	return "", false
}

func describeWithheld(w map[string]int) string {
	keys := make([]string, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return w[keys[i]] > w[keys[j]] })
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d %s", w[k], k))
	}
	return strings.Join(parts, ", ")
}

const shareUsage = `steward share — hand your permission rules to someone else

A team should not each rebuild the same allow list by clicking "Yes, and don't
ask again" a thousand times. Sharing is also how you hand someone a credential
by accident, so a bundle carries only rules that work and states what was left
behind.

Usage:
  steward share --out team-rules.json --note "backend service"
  steward share                          print it instead

Flags:
      --out FILE    write the bundle here
      --note TEXT   one line saying what this ruleset is for
      --cwd DIR     export the rules in force in this directory
      --json        a summary of what was exported, and what was not

Never exported: rules carrying a credential, rules that can never match, rules
Claude Code ignores, and one-off literals from someone else's afternoon.
Import one with steward adopt.
`
