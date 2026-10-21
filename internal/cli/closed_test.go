package cli

import (
	"reflect"
	"testing"
	"time"
)

// archivedConfig is a config with an archived status, which the default config lacks.
const archivedConfig = "---\nstatuses: [backlog, open, in-progress, done, archived]\n---\n"

// statuses adds one issue per status, in that order, set to it, and returns their ids in that order.
func (tr *tracker) statuses(statuses ...string) []string {
	tr.t.Helper()
	ids := tr.many(len(statuses), "issue")
	for n, s := range statuses {
		if s != "backlog" {
			tr.issue("edit", ids[n], "--status", s)
		}
	}
	return ids
}

// checkRows checks the ids a list command's envelope holds and its total.
func checkRows(t *testing.T, tr *tracker, args []string, want []string) {
	t.Helper()
	env, _ := tr.envelope(args...)
	checkCut(t, args, env, want, len(want), "")
}

// ls leaves out closed issues, done by default, unless --closed or a --status asks for them (issue #45). --all only
// lifts the row cap.
func TestLsHidesClosed(t *testing.T) {
	tr := newTracker(t)
	ids := tr.statuses("backlog", "done", "open", "in-progress", "done")
	open := []string{ids[0], ids[2], ids[3]}
	for _, c := range []struct {
		args []string
		rows []string
	}{
		{[]string{"ls"}, open},
		{[]string{"ls", "--all"}, open},
		{[]string{"ls", "--limit", "5"}, open},
		{[]string{"ls", "--closed"}, ids},
		{[]string{"ls", "--closed", "--all"}, ids},
		{[]string{"ls", "--status", "done"}, []string{ids[1], ids[4]}},
		{[]string{"ls", "--status", "done", "--closed"}, []string{ids[1], ids[4]}},
		{[]string{"ls", "--status", "open"}, []string{ids[2]}},
		// --archived on a tracker without archived issues adds nothing.
		{[]string{"ls", "--archived"}, open},
	} {
		checkRows(t, tr, c.args, c.rows)
	}

	// The cut comes after the hiding, so total counts only what ls would list.
	env, _ := tr.envelope("ls", "--limit", "1")
	checkCut(t, []string{"ls", "--limit", "1"}, env, []string{ids[3]}, 3, "1 of 3 shown; --limit N or --all for more")
	env, _ = tr.envelope("ls", "--closed", "--limit", "2")
	checkCut(t, []string{"ls", "--closed", "--limit", "2"}, env, ids[3:], 5, "2 of 5 shown; --limit N or --all for more")

	if code, out, errOut := tr.run("ls"); code != 0 || errOut != "" ||
		out != ids[0]+"\tbacklog\tissue 1\n"+ids[2]+"\topen\tissue 3\n"+ids[3]+"\tin-progress\tissue 4\n" {
		t.Fatalf("text: exit %d, %q, stderr %q", code, out, errOut)
	}
}

// Archived issues are left out of ls even with --closed, unless --archived or --status archived asks for them.
func TestLsHidesArchived(t *testing.T) {
	tr := newTracker(t)
	tr.writeConfig(archivedConfig)
	ids := tr.statuses("open", "done", "archived", "backlog")
	for _, c := range []struct {
		args []string
		rows []string
	}{
		{[]string{"ls"}, []string{ids[0], ids[3]}},
		{[]string{"ls", "--all"}, []string{ids[0], ids[3]}},
		{[]string{"ls", "--closed"}, []string{ids[0], ids[1], ids[3]}},
		{[]string{"ls", "--archived"}, []string{ids[0], ids[2], ids[3]}},
		{[]string{"ls", "--closed", "--archived"}, ids},
		{[]string{"ls", "--closed", "--archived", "--all"}, ids},
		{[]string{"ls", "--status", "archived"}, []string{ids[2]}},
		{[]string{"ls", "--status", "archived", "--tag", "area:x"}, []string{}},
	} {
		checkRows(t, tr, c.args, c.rows)
	}
}

