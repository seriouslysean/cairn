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
- `internal/model`: the `Issue`, `Comment`, `Milestone` and `Config` types, converted to and from `doc.Field` lists,
  with the spec's defaults.
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
matches a retracted one triggers the warning. The match is the word-overlap score from `internal/words`. The contract is
the package comment in `internal/cli/cli.go`.

Drafter edits:
- internal/cli/cli.go
- internal/store/store.go
- internal/words/words.go

Read-only context:
- docs/spec.md
- internal/model/model.go
- internal/id/id.go
- internal/cli/cli_test.go
- internal/cli/comment_test.go
- internal/store/store_test.go
- internal/words/words_test.go

## Step 4b: issue creators
`new` records the actor, persona and model on the issue as `by`, `persona` and `model`, set once. Edits and sync never
change them. Recording who edited an issue is a potential follow-up (issues #4 and #5), not part of v0.1.0.

## Step 5: lists and search
`tags` (with counts), `milestones`, and `search` by words. Search splits words with `internal/words`, as the
retracted-comment warning does. The contract is the package comment in `internal/cli/cli.go`.

Drafter edits:
- internal/cli/cli.go
- internal/store/store.go

Read-only context:
- docs/spec.md
- internal/model/model.go
- internal/words/words.go
- internal/cli/cli_test.go
- internal/cli/comment_test.go
- internal/cli/lists_test.go
- internal/store/store_test.go

## Step 6: milestone commands
`milestone new` and `milestone edit`, so an agent never has to write a milestone file by hand. They set the id, title,
status and due date; the body stays hand-written. The contract is the package comment in `internal/cli/cli.go`.

Drafter edits:
- internal/cli/cli.go
- internal/store/store.go

Read-only context:
- docs/spec.md
- internal/model/model.go
- internal/cli/cli_test.go
- internal/cli/lists_test.go
- internal/cli/milestone_test.go
- internal/store/store_test.go

## Version 0.1.0
A working tracker on one checkout: steps 1 to 6 (with 4b), then the release step below. Everything under "Later" comes
after.

Release step:
- CI runs `scripts/check.sh` on every pull request, and `go vet` and `go test` on Go 1.24, the oldest version `go.mod`
  allows. This part landed before step 4, so every later pull request is checked.
- `cairn version` reports the tag. It prints the main module's version as Go records it in the binary
  (`runtime/debug.ReadBuildInfo`), which is the tag for `go install …@v0.1.0`; `-ldflags` never run under `go install`.
- `go install github.com/seriouslysean/cairn/cmd/cairn@v0.1.0` works, and the README says how to install and start.
- Tag `v0.1.0` on main, with a GitHub release whose notes list what works and what is not in it yet.

## Step 7: --branches
`ls --branches` and `show --branches`, so an agent sees what other branches and worktrees have committed to the
tracker before it merges. It comes first after v0.1.0 because several agents on several branches is the case
dogfooding meets first. The contract is the package comment in `internal/cli/cli.go` and the doc comment of
`branchIssues` in `internal/cli/branches.go`. `store.ReadIssue` reads an issue folder wherever its files are, on disk
or in a commit, so a branch's issues get the same checks as the checkout's.

Drafter edits:
- internal/cli/branches.go
- internal/cli/cli.go
- internal/cli/help.go
- internal/cli/issues.go
- internal/cli/json.go
- internal/cli/lists.go
- internal/store/store.go

Read-only context:
- docs/spec.md
- internal/cli/branches_test.go
- internal/store/store_test.go
- internal/model/model.go
- internal/id/id.go

## Step 8: commit on write
With `commit: true` in `config.yml`, each write commits what it changed, so a tracker kept in a repository of its own
needs no watcher and no separate commit. It is off by default because it commits on whatever branch is checked out.
A commit that fails is a warning, so the JSON envelope gains `warnings`, and an agent reading only stdout sees it.
The contract is the package comment in `internal/cli/cli.go`.

Drafter edits:
- internal/cli/cli.go
- internal/cli/commit.go
- internal/cli/comments.go
- internal/cli/issues.go
- internal/cli/json.go
- internal/cli/milestones.go
- internal/model/fields.go
- internal/model/model.go
- internal/store/store.go

Read-only context:
- docs/spec.md
- internal/cli/branches.go
- internal/cli/commit_test.go
- internal/model/model_test.go

## Step 9: clearer names
Three names said less than they meant, so they change, with no alias for the old ones:
- `show --all` is `show --retracted`: it adds the retracted comments. That leaves `--all` free for step 10 to mean
  every row.
- The config key `dupe_text` is `retracted_match`: the overlap score at which a new comment warns that it matches a
  retracted one. A config that still has `dupe_text` keeps the line as an unknown key, and `retracted_match` takes its
  default.
- `comment --json` lists those matches as `data.retracted_matches`, not `data.warnings`, so the name is no longer
  shared with the envelope's `warnings`.

The spec said `ls` leaves retracted comments out, but `ls` never prints comments, so that line now names `show` only.
The contract is the package comment in `internal/cli/cli.go`.

Drafter edits:
- internal/cli/cli.go
- internal/cli/comments.go
- internal/cli/help.go
- internal/cli/issues.go
- internal/cli/json.go
- internal/model/model.go

Read-only context:
- docs/spec.md
- internal/cli/comment_test.go
- internal/model/model_test.go

## Step 10: default limits
`ls` and `search` print at most 30 rows unless given `--limit N` or `--all`, which step 9 left free to mean every row.
`--limit` takes a whole number of 1 or more, and anything else is a usage error, as is `--limit` with `--all`. `ls`
keeps its last 30 rows: the newest issues, and with `--branches` the branches' rows, which it prints last. `search`
keeps its 30 best matches. The cut comes after filtering and sorting.

When rows are left out, the command warns `30 of 672 shown; --limit N or --all for more`, the same with or without a
terminal. With `--json` the envelope gains `"total"` on every `ls` and `search`: how many rows there were before the
cut, whether or not any were cut. There are no pages: an agent narrows with a filter or a better query. `show`, `tags`
and `milestones` don't change. The spec's Commands table and `--json` line change with it, and the contract is the
package comment in `internal/cli/cli.go`.

`help` gains a second line that says what `--json` prints: the envelope, `"warnings"` and `"total"` included, and the
failure's. An agent with only the binary otherwise has no way to learn the shape. `help --json` stays
`{"usage", "rules"}`: it is already in the envelope it would describe.

It comes before `pull`, which brings in issues by the hundred. Reading isn't the problem: at 10,000 issues `ls` takes
under 0.4 s and `search` about 1 s. Output is: there, `ls --json` prints 2.7 MB, and a tracker of 672 issues already
prints 195 KB.

Drafter edits:
- internal/cli/cli.go
- internal/cli/help.go
- internal/cli/json.go
- internal/cli/lists.go

Read-only context:
- internal/cli/limits_test.go

## Step 11: a branch of its own
`init --own-branch` keeps the tracker on a branch named `cairn`, in a git repository of its own at `.cairn/` that the
code repository ignores, so commit on write never lands on the code's branches and every worktree shares one tracker.
It runs in the main checkout's top folder. It attaches `origin/cairn` when that holds a tracker, and otherwise makes
one with commit on write on. It never pushes.
- `store.Find` follows a linked worktree's `.git` file to the main checkout's tracker when that is on its own branch.
  It reads files and runs no git: at 27 µs, against 8 ms to ask `git rev-parse`.
- `init` without the flag will not start a second tracker: not in a checkout's top folder where `origin/cairn` holds
  one, and not where a tracker on its own branch already serves the folder.
- `--branches` adds nothing on such a tracker.

The contract is the package comment in `internal/cli/cli.go` and the doc comment of `store.Find`. It was built before
step 10: worktrees outside the repository, which several agent tools make, found no tracker at all.

Drafter edits:
- internal/cli/branches.go
- internal/cli/cli.go
- internal/cli/help.go
- internal/cli/issues.go
- internal/cli/json.go
- internal/cli/ownbranch.go
- internal/store/store.go

Read-only context:
- docs/spec.md
- internal/cli/help_test.go
- internal/cli/ownbranch_test.go
- internal/store/store_test.go

## Later, in this order
1. `pull`, one way: GitHub issues and their comments become Cairn issues and comments.
2. `push`.
3. Meaning search (`.index/`).
4. `board` and `board --html`.
5. Release: CI, tagged builds, a Homebrew tap.

Not planned: duplicate checks. The spec had `dupes` and a check before `new` (exit code 3, `--force`); word overlap
could not tell a duplicate from a template title, so both wait in the backlog as #15 and #16.
