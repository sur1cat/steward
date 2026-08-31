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
