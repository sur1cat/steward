package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// The file is named cmd_trial.go rather than cmd_test.go because Go excludes
// anything ending in _test.go from a build, which silently dropped this whole
// command from the binary.
//
// cmdTest runs a hook against a payload without Claude Code being involved.
//
// Hooks are the one part of a Claude Code setup with no way to try it: you
// change a script, start a session, and find out from behaviour whether it
// worked. Both community tools for this are abandoned. Since steward already
// answers these events, it can answer them on demand too.
func cmdTest(args []string) error {
	var tool, cwd, mode string
	var event string
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() { fmt.Print(testUsage) }
	fs.StringVar(&event, "event", "permission-request", "which hook to answer")
	fs.StringVar(&tool, "tool", "Bash", "the tool the call would use")
	fs.StringVar(&cwd, "cwd", "", "run as if the session were in this directory")
	fs.StringVar(&mode, "mode", "default", "the permission mode of the session")
	if err := fs.Parse(hoistFlags(args)); err != nil {
		return err
	}
	subject := strings.Join(fs.Args(), " ")
	if subject == "" {
		return fmt.Errorf(`give the call to test, e.g. steward test "rm -rf build"`)
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	field := map[string]string{"Bash": "command", "PowerShell": "command",
		"Read": "file_path", "Edit": "file_path", "Write": "file_path", "WebFetch": "url"}[tool]
	if field == "" {
		field = "command"
	}
	payload, err := json.Marshal(map[string]any{
		"session_id": "steward-test", "cwd": cwd, "permission_mode": mode,
		"hook_event_name": hookEventName(event), "tool_name": tool,
		"tool_input": map[string]any{field: subject}, "tool_use_id": "toolu_test",
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s\n%s\n\n%s\n", bold("what Claude Code would send"), string(payload),
		bold("what steward answers"))

	// The payload goes through the same entry point Claude Code uses, so a
	// dry run and the real thing cannot diverge.
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	w.Close()
	stdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = stdin }()
	return cmdHook([]string{event})
}

// hookEventName maps the command-line spelling to the wire name.
func hookEventName(event string) string {
	switch event {
	case "pre-tool-use", "PreToolUse":
		return "PreToolUse"
	default:
		return "PermissionRequest"
	}
}

func bold(s string) string { return "\033[1m" + s + "\033[0m" }

const testUsage = `steward test — answer a hook without starting a session

Hooks are the one part of a Claude Code setup you cannot try: you change a
script, start a session, and learn from behaviour whether it worked. This sends
the payload Claude Code would send, through the same code that answers the real
event, and prints both.

Usage:
  steward test "CMD"                    what the permission hook would answer
  steward test --event pre-tool-use "CMD"   what the write guard would answer

Flags:
      --event NAME   permission-request (default) or pre-tool-use
      --tool NAME    the tool the call would use (default Bash)
      --cwd DIR      run as if the session were in this directory
      --mode NAME    the session's permission mode
`
