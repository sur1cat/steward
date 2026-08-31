// Package gate decides what to do with a tool call that Claude Code is about
// to ask you about.
//
// The decision is deliberately hard to get dangerously wrong. PermissionRequest
// fires only after Claude Code has already evaluated its own rules and settled
// on asking, and a matching deny rule blocks the call whatever a hook returns.
// So this package can turn a question into an approval or into a refusal, and
// it cannot turn a refusal into an approval. Everything it does not understand
// becomes Defer, which is the unchanged behaviour: Claude Code asks you.
package gate

import (
	"fmt"
	"strings"

	"github.com/sur1cat/pitwall/perms"
)

// Decision is what the hook reports back to Claude Code.
type Decision string

const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
	Defer Decision = "defer"
)

// Request is the part of the hook payload a decision depends on.
type Request struct {
	Tool  string
	Input map[string]any
	CWD   string
	Mode  string
}

// Result carries the decision and the reason for it. The reason is not
// decoration: a tool that silently decides what may run is one nobody should
// install, so every decision has to be explainable after the fact.
type Result struct {
	Decision Decision
	Reason   string
	Rule     string // the rule that decided it, when one did
	Subject  string // what was actually matched against
}

// Options control how far the gate is willing to go.
type Options struct {
	// AutoAllow lets a covered call through without asking. It is off by
	// default: approving on the user's behalf is the one thing here that can
	// do harm, and it should be a decision they made on purpose.
	AutoAllow bool
}

// subjectFields maps a tool to the input field its rules match against.
// A rule for any other field is ignored by Claude Code, so matching one here
// would enforce something the settings file does not actually say.
var subjectFields = map[string]string{
	"Bash":       "command",
	"PowerShell": "command",
	"Read":       "file_path",
	"Edit":       "file_path",
	"Write":      "file_path",
	"WebFetch":   "url",
}

// Subject pulls the text a rule would match against, and reports whether this
// tool has one at all.
func Subject(tool string, input map[string]any) (string, bool) {
	field, ok := subjectFields[tool]
	if !ok {
		return "", false
	}
	v, ok := input[field].(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// Evaluate reaches a decision for one request against a ruleset.
func Evaluate(req Request, rules []perms.Rule, opt Options) Result {
	// A mode that never prompts has already made the decision; adding to it
	// would be acting on a call the user never expected to be asked about.
	if req.Mode == "bypassPermissions" {
		return Result{Decision: Defer, Reason: "permission mode bypasses the prompt"}
	}
	subject, ok := Subject(req.Tool, req.Input)
	if !ok {
		return Result{Decision: Defer,
			Reason: fmt.Sprintf("no rule syntax covers %s, so there is nothing to check", req.Tool)}
	}

	scoped := forTool(rules, req.Tool)

	// Deny first, then ask, then allow — the order Claude Code documents. A
	// deny match here is belt and braces: Claude Code should have blocked the
	// call before the hook ran, so reaching this means its matcher disagreed.
	if r, ok := firstMatch(scoped, "deny", req.Tool, subject); ok {
		return Result{Decision: Deny, Rule: r.Raw, Subject: subject,
			Reason: "a deny rule covers this"}
	}
	if r, ok := firstMatch(scoped, "ask", req.Tool, subject); ok {
		return Result{Decision: Defer, Rule: r.Raw, Subject: subject,
			Reason: "an ask rule covers this, so it must be asked"}
	}
	if r, ok := firstMatch(scoped, "allow", req.Tool, subject); ok {
		if !opt.AutoAllow {
			return Result{Decision: Defer, Rule: r.Raw, Subject: subject,
				Reason: "an allow rule covers this, but auto-approval is off"}
		}
		return Result{Decision: Allow, Rule: r.Raw, Subject: subject,
			Reason: "an allow rule covers this"}
	}
	return Result{Decision: Defer, Subject: subject, Reason: "no rule covers this"}
}

// forTool narrows a ruleset to the rules that could apply to one tool.
func forTool(rules []perms.Rule, tool string) []perms.Rule {
	var out []perms.Rule
	for _, r := range rules {
		if r.Tool == tool || (r.Bare && r.Tool == tool) {
			out = append(out, r)
		}
	}
	return out
}

// firstMatch finds the first rule of a kind that covers the subject.
func firstMatch(rules []perms.Rule, kind, tool, subject string) (perms.Rule, bool) {
	for _, r := range rules {
		if r.Kind != kind {
			continue
		}
		if r.Bare {
			return r, true // a bare tool name matches every call to it
		}
		if matches(tool, r.Arg, subject) {
			return r, true
		}
	}
	return perms.Rule{}, false
}

// matches applies the right comparison for the tool. Only Bash is decided
// here; path and URL rules use anchoring that depends on which settings file a
// rule came from, and guessing at it would enforce the wrong thing.
func matches(tool, pattern, subject string) bool {
	switch tool {
	case "Bash", "PowerShell":
		return perms.MatchBash(pattern, subject)
	case "WebFetch":
		host := strings.TrimPrefix(pattern, "domain:")
		return host != pattern && hostOf(subject) == host
	default:
		return false
	}
}

// hostOf pulls the host out of a URL without parsing the whole thing.
func hostOf(raw string) string {
	s := raw
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}
