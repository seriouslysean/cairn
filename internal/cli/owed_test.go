package cli

import (
	"testing"
	"time"
)

// say adds a comment of kind to id a second after the last one, so the comments' at order is the order they are said
// in, and returns the new comment's id.
func (tr *tracker) say(id, kind string) string {
	tr.t.Helper()
	tr.now = tr.now.Add(time.Second)
	args := []string{"comment", id, "--kind", kind}
	if kind == "evidence" {
		args = append(args, "--source", "log.txt")
	}
	return tr.comment(append(args, kind+" text")...).Comment.ID
}

// ls --owed keeps the issues whose latest active decision has no active result after it (issue #54).
func TestLsOwed(t *testing.T) {
	tr := newTracker(t)
	ids := tr.many(9, "issue")
	tr.say(ids[0], "decision")
	tr.say(ids[1], "decision")
	tr.say(ids[1], "result")
	tr.say(ids[2], "result")
	tr.say(ids[2], "decision")
	// A result counts only after the latest decision.
	tr.say(ids[3], "decision")
	tr.say(ids[3], "result")
	tr.say(ids[3], "decision")
	// A retracted decision does not count, so the result before it closes the one before that.
	tr.say(ids[4], "decision")
	tr.say(ids[4], "result")
	tr.retract(ids[4], tr.say(ids[4], "decision"), "--reason", "misquoted")
	// A retracted result does not count.
	tr.say(ids[5], "decision")
	tr.retract(ids[5], tr.say(ids[5], "result"), "--reason", "wrong pr")
	// Evidence is not a result.
	tr.say(ids[6], "decision")
	tr.say(ids[6], "evidence")
	// No decision, nothing owed: ids[7] has a result only, and ids[8] no comments.
	tr.say(ids[7], "result")
	owed := []string{ids[0], ids[2], ids[3], ids[5], ids[6]}
	checkRows(t, tr, []string{"ls", "--owed"}, owed)
	checkRows(t, tr, []string{"ls", "--owed", "--all"}, owed)
	checkRows(t, tr, []string{"ls"}, ids)

	if code, out, errOut := tr.run("ls", "--owed", "--limit", "1"); code != 0 ||
		out != ids[6]+"\tbacklog\tissue 7\n" || errOut != warningLine("1 of 5 shown; --limit N or --all for more") {
		t.Fatalf("text: exit %d, %q, stderr %q", code, out, errOut)
	}
}

// A comment at the same instant as the decision is not after it.
func TestLsOwedSameInstant(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "one").ID
	tr.comment("comment", id, "--kind", "decision", "go")
	tr.comment("comment", id, "--kind", "result", "done")
	checkRows(t, tr, []string{"ls", "--owed"}, []string{id})
}

// --owed combines with the other filters, and closed and archived issues stay hidden unless asked for.
func TestLsOwedWithFilters(t *testing.T) {
	tr := newTracker(t)
	tr.writeConfig(archivedConfig)
	ids := tr.statuses("open", "done", "archived", "open")
	for _, id := range ids {
		tr.say(id, "decision")
	}
	tr.issue("edit", ids[3], "--tag", "area:x")
	for _, c := range []struct {
		args []string
		rows []string
	}{
		{[]string{"ls", "--owed"}, []string{ids[0], ids[3]}},
		{[]string{"ls", "--owed", "--closed"}, []string{ids[0], ids[1], ids[3]}},
		{[]string{"ls", "--owed", "--closed", "--archived"}, ids},
		{[]string{"ls", "--owed", "--status", "archived"}, []string{ids[2]}},
		{[]string{"ls", "--owed", "--tag", "area:x"}, []string{ids[3]}},
	} {
		checkRows(t, tr, c.args, c.rows)
	}
}

// ls --branches --owed judges a branch version by its own comments, not the checkout's.
func TestLsBranchesOwed(t *testing.T) {
	tr := gitTracker(t, "main", ".")
	ids := tr.many(2, "issue")
	tr.say(ids[0], "decision")
	tr.say(ids[0], "result")
	tr.say(ids[1], "decision")
	tr.commit(t0.Add(-36*time.Hour), "decisions")
	tr.git("switch", "-q", "-c", "feat")
	tr.say(ids[0], "decision")
	tr.commit(t0.Add(-time.Hour), "feat")
	tr.git("switch", "-q", "main")
	tr.say(ids[1], "result")
	// The checkout owes nothing; feat's version of ids[0] owes its new decision.
	env, _ := tr.envelope("ls", "--branches", "--owed")
	items, _ := env["data"].([]any)
	if len(items) != 1 {
		t.Fatalf("rows %v, want feat's %s", items, ids[0])
	}
	if row := items[0].(map[string]any); row["id"] != ids[0] || row["branch"] != "feat" {
		t.Errorf("row %v, want %s on feat", row, ids[0])
	}
	checkRows(t, tr, []string{"ls", "--owed"}, []string{})
}

// Only at orders comments: a result said later but dated before the decision does not pay it.
func TestLsOwedByAt(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "one").ID
	tr.now = t0.Add(time.Hour)
	tr.comment("comment", id, "--kind", "decision", "go")
	tr.now = t0.Add(time.Minute)
	tr.comment("comment", id, "--kind", "result", "done")
	checkRows(t, tr, []string{"ls", "--owed"}, []string{id})
	tr.now = t0.Add(2 * time.Hour)
	tr.comment("comment", id, "--kind", "result", "done")
	checkRows(t, tr, []string{"ls", "--owed"}, []string{})
}

func TestOwedUsageErrors(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "one").ID
	for _, args := range [][]string{
		{"ls", "--owed=yes"},
		{"search", "one", "--owed"},
		{"show", id, "--owed"},
		{"tags", "--owed"},
	} {
		tr.wantError(2, "usage", args...)
	}
}