// The config's closed and archived lists name the statuses that count as each. A status in both counts as archived,
// and a name that is not one of the statuses matches nothing.
func TestClosedAndArchivedFromConfig(t *testing.T) {
	tr := newTracker(t)
	tr.writeConfig("---\nstatuses: [todo, done, finished, wontfix, old]\nclosed: [finished, wontfix, gone]\n" +
		"archived: [old, wontfix]\n---\n")
	ids := tr.many(5, "issue")
	for n, s := range []string{"todo", "done", "finished", "wontfix", "old"} {
		if n > 0 {
			tr.issue("edit", ids[n], "--status", s)
		}
	}
	for _, c := range []struct {
		args []string
		rows []string
	}{
		// done is an ordinary status here: the config's closed list does not name it.
		{[]string{"ls"}, ids[:2]},
		{[]string{"ls", "--closed"}, ids[:3]},
		{[]string{"ls", "--archived"}, []string{ids[0], ids[1], ids[3], ids[4]}},
		{[]string{"ls", "--closed", "--archived"}, ids},
		{[]string{"ls", "--status", "wontfix"}, []string{ids[3]}},
	} {
		checkRows(t, tr, c.args, c.rows)
	}

	// Empty lists hide nothing, so ls lists every issue, as it did before closed and archived existed.
	tr.writeConfig("---\nstatuses: [todo, done, old]\nclosed: []\narchived: []\n---\n")
	checkRows(t, tr, []string{"ls"}, ids)
}

// search leaves out archived issues unless --archived is given. Closed issues are still searched.
func TestSearchHidesArchived(t *testing.T) {
	tr := newTracker(t)
	tr.writeConfig(archivedConfig)
	ids := tr.statuses("open", "done", "archived")
	checkRows(t, tr, []string{"search", "issue"}, ids[:2])
	checkRows(t, tr, []string{"search", "issue", "--archived"}, ids)
	checkRows(t, tr, []string{"search", "--archived", "--all", "issue"}, ids)
	if code, out, _ := tr.run("search", "issue", "--limit", "1"); code != 0 || out != ids[0]+"\t1\tissue 1\n" {
		t.Fatalf("text: exit %d, %q", code, out)
	}
}

// show reads any issue, closed or archived.
func TestShowArchived(t *testing.T) {
	tr := newTracker(t)
	tr.writeConfig(archivedConfig)
	ids := tr.statuses("done", "archived")
	for n, want := range []string{"done", "archived"} {
		if got := tr.issue("show", ids[n]); got.ID != ids[n] || got.Status != want {
			t.Errorf("show %s: %+v", ids[n], got)
		}
	}
}

// ls --branches hides a branch's closed and archived versions as it hides the checkout's.
func TestLsBranchesHidesClosed(t *testing.T) {
	tr := gitTracker(t, "main", ".")
	tr.writeConfig(archivedConfig)
	tr.commit(t0.Add(-36*time.Hour), "archived status")
	tr.git("switch", "-q", "-c", "feat")
	ids := tr.statuses("open", "done", "archived")
	tr.commit(t0.Add(-time.Hour), "feat")
	tr.git("switch", "-q", "main")
	for _, c := range []struct {
		args []string
		rows []string
	}{
		{[]string{"ls", "--branches"}, ids[:1]},
		{[]string{"ls", "--branches", "--closed"}, ids[:2]},
		{[]string{"ls", "--branches", "--closed", "--archived"}, ids},
	} {
		env, _ := tr.envelope(c.args...)
		if got := rowIDs(t, env); !reflect.DeepEqual(got, c.rows) {
			t.Errorf("%q: rows %q, want %q", c.args, got, c.rows)
		}
	}
}

func TestClosedArchivedUsageErrors(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "one").ID
	for _, args := range [][]string{
		{"ls", "--closed=yes"},
		{"ls", "--archived=no"},
		{"search", "one", "--closed"},
		{"show", id, "--closed"},
		{"show", id, "--archived"},
		{"tags", "--archived"},
		{"milestones", "--closed"},
	} {
		tr.wantError(2, "usage", args...)
	}
}
