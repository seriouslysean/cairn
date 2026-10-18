package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// milestone runs "milestone <args>" with --json and returns its data as a map.
func (tr *tracker) milestone(args ...string) map[string]any {
	tr.t.Helper()
	full := append([]string{"milestone"}, args...)
	code, data, errCode := tr.json(full...)
	if code != 0 {
		tr.t.Fatalf("%q: exit %d (%s)", full, code, errCode)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		tr.t.Fatalf("%q: data is not an object: %s", full, data)
	}
	return m
}

// milestonePath is the path of a milestone's file.
func (tr *tracker) milestonePath(id string) string {
	return filepath.Join(tr.dir, ".cairn", "milestones", id+".md")
}

// milestoneFile is the text of a milestone's file.
func (tr *tracker) milestoneFile(id string) string {
	tr.t.Helper()
	data, err := os.ReadFile(tr.milestonePath(id))
	if err != nil {
		tr.t.Fatal(err)
	}
	return string(data)
}

func (tr *tracker) writeMilestone(id, text string) {
	tr.t.Helper()
	if err := os.WriteFile(tr.milestonePath(id), []byte(text), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

func TestMilestoneNew(t *testing.T) {
	tr := newTracker(t)
	got := tr.milestone("new", "v0.4", "First release", "--due", "2026-12-01")
	want := map[string]any{"id": "v0.4", "title": "First release", "status": "backlog", "due": "2026-12-01"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	if f := tr.milestoneFile("v0.4"); f != "---\nid: v0.4\ntitle: First release\nstatus: backlog\ndue: 2026-12-01\n---\n" {
		t.Fatalf("file: %q", f)
	}
	if code, out, errOut := tr.run("milestone", "new", "beta", "Beta", "--as", "bob", "--persona", "skeptic",
		"--model", "example-model-1"); code != 0 || out != "beta\n" || errOut != "" {
		t.Fatalf("text: exit %d, %q, %q", code, out, errOut)
	}
	// The actor is checked but stored nowhere: a milestone has no by.
	if f := tr.milestoneFile("beta"); f != "---\nid: beta\ntitle: Beta\nstatus: backlog\n---\n" {
		t.Fatalf("file: %q", f)
	}
	if got := tr.milestone("new", "gamma", "Gamma"); got["due"] != "" || len(got) != 4 {
		t.Fatalf("no due: %v", got)
	}
	items := tr.list("milestones")
	if len(items) != 3 || items[0]["id"] != "v0.4" || items[0]["title"] != "First release" {
		t.Fatalf("milestones: %v", items)
	}
}

// The folder can be gone, for example after a hand clean-up; new makes it again.
func TestMilestoneNewMakesTheFolder(t *testing.T) {
	tr := newTracker(t)
	if err := os.RemoveAll(filepath.Join(tr.dir, ".cairn", "milestones")); err != nil {
		t.Fatal(err)
	}
	tr.milestone("new", "v1", "One")
	if f := tr.milestoneFile("v1"); f != "---\nid: v1\ntitle: One\nstatus: backlog\n---\n" {
		t.Fatalf("file: %q", f)
	}
}

func TestMilestoneNewErrors(t *testing.T) {
	tr := newTracker(t)
	for _, args := range [][]string{
		{"milestone", "new"},
		{"milestone", "new", "v1"},
		{"milestone", "new", "v1", "  "},
		{"milestone", "new", "v1", "One", "extra"},
		{"milestone", "new", "", "One"},
		{"milestone", "new", "a/b", "One"},
		{"milestone", "new", ".hidden", "One"},
		{"milestone", "new", "..", "One"},
		{"milestone", "new", "v1", "One", "--due", "2026-13-01"},
		{"milestone", "new", "v1", "One", "--status", "open"},
		{"milestone", "new", "v1", "One", "--body", "text"},
	} {
		tr.wantError(2, "usage", args...)
	}
	if entries, err := os.ReadDir(filepath.Join(tr.dir, ".cairn", "milestones")); err != nil || len(entries) != 0 {
		t.Fatalf("usage errors wrote files: %v, %v", entries, err)
	}

	tr.milestone("new", "v1", "One")
	tr.wantError(1, "exists", "milestone", "new", "v1", "Other")
	if f := tr.milestoneFile("v1"); f != "---\nid: v1\ntitle: One\nstatus: backlog\n---\n" {
		t.Fatalf("exists changed the file: %q", f)
	}

	tr.writeMilestone("v2", "nope")
	tr.wantError(1, "error", "milestone", "new", "v2", "Two")
	if f := tr.milestoneFile("v2"); f != "nope" {
		t.Fatalf("a broken file was replaced: %q", f)
	}
}

func TestMilestoneEdit(t *testing.T) {
	tr := newTracker(t)
	tr.milestone("new", "v0.4", "First release", "--due", "2026-12-01")
	got := tr.milestone("edit", "v0.4", "--title", "First public release", "--status", "done", "--due", "")
	want := map[string]any{"id": "v0.4", "title": "First public release", "status": "done", "due": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	if f := tr.milestoneFile("v0.4"); f != "---\nid: v0.4\ntitle: First public release\nstatus: done\n---\n" {
		t.Fatalf("file: %q", f)
	}
	if code, out, _ := tr.run("milestone", "edit", "v0.4", "--due", "2027-01-15"); code != 0 || out != "v0.4\n" {
		t.Fatalf("text: exit %d, %q", code, out)
	}
	if got := tr.milestone("edit", "v0.4", "--status", "open"); got["due"] != "2027-01-15" ||
		got["title"] != "First public release" {
		t.Fatalf("an edit changed what it was not given: %v", got)
	}
}

// Keys Cairn does not know, and the body, are written by hand; an edit keeps both.
func TestMilestoneEditKeepsTheRest(t *testing.T) {
	tr := newTracker(t)
	tr.writeMilestone("v1", "---\nid: v1\ntitle: One\nstatus: open\nowner: bob\n---\nWhat ships.\n")
	tr.milestone("edit", "v1", "--status", "done")
	if f := tr.milestoneFile("v1"); f != "---\nid: v1\ntitle: One\nstatus: done\nowner: bob\n---\nWhat ships.\n" {
		t.Fatalf("file: %q", f)
	}
}

func TestMilestoneEditErrors(t *testing.T) {
	tr := newTracker(t)
	tr.milestone("new", "v1", "One")
	before := tr.milestoneFile("v1")
	for _, args := range [][]string{
		{"milestone", "edit"},
		{"milestone", "edit", "v1"},
		{"milestone", "edit", "v1", "--title", " "},
		{"milestone", "edit", "v1", "--status", "nope"},
		{"milestone", "edit", "v1", "--due", "soon"},
		{"milestone", "edit", "v1", "extra", "--title", "x"},
		{"milestone", "edit", "v1", "--body", "x"},
	} {
		tr.wantError(2, "usage", args...)
	}
	if f := tr.milestoneFile("v1"); f != before {
		t.Fatalf("usage errors changed the file: %q", f)
	}
	for _, id := range []string{"v9", "a/b", ".hidden"} {
		tr.wantError(4, "not_found", "milestone", "edit", id, "--title", "x")
	}
	_, out, errOut := tr.run("milestone", "edit", "v9", "--title", "x")
	if out != "" || !strings.Contains(errOut, `"v9"`) {
		t.Fatalf("not found must name the id: %q, %q", out, errOut)
	}
	tr.writeMilestone("v2", "nope")
	tr.wantError(1, "error", "milestone", "edit", "v2", "--title", "x")
	if f := tr.milestoneFile("v2"); f != "nope" {
		t.Fatalf("a broken file was replaced: %q", f)
	}
}

// milestones lists a milestone that issues name but that has no file. edit cannot reach it; new gives it a file, and
// the issues still count.
func TestMilestoneForANamedMilestone(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a", "--milestone", "beta")
	tr.wantError(4, "not_found", "milestone", "edit", "beta", "--title", "Beta")
	tr.milestone("new", "beta", "Beta")
	want := []map[string]any{{"id": "beta", "title": "Beta", "status": "backlog", "due": "", "issues": float64(1)}}
	if got := tr.list("milestones"); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestMilestoneWritesNeedAnActor(t *testing.T) {
	tr := newTracker(t)
	tr.milestone("new", "v1", "One")
	tr.env = map[string]string{}
	tr.wantError(2, "usage", "milestone", "new", "v2", "Two")
	tr.wantError(2, "usage", "milestone", "edit", "v1", "--title", "Changed")
	if _, err := os.Stat(tr.milestonePath("v2")); !os.IsNotExist(err) {
		t.Fatalf("new without an actor wrote a file: %v", err)
	}
	if f := tr.milestoneFile("v1"); f != "---\nid: v1\ntitle: One\nstatus: backlog\n---\n" {
		t.Fatalf("edit without an actor changed the file: %q", f)
	}
	tr.milestone("edit", "v1", "--title", "Changed", "--as", "bob")
}

func TestMilestoneSubcommands(t *testing.T) {
	tr := newTracker(t)
	for _, args := range [][]string{{"milestone"}, {"milestone", "rm", "v1"}, {"milestone", "show", "v1"}} {
		tr.wantError(2, "usage", args...)
	}
	empty := &tracker{t: t, dir: t.TempDir(), now: t0, env: map[string]string{"CAIRN_ACTOR": "alice"}, rnd: &seq{}}
	empty.wantError(1, "no_tracker", "milestone", "new", "v1", "One")
	empty.wantError(1, "no_tracker", "milestone", "edit", "v1", "--title", "x")
}

func TestHelpListsMilestone(t *testing.T) {
	tr := newTracker(t)
	_, out, _ := tr.run("help")
	if !slices.Contains(strings.Fields(out), "milestone") {
		t.Errorf("help does not list milestone: %q", out)
	}
}
