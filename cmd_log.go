package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/sur1cat/steward/internal/audit"
)

// cmdLog prints what steward decided. This is the answer to "why did that go
// through" and to "what has it been doing", and it is the reason the rest of
// the tool is reasonable to install at all.
func cmdLog(args []string) error {
	var since time.Duration
	var asJSON bool
	var limit int
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	fs.DurationVar(&since, "since", 24*time.Hour, "only decisions from the last span")
	fs.IntVar(&limit, "n", 40, "how many to show")
	fs.BoolVar(&asJSON, "json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	entries, err := audit.Read(stewardDir(), time.Now().Add(-since))
	if err != nil {
		return err
	}
	if asJSON {
		return emitJSON(map[string]any{"entries": entries})
	}
	if len(entries) == 0 {
		fmt.Printf("Nothing recorded in the last %s.\n", since)
		fmt.Println("steward records a decision only when Claude Code asks for one;")
		fmt.Println("run 'steward install' if you have not wired the hook in yet.")
		return nil
	}

	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Decision]++
	}
	fmt.Printf("%d decisions in the last %s — %d allowed · %d denied · %d asked\n\n",
		len(entries), since, counts["allow"], counts["deny"], counts["defer"])

	start := 0
	if len(entries) > limit {
		start = len(entries) - limit
		fmt.Printf("  showing the last %d\n\n", limit)
	}
	for _, e := range entries[start:] {
		subject := e.Subject
		if len(subject) > 68 {
			subject = subject[:65] + "…"
		}
		fmt.Printf("  %s  %-6s %-10s %s\n", e.At.Format("15:04:05"),
			e.Decision, e.Tool, subject)
		if e.Rule != "" {
			fmt.Printf("             %s\n", e.Rule)
		}
	}
	return nil
}
