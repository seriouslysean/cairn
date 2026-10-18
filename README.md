# Cairn

An issue tracker for coding agents first and people second. Issues are markdown files in a `.cairn/` folder inside
your repository, committed with your code. One `cairn` command, installed once per machine, reads and writes them,
and every command can answer in JSON.

Status: early alpha (v0.1.0). It works on one checkout. Not in yet: reading other branches (`--branches`), search by
meaning, the board, and GitHub sync. See [docs/plan.md](docs/plan.md).

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

Commit `.cairn/` with your code.

## For agents

- Add `--json` to any command. Success prints `{"ok": true, "data": …}`; failure prints
  `{"ok": false, "error": {"code": "…", "message": "…"}}`.
- Exit codes: 0 ok, 1 error, 2 usage, 4 not found.
- Cairn does not look for duplicates: run `cairn search` before `cairn new`.
- A comment is `evidence` (with `--source`), a `decision` or a `result`. Retract a wrong one with `cairn retract`
  rather than deleting it.
- `cairn help` lists every command, and [docs/spec.md](docs/spec.md) has the rest.

## Requests

Open an issue at https://github.com/seriouslysean/cairn/issues and say what you were trying to do.

License: MIT.
