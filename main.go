// Command steward decides what a coding agent may execute, and records what it
// decided.
//
// It is the half of the job pitwall deliberately does not do. pitwall runs when
// you run it and only reads; steward runs when Claude Code runs, on every tool
// call that needs a decision. That difference is why they are separate
// binaries: one asks you to look at numbers, the other asks for far more trust,
// and bundling them would make the safe half harder to adopt.
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := ""
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "hook":
		return cmdHook(args)
	case "check":
		return cmdCheck(args)
	case "rules":
		return cmdRules(args)
	case "log":
		return cmdLog(args)
	case "status":
		return cmdStatus(args)
	case "install":
		return cmdInstall(args)
	case "version", "--version", "-v":
		fmt.Println("steward", version)
		return nil
	case "", "help", "--help", "-h":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q — try 'steward help'", cmd)
	}
}

const usage = `steward — decide what your agent may execute, and record it

Usage:
  steward check "CMD"    what would happen to this command, and why
  steward rules          the permission rules in force here, in order
  steward log            every decision made, newest last
  steward status         whether the hook is wired in, and what it has decided
  steward install        wire the hook into Claude Code
  steward hook EVENT     answer a hook (Claude Code calls this, not you)
  steward version        print the version

steward only sees a call that Claude Code was already going to ask you about:
its own rules run first, and a deny rule blocks the call whatever a hook says.
So steward can answer a question for you or refuse outright, and it cannot turn
a refusal into an approval.

Approving on your behalf is off until you turn it on:

  steward install --auto-allow

Without it steward watches and records, and every prompt you would have seen,
you still see.
`
