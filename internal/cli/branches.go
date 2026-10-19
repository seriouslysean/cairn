package cli

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/seriouslysean/cairn/internal/id"
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
// name, and each branch's issues in store order (created, then id). It runs git through os/exec.
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
// The candidates come from git for-each-ref --no-merged=<default> --format='%(refname:short) %(committerdate:unix)'
// refs/heads/, not git branch: that marks a branch checked out in another worktree with "+", and those are the
// branches that matter most. A branch with no history in common with the default (gh-pages, say) has no merge base
// to diff from, so it is skipped.
//
// From each active branch, every issue folder that diff touches is read whole from the tip, with the checks
// store.LoadIssue makes (store.ParseIssueFiles): git ls-tree lists the folder and git cat-file reads each file. An
// issue the branch removed (no issue.md at the tip) is left out. A file that does not parse leaves its issue out, and
// adds the warning "branch <branch>: issue <id>: <the error>". Only committed work counts.
//
// The tracker can sit below the repository's top folder. Paths in git are relative to that top, and git ls-tree
// prints them that way when it runs there, so every command after the first two runs in the top folder. The
// tracker's own path comes from git rev-parse --show-prefix run in s.Dir, never from comparing absolute paths: on
// macOS the temporary folder is reached through a symlink, so git's top folder and s.Dir do not share a prefix.
// Paths are read with -z, so git never quotes them.
func branchIssues(env Env, s *store.Store) (versions []branchVersion, warnings []string, err error) {
	top, err := runGit(s.Dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, fmt.Errorf("--branches needs git and a repository: %w", err)
	}
	top = strings.TrimSpace(top)
	prefix, err := runGit(s.Dir, "rev-parse", "--show-prefix")
	if err != nil {
		return nil, nil, err
	}
	issues := strings.TrimSpace(prefix) + "issues/"
	def, err := defaultBranch(top)
	if err != nil {
		return nil, nil, err
	}
	current, _ := runGit(top, "symbolic-ref", "--quiet", "--short", "HEAD") // fails on a detached HEAD
	current = strings.TrimSpace(current)
	refs, err := runGit(top, "for-each-ref", "--no-merged="+def,
		"--format=%(refname:short) %(committerdate:unix)", "refs/heads/")
	if err != nil {
		return nil, nil, err
	}
	cutoff := env.stamp().Add(-14 * 24 * time.Hour)
	for line := range strings.Lines(refs) {
		branch, unix, _ := strings.Cut(strings.TrimSpace(line), " ")
		secs, err := strconv.ParseInt(unix, 10, 64)
		if err != nil || branch == def || branch == current || !time.Unix(secs, 0).After(cutoff) {
			continue
		}
		diff, err := runGit(top, "diff", "-z", "--name-only", def+"..."+branch, "--", issues)
		if err != nil {
			continue // no merge base
		}
		var touched []string
		for p := range strings.SplitSeq(diff, "\x00") {
			rest, ok := strings.CutPrefix(p, issues)
			issueID, _, inFolder := strings.Cut(rest, "/")
			if ok && inFolder && id.Valid(issueID) && !slices.Contains(touched, issueID) {
				touched = append(touched, issueID)
			}
		}
		for _, issueID := range touched {
			issue, comments, err := issueAtTip(top, branch, issues+issueID+"/", issueID)
			switch {
			case errors.Is(err, store.ErrNotFound): // the branch removed it
			case err != nil:
				warnings = append(warnings, fmt.Sprintf("branch %s: issue %s: %v", branch, issueID, err))
			default:
				versions = append(versions, branchVersion{Branch: branch, Issue: issue, Comments: comments})
			}
		}
	}
	slices.SortFunc(versions, func(a, b branchVersion) int {
		return cmp.Or(strings.Compare(a.Branch, b.Branch), a.Issue.Created.Compare(b.Issue.Created),
			strings.Compare(a.Issue.ID, b.Issue.ID))
	})
	return versions, warnings, nil
}

// defaultBranch returns the local branch that origin/HEAD names, else main, else master.
func defaultBranch(top string) (string, error) {
	names := []string{"main", "master"}
	if head, err := runGit(top, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		names = slices.Insert(names, 0, strings.TrimPrefix(strings.TrimSpace(head), "refs/remotes/origin/"))
	}
	for _, name := range names {
		if _, err := runGit(top, "rev-parse", "--quiet", "--verify", "refs/heads/"+name); err == nil {
			return name, nil
		}
	}
	return "", errors.New("--branches needs a default branch: origin/HEAD names none here, and there is no main or " +
		"master")
}

// issueAtTip reads the issue folder at path folder (ending in "/") as branch's tip commit has it.
func issueAtTip(top, branch, folder, issueID string) (model.Issue, []model.Comment, error) {
	list, err := runGit(top, "ls-tree", "-r", "-z", "--name-only", branch, "--", folder)
	if err != nil {
		return model.Issue{}, nil, err
	}
	var names []string
	for p := range strings.SplitSeq(list, "\x00") {
		if name, ok := strings.CutPrefix(p, folder); ok && name != "" && !strings.Contains(name, "/") {
			names = append(names, name)
		}
	}
	return store.ParseIssueFiles(issueID, names, func(name string) ([]byte, error) {
		data, err := runGit(top, "cat-file", "blob", branch+":"+folder+name)
		return []byte(data), err
	})
}

// runGit runs git in dir and returns what it printed. When git fails, the error carries what git said.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(bytes.TrimSpace(exit.Stderr)) > 0 {
		return "", fmt.Errorf("git %s: %s", args[0], bytes.TrimSpace(exit.Stderr))
	}
	return "", fmt.Errorf("git %s: %w", args[0], err)
}
