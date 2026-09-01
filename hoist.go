package main

import "strings"

// hoistFlags moves flag arguments ahead of positional ones. Go's flag package
// stops parsing at the first non-flag argument, so `test "cmd" --event x`
// would silently ignore the flag. Anything after a bare "--" is left alone.
func hoistFlags(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) &&
				!strings.HasPrefix(args[i+1], "-") && flagTakesValue(strings.TrimLeft(a, "-")) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return append(flags, positional...)
}

// flagTakesValue lists the flags that consume the next argument. A boolean
// flag must not, or it would swallow the command being tested.
func flagTakesValue(name string) bool {
	switch name {
	case "event", "tool", "cwd", "mode", "project", "since", "n", "to":
		return true
	}
	return false
}
