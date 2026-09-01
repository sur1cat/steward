# steward

**Decide what your coding agent may execute, and record what was decided.**

`steward` sits on Claude Code's `PermissionRequest` hook. It sees a tool call
only when Claude Code has already decided to ask you about it, and answers with
one of three things: allow it, deny it, or defer to the prompt you would have
seen anyway.

It is the half of the job [pitwall](https://github.com/sur1cat/pitwall)
deliberately does not do. pitwall runs when you run it, and only reads. steward
runs when Claude Code runs — on this machine that is about 4,600 tool calls a
day — and its answers change what executes. Those are different engineering
problems and different amounts of trust to ask for, which is why they are
different binaries.

## What it cannot do

This matters more than what it can, so it comes first.

- **It cannot override a deny rule.** Claude Code evaluates deny and ask rules
  before the hook runs and regardless of what it returns. A call your settings
  forbid stays forbidden.
- **It cannot see a call you were not going to be asked about.** Anything your
  rules already approve never reaches it.
- **It approves nothing until you turn that on.** Out of the box steward watches
  and records; every prompt you would have seen, you still see.
- **It fails to `defer`.** Unreadable rules, malformed input, an unknown tool, a
  permission mode that never prompts — all of it ends in the unchanged
  behaviour. There is no path through this program that turns an error into an
  approval.

## Install

```sh
brew install sur1cat/tap/steward
steward install
```

`steward install` adds one entry to the `hooks` block of
`~/.claude/settings.json` and backs the file up first. It leaves every other
hook alone, and `steward install --remove` takes out only its own entry.

## Use

```sh
steward log                      # every decision, newest last
steward check "git push origin"  # what would happen to a call, and why
steward rules                    # the rules in force here, and where they came from
```

`check` is the one to reach for when a decision looks wrong. An enforcement
tool you cannot interrogate is one nobody should install.

## Letting it answer for you

```sh
steward install --auto-allow     # and --no-auto-allow to go back
```

With this on, a call your own rules already cover is approved instead of
prompting. The value is that steward's matcher is the one pitwall audits your
rules with — it follows Claude Code's documented matching table, including the
part that trips people up:

> Claude Code is aware of shell operators, so a rule like `Bash(safe-cmd *)`
> won't give it permission to run the command `safe-cmd && other-cmd`.

Every subcommand must be covered independently. `psql -h db` is approved by
`Bash(psql:*)`; `psql -h db && rm -rf /tmp/x` is not, and never will be.

## Asking before Bash writes a file

```sh
steward install --guard-writes    # and --no-guard-writes to stop
```

A file written by `sed -i`, a shell redirect or `tee` never passes through the
Edit or Write tools. That has three consequences nobody is told about: `/rewind`
cannot undo it, because Claude Code's file history never saw it; the next read
pays full price, because the Read cache does not know the file changed; and
the change is missing from the structured patch record, so nothing downstream
can see what happened.

On the machine steward was built for there were 125,722 Bash calls against
6,481 Edit and Write calls, so this is not a rare path.

The guard **asks** rather than refuses, because writing a file from the shell
is sometimes exactly what was meant and a guard that blocks real work gets
uninstalled. Without the setting steward records these calls and says nothing.

## Trying a hook without starting a session

```sh
steward test "make build && rm -rf build"
steward test --event pre-tool-use "sed -i '' s/a/b/ main.go"
```

Hooks are the one part of a Claude Code setup you cannot try. You change a
script, start a session, and learn from behaviour whether it worked; the two
community tools for this are both abandoned. `steward test` sends the payload
Claude Code would send, through the same code that answers the real event, and
prints both halves — so a dry run and the real thing cannot diverge.

## Using a secret without putting it in the transcript

```sh
steward secret set PGPASSWORD                  # you type it, once, in your terminal
steward secret use PGPASSWORD -- psql -h db    # the agent can write and record this
steward secret write STRIPE_KEY --to .env      # value goes to the file, not the screen
steward secret list                            # names only
```

Typing a value into a prompt writes it into the transcript forever, where it is
replayed into the model on every later turn and kept in backups. Passing it as
an argument does the same and puts it in `ps` as well. So it is typed once into
a terminal the agent is not reading, stored, and referred to only by name.

On the machine steward was built for, `pitwall perms` found **113 distinct
credentials** that had reached permission rules exactly this way, and a database
password appears in the prompt history twenty-three times.

The value has two places and no others: the store, and the environment of the
process that needs it. It is never an argument, never printed — not even
partially — and no subcommand here returns it to a caller that might log it.
`set` refuses to read from a pipe, because the only way a value arrives that
way is a command line that has already leaked it.

On macOS the store is the login Keychain. Elsewhere it is a file with mode
0600, which is weaker and says so when you ask.

## The log

```
3 decisions in the last 1h — 1 allowed · 0 denied · 2 asked

  16:44:39  allow  Bash   psql -h localhost
             Bash(psql:*)
  16:44:39  defer  Bash   psql -h localhost && rm -rf /tmp/x
```

One JSON object per line in `~/.claude/steward/decisions.jsonl`, written before
the answer is returned, so nothing can be acted on without also being recorded.
A failure to write it never changes a decision: losing a log line is a nuisance,
refusing a tool call because the disk is full is a broken session.

## Where the rules come from

The same place Claude Code reads them, in the same order: the managed file, then
`~/.claude/settings.json`, then the project's `.claude/settings.json`, then its
`.claude/settings.local.json`. A worktree resolves to its main checkout, because
that is where Claude Code loads the local file from.

The matcher itself lives in `github.com/sur1cat/pitwall/perms` and is shared
with pitwall on purpose. An audit that calls a rule dead while the enforcer
still honours it would be worse than either tool alone.

Run `pitwall perms` first. On the machine this was built on, 436 of 1,053 allow
rules could never match anything, and 134 had a credential in the text. There is
no point enforcing a ruleset in that state.

## License

MIT
