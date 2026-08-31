package main

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/sur1cat/steward/internal/audit"
	"github.com/sur1cat/steward/internal/bashedit"
)

// preToolOutput is the envelope PreToolUse reads back. Unlike
// PermissionRequest, this event does honour a decision that turns a silent
// call into a prompt.
type preToolOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision,omitempty"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

// hookPreToolUse watches for a Bash command that writes a file directly. It
// asks rather than refuses: writing a file from the shell is sometimes exactly
// what was meant, and a guard that blocks legitimate work gets uninstalled.
// Silence is the default — without the setting it only records.
func hookPreToolUse() error {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 8<<20))
	if err != nil {
		return emitPreTool("", "")
	}
	var in hookInput
	if json.Unmarshal(raw, &in) != nil {
		return emitPreTool("", "")
	}
	if in.ToolName != "Bash" {
		return emitPreTool("", "")
	}
	command, _ := in.ToolInput["command"].(string)
	found := bashedit.Detect(command)
	if len(found) == 0 {
		return emitPreTool("", "")
	}

	reason := bashedit.Explain(found)
	decision := ""
	if loadConfig().GuardBashWrites {
		decision = "ask"
	}
	_ = audit.Append(stewardDir(), audit.Entry{
		At: time.Now(), Session: in.SessionID, Tool: "Bash",
		Subject: command, Decision: decisionLabel(decision),
		Reason: reason, CWD: in.CWD, Agent: in.AgentType,
	})
	return emitPreTool(decision, reason)
}

// decisionLabel names what was recorded, so the log distinguishes a call that
// was turned into a prompt from one that was merely noticed.
func decisionLabel(decision string) string {
	if decision == "" {
		return "noted"
	}
	return decision
}

// emitPreTool answers the hook. An empty decision means "no opinion", which
// leaves the call exactly as it would have been.
func emitPreTool(decision, reason string) error {
	var out preToolOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	if decision != "" {
		out.HookSpecificOutput.PermissionDecision = decision
		out.HookSpecificOutput.PermissionDecisionReason = reason
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
