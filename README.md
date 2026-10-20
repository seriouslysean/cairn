# Cairn

An issue tracker for coding agents first and people second. Issues are markdown files in a `.cairn/` folder inside
your repository, committed with your code. One `cairn` command, installed once per machine, reads and writes them,
and every command can answer in JSON.

Status: early alpha. `cairn help` lists the commands that are built. See [docs/plan.md](docs/plan.md) for what comes
next.

## Install

Needs Go 1.24 or later.

```sh
go install github.com/seriouslysean/cairn/cmd/cairn@latest
cairn version
```

## Start

```sh
cd your-repo
cairn init
export CAIRN_ACTOR=alice    # or pass --as alice to each write
cairn new "Export drops the last row" --tag area:export    # prints the new id, such as 7f3k
cairn ls
cairn comment 7f3k --kind evidence --source parser.go:42 "Rows are copied twice"
cairn edit 7f3k --status done
cairn search export row
```

Commit `.cairn/` with your code, or keep it out of your code's history on a branch of its own:

```sh
cairn init --own-branch    # in the main checkout's top folder; attaches origin/cairn when it holds a tracker
git -C .cairn push         # shares the writes; git -C .cairn pull brings in other people's
```

Every worktree of the repository then uses that one tracker. See "A branch of its own" in
[docs/spec.md](docs/spec.md).

## For agents

Put this in your repository's AGENTS.md:

```md
## Issues

This repo tracks issues with Cairn. Run `cairn help` before using it, and set `CAIRN_ACTOR` to your name.
```

`cairn help` prints every command's usage and the rules. Add `--json` to any command: success prints
`{"ok": true, "data": …}`, plus `"warnings"` when there are any and `"total"` on `ls` and `search`, failure prints
`{"ok": false, "error": {"code": "…", "message": "…"}}`, and the exit codes are 0 ok, 1 error, 2 usage, 4 not found.
`ls` and `search` print at most 30 rows, with a warning when they leave some out: narrow with a filter or a better
query, or pass `--limit N` or `--all`. [docs/spec.md](docs/spec.md) has the rest.

## Requests

Open an issue at https://github.com/seriouslysean/cairn/issues and say what you were trying to do.

License: MIT.
