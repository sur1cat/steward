package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sur1cat/steward/internal/vault"
)

// cmdSecret keeps a value out of the agent's context while still letting the
// agent use it.
//
// The shape follows from where a secret leaks. Typing it into a prompt puts it
// in the transcript; passing it as an argument puts it in the transcript and in
// ps; printing it puts it in the scrollback and in the next request. So the
// value is typed once into a terminal the agent is not reading, stored, and
// afterwards referred to only by name. The agent writes `steward secret use
// DB_PASSWORD -- psql …`, which is safe to record because it contains nothing.
func cmdSecret(args []string) error {
	sub := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	store := vault.New(filepath.Join(stewardDir(), "secrets"))
	switch sub {
	case "set":
		return secretSet(store, args)
	case "use", "run":
		return secretUse(store, args)
	case "write":
		return secretWrite(store, args)
	case "list", "ls":
		return secretList(store)
	case "forget", "rm":
		return secretForget(store, args)
	default:
		fmt.Print(secretUsage)
		return nil
	}
}

// secretSet reads a value from the terminal without echoing it. It refuses to
// read from a pipe, because the only reason a value would arrive that way is a
// command line that already leaked it.
func secretSet(store *vault.Store, args []string) error {
	if len(args) != 1 {
		return errors.New(`give one name, e.g. steward secret set DB_PASSWORD`)
	}
	name := args[0]
	if !isTerminal() {
		return errors.New("a secret is only read from a terminal — run this yourself,\n" +
			"not through an agent, so the value never reaches a transcript")
	}
	value, err := readHidden(fmt.Sprintf("value for %s (not shown): ", name))
	if err != nil {
		return err
	}
	if err := store.Set(name, value); err != nil {
		return err
	}
	fmt.Printf("stored %s in %s\n", name, store.Backend())
	fmt.Printf("  use it with: steward secret use %s -- your-command\n", name)
	return nil
}

// secretUse runs a command with the named secrets in its environment. The
// value is placed in the child's environment, never in its arguments, so it
// appears in no transcript and in no process listing.
func secretUse(store *vault.Store, args []string) error {
	var names []string
	i := 0
	for ; i < len(args); i++ {
		if args[i] == "--" {
			i++
			break
		}
		names = append(names, args[i])
	}
	rest := args[i:]
	if len(names) == 0 || len(rest) == 0 {
		return errors.New(`usage: steward secret use NAME [NAME…] -- command args…`)
	}

	env := os.Environ()
	for _, n := range names {
		v, err := store.Get(n)
		if err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		env = append(env, n+"="+v)
	}
	cmd := exec.Command(rest[0], rest[1:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	return nil
}

// secretWrite puts NAME=value into a file, which is what a .env needs. The
// value passes from the store to the file without being printed.
func secretWrite(store *vault.Store, args []string) error {
	var to string
	fs := flag.NewFlagSet("secret write", flag.ContinueOnError)
	fs.StringVar(&to, "to", ".env", "the file to write into")
	if err := fs.Parse(hoistFlags(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`give one name, e.g. steward secret write DB_PASSWORD --to .env`)
	}
	name := fs.Arg(0)
	value, err := store.Get(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	lines, err := readLines(to)
	if err != nil {
		return err
	}
	replaced := false
	for i, l := range lines {
		if strings.HasPrefix(l, name+"=") {
			lines[i] = name + "=" + value
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, name+"="+value)
	}
	body := strings.Join(lines, "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.WriteFile(to, []byte(body), 0o600); err != nil {
		return err
	}
	action := "added"
	if replaced {
		action = "replaced"
	}
	// The value is deliberately not echoed back, not even partially.
	fmt.Printf("%s %s in %s (mode 0600)\n", action, name, to)
	return nil
}

func secretList(store *vault.Store) error {
	names, err := store.Names()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Printf("Nothing stored yet, in %s\n", store.Backend())
		fmt.Println("  steward secret set NAME     type a value in, once, yourself")
		return nil
	}
	fmt.Printf("%d stored in %s\n\n", len(names), store.Backend())
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}
	fmt.Printf("\nNames only. No command here prints a value.\n")
	return nil
}

func secretForget(store *vault.Store, args []string) error {
	if len(args) != 1 {
		return errors.New("give one name to forget")
	}
	if err := store.Remove(args[0]); err != nil {
		return err
	}
	fmt.Printf("forgot %s\n", args[0])
	return nil
}

// isTerminal reports whether stdin is a terminal rather than a pipe. A
// character device is the distinction that matters: a pipe means something
// upstream is feeding the value, and that something has already recorded it.
func isTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// readHidden reads a line with echo off, restoring the terminal afterwards
// however it exits. stty is used rather than a terminal library because this
// program has no dependencies outside the standard library and is not going to
// acquire one for a single call.
func readHidden(prompt string) (string, error) {
	restore := func() {}
	if out, err := exec.Command("stty", "-f", "/dev/tty", "-g").Output(); err == nil {
		state := strings.TrimSpace(string(out))
		restore = func() { _ = exec.Command("stty", "-f", "/dev/tty", state).Run() }
	}
	defer restore()
	if err := exec.Command("stty", "-f", "/dev/tty", "-echo").Run(); err != nil {
		return "", errors.New("cannot turn off echo, refusing to read a secret that would be shown")
	}

	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Fprintln(os.Stderr)
	if err != nil && line == "" {
		return "", err
	}
	value := strings.TrimRight(line, "\r\n")
	if value == "" {
		return "", errors.New("nothing entered")
	}
	return value, nil
}

// readLines returns a file's lines, treating an absent file as empty.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}

const secretUsage = `steward secret — use a secret without putting it in the transcript

Typing a value into a prompt writes it into the transcript forever, where it is
replayed into the model on every later turn and kept in backups. Passing it as
an argument does the same and puts it in ps as well. So the value is typed once
into your own terminal, stored, and referred to only by name afterwards.

Usage:
  steward secret set NAME              type a value in, hidden, yourself
  steward secret use NAME -- command   run something with it in the environment
  steward secret write NAME --to .env  put NAME=value into a file
  steward secret list                  the names, never the values
  steward secret forget NAME           remove it

The agent can safely write and record this:

  steward secret use PGPASSWORD -- psql -h localhost -U app

because the line contains no value. "set" refuses to read from a pipe: the only
way a value arrives that way is a command line that has already leaked it.
`
