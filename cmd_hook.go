package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sur1cat/pitwall/perms"
	"github.com/sur1cat/steward/internal/audit"
	"github.com/sur1cat/steward/internal/gate"
)

// hookInput is the payload Claude Code writes to the hook's stdin.
type hookInput struct {
	SessionID  string         `json:"session_id"`
	CWD        string         `json:"cwd"`
	Mode       string         `json:"permission_mode"`
	Event      string         `json:"hook_event_name"`
	ToolName   string         `json:"tool_name"`
	ToolInput  map[string]any `json:"tool_input"`
	ToolUseID  string         `json:"tool_use_id"`
	AgentType  string         `json:"agent_type"`
	Transcript string         `json:"transcript_path"`
	PromptID   string         `json:"prompt_id"`
}

// hookOutput is the envelope Claude Code reads back.
type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName string `json:"hookEventName"`
		Decision      string `json:"decision"`
	} `json:"hookSpecificOutput"`
}

// cmdHook answers a Claude Code hook. It is written so that every path that
// can go wrong ends in "defer": Claude Code then asks the way it always did.
// Exit codes are meaningless for this event, so the decision travels entirely
// in the JSON, and a malformed answer is treated by Claude Code as no answer.
func cmdHook(args []string) error {
	event := ""
	if len(args) > 0 {
		event = args[0]
	}
	switch event {
	case "permission-request", "PermissionRequest":
	case "pre-tool-use", "PreToolUse":
		return hookPreToolUse()
	default:
		return fmt.Errorf("unknown hook event %q", event)
	}

	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 8<<20))
	if err != nil {
		return emit(gate.Defer)
	}
	var in hookInput
	if json.Unmarshal(raw, &in) != nil {
		return emit(gate.Defer)
	}

	res := decide(in)

	// The record is written before the answer so that a decision can never be
	// acted on without also being logged.
	_ = audit.Append(stewardDir(), audit.Entry{
		At: time.Now(), Session: in.SessionID, Tool: in.ToolName,
		Subject: res.Subject, Decision: string(res.Decision),
		Reason: res.Reason, Rule: res.Rule, CWD: in.CWD, Agent: in.AgentType,
	})
	return emit(res.Decision)
}

// decide loads the rules that apply where the call was made and evaluates it.
// Any failure to read the rules yields Defer rather than a guess.
func decide(in hookInput) gate.Result {
	cfg := loadConfig()
	rules, err := rulesFor(in.CWD)
	if err != nil {
		return gate.Result{Decision: gate.Defer, Reason: "could not read the permission rules"}
	}
	return gate.Evaluate(gate.Request{
		Tool: in.ToolName, Input: in.ToolInput, CWD: in.CWD, Mode: in.Mode,
	}, rules, gate.Options{AutoAllow: cfg.AutoAllow})
}

// rulesFor resolves the settings that apply in a directory, in precedence
// order: the managed file first, then user, then the project's shared file,
// then its local one.
func rulesFor(cwd string) ([]perms.Rule, error) {
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return nil, err
		}
	}
	var out []perms.Rule
	for _, src := range perms.Discover(claudeDir(), []string{cwd, repoRoot(cwd)}) {
		out = append(out, perms.Read(src)...)
	}
	return out, nil
}

func emit(d gate.Decision) error {
	var out hookOutput
	out.HookSpecificOutput.HookEventName = "PermissionRequest"
	out.HookSpecificOutput.Decision = string(d)
	return json.NewEncoder(os.Stdout).Encode(out)
}
