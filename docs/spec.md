# Cairn v0 spec

Cairn is an issue tracker for coding agents first and people second. Issues live as markdown files in a `.cairn/`
folder inside the repository they belong to, committed with the code. One `cairn` command, installed once per machine,
reads and writes them. An agent can do everything without a person: no prompts, no editor, JSON on request.

## Storage

```
.cairn/
  config.yml               statuses, tag groups, GitHub repo, embeddings URL
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
created: 2026-10-17T19:00:00Z
updated: 2026-10-17T19:20:00Z
github: 123
---
What is wrong, why it matters, and what "done" means.
```
`github` appears only after a sync. Dates are `YYYY-MM-DD`; times are UTC RFC 3339, with fractional seconds when the time has them, so comments made in
the same second still sort in order.

### Comment
```
---
id: c-x9k2
kind: evidence
by: alice
persona: skeptic
model: example-model-1
at: 2026-10-17T19:20:00Z
source: export_test.go:42
---
The last row is dropped when the file has no trailing newline.
```
A retracted comment adds `retracted_by`, `retracted_at`, `retracted_reason` and, optionally, `disproved_by: c-ab12`.

### Milestone
Same frontmatter as an issue (`id`, `title`, `status`, `due`), and a body. It has no comments.

### Config
`config.yml` is frontmatter with an empty body, like every other file:
```
---
statuses: [backlog, open, in-progress, done]
tag_groups: [severity, effort, area]
github_repo: owner/name
embeddings_url: http://127.0.0.1:8080/v1/embeddings
dupe_text: 0.6
dupe_meaning: 0.85
---
```
Every key is optional. Shown above with the defaults, except that `github_repo` and `embeddings_url` are off when absent.

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
| Add | `cairn new "title" [--body text] [--tag t]... [--milestone m] [--due d] [--force]` |
| Edit | `cairn edit <id> [--title t] [--body text] [--status s] [--tag +t] [--tag -t] [--milestone m] [--due d]` |
| Delete | `cairn rm <id>` |
| Read | `cairn show <id> [--all] [--branches]` |
| Comment | `cairn comment <id> --kind evidence\|decision\|result [--source s] "text"` |
| Retract | `cairn retract <id> <comment> --reason "…" [--disproved-by <comment>]` |
| Purge | `cairn purge <id> <comment>` |
| Search | `cairn search "words"`, `cairn search --meaning "text"`, `cairn dupes [<id>\|"text"]` |
| Board | `cairn board`, `cairn board --html <file>` |
| Sync | `cairn pull`, `cairn push [--dry-run]` |

- Every write takes `--as <actor>` and, optionally, `--persona <p>` and `--model <m>`. The actor can also come from the
  `CAIRN_ACTOR` environment variable.
- Every command takes `--json`. On success it prints `{"ok": true, "data": …}`. On failure it prints
  `{"ok": false, "error": {"code": "…", "message": "…"}}` on stdout, and a one-line message on stderr.
- Exit codes:
  - 0: ok;
  - 1: error;
  - 2: usage;
  - 3: possible duplicates found (`new` without `--force`);
  - 4: not found.

## Comments are data, not chat
- A writer may add three kinds, and `comment` refuses anything else (`pull` also creates kind `github`, below):
  - `evidence` requires `--source`: a URL, a file and line, a log path or a command;
  - `decision` holds the owner's words, quoted;
  - `result` is what was done: a PR, a commit or test output.
- `retract` keeps the comment where it is and adds the retracted fields.
- `show`, `ls` and `--json` leave retracted comments out. `--all` includes them, marked.
- A new comment that closely matches a retracted one is still added, with a warning that names the retracted comment and
  its reason.
- `purge` deletes the comment file, for mistakes such as a pasted secret. Git history still holds it.

## Search and duplicates
- `search` scans titles, bodies and active comments for the words given, and ranks by how many match.
- `--meaning` is optional:
  - It needs `embeddings_url`, an OpenAI-compatible `/v1/embeddings` endpoint.
  - Vectors are cached in `.cairn/.index/`, keyed by each file's hash.
  - Search compares the query vector with every cached vector (cosine similarity).
- `dupes` gives each candidate a word-overlap score and, when embeddings are configured, a meaning score. A candidate
  counts when either score reaches its threshold from the config.
- `new` runs the same check first. If candidates exist, it exits with code 3 and lists them, unless `--force` is given.

## Branches
- By default, commands read the current checkout only.
- `--branches` adds what active branches changed. An active branch is a local branch that:
  - is not yet merged into the default branch;
  - has a commit in the last 14 days;
  - has files in `.cairn/` that changed since it left the default branch (`git diff --name-only <default>...<branch> -- .cairn/`).
- Only those files are read from each branch.
- An issue changed on both the default branch and a branch shows both versions, marked.

## GitHub sync (optional, through the `gh` CLI)
- `pull`:
  - New GitHub issues become Cairn issues with `github: N`.
  - For issues already linked, state and labels update.
  - The title and body belong to Cairn: edits made on GitHub are reported, not merged.
  - GitHub comments arrive as comments of kind `github`.
- `push`:
  - Issues without `github` are created on GitHub.
  - Linked issues get their title, body, labels and state updated, and their new active comments posted.
  - Issues closed on GitHub are skipped and reported.
  - `--dry-run` prints what would change.
- Cairn never pushes on its own. Pushing is the user's call.

## Not in v0
Locks, leases or claims; scheduled jobs; a server; hiding GitHub comments that were already posted before being
retracted.
