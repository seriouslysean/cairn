package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type retraction struct {
	By          string `json:"by"`
	At          string `json:"at"`
	Reason      string `json:"reason"`
	DisprovedBy string `json:"disproved_by"`
}

type comment struct {
	ID        string      `json:"id"`
	Kind      string      `json:"kind"`
	By        string      `json:"by"`
	Persona   string      `json:"persona"`
	Model     string      `json:"model"`
	At        string      `json:"at"`
	Source    string      `json:"source"`
	Body      string      `json:"body"`
	Retracted *retraction `json:"retracted"`
}

type warning struct {
	Comment string  `json:"comment"`
	Reason  string  `json:"reason"`
	Score   float64 `json:"score"`
}

type added struct {
	Comment  comment   `json:"comment"`
	Warnings []warning `json:"warnings"`
}

// comment runs a comment command with --json and returns its data.
func (tr *tracker) comment(args ...string) added {
	tr.t.Helper()
	code, data, errCode := tr.json(args...)
	if code != 0 {
		tr.t.Fatalf("%q: exit %d (%s)", args, code, errCode)
	}
	var a added
	if err := json.Unmarshal(data, &a); err != nil {
		tr.t.Fatalf("%q: data is not a comment with warnings: %s", args, data)
	}
	return a
}

// retract runs a retract command with --json and returns the retracted comment.
func (tr *tracker) retract(args ...string) comment {
	tr.t.Helper()
	code, data, errCode := tr.json(append([]string{"retract"}, args...)...)
	if code != 0 {
		tr.t.Fatalf("retract %q: exit %d (%s)", args, code, errCode)
	}
	var c comment
	if err := json.Unmarshal(data, &c); err != nil {
		tr.t.Fatalf("retract %q: data is not a comment: %s", args, data)
	}
	return c
}

// comments runs show with --json and the extra arguments, checks that "comments" is a list, and returns it.
func (tr *tracker) comments(args ...string) []comment {
	tr.t.Helper()
	code, data, errCode := tr.json(append([]string{"show"}, args...)...)
	if code != 0 {
		tr.t.Fatalf("show %q: exit %d (%s)", args, code, errCode)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		tr.t.Fatalf("show %q: data is not an object: %s", args, data)
	}
	raw, ok := m["comments"]
	if !ok || !bytes.HasPrefix(raw, []byte("[")) {
		tr.t.Fatalf(`show %q: "comments" must be a list, got %s`, args, raw)
	}
	cs := []comment{}
	if err := json.Unmarshal(raw, &cs); err != nil {
		tr.t.Fatalf("show %q: comments: %v", args, err)
	}
	return cs
}

