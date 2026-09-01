package main

import (
	"reflect"
	"testing"
)

func TestHoistFlagsKeepsFlagsWithTheirValues(t *testing.T) {
	// Go's flag package stops at the first positional argument, so
	// `test "cmd" --event x` would ignore the flag entirely.
	got := hoistFlags([]string{"rm -rf build", "--event", "pre-tool-use"})
	want := []string{"--event", "pre-tool-use", "rm -rf build"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	// A boolean flag must not swallow the command being tested.
	got = hoistFlags([]string{"--json", "git status"})
	want = []string{"--json", "git status"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	// Everything after -- is left alone.
	got = hoistFlags([]string{"--", "--not-a-flag"})
	want = []string{"--", "--not-a-flag"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEveryValueFlagIsHoistable(t *testing.T) {
	// Every flag any subcommand declares with a value belongs here. Three
	// separate bugs have come from one being missing: the flag is silently
	// separated from its argument and the command reads the wrong thing.
	for _, n := range []string{"event", "tool", "cwd", "mode", "project", "since", "n", "to"} {
		if !flagTakesValue(n) {
			t.Errorf("--%s takes a value but is not hoistable", n)
		}
	}
	for _, n := range []string{"json", "write", "remove", "auto-allow", "mine"} {
		if flagTakesValue(n) {
			t.Errorf("--%s is a boolean and must not consume the next argument", n)
		}
	}
}
