package cli

import (
	"fmt"
	"path"

	"github.com/seriouslysean/cairn/internal/store"
)

// write is what a write changed, for commit on write: the tracker, the issue's or milestone's id, and the path to
// commit, relative to the tracker's folder.
type write struct {
	store *store.Store
	id    string
	path  string
}

// What commit on write puts before its message, and before its warning.
const (
	commitPrefix       = "chore(cairn): "
	notCommittedPrefix = "not committed: "
)

// wroteIssue records that the write changed the issue issueID.
func (c *call) wroteIssue(s *store.Store, issueID string) {
	c.wrote = &write{store: s, id: issueID, path: path.Join(store.IssuesDir, issueID)}
}

// wroteMilestone records that the write changed the milestone milestoneID.
func (c *call) wroteMilestone(s *store.Store, milestoneID string) {
	c.wrote = &write{store: s, id: milestoneID, path: store.MilestoneFile(milestoneID)}
}

// commit commits what the write recorded when the config's commit is true, with command in the message, as the
// package doc describes. A failure is a warning: the write stands.
func (c *call) commit(command string) {
	w := c.wrote
	cfg, err := w.store.Config()
	if err != nil {
		c.warn(notCommittedPrefix + err.Error())
		return
	}
	if !cfg.Commit {
		return
	}
	actor, _ := c.actor() // the write has checked it
	msg := fmt.Sprintf("%s%s %s (by %s)", commitPrefix, command, w.id, actor)
	g := gitRepo{dir: w.store.Dir}
	_, err = g.run("add", "-A", "--", w.path)
	if err == nil {
		_, err = g.run("commit", "-m", msg, "--", w.path)
	}
	if err != nil && !g.clean(w.path) {
		c.warn(notCommittedPrefix + err.Error())
	}
}

// clean reports whether git status shows nothing to commit at p.
func (g gitRepo) clean(p string) bool {
	out, err := g.run("status", "--porcelain", "--", p)
	return err == nil && len(out) == 0
}
