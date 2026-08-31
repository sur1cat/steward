// Package audit records every decision the gate makes.
//
// This is what makes the rest of the tool defensible. Asking someone to let a
// binary decide what their agent may execute is a large request, and the thing
// that makes it reasonable is being able to read afterwards exactly what it
// decided and why. The log is append-only, one JSON object per line, and it is
// the first thing to look at when something behaves unexpectedly.
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry is one recorded decision.
type Entry struct {
	At       time.Time `json:"at"`
	Session  string    `json:"session,omitempty"`
	Tool     string    `json:"tool"`
	Subject  string    `json:"subject"`
	Decision string    `json:"decision"`
	Reason   string    `json:"reason"`
	Rule     string    `json:"rule,omitempty"`
	CWD      string    `json:"cwd,omitempty"`
	Agent    string    `json:"agent,omitempty"`
}

// Append writes one entry. A failure here must never change what the hook
// decides: losing a log line is a nuisance, refusing a tool call because the
// disk is full is a broken session.
func Append(dir string, e Entry) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "decisions.jsonl"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// Read returns the entries recorded since a cutoff, newest last.
func Read(dir string, since time.Time) ([]Entry, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "decisions.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	dec := json.NewDecoder(newLineReader(raw))
	for {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			break
		}
		if e.At.Before(since) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
