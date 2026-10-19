package cli

import (
	"fmt"
	"math/rand/v2"
	"path"
	"strings"
	"time"

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

// How a git step waits for another writer: up to lockTries tries, with a pause between them drawn from
// [lockPause/2, lockPause*3/2), so writers that collide do not try again in step.
const (
	lockExt   = ".lock" // ends the lock file git's error names, in any language: index.lock, or a ref's
	lockTries = 20
	lockPause = 50 * time.Millisecond
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
	for _, args := range [][]string{{"add", "-A", "--", w.path}, {"commit", "-m", msg, "--", w.path}} {
		if err := g.step(args...); err != nil {
			c.warn(notCommittedPrefix + err.Error())
			return
		}
	}
}

// step runs one git step, and tries again while it fails because another writer is in the way: git's error names a
// lock file, or HEAD moved while the step ran. A commit given paths reads HEAD before it takes the index lock, so a
// writer that commits in between makes it fail without naming a lock.
func (g gitRepo) step(args ...string) error {
	for try := 1; ; try++ {
		before := g.head()
		_, err := g.run(args...)
		if err == nil || try == lockTries {
			return err
		}
		if inTheWay := strings.Contains(err.Error(), lockExt) || g.head() != before; !inTheWay {
			return err
		}
		time.Sleep(lockPause/2 + rand.N(lockPause))
	}
}

// head is HEAD's commit id, or "" when there is none or git cannot say.
func (g gitRepo) head() string {
	out, _ := g.run("rev-parse", "--quiet", "--verify", "HEAD")
	return strings.TrimSpace(string(out))
}
