package main

import (
	"encoding/json"
	"testing"
)

// doc parses a settings fragment the way the installer reads one.
func doc(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSetHookLeavesOtherHooksAlone(t *testing.T) {
	d := doc(t, `{
      "model": "opus",
      "hooks": {
        "PreToolUse": [{"hooks": [{"type": "command", "command": "somebody-else"}]}],
        "PermissionRequest": [{"hooks": [{"type": "command", "command": "another-tool check"}]}]
      }
    }`)
	if !setHook(d, "/usr/local/bin/steward", false) {
		t.Fatal("installing should report a change")
	}
	if d["model"] != "opus" {
		t.Error("unrelated keys must survive")
	}
	hooks := d["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("another event's hooks must not be touched")
	}
	list := hooks["PermissionRequest"].([]any)
	if len(list) != 2 {
		t.Fatalf("someone else's PermissionRequest hook must be kept, got %d entries", len(list))
	}

	// Installing twice must not add a second copy.
	if setHook(d, "/usr/local/bin/steward", false) {
		t.Error("installing again should report no change")
	}
	if len(d["hooks"].(map[string]any)["PermissionRequest"].([]any)) != 2 {
		t.Error("a second install must not duplicate the entry")
	}
}

func TestSetHookRemovesOnlyItsOwnEntry(t *testing.T) {
	d := doc(t, `{"hooks": {"PermissionRequest": [{"hooks": [{"type": "command", "command": "another-tool"}]}]}}`)
	setHook(d, "/bin/steward", false)
	if !setHook(d, "/bin/steward", true) {
		t.Fatal("removing should report a change")
	}
	list := d["hooks"].(map[string]any)["PermissionRequest"].([]any)
	if len(list) != 1 {
		t.Fatalf("the other tool's entry must remain, got %d", len(list))
	}
	if isStewardEntry(list[0]) {
		t.Error("the wrong entry was kept")
	}
	if setHook(d, "/bin/steward", true) {
		t.Error("removing again should report no change")
	}
}

func TestSetHookTidiesUpAfterItself(t *testing.T) {
	// When steward's entry was the only thing in the file, removing it should
	// not leave empty scaffolding behind.
	d := doc(t, `{}`)
	setHook(d, "/bin/steward", false)
	setHook(d, "/bin/steward", true)
	if _, ok := d["hooks"]; ok {
		t.Errorf("an empty hooks block should be removed, got %v", d)
	}
}

func TestSetHookRecognisesAHandEditedPath(t *testing.T) {
	// The entry is identified by the command it runs, not by where it sits,
	// so moving the binary or reordering the list still works.
	d := doc(t, `{"hooks": {"PermissionRequest": [
	    {"hooks": [{"type": "command", "command": "/opt/elsewhere/steward hook permission-request"}]}
	]}}`)
	if setHook(d, "/usr/local/bin/steward", false) {
		t.Error("an existing steward entry at another path should be recognised")
	}
	if !setHook(d, "/usr/local/bin/steward", true) {
		t.Error("and should be removable")
	}
}
