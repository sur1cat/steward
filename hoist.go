package main

import (
	"flag"
	"strings"
)

// hoistFlags moves flag arguments ahead of positional ones. Go's flag package
// stops parsing at the first non-flag argument, so `share "note" --out f`
// would silently ignore the flag.
//
// Which flags consume the next argument is asked of the FlagSet rather than
// kept in a list. A hand-maintained list produced the same bug four separate
// times — a new value flag was added, nobody remembered the list, and the flag
// was quietly separated from its argument — and each time the fix was to add
// one more entry to the thing that keeps being forgotten.
func hoistFlags(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if !strings.Contains(a, "=") && i+1 < len(args) &&
				!strings.HasPrefix(args[i+1], "-") && takesValue(fs, name) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return append(flags, positional...)
}

// takesValue reports whether a flag consumes the next argument. A boolean flag
// does not, and would otherwise swallow whatever follows it.
func takesValue(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !(ok && b.IsBoolFlag())
}
