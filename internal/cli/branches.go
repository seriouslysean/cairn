package cli

import (
	"errors"

	"github.com/seriouslysean/cairn/internal/model"
	"github.com/seriouslysean/cairn/internal/store"
)

// branchVersion is an issue as an active branch's tip commit has it. Branch is "" for the checkout's own version.
type branchVersion struct {
	Branch   string
	Issue    model.Issue
	Comments []model.Comment // every comment, retracted ones included, in the order store.LoadIssue gives
}

// branchIssues returns every issue the active branches changed, as each branch's tip commit has it: branches by
// name, and each branch's issues in store order (created, then id). It runs git in s.Dir, through os/exec.
//
// The default branch is the local branch that refs/remotes/origin/HEAD names (refs/remotes/origin/trunk names
// trunk), else main, else master; when none of them exists it is an error. An active branch is a local branch
// (refs/heads/) that:
//   - is not the default branch, and not the branch checked out here (git symbolic-ref --quiet --short HEAD; a
//     detached HEAD has none);
//   - is not merged into the default branch;
//   - has a tip commit whose committer date is less than 14 days before env.stamp();
//   - changes a file under the tracker's issues folder in git diff --name-only <default>...<branch>.
//
// List the candidates with git for-each-ref --no-merged=<default> --format='%(refname:short) %(committerdate:unix)'
// refs/heads/. Do not parse git branch: it marks a branch checked out in another worktree with "+", and those are the
// branches that matter most.
//
// From each active branch, every issue folder that diff touches is read whole from the tip: its issue.md and its
// c-<id>.md comment files (git ls-tree --name-only <branch> <folder>/, then git show <branch>:<path>), parsed with
// doc.Parse and model.ParseIssue or model.ParseComment, with the same checks store.LoadIssue makes. An issue the
// branch removed (no issue.md at the tip) is left out. A file that does not parse leaves its issue out, and adds the
// warning "branch <branch>: issue <id>: <the error>". Only committed work counts.
//
// The tracker can sit below the repository's top folder, and paths in git are relative to that top. Get the
// tracker's path with git rev-parse --show-prefix run in s.Dir. Never work it out by comparing absolute paths: on
// macOS the temporary folder is reached through a symlink, so git's top folder and s.Dir do not share a prefix.
func branchIssues(env Env, s *store.Store) (versions []branchVersion, warnings []string, err error) {
	return nil, nil, errors.New("--branches is not built yet")
}
