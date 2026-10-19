# Cairn v0 spec

Cairn is an issue tracker for coding agents first and people second. Issues live as markdown files in a `.cairn/`
folder inside the repository they belong to, committed with the code. One `cairn` command, installed once per machine,
reads and writes them. An agent can do everything without a person: no prompts, no editor, JSON on request.

## Storage

```
.cairn/
  config.yml               statuses, tag groups, GitHub repo, embeddings URL, commit on write
  issues/7f3k/issue.md     one folder per issue
  issues/7f3k/c-x9k2.md    one file per comment
  milestones/v0.4.md       one file per milestone
  .index/                  search cache, gitignored, rebuilt from the files
```

- Every file starts with frontmatter: a strict subset of YAML (see `internal/doc`), with flat keys and values that are
  plain text, a double-quoted string, or a flow list `[a, b]`.
- A comment is its own file, so two branches that comment on the same issue never conflict when they merge.
- Every write replaces the whole file: it goes to a temporary file in the same folder, which is then renamed into place.
  When two writers edit the same file at once, the last write wins. There is no lock and no queue.

### Issue
```
---
id: 7f3k
title: "Export drops the last row"
status: in-progress
tags: [severity:high, effort:2, area:export]
milestone: v0.4
due: 2026-11-01
by: alice
model: example-model-1
created: 2026-10-17T19:00:00.000Z
updated: 2026-10-17T19:20:00.000Z
github: 123
---
What is wrong, why it matters, and what "done" means.
```
- `by`, `persona` and `model` say who created the issue. `new` sets them once; edits and sync never change them. Who
  edited an issue is not recorded.
- An issue without them was made before Cairn recorded creators, and its creator is unknown: nothing fills them in
  later.
- They are what the writer gave, not checked identities. `model` names the model that ran the command, not
  necessarily the author of every word.
`github` appears only after a sync. Dates are `YYYY-MM-DD`. Times are UTC RFC 3339 with exactly three fraction
digits, such as `2026-10-17T19:20:00.000Z`, so every time is 24 characters and times sort the same as text as they
do as times. Files written by v0.1.0, which dropped trailing zeros, still load.
Frontmatter lines may end in CRLF, as in a Windows checkout. Cairn writes them with LF and keeps a body as it is.

### Comment
```
---
id: c-x9k2
kind: evidence
by: alice
persona: skeptic
model: example-model-1
at: 2026-10-17T19:20:00.000Z
source: export_test.go:42
---
The last row is dropped when the file has no trailing newline.
```
A retracted comment adds `retracted_by`, `retracted_at`, `retracted_reason` and, optionally, `disproved_by: c-ab12`.

### Milestone
Same frontmatter as an issue (`id`, `title`, `status`, `due`), and a body. It has no comments. Its status is one of the
config's statuses. Commands keep the body but don't write it; it is edited by hand. Its id is its file name without
`.md`: not empty, with no `/`, and not starting with `.`. The milestone an issue names follows the same rule, or is
empty.

### Config
`config.yml` is frontmatter with an empty body, like every other file:
```
---
statuses: [backlog, open, in-progress, done]
tag_groups: [severity, effort, area]
github_repo: owner/name
embeddings_url: http://127.0.0.1:8080/v1/embeddings
dupe_text: 0.6
commit: false
---
```
Every key is optional. Shown above with the defaults, except that `github_repo` and `embeddings_url` are off when
absent. `commit` is `true` or `false`.

## IDs
- Issues: 4 lowercase characters from Crockford's base32 alphabet (`0-9a-z` without `i l o u`), checked against existing
  ids. The length grows to 5 once a tracker holds 1,000 issues, and to 6 at 32,000.
- Comments: `c-` plus 4 characters, unique within their issue.
- The file or folder name is the id, so changing a title never moves a file.

## Commands

| Need | Command |
|---|---|
| Start | `cairn init` |
| Lists | `cairn tags`, `cairn milestones`, `cairn ls [--status s] [--tag t] [--milestone m] [--due-before d] [--branches]` |
| Milestones | `cairn milestone new <id> "title" [--due d]`, `cairn milestone edit <id> [--title t] [--status s] [--due d]` |
| Add | `cairn new "title" [--body text] [--tag t]... [--milestone m] [--due d]` |
| Edit | `cairn edit <id> [--title t] [--body text] [--status s] [--tag +t] [--tag -t] [--milestone m] [--due d]` |
| Delete | `cairn rm <id>` |
| Read | `cairn show <id> [--all] [--branches]` |
| Comment | `cairn comment <id> --kind evidence\|decision\|result [--source s] "text"` |
| Retract | `cairn retract <id> <comment> --reason "…" [--disproved-by <comment>]` |
| Purge | `cairn purge <id> <comment>` |
| Search | `cairn search "words"`, `cairn search --meaning "text"` |
| Board | `cairn board`, `cairn board --html <file>` |
| Sync | `cairn pull`, `cairn push [--dry-run]` |
| Help | `cairn help`, `cairn version` |

