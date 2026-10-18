# AGENTS.md

Cairn is an issue tracker for coding agents first and people second. `docs/spec.md` says what it does, and
`docs/plan.md` gives the build order and how each step is made.

## Build and test
- Go 1.24 or later, standard library only unless `docs/plan.md` names a dependency.
- `gofmt -l .` prints nothing, and `go vet ./...` and `go test ./...` pass.

## Rules
- Tests are the spec. Each step's tests are written before its code and are never edited to make the code pass. If a
  test looks wrong, say so in the PR.
- Every command works without prompts or an editor, takes `--json`, and uses the exit codes in `docs/spec.md`.
- All writes go through `store.WriteFileAtomic`.
- No personal data, machine paths or names from anyone's private setup in code, tests, docs or fixtures. Examples use
  generic names (alice, bob, skeptic, example-model-1).
- One plan step per PR, on its own branch.
- Commit messages: `type(scope): summary`, where type is feat, fix, test, docs or chore.
