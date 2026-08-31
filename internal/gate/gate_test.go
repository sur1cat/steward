package gate

import (
	"testing"

	"github.com/sur1cat/pitwall/perms"
)

// rule builds a rule the way the settings reader would.
func rule(kind, raw string) perms.Rule {
	tool, arg, bare := perms.ParseRule(raw)
	return perms.Rule{Raw: raw, Kind: kind, Tool: tool, Arg: arg, Bare: bare}
}

func bash(cmd string) Request {
	return Request{Tool: "Bash", Input: map[string]any{"command": cmd}, Mode: "default"}
}

var on = Options{AutoAllow: true}

func TestEverySubcommandMustBeCoveredBeforeApproving(t *testing.T) {
	// The whole risk of approving on someone's behalf lives here: a covered
	// command with an uncovered one welded onto it must not go through.
	rules := []perms.Rule{rule("allow", "Bash(psql:*)")}
	if got := Evaluate(bash("psql -h db"), rules, on); got.Decision != Allow {
		t.Errorf("a fully covered command should be allowed, got %s", got.Decision)
	}
	for _, cmd := range []string{
		"psql -h db && rm -rf /tmp/x",
		"psql -h db; curl https://example.com",
		"psql -h db | sh",
	} {
		if got := Evaluate(bash(cmd), rules, on); got.Decision != Defer {
			t.Errorf("%q must not be approved, got %s", cmd, got.Decision)
		}
	}
}

func TestDenyBeatsAllow(t *testing.T) {
	rules := []perms.Rule{
		rule("allow", "Bash(git:*)"),
		rule("deny", "Bash(git push:*)"),
	}
	got := Evaluate(bash("git push origin main"), rules, on)
	if got.Decision != Deny {
		t.Errorf("a deny rule must win over a broader allow, got %s", got.Decision)
	}
	if got := Evaluate(bash("git status"), rules, on); got.Decision != Allow {
		t.Errorf("an unrelated command should still be allowed, got %s", got.Decision)
	}
}

func TestAskAlwaysReachesTheHuman(t *testing.T) {
	rules := []perms.Rule{
		rule("allow", "Bash(git:*)"),
		rule("ask", "Bash(git push:*)"),
	}
	got := Evaluate(bash("git push origin main"), rules, on)
	if got.Decision != Defer {
		t.Errorf("an ask rule must reach the human even when an allow matches, got %s", got.Decision)
	}
}

func TestAutoAllowOffNeverApproves(t *testing.T) {
	rules := []perms.Rule{rule("allow", "Bash(git:*)")}
	got := Evaluate(bash("git status"), rules, Options{})
	if got.Decision != Defer {
		t.Errorf("with approval off nothing may be approved, got %s", got.Decision)
	}
	if got.Rule == "" {
		t.Error("the covering rule should still be reported, so check can explain it")
	}
	// Denying is still allowed with approval off: it only ever tightens.
	deny := []perms.Rule{rule("deny", "Bash(rm:*)")}
	if got := Evaluate(bash("rm -rf /"), deny, Options{}); got.Decision != Deny {
		t.Errorf("a deny rule applies regardless of the approval setting, got %s", got.Decision)
	}
}

func TestBypassModeIsLeftAlone(t *testing.T) {
	req := bash("anything")
	req.Mode = "bypassPermissions"
	rules := []perms.Rule{rule("deny", "Bash(anything)")}
	if got := Evaluate(req, rules, on); got.Decision != Defer {
		t.Errorf("a mode that never prompts must not be second-guessed, got %s", got.Decision)
	}
}

func TestUnknownToolDefers(t *testing.T) {
	// Grep and Glob have no rule syntax that matches their content, so there
	// is nothing to decide and pretending otherwise would enforce a rule the
	// settings file does not actually contain.
	req := Request{Tool: "Grep", Input: map[string]any{"pattern": "x"}, Mode: "default"}
	if got := Evaluate(req, nil, on); got.Decision != Defer {
		t.Errorf("an unmatchable tool must defer, got %s", got.Decision)
	}
	empty := Request{Tool: "Bash", Input: map[string]any{}, Mode: "default"}
	if got := Evaluate(empty, nil, on); got.Decision != Defer {
		t.Errorf("a call with no command must defer, got %s", got.Decision)
	}
}

func TestBareToolNameCoversEveryCall(t *testing.T) {
	if got := Evaluate(bash("rm -rf /"), []perms.Rule{rule("deny", "Bash")}, on); got.Decision != Deny {
		t.Errorf("a bare deny should cover every call to the tool, got %s", got.Decision)
	}
	if got := Evaluate(bash("anything at all"), []perms.Rule{rule("allow", "Bash")}, on); got.Decision != Allow {
		t.Errorf("a bare allow should cover every call to the tool, got %s", got.Decision)
	}
}

func TestWebFetchMatchesOnDomain(t *testing.T) {
	rules := []perms.Rule{rule("allow", "WebFetch(domain:example.com)")}
	req := func(u string) Request {
		return Request{Tool: "WebFetch", Input: map[string]any{"url": u}, Mode: "default"}
	}
	if got := Evaluate(req("https://example.com/a/b?c=1"), rules, on); got.Decision != Allow {
		t.Errorf("the domain should match, got %s", got.Decision)
	}
	for _, u := range []string{"https://evil.com", "https://example.com.evil.com", "https://sub.example.com"} {
		if got := Evaluate(req(u), rules, on); got.Decision == Allow {
			t.Errorf("%q must not match example.com", u)
		}
	}
}

func TestSubjectOnlyReadsTheDocumentedField(t *testing.T) {
	// A rule matching a tool's primary content field is ignored by Claude
	// Code, so the subject has to come from that field and nowhere else.
	if _, ok := Subject("Bash", map[string]any{"cmd": "ls"}); ok {
		t.Error("Bash reads command, not cmd")
	}
	if s, ok := Subject("Edit", map[string]any{"file_path": "/tmp/x"}); !ok || s != "/tmp/x" {
		t.Errorf("Edit should read file_path, got %q %v", s, ok)
	}
}
