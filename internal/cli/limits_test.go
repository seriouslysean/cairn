package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// envelope runs a command that must succeed with --json added at the end, and returns its whole envelope and stderr.
func (tr *tracker) envelope(args ...string) (map[string]any, string) {
	tr.t.Helper()
	code, out, errOut := tr.run(append(args, "--json")...)
	var env map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &env) != nil {
		tr.t.Fatalf("%q: exit %d, %q, %q", args, code, out, errOut)
	}
	return env, errOut
}

// many adds n issues titled "<title> 1" to "<title> n", one second apart, and returns their ids in that order, which
// is store order. seq makes only 32 different 4-character ids, one for each first byte mod 32, so many starts each
// issue's bytes one past the last's: a test that makes all its issues with many gets distinct ids for up to 32.
func (tr *tracker) many(n int, title string) []string {
	tr.t.Helper()
	ids := make([]string, n)
	next := tr.rnd.n
	for k := range ids {
		tr.now = tr.now.Add(time.Second)
		tr.rnd.n = next
		ids[k] = tr.issue("new", fmt.Sprintf("%s %d", title, k+1)).ID
		next++
	}
	tr.rnd.n = next
	return ids
}

// rowIDs is the "id" of each item in an envelope's data, which must be a list.
func rowIDs(t *testing.T, env map[string]any) []string {
	t.Helper()
	items, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data is not a list: %v", env["data"])
	}
	ids := []string{}
	for _, it := range items {
		ids = append(ids, it.(map[string]any)["id"].(string))
	}
	return ids
}

// checkCut checks a list command's envelope: the ids it kept, its total, and its one warning, or none when warning is
// "".
func checkCut(t *testing.T, args []string, env map[string]any, ids []string, total int, warning string) {
	t.Helper()
	if got := rowIDs(t, env); !reflect.DeepEqual(got, ids) {
		t.Errorf("%q: rows %q, want %q", args, got, ids)
	}
	if got, ok := env["total"].(float64); !ok || got != float64(total) {
		t.Errorf("%q: total %v, want %d", args, env["total"], total)
	}
	got, has := env["warnings"]
	switch {
	case warning == "" && has:
		t.Errorf("%q: warnings %v, want none", args, got)
	case warning != "" && !reflect.DeepEqual(got, []any{warning}):
		t.Errorf("%q: warnings %v, want [%q]", args, got, warning)
	}
}

// ls prints at most 30 rows unless told otherwise, and keeps the last ones: the newest issues.
func TestLsLimit(t *testing.T) {
	tr := newTracker(t)
	env, _ := tr.envelope("ls")
	checkCut(t, []string{"ls"}, env, []string{}, 0, "")

	ids := tr.many(32, "issue")
	for _, c := range []struct {
		args    []string
		rows    []string
		warning string
	}{
		{[]string{"ls"}, ids[2:], "30 of 32 shown; --limit N or --all for more"},
		{[]string{"ls", "--all"}, ids, ""},
		{[]string{"ls", "--limit", "5"}, ids[27:], "5 of 32 shown; --limit N or --all for more"},
		{[]string{"ls", "--limit=1"}, ids[31:], "1 of 32 shown; --limit N or --all for more"},
		{[]string{"ls", "--limit", "32"}, ids, ""},
		{[]string{"ls", "--limit", "100"}, ids, ""},
		// The last --limit wins, as for any flag but --tag.
		{[]string{"ls", "--limit", "1", "--limit", "2"}, ids[30:], "2 of 32 shown; --limit N or --all for more"},
	} {
		env, errOut := tr.envelope(c.args...)
		checkCut(t, c.args, env, c.rows, 32, c.warning)
		if want := warningLine(c.warning); errOut != want {
			t.Errorf("%q: stderr %q, want %q", c.args, errOut, want)
		}
	}

	// The cut comes after the filters, and total counts what the filters kept.
	for _, id := range []string{ids[0], ids[1], ids[31]} {
		tr.issue("edit", id, "--status", "open")
	}
	args := []string{"ls", "--status", "open", "--limit", "2"}
	env, _ = tr.envelope(args...)
	checkCut(t, args, env, []string{ids[1], ids[31]}, 3, "2 of 3 shown; --limit N or --all for more")
	env, _ = tr.envelope("ls", "--status", "done")
	checkCut(t, []string{"ls", "--status", "done"}, env, []string{}, 0, "")
}

// warningLine is what stderr gets for warning, or "" for none.
func warningLine(warning string) string {
	if warning == "" {
		return ""
	}
	return "cairn: warning: " + warning + "\n"
}

