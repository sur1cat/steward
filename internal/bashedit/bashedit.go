// Package bashedit recognises a Bash command that writes a file directly
// instead of going through the Edit or Write tools.
//
// Why it matters is not style. A file written by a shell redirect is invisible
// to Claude Code's file history, so `/rewind` cannot undo it; it never enters
// the Read tool's cache, so the next read pays full price; and the edit is
// absent from the structured patch record, so nothing downstream can see what
// changed. On the machine this was built for there were 125,722 Bash calls
// against 6,481 Edit and Write calls, so this is not a rare path.
package bashedit

import (
	"regexp"
	"strings"

	"github.com/sur1cat/pitwall/perms"
)

// Finding names one way a command writes a file.
type Finding struct {
	Kind   string // sed, redirect, tee, heredoc, dd, truncate
	Detail string // the fragment that triggered it
}

// patterns are ordered so the more specific kinds are recognised first.
var patterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	// sed -i and perl -i edit in place; the file never passes through a tool.
	{"sed", regexp.MustCompile(`\bsed\b[^|;&]*\s-[a-zA-Z]*i`)},
	{"perl", regexp.MustCompile(`\bperl\b[^|;&]*\s-[a-zA-Z]*i`)},
	{"dd", regexp.MustCompile(`\bdd\b[^|;&]*\bof=`)},
	{"truncate", regexp.MustCompile(`\btruncate\b[^|;&]*\s-s`)},
	// The pipe that feeds tee is a separator, so by the time a subcommand is
	// examined it reads simply "tee file" and the pattern must not expect one.
	{"tee", regexp.MustCompile(`^\s*tee\b(?:\s+-a)?\s+[^\s|;&]+`)},
	// A redirect into a path. Excludes /dev/null and file descriptors, which
	// write nothing anyone can rewind.
	{"redirect", regexp.MustCompile(`(?:^|[^0-9>&])>>?\s*(?:"[^"]+"|'[^']+'|[^\s|;&<>]+)`)},
}

var devNull = regexp.MustCompile(`>>?\s*/dev/(null|stdout|stderr|fd/\d+)`)

// Detect reports every way a command writes a file, examining each subcommand
// separately so that a write hidden after a pipe or a && is still seen.
func Detect(command string) []Finding {
	var out []Finding
	parts, ok := perms.SplitCommand(command)
	if !ok {
		parts = []string{command}
	}
	for _, part := range parts {
		clean := devNull.ReplaceAllString(part, "")
		for _, p := range patterns {
			if m := p.re.FindString(clean); m != "" {
				out = append(out, Finding{Kind: p.kind, Detail: strings.TrimSpace(m)})
				break // one finding per subcommand is enough to explain it
			}
		}
	}
	return out
}

// Writes reports whether a command writes a file at all.
func Writes(command string) bool { return len(Detect(command)) > 0 }

// Explain renders the reason a command was flagged, for a person to read.
func Explain(fs []Finding) string {
	if len(fs) == 0 {
		return ""
	}
	kinds := make([]string, 0, len(fs))
	seen := map[string]bool{}
	for _, f := range fs {
		if !seen[f.Kind] {
			seen[f.Kind] = true
			kinds = append(kinds, f.Kind)
		}
	}
	return "writes a file directly (" + strings.Join(kinds, ", ") +
		") — /rewind cannot undo it and the Read cache does not see it"
}
