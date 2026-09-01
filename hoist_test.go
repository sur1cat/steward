package main

import (
	"flag"
	"reflect"
	"testing"
)

func TestHoistFlagsAsksTheFlagSet(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	var out, note string
	var write bool
	fs.StringVar(&out, "out", "", "")
	fs.StringVar(&note, "note", "", "")
	fs.BoolVar(&write, "write", false, "")

	// The bug this replaces: --note was not on a hand-kept list, so its value
	// became positional and --out swallowed the wrong argument.
	got := hoistFlags(fs, []string{"--note", "a note", "--out", "f.json"})
	want := []string{"--note", "a note", "--out", "f.json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// A boolean must not consume what follows it.
	got = hoistFlags(fs, []string{"--write", "positional"})
	want = []string{"--write", "positional"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// Positional first is the case the whole function exists for.
	got = hoistFlags(fs, []string{"rm -rf build", "--out", "f.json"})
	want = []string{"--out", "f.json", "rm -rf build"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// An unknown flag is left alone rather than guessed at.
	got = hoistFlags(fs, []string{"--unknown", "value"})
	want = []string{"--unknown", "value"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// Everything after -- is untouched.
	got = hoistFlags(fs, []string{"--", "--not-a-flag"})
	want = []string{"--", "--not-a-flag"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
