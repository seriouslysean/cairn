# Build plan

Each step is one PR, made like this:
1. The reviewer writes the step's tests first, as the spec.
2. The drafter (any model, local or hosted) writes only the files listed under "Drafter edits". The tests are context
   and must not change.
3. The reviewer runs `scripts/check.sh`, reads the diff, and opens the PR.

A drafter replies with every file it changes, in full, and nothing else:

```
=== FILE: internal/id/id.go
<complete file>
=== END FILE
```

## Step 1: foundations
IDs, atomic writes, and frontmatter. The tests and the stubs (which panic with "todo") are in the repo.

Drafter edits:
- internal/id/id.go
- internal/store/atomic.go
- internal/doc/front.go

Read-only context:
- docs/spec.md
- internal/id/id_test.go
- internal/store/atomic_test.go
- internal/doc/front_test.go

Done when: all three packages pass their tests, use the standard library only, and keep their exported signatures.

## Step 2: model and layout
- `internal/model`: the `Issue`, `Comment`, `Milestone` and `Config` types, converted to and from `doc.Field` lists, with
  the spec's defaults.
- `internal/store`:
  - find `.cairn/` by walking up from the working folder;
  - `init`;
  - load and save an issue folder;
  - list issues;
  - new ids through `id.New` with `id.Length`.

The tests and the stubs (which panic with "todo") are in the repo.

Drafter edits:
- internal/id/id.go
- internal/model/model.go
- internal/store/store.go

Read-only context:
- docs/spec.md
- internal/doc/front.go
- internal/store/atomic.go
- internal/id/id_test.go
- internal/model/model_test.go
- internal/store/store_test.go

## Step 3: commands
`init`, `new`, `edit`, `rm`, `show` and `ls`, with `--json`, `--as` and `CAIRN_ACTOR`, and the exit codes from the spec.
The contract is the package comment in `internal/cli/cli.go`; `cmd/cairn` only wires it to the process.

Drafter edits:
- internal/cli/cli.go
- internal/store/store.go

Read-only context:
- docs/spec.md
- internal/model/model.go
- internal/id/id.go
- cmd/cairn/main.go
- internal/cli/cli_test.go

## Step 4: comments
`comment`, `retract` and `purge`. Retracted comments are hidden by default, `--all` shows them, and a new comment that
matches a retracted one triggers the warning.

## Step 5: lists and search
`tags` (with counts), `milestones`, and `search` by words.

## Step 6: duplicates by words
`dupes` with the word-overlap score, plus the check that `new` runs (exit code 3, `--force`).

## Version 0.1.0
A working tracker on one checkout: steps 1 to 6, then the release step below. Everything under "Later" comes after.

Release step:
- CI runs `scripts/check.sh` on every pull request.
- `cairn version` reports the tag: the version is set at build time with `-ldflags "-X main.version=…"`.
- `go install github.com/seriouslysean/cairn/cmd/cairn@v0.1.0` works, and the README says how to install and start.
- Tag `v0.1.0` on main, with a GitHub release whose notes list what works and what is not in it yet.

## Later, in this order
1. `--branches`.
2. Meaning search and meaning-based duplicates (`.index/`).
3. `board` and `board --html`.
4. `pull`.
5. `push`.
6. Release: CI, tagged builds, a Homebrew tap.