// files lists the names in an issue folder.
func (tr *tracker) files(issueID string) []string {
	tr.t.Helper()
	entries, err := os.ReadDir(filepath.Join(tr.dir, ".cairn", "issues", issueID))
	if err != nil {
		tr.t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// warningLines checks that stderr is only warning lines and returns them.
func warningLines(t *testing.T, stderr string) []string {
	t.Helper()
	if stderr == "" {
		return nil
	}
	if !strings.HasSuffix(stderr, "\n") {
		t.Fatalf("stderr does not end in a newline: %q", stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "cairn: warning: ") {
			t.Fatalf("stderr line is not a warning: %q", l)
		}
	}
	return lines
}

func TestComment(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "Export drops the last row")
	got := tr.comment("comment", "0123", "--kind", "evidence", "--source", "export_test.go:42",
		"--persona", "skeptic", "--model", "example-model-1", "The last row is dropped")
	want := comment{ID: "c-4567", Kind: "evidence", By: "alice", Persona: "skeptic", Model: "example-model-1",
		At: "2026-10-18T09:30:00.123Z", Source: "export_test.go:42", Body: "The last row is dropped\n"}
	if !reflect.DeepEqual(got.Comment, want) {
		t.Fatalf("comment\ngot  %+v\nwant %+v", got.Comment, want)
	}
	if got.Warnings == nil || len(got.Warnings) != 0 {
		t.Fatalf("warnings must be an empty list, got %#v", got.Warnings)
	}
	if _, err := os.Stat(filepath.Join(tr.dir, ".cairn", "issues", "0123", "c-4567.md")); err != nil {
		t.Fatal(err)
	}
	if cs := tr.comments("0123"); !reflect.DeepEqual(cs, []comment{want}) {
		t.Fatalf("show\ngot  %+v\nwant %+v", cs, []comment{want})
	}

	// --as wins over CAIRN_ACTOR, and decision and result need no source.
	d := tr.comment("comment", "0123", "--kind", "decision", "--as", "bob", `"Ship it without the flag."`).Comment
	if d.ID != "c-89ab" || d.By != "bob" || d.Source != "" || d.Persona != "" || d.Model != "" {
		t.Fatalf("decision: %+v", d)
	}
	if r := tr.comment("comment", "0123", "--kind", "result", "PR 12 merged\n").Comment; r.Body != "PR 12 merged\n" {
		t.Fatalf("result body: %q", r.Body)
	}

	if code, out, errOut := tr.run("comment", "0123", "--kind", "result", "Tests pass"); code != 0 ||
		out != "c-ghjk\n" || errOut != "" {
		t.Fatalf("text: exit %d, %q, %q", code, out, errOut)
	}
}

func TestCommentJSONShape(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	_, data, _ := tr.json("comment", "0123", "--kind", "result", "done")
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil || len(top) != 2 || string(top["warnings"]) != "[]" {
		t.Fatalf(`want {"comment", "warnings": []}, got %s`, data)
	}
	var c map[string]any
	if err := json.Unmarshal(top["comment"], &c); err != nil {
		t.Fatal(err)
	}
	keys := []string{"id", "kind", "by", "persona", "model", "at", "source", "body", "retracted"}
	for _, k := range keys {
		if _, ok := c[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if len(c) != len(keys) || c["retracted"] != nil || c["persona"] != "" || c["source"] != "" {
		t.Errorf("comment: %v", c)
	}

	// show adds "comments" to the issue; new and edit do not.
	_, data, _ = tr.json("show", "0123")
	var shown map[string]any
	if err := json.Unmarshal(data, &shown); err != nil || len(shown) != 14 {
		t.Fatalf("show: want the 13 issue keys and comments, got %s", data)
	}
	_, data, _ = tr.json("edit", "0123", "--title", "b")
	var edited map[string]any
	if err := json.Unmarshal(data, &edited); err != nil || len(edited) != 13 {
		t.Fatalf("edit: want 13 keys, got %s", data)
	}

	tr.retract("0123", "c-4567", "--reason", "wrong issue")
	_, data, _ = tr.json("show", "0123", "--all")
	var all struct {
		Comments []map[string]any `json:"comments"`
	}
	if err := json.Unmarshal(data, &all); err != nil || len(all.Comments) != 1 {
		t.Fatalf("show --all: %s", data)
	}
	wantRetracted := map[string]any{"by": "alice", "at": "2026-10-18T09:30:00.123Z", "reason": "wrong issue",
		"disproved_by": ""}
	if got := all.Comments[0]["retracted"]; !reflect.DeepEqual(got, wantRetracted) {
		t.Fatalf("retracted: got %#v, want %#v", got, wantRetracted)
	}
	if cs := tr.comments("0123"); len(cs) != 0 {
		t.Fatalf("no active comments: want [], got %+v", cs)
	}
}

func TestCommentOrder(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.now = t0.Add(2 * time.Minute)
	tr.comment("comment", "0123", "--kind", "result", "late") // c-4567
	tr.now = t0.Add(time.Minute)
	tr.comment("comment", "0123", "--kind", "result", "early")                    // c-89ab
	tr.comment("comment", "0123", "--kind", "result", "also early, same instant") // c-cdef
	var ids []string
	for _, c := range tr.comments("0123") {
		ids = append(ids, c.ID)
	}
	if want := []string{"c-89ab", "c-cdef", "c-4567"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("order %q, want %q", ids, want)
	}
}

// A comment is its own file so that branches never conflict on issue.md: commenting must not touch it.
func TestCommentsLeaveTheIssueFileAlone(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	path := filepath.Join(tr.dir, ".cairn", "issues", "0123", "issue.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tr.now = t0.Add(time.Hour)
	tr.comment("comment", "0123", "--kind", "result", "one")
	tr.comment("comment", "0123", "--kind", "result", "two")
	tr.retract("0123", "c-4567", "--reason", "no")
	if code, _, errCode := tr.json("purge", "0123", "c-89ab"); code != 0 {
		t.Fatalf("purge: exit %d (%s)", code, errCode)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("issue.md changed:\n%s\nto\n%s", before, after)
	}
}

func TestCommentUsageErrors(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	for _, args := range [][]string{
		{"comment"},
		{"comment", "0123"},
		{"comment", "0123", "--kind", "result"},
		{"comment", "0123", "no kind"},
		{"comment", "0123", "--kind", "evidence", "no source"},
		{"comment", "0123", "--kind", "evidence", "--source", "", "empty source"},
		{"comment", "0123", "--kind", "evidence", "--source", "  ", "blank source"},
		{"comment", "0123", "--kind", "github", "pull makes these"},
		{"comment", "0123", "--kind", "note", "not a kind"},
		{"comment", "0123", "--kind", "Result", "kinds are exact"},
		{"comment", "0123", "--kind", "result", "   "},
		{"comment", "0123", "--kind", "result", "two", "texts"},
		{"comment", "0123", "--kind", "result", "--all", "show only"},
		{"comment", "0123", "--kind", "result", "--branches", "later"},
		{"comment", "0123", "--kind"},
	} {
		tr.wantError(2, "usage", args...)
	}
	if got := tr.files("0123"); !reflect.DeepEqual(got, []string{"issue.md"}) {
		t.Fatalf("a refused comment left files: %q", got)
	}
}

func TestCommentNotFound(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.wantError(4, "not_found", "comment", "zzzz", "--kind", "result", "x")
	tr.wantError(4, "not_found", "comment", "../0123", "--kind", "result", "x")
	code, _, errOut := tr.run("comment", "zzzz", "--kind", "result", "x")
	if code != 4 || !strings.Contains(errOut, "zzzz") {
		t.Fatalf("exit %d, %q", code, errOut)
	}
	empty := &tracker{t: t, dir: t.TempDir(), now: t0, env: map[string]string{"CAIRN_ACTOR": "alice"}, rnd: &seq{}}
	empty.wantError(1, "no_tracker", "comment", "0123", "--kind", "result", "x")
	empty.wantError(1, "no_tracker", "retract", "0123", "c-4567", "--reason", "x")
	empty.wantError(1, "no_tracker", "purge", "0123", "c-4567")
}

func TestCommentWritesNeedAnActor(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.comment("comment", "0123", "--kind", "result", "done")
	tr.env = map[string]string{}
	tr.wantError(2, "usage", "comment", "0123", "--kind", "result", "again")
	tr.wantError(2, "usage", "retract", "0123", "c-4567", "--reason", "x")
	tr.wantError(2, "usage", "purge", "0123", "c-4567")
	if got := tr.files("0123"); !reflect.DeepEqual(got, []string{"c-4567.md", "issue.md"}) {
		t.Fatalf("files: %q", got)
	}
	if cs := tr.comments("0123"); len(cs) != 1 || cs[0].Retracted != nil {
		t.Fatalf("comments: %+v", cs)
	}
}

func TestRetract(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	first := tr.comment("comment", "0123", "--kind", "evidence", "--source", "log.txt", "The cache is stale").Comment
	tr.now = t0.Add(time.Minute)
	second := tr.comment("comment", "0123", "--kind", "evidence", "--source", "trace.txt",
		"The cache was rebuilt at start").Comment
	tr.now = t0.Add(2 * time.Minute)
	got := tr.retract("0123", "c-4567", "--as", "bob", "--reason", "the trace shows a rebuild",
		"--disproved-by", "c-89ab")
	want := first
	want.Retracted = &retraction{By: "bob", At: "2026-10-18T09:32:00.123Z", Reason: "the trace shows a rebuild",
		DisprovedBy: "c-89ab"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retract\ngot  %+v\nwant %+v", got, want)
	}

	// show leaves it out, and --all keeps it in its place, marked.
	if cs := tr.comments("0123"); !reflect.DeepEqual(cs, []comment{second}) {
		t.Fatalf("show\ngot  %+v\nwant %+v", cs, []comment{second})
	}
	if cs := tr.comments("0123", "--all"); !reflect.DeepEqual(cs, []comment{want, second}) {
		t.Fatalf("show --all\ngot  %+v\nwant %+v", cs, []comment{want, second})
	}

	// The file keeps the comment and adds the spec's fields.
	data, err := os.ReadFile(filepath.Join(tr.dir, ".cairn", "issues", "0123", "c-4567.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"retracted_by: bob\n", "retracted_at: 2026-10-18T09:32:00.123Z\n",
		"retracted_reason: the trace shows a rebuild\n", "disproved_by: c-89ab\n", "\nThe cache is stale\n"} {
		if !strings.Contains(string(data), s) {
			t.Errorf("file has no %q:\n%s", s, data)
		}
	}

	// Without --disproved-by there is none, and the text output names the comment.
	if code, out, _ := tr.run("retract", "0123", "c-89ab", "--reason", "also wrong"); code != 0 ||
		out != "retracted c-89ab\n" {
		t.Fatalf("text: exit %d, %q", code, out)
	}
	if cs := tr.comments("0123", "--all"); len(cs) != 2 || cs[1].Retracted == nil || cs[1].Retracted.DisprovedBy != "" {
		t.Fatalf("show --all: %+v", cs)
	}
}

func TestRetractErrors(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.comment("comment", "0123", "--kind", "result", "one") // c-4567
	tr.comment("comment", "0123", "--kind", "result", "two") // c-89ab
	for _, args := range [][]string{
		{"retract"},
		{"retract", "0123"},
		{"retract", "0123", "c-4567"},
		{"retract", "0123", "c-4567", "--reason", ""},
		{"retract", "0123", "c-4567", "--reason", "   "},
		{"retract", "0123", "c-4567", "c-89ab", "--reason", "x"},
		{"retract", "0123", "c-4567", "--reason", "x", "--disproved-by", "c-4567"},
		{"retract", "0123", "c-4567", "--reason", "x", "--all"},
	} {
		tr.wantError(2, "usage", args...)
	}
	for _, missing := range [][]string{
		{"zzzz", "retract", "zzzz", "c-4567", "--reason", "x"},
		{"c-zzzz", "retract", "0123", "c-zzzz", "--reason", "x"},
		{"c-zzzz", "retract", "0123", "c-4567", "--reason", "x", "--disproved-by", "c-zzzz"},
		{"../x", "retract", "0123", "../x", "--reason", "x"},
	} {
		tr.wantError(4, "not_found", missing[1:]...)
		if code, _, errOut := tr.run(missing[1:]...); code != 4 || !strings.Contains(errOut, missing[0]) {
			t.Errorf("%q: exit %d, want a message naming %q: %q", missing[1:], code, missing[0], errOut)
		}
	}
	if cs := tr.comments("0123"); len(cs) != 2 {
		t.Fatalf("a refused retract changed something: %+v", cs)
	}

	// A comment is retracted once, and a second retract leaves the first one as it was.
	tr.retract("0123", "c-4567", "--reason", "first reason")
	tr.wantError(1, "error", "retract", "0123", "c-4567", "--reason", "second reason")
	if cs := tr.comments("0123", "--all"); cs[0].Retracted == nil || cs[0].Retracted.Reason != "first reason" {
		t.Fatalf("after a second retract: %+v", cs[0])
	}
}

func TestPurge(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.comment("comment", "0123", "--kind", "result", "pasted a secret")         // c-4567
	keep := tr.comment("comment", "0123", "--kind", "result", "keep me").Comment // c-89ab
	code, data, errCode := tr.json("purge", "0123", "c-4567")
	var purged map[string]string
	if code != 0 || json.Unmarshal(data, &purged) != nil ||
		!reflect.DeepEqual(purged, map[string]string{"id": "0123", "comment": "c-4567"}) {
		t.Fatalf("purge: exit %d (%s), %s", code, errCode, data)
	}
	if got := tr.files("0123"); !reflect.DeepEqual(got, []string{"c-89ab.md", "issue.md"}) {
		t.Fatalf("files: %q", got)
	}
	if cs := tr.comments("0123", "--all"); !reflect.DeepEqual(cs, []comment{keep}) {
		t.Fatalf("show --all: %+v", cs)
	}
	tr.wantError(4, "not_found", "purge", "0123", "c-4567")

	// A retracted comment can be purged too, and the text output names it.
	tr.retract("0123", "c-89ab", "--reason", "no")
	if code, out, _ := tr.run("purge", "0123", "c-89ab"); code != 0 || out != "purged c-89ab\n" {
		t.Fatalf("text: exit %d, %q", code, out)
	}
	if cs := tr.comments("0123", "--all"); len(cs) != 0 {
		t.Fatalf("show --all: %+v", cs)
	}
}

// Comment ids become paths, so a bad one must never reach the file system.
func TestPurgeBadIDs(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.comment("comment", "0123", "--kind", "result", "one") // c-4567
	for _, bad := range []string{"../0123/issue", "c-../x", "issue", "issue.md", "c-4567.md", "", "c-45", "c-zzzz"} {
		tr.wantError(4, "not_found", "purge", "0123", bad)
	}
	if code, _, errOut := tr.run("purge", "0123", "c-zzzz"); code != 4 || !strings.Contains(errOut, "c-zzzz") {
		t.Fatalf("exit %d, want a message naming c-zzzz: %q", code, errOut)
	}
	tr.wantError(4, "not_found", "purge", "zzzz", "c-4567")
	tr.wantError(4, "not_found", "purge", "../.cairn", "c-4567")
	tr.wantError(2, "usage", "purge", "0123")
	tr.wantError(2, "usage", "purge", "0123", "c-4567", "extra")
	if got := tr.files("0123"); !reflect.DeepEqual(got, []string{"c-4567.md", "issue.md"}) {
		t.Fatalf("files: %q", got)
	}
	if _, err := os.Stat(filepath.Join(tr.dir, ".cairn", "config.yml")); err != nil {
		t.Fatal(err)
	}
}

func TestRetractedMatchWarns(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.comment("comment", "0123", "--kind", "evidence", "--source", "log.txt", "a b c d") // c-4567
	tr.retract("0123", "c-4567", "--reason", "wrong:\nthe log was old")

	// 3 shared words of 5 is 0.6, which reaches dupe_text: the comment is still added, with a warning.
	code, out, errOut := tr.run("comment", "0123", "--kind", "result", "A b, c e.")
	if code != 0 || out != "c-89ab\n" {
		t.Fatalf("text: exit %d, %q", code, out)
	}
	lines := warningLines(t, errOut)
	if len(lines) != 1 || !strings.Contains(lines[0], "c-4567") || !strings.Contains(lines[0], "the log was old") {
		t.Fatalf("text warning: %q", errOut)
	}

	code, out, errOut = tr.run("comment", "0123", "--kind", "result", "a b c e", "--json")
	var env struct {
		OK   bool  `json:"ok"`
		Data added `json:"data"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &env) != nil || !env.OK {
		t.Fatalf("json: exit %d, %q", code, out)
	}
	want := []warning{{Comment: "c-4567", Reason: "wrong:\nthe log was old", Score: 0.6}}
	if !reflect.DeepEqual(env.Data.Warnings, want) || env.Data.Comment.ID != "c-cdef" {
		t.Fatalf("json warnings\ngot  %+v\nwant %+v", env.Data, want)
	}
	if lines := warningLines(t, errOut); len(lines) != 1 || !strings.Contains(lines[0], "c-4567") {
		t.Fatalf("json warning on stderr: %q", errOut)
	}

	// 2 of 6 does not reach it.
	if code, _, errOut := tr.run("comment", "0123", "--kind", "result", "a b e f"); code != 0 || errOut != "" {
		t.Fatalf("below the threshold: exit %d, %q", code, errOut)
	}

	// A near copy of an active comment does not warn: only retracted comments count.
	tr.comment("comment", "0123", "--kind", "result", "x y z")
	if a := tr.comment("comment", "0123", "--kind", "result", "x y z"); len(a.Warnings) != 0 {
		t.Fatalf("active match warned: %+v", a.Warnings)
	}

	// Two retracted matches give two warnings, in comment order.
	tr.comment("comment", "0123", "--kind", "result", "a b c d") // c-wxyz
	tr.retract("0123", "c-wxyz", "--reason", "again")
	a := tr.comment("comment", "0123", "--kind", "result", "a b c d")
	if len(a.Warnings) != 2 || a.Warnings[0].Comment != "c-4567" || a.Warnings[1].Comment != "c-wxyz" ||
		a.Warnings[1].Score != 1 {
		t.Fatalf("two matches: %+v", a.Warnings)
	}
}

func TestRetractedMatchScope(t *testing.T) {
	tr := newTracker(t)
	first := tr.issue("new", "first").ID
	second := tr.issue("new", "second").ID
	c := tr.comment("comment", first, "--kind", "result", "the cache is stale").Comment
	tr.retract(first, c.ID, "--reason", "it was not")

	// Only the same issue counts.
	if a := tr.comment("comment", second, "--kind", "result", "the cache is stale"); len(a.Warnings) != 0 {
		t.Fatalf("another issue's retracted comment warned: %+v", a.Warnings)
	}

	// The threshold is the config's dupe_text.
	path := filepath.Join(tr.dir, ".cairn", "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raised := strings.Replace(string(data), "dupe_text: 0.6\n", "dupe_text: 0.9\n", 1)
	if raised == string(data) {
		t.Fatalf("config has no dupe_text: 0.6:\n%s", data)
	}
	if err := os.WriteFile(path, []byte(raised), 0o644); err != nil {
		t.Fatal(err)
	}
	if a := tr.comment("comment", first, "--kind", "result", "the cache is stale now"); len(a.Warnings) != 0 {
		t.Fatalf("0.8 warned with dupe_text 0.9: %+v", a.Warnings)
	}
	if a := tr.comment("comment", first, "--kind", "result", "The cache is STALE"); len(a.Warnings) != 1 {
		t.Fatalf("1.0 did not warn with dupe_text 0.9: %+v", a.Warnings)
	}
}

func TestShowTextComments(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "First")
	tr.comment("comment", "0123", "--kind", "evidence", "--source", "export_test.go:42", "--persona", "skeptic",
		"--model", "example-model-1", "Rows go missing") // c-4567
	tr.comment("comment", "0123", "--kind", "decision", "--as", "bob", "Keep the old format") // c-89ab
	tr.retract("0123", "c-89ab", "--reason", "the owner changed their mind")

	_, out, _ := tr.run("show", "0123")
	for _, s := range []string{"c-4567", "evidence", "alice", "skeptic", "example-model-1", "export_test.go:42",
		"Rows go missing"} {
		if !strings.Contains(out, s) {
			t.Errorf("show: no %q in %q", s, out)
		}
	}
	if strings.Index(out, "First") > strings.Index(out, "c-4567") {
		t.Errorf("show: the issue must come before its comments: %q", out)
	}
	for _, s := range []string{"c-89ab", "Keep the old format", "the owner changed their mind"} {
		if strings.Contains(out, s) {
			t.Errorf("show without --all has %q: %q", s, out)
		}
	}

	_, out, _ = tr.run("show", "0123", "--all")
	for _, s := range []string{"c-4567", "c-89ab", "Keep the old format", "retracted", "the owner changed their mind"} {
		if !strings.Contains(out, s) {
			t.Errorf("show --all: no %q in %q", s, out)
		}
	}
}

func TestAllIsOnlyForShow(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.wantError(2, "usage", "ls", "--all")
	tr.wantError(2, "usage", "show", "0123", "--all", "--branches")
}

func TestHelpListsCommentCommands(t *testing.T) {
	tr := newTracker(t)
	_, out, _ := tr.run("help")
	for _, cmd := range []string{"comment", "retract", "purge"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("help does not list %q: %q", cmd, out)
		}
	}
}

// A failed write is exit 1 with the code "error", like every other failure the spec does not name.
func TestCommentSaveFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes to read-only folders")
	}
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.comment("comment", "0123", "--kind", "result", "one") // c-4567
	folder := filepath.Join(tr.dir, ".cairn", "issues", "0123")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(folder, 0o755) })
	tr.wantError(1, "error", "comment", "0123", "--kind", "result", "two")
	tr.wantError(1, "error", "retract", "0123", "c-4567", "--reason", "x")
}

// purge is how a bad comment file goes away, so it must work even when that file does not parse.
func TestPurgeBrokenComment(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	broken := filepath.Join(tr.dir, ".cairn", "issues", "0123", "c-zzzz.md")
	if err := os.WriteFile(broken, []byte("not frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errCode := tr.json("purge", "0123", "c-zzzz"); code != 0 {
		t.Fatalf("purge: exit %d (%s)", code, errCode)
	}
	if _, err := os.Stat(broken); !os.IsNotExist(err) {
		t.Fatalf("broken comment still there: %v", err)
	}
}

// A purge on a missing issue names the issue, not the comment.
func TestPurgeMissingIssueNamesIt(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	code, _, errOut := tr.run("purge", "zzzz", "c-4567")
	if code != 4 || !strings.Contains(errOut, "zzzz") {
		t.Fatalf("exit %d, want a message naming zzzz: %q", code, errOut)
	}
}