// Text mode cuts the same rows and gives the same warning, on stderr.
func TestLsLimitText(t *testing.T) {
	tr := newTracker(t)
	ids := tr.many(31, "issue")
	code, out, errOut := tr.run("ls")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if code != 0 || len(lines) != 30 || lines[0] != ids[1]+"\tbacklog\tissue 2" || lines[29] != ids[30]+"\tbacklog\tissue 31" {
		t.Fatalf("exit %d, %d lines, first %q, last %q", code, len(lines), lines[0], lines[len(lines)-1])
	}
	if errOut != "cairn: warning: 30 of 31 shown; --limit N or --all for more\n" {
		t.Fatalf("stderr %q", errOut)
	}
	if code, out, errOut := tr.run("ls", "--all"); code != 0 || strings.Count(out, "\n") != 31 || errOut != "" {
		t.Fatalf("--all: exit %d, %d lines, stderr %q", code, strings.Count(out, "\n"), errOut)
	}
}

// search keeps its best matches: the first rows, after the sort by matches.
func TestSearchLimit(t *testing.T) {
	tr := newTracker(t)
	alpha := tr.many(31, "alpha")
	best := tr.many(1, "alpha beta")[0]
	all := append([]string{best}, alpha...)
	for _, c := range []struct {
		args    []string
		rows    []string
		warning string
	}{
		{[]string{"search", "alpha", "beta"}, all[:30], "30 of 32 shown; --limit N or --all for more"},
		{[]string{"search", "alpha", "beta", "--limit", "1"}, all[:1], "1 of 32 shown; --limit N or --all for more"},
		{[]string{"search", "--all", "alpha", "beta"}, all, ""},
		{[]string{"search", "alpha", "beta", "--limit", "32"}, all, ""},
	} {
		env, errOut := tr.envelope(c.args...)
		checkCut(t, c.args, env, c.rows, 32, c.warning)
		if want := warningLine(c.warning); errOut != want {
			t.Errorf("%q: stderr %q, want %q", c.args, errOut, want)
		}
	}
	env, _ := tr.envelope("search", "nothing")
	checkCut(t, []string{"search", "nothing"}, env, []string{}, 0, "")

	want := best + "\t2\talpha beta 1\n" + alpha[0] + "\t1\talpha 1\n"
	code, out, errOut := tr.run("search", "alpha", "beta", "--limit", "2")
	if code != 0 || out != want || errOut != "cairn: warning: 2 of 32 shown; --limit N or --all for more\n" {
		t.Fatalf("text: exit %d, %q, stderr %q", code, out, errOut)
	}
}

// With --branches, ls prints the branches' rows last, so the cut keeps them and drops the oldest of the checkout's.
func TestLsBranchesLimit(t *testing.T) {
	tr := gitTracker(t, "main", ".")
	tr.git("switch", "-q", "-c", "feat")
	onFeat := tr.many(1, "On feat")[0]
	tr.commit(t0.Add(-time.Hour), "feat")
	tr.git("switch", "-q", "main")
	ids := tr.many(30, "issue")

	args := []string{"ls", "--branches"}
	env, _ := tr.envelope(args...)
	checkCut(t, args, env, append(ids[1:], onFeat), 31, "30 of 31 shown; --limit N or --all for more")
	if code, out, _ := tr.run("ls", "--branches", "--limit", "1"); code != 0 || out != onFeat+"\tbacklog\tOn feat 1\tfeat\n" {
		t.Fatalf("--limit 1: exit %d, %q", code, out)
	}
}

func TestLimitUsageErrors(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "one").ID
	for _, args := range [][]string{
		{"ls", "--limit", "0"},
		{"ls", "--limit", "-1"},
		{"ls", "--limit", "two"},
		{"ls", "--limit", "1.5"},
		{"ls", "--limit="},
		{"ls", "--limit"},
		{"ls", "--limit", "3", "--all"},
		{"ls", "--all", "--limit", "3"},
		{"ls", "--all=yes"},
		{"search", "one", "--limit", "0"},
		{"search", "one", "--all", "--limit", "2"},
		{"show", id, "--all"},
		{"show", id, "--limit", "1"},
		{"tags", "--all"},
		{"milestones", "--limit", "3"},
	} {
		tr.wantError(2, "usage", args...)
	}
}

// Only ls and search have a total, whatever their rows; no other command's envelope has one.
func TestTotalOnlyOnLsAndSearch(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "one").ID
	tr.issue("milestone", "new", "v1", "Version 1")
	for _, args := range [][]string{{"show", id}, {"tags"}, {"milestones"}, {"help"}, {"version"},
		{"edit", id, "--title", "two"}} {
		if env, _ := tr.envelope(args...); env["total"] != nil {
			t.Errorf("%q has a total: %v", args, env["total"])
		}
	}
}