- Every write takes `--as <actor>` and, optionally, `--persona <p>` and `--model <m>`. The actor can also come from the
  `CAIRN_ACTOR` environment variable. `new` and `comment` record all three as `by`, `persona` and `model`; `retract`
  records the actor as `retracted_by`.
- Every command takes `--json`. On success it prints `{"ok": true, "data": …}`. On failure it prints
  `{"ok": false, "error": {"code": "…", "message": "…"}}` on stdout, and a one-line message on stderr.
- Exit codes:
  - 0: ok;
  - 1: error;
  - 2: usage;
  - 4: not found (3 is not used).
- `help` prints one usage line per command, then the rules below, word for word. With `--json` its data is
  `{"usage": [...], "rules": [...]}`.

## Rules
An agent with only the binary learns these from `cairn help`:
- Run cairn search before cairn new: Cairn does not look for duplicates.
- Comment on an issue someone else owns rather than editing it: comments never conflict across branches.
- evidence needs --source, decision quotes the owner, result says what was done.
- To close an issue, add a result comment, then cairn edit <id> --status done.
- Retract a wrong comment instead of deleting it. purge is only for mistakes like a pasted secret.

## Comments are data, not chat
- A writer may add three kinds, and `comment` refuses anything else (`pull` also creates kind `github`, below):
  - `evidence` requires `--source`: a URL, a file and line, a log path or a command;
  - `decision` holds the owner's words, quoted;
  - `result` is what was done: a PR, a commit or test output.
- `retract` keeps the comment where it is and adds the retracted fields.
- `show`, `ls` and `--json` leave retracted comments out. `--all` includes them, marked.
- A new comment whose word-overlap score (below) with a retracted comment on the same issue reaches `dupe_text` is still
  added, with a warning that names the retracted comment and its reason.
- `purge` deletes the comment file, for mistakes such as a pasted secret. Git history still holds it.

## Search
- `search` scans titles, bodies and active comments for the words given, and ranks by how many match.
- `--meaning` is optional:
  - It needs `embeddings_url`, an OpenAI-compatible `/v1/embeddings` endpoint.
  - Vectors are cached in `.cairn/.index/`, keyed by each file's hash.
  - Search compares the query vector with every cached vector (cosine similarity).
- The word-overlap score of two texts is the number of distinct words they share divided by the number of distinct
  words in either. A word is a run of letters and digits, compared in lowercase.
- Cairn does not look for duplicates. Run `search` before `new`.

## Branches
- By default, commands read the current checkout only. Only `--branches` and commit on write (below) run git.
- `ls --branches` and `show --branches` add what active branches committed. The default branch is the one
  `origin/HEAD` names, else `main`, else `master`. An active branch is a local branch, in any worktree, that:
  - is not the default branch or the branch checked out here;
  - shares history with the default branch, and is not yet merged into it;
  - has a last commit less than 14 days old;
  - changed files in `.cairn/issues/` since it left the default branch.
- From each active branch, every issue it changed is read whole, the issue and its comments, as its last commit has
  it. Uncommitted work, here or in another worktree, is not seen. An issue the branch removed is left out.
- `ls` lists the checkout's issues, then each branch's, marked with the branch name. `show` prints the checkout's
  version, then each branch's under a `=== branch <name> ===` line, so an issue changed on two branches shows both.

## Commit on write (optional)
- With `commit: true`, each write that succeeds commits what it changed: the issue's folder, or for `milestone new`
  and `milestone edit` the milestone's file, as it is on disk. Nothing else is staged or committed. `init` commits
  nothing.
- The message is `chore(cairn): <command> <id> (by <actor>)`, for example `chore(cairn): comment 7f3k (by alice)`.
  The author is whoever git's config names, and the repository's hooks run.
- When the commit fails (no git, no repository, another writer's lock, a hook), the write stands and the command
  succeeds with a warning. The next write to the same issue commits what was left.
- It commits on the branch checked out, so it suits a tracker kept in a repository of its own. In a repository that
  works through pull requests, leave it off.

## GitHub sync (optional, through the `gh` CLI)
- `pull`:
  - New GitHub issues become Cairn issues with `github: N` and `by: github:<login>` for their GitHub author, with no
    persona or model: the actor running `pull` is not their creator.
  - For issues already linked, state and labels update.
  - The title and body belong to Cairn: edits made on GitHub are reported, not merged.
  - GitHub comments arrive as comments of kind `github`, with `by: github:<login>`.
- `push`:
  - Issues without `github` are created on GitHub. On GitHub their author is whoever owns the `gh` login; `by` stays
    as it is.
  - Linked issues get their title, body, labels and state updated, and their new active comments posted.
  - Issues closed on GitHub are skipped and reported.
  - `--dry-run` prints what would change.
- Cairn never pushes on its own. Pushing is the user's call.

## Not in v0
Locks, leases or claims; scheduled jobs; a server; hiding GitHub comments that were already posted before being
retracted.
