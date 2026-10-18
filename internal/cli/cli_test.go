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

	"github.com/seriouslysean/cairn/internal/model"
	"github.com/seriouslysean/cairn/internal/store"
)

var t0 = time.Date(2026, 10, 18, 9, 30, 0, 123456789, time.UTC)

// seq stands in for crypto/rand. It yields 0, 1, 2, …, so issue ids come out as "0123", "4567", "89ab", ….
type seq struct{ n byte }

func (s *seq) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = s.n
		s.n++
	}
	return len(p), nil
}

type tracker struct {
	t   *testing.T
	dir string
	now time.Time
	env map[string]string
	rnd *seq
}

// newTracker runs init in a fresh folder, with CAIRN_ACTOR=alice and the clock at t0.
func newTracker(t *testing.T) *tracker {
	t.Helper()
	tr := &tracker{t: t, dir: t.TempDir(), now: t0, env: map[string]string{"CAIRN_ACTOR": "alice"}, rnd: &seq{}}
	if code, _, errOut := tr.run("init"); code != 0 {
		t.Fatalf("init: exit %d: %s", code, errOut)
	}
	return tr
}

func (tr *tracker) run(args ...string) (int, string, string) {
	tr.t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, Env{Version: "1.2.3", Dir: tr.dir, Stdout: &out, Stderr: &errOut,
		Getenv: func(k string) string { return tr.env[k] }, Now: func() time.Time { return tr.now }, Rand: tr.rnd})
	return code, out.String(), errOut.String()
}

// json runs a command with --json added at the end and checks the envelope. It returns the exit code, the data on
// success, and the error code on failure.
func (tr *tracker) json(args ...string) (int, json.RawMessage, string) {
	tr.t.Helper()
	code, out, errOut := tr.run(append(args, "--json")...)
	var e struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		tr.t.Fatalf("%q: stdout is not one JSON envelope: %q", args, out)
	}
	if e.OK != (code == 0) {
		tr.t.Fatalf("%q: exit %d with ok=%v", args, code, e.OK)
	}
	if code == 0 {
		return code, e.Data, ""
	}
	if e.Error == nil || e.Error.Code == "" || e.Error.Message == "" {
		tr.t.Fatalf("%q: failure without an error code and message: %q", args, out)
	}
	if strings.Count(errOut, "\n") != 1 || !strings.HasSuffix(errOut, "\n") {
		tr.t.Fatalf("%q: stderr must be one line: %q", args, errOut)
	}
	return code, nil, e.Error.Code
}

type issue struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Tags      []string `json:"tags"`
	Milestone string   `json:"milestone"`
	Due       string   `json:"due"`
	Created   string   `json:"created"`
	Updated   string   `json:"updated"`
	GitHub    int      `json:"github"`
	Body      string   `json:"body"`
}

func (tr *tracker) issue(args ...string) issue {
	tr.t.Helper()
	code, data, errCode := tr.json(args...)
	if code != 0 {
		tr.t.Fatalf("%q: exit %d (%s)", args, code, errCode)
	}
	var i issue
	if err := json.Unmarshal(data, &i); err != nil {
		tr.t.Fatalf("%q: data is not an issue: %s", args, data)
	}
	return i
}

// compact is data with the whitespace taken out, for comparing JSON however it was indented.
func compact(t *testing.T, data json.RawMessage) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, data); err != nil {
		t.Fatalf("bad JSON %q: %v", data, err)
	}
	return b.String()
}

func (tr *tracker) wantError(exit int, errCode string, args ...string) {
	tr.t.Helper()
	if code, _, got := tr.json(args...); code != exit || got != errCode {
		tr.t.Errorf("%q: exit %d %q, want %d %q", args, code, got, exit, errCode)
	}
}

func TestInit(t *testing.T) {
	tr := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}}
	code, data, _ := tr.json("init")
	var got map[string]string
	if code != 0 || json.Unmarshal(data, &got) != nil || got["dir"] != filepath.Join(tr.dir, ".cairn") {
		t.Fatalf("exit %d, data %s", code, data)
	}
	if _, err := os.Stat(filepath.Join(tr.dir, ".cairn", "config.yml")); err != nil {
		t.Fatal(err)
	}
	tr.wantError(1, "exists", "init")
}

func TestNoTracker(t *testing.T) {
	tr := &tracker{t: t, dir: t.TempDir(), now: t0, env: map[string]string{"CAIRN_ACTOR": "alice"}, rnd: &seq{}}
	tr.wantError(1, "no_tracker", "ls")
	tr.wantError(1, "no_tracker", "new", "title")
	code, out, errOut := tr.run("show", "0123")
	if code != 1 || out != "" || !strings.HasPrefix(errOut, "cairn: ") || strings.Count(errOut, "\n") != 1 {
		t.Fatalf("text failure: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestNewAndShow(t *testing.T) {
	tr := newTracker(t)
	got := tr.issue("new", "Export drops the last row", "--body", "What is wrong", "--tag", "severity:high",
		"--tag", "area:export", "--milestone", "v0.4", "--due", "2026-11-01")
	want := issue{ID: "0123", Title: "Export drops the last row", Status: "backlog",
		Tags: []string{"severity:high", "area:export"}, Milestone: "v0.4", Due: "2026-11-01",
		Created: "2026-10-18T09:30:00.123Z", Updated: "2026-10-18T09:30:00.123Z", Body: "What is wrong\n"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("new\ngot  %+v\nwant %+v", got, want)
	}
	if shown := tr.issue("show", "0123"); !reflect.DeepEqual(shown, want) {
		t.Fatalf("show\ngot  %+v\nwant %+v", shown, want)
	}
	if _, err := os.Stat(filepath.Join(tr.dir, ".cairn", "issues", "0123", "issue.md")); err != nil {
		t.Fatal(err)
	}
}

func TestIssueJSONShape(t *testing.T) {
	tr := newTracker(t)
	_, data, _ := tr.json("new", "bare")
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "title", "status", "tags", "milestone", "due", "created", "updated", "github", "body"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if len(m) != 10 {
		t.Errorf("want exactly 10 keys, got %v", m)
	}
	if tags, ok := m["tags"].([]any); !ok || len(tags) != 0 {
		t.Errorf("tags must be an empty list, got %#v", m["tags"])
	}
	if m["github"] != float64(0) || m["milestone"] != "" || m["due"] != "" || m["body"] != "" {
		t.Errorf("empty values: %v", m)
	}
}

func TestTextOutput(t *testing.T) {
	tr := newTracker(t)
	if code, out, errOut := tr.run("new", "First"); code != 0 || out != "0123\n" || errOut != "" {
		t.Fatalf("new: exit %d, %q, %q", code, out, errOut)
	}
	if code, out, _ := tr.run("edit", "0123", "--status", "open"); code != 0 || out != "0123\n" {
		t.Fatalf("edit: exit %d, %q", code, out)
	}
	tr.run("new", "Second")
	if code, out, _ := tr.run("ls"); code != 0 || out != "0123\topen\tFirst\n4567\tbacklog\tSecond\n" {
		t.Fatalf("ls: exit %d, %q", code, out)
	}
	tr.run("edit", "0123", "--body", "The details.")
	code, out, _ := tr.run("show", "0123")
	for _, s := range []string{"0123", "First", "open", "The details."} {
		if code != 0 || !strings.Contains(out, s) {
			t.Errorf("show: exit %d, no %q in %q", code, s, out)
		}
	}
	if code, out, _ := tr.run("rm", "4567"); code != 0 || out != "removed 4567\n" {
		t.Fatalf("rm: exit %d, %q", code, out)
	}
}

func TestNewUsageErrors(t *testing.T) {
	tr := newTracker(t)
	for _, args := range [][]string{
		{"new"},
		{"new", "   "},
		{"new", "a", "b"},
		{"new", "a", "--due", "2026-13-01"},
		{"new", "a", "--due", "tomorrow"},
		{"new", "a", "--tag", "has space"},
		{"new", "a", "--tag", "a,b"},
		{"new", "a", "--tag", "colour:blue"},
		{"new", "a", "--tag", ""},
		{"new", "a", "--milestone", "a/b"},
		{"new", "a", "--nope"},
		{"new", "a", "--tag"},
		{"new", "a", "--json=true"},
	} {
		tr.wantError(2, "usage", args...)
	}
	if _, data, _ := tr.json("ls"); compact(t, data) != "[]" {
		t.Fatalf("a failed new must not add an issue: %s", data)
	}
}

func TestActor(t *testing.T) {
	tr := newTracker(t)
	delete(tr.env, "CAIRN_ACTOR")
	tr.wantError(2, "usage", "new", "a")
	// The id comes from the reply: a failed new may or may not have used up random bytes.
	i := tr.issue("new", "a", "--as", "bob", "--persona", "skeptic", "--model", "example-model-1")
	tr.wantError(2, "usage", "edit", i.ID, "--title", "b")
	tr.wantError(2, "usage", "rm", i.ID)
	tr.env["CAIRN_ACTOR"] = "carol"
	if code, _, _ := tr.json("edit", i.ID, "--title", "b"); code != 0 {
		t.Fatalf("CAIRN_ACTOR: exit %d", code)
	}
}

func TestFlagForms(t *testing.T) {
	tr := newTracker(t)
	if i := tr.issue("new", "--tag", "area:x", "--due=2026-11-01", "Title first or last"); i.Title != "Title first or last" ||
		i.Due != "2026-11-01" || !reflect.DeepEqual(i.Tags, []string{"area:x"}) {
		t.Fatalf("got %+v", i)
	}
	code, out, _ := tr.run("new", "--json", "--", "--looks-like-a-flag")
	var e struct {
		Data issue `json:"data"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &e) != nil || e.Data.Title != "--looks-like-a-flag" {
		t.Fatalf("after --: exit %d, %q", code, out)
	}
}

func TestEdit(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "Old", "--body", "keep me", "--tag", "severity:high", "--tag", "area:export", "--due", "2026-11-01",
		"--milestone", "v1")
	tr.now = t0.Add(time.Hour)
	got := tr.issue("edit", "0123", "--title", "New", "--status", "in-progress", "--tag", "-severity:high",
		"--tag", "+effort:2", "--tag", "area:export", "--due", "")
	want := issue{ID: "0123", Title: "New", Status: "in-progress", Tags: []string{"area:export", "effort:2"},
		Milestone: "v1", Created: "2026-10-18T09:30:00.123Z", Updated: "2026-10-18T10:30:00.123Z", Body: "keep me\n"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if got := tr.issue("edit", "0123", "--milestone", ""); got.Milestone != "" || got.Title != "New" {
		t.Fatalf("clear milestone: %+v", got)
	}
}

func TestEditErrors(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.wantError(2, "usage", "edit", "0123")
	tr.wantError(2, "usage", "edit")
	tr.wantError(2, "usage", "edit", "0123", "--status", "nope")
	tr.wantError(2, "usage", "edit", "0123", "--tag", "+colour:blue")
	tr.wantError(2, "usage", "edit", "0123", "--due", "soon")
	tr.wantError(4, "not_found", "edit", "zzzz", "--title", "x")
	tr.wantError(4, "not_found", "edit", "../x", "--title", "x")
	tr.wantError(4, "not_found", "show", "zzzz")
}

func TestConfigStatuses(t *testing.T) {
	tr := newTracker(t)
	cfg := filepath.Join(tr.dir, ".cairn", "config.yml")
	if err := os.WriteFile(cfg, []byte("---\nstatuses: [todo, doing, done]\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if i := tr.issue("new", "a"); i.Status != "todo" {
		t.Fatalf("first status: %q", i.Status)
	}
	tr.wantError(2, "usage", "edit", "0123", "--status", "in-progress")
	if i := tr.issue("edit", "0123", "--status", "doing"); i.Status != "doing" {
		t.Fatalf("got %q", i.Status)
	}
}

func TestRm(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	s, err := store.Find(tr.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveComment("0123", model.Comment{ID: "c-0001", Kind: "evidence", By: "alice", At: t0}); err != nil {
		t.Fatal(err)
	}
	code, data, _ := tr.json("rm", "0123")
	if code != 0 || compact(t, data) != `{"id":"0123"}` {
		t.Fatalf("exit %d, data %s", code, data)
	}
	if _, err := os.Stat(filepath.Join(tr.dir, ".cairn", "issues", "0123")); !os.IsNotExist(err) {
		t.Fatalf("folder still there: %v", err)
	}
	tr.wantError(4, "not_found", "show", "0123")
	tr.wantError(4, "not_found", "rm", "0123")
	tr.wantError(2, "usage", "rm")
}

func TestLs(t *testing.T) {
	tr := newTracker(t)
	if _, data, _ := tr.json("ls"); compact(t, data) != "[]" {
		t.Fatalf("empty: %s", data)
	}
	tr.issue("new", "one", "--tag", "severity:high", "--tag", "area:export", "--due", "2026-10-31", "--body", "b")
	tr.now = tr.now.Add(time.Second)
	tr.issue("new", "two", "--tag", "severity:high", "--milestone", "v1", "--due", "2026-11-01")
	tr.now = tr.now.Add(time.Second)
	tr.issue("new", "three", "--milestone", "v1")
	tr.issue("edit", "4567", "--status", "in-progress")
	ids := func(args ...string) string {
		t.Helper()
		code, data, _ := tr.json(append([]string{"ls"}, args...)...)
		var items []map[string]any
		if code != 0 || json.Unmarshal(data, &items) != nil {
			t.Fatalf("ls %q: exit %d, %s", args, code, data)
		}
		var got []string
		for _, it := range items {
			if _, ok := it["body"]; ok {
				t.Errorf("ls items must not have a body: %v", it)
			}
			got = append(got, it["id"].(string))
		}
		return strings.Join(got, " ")
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "0123 4567 89ab"},
		{[]string{"--status", "in-progress"}, "4567"},
		{[]string{"--tag", "severity:high"}, "0123 4567"},
		{[]string{"--tag", "severity:high", "--tag", "area:export"}, "0123"},
		{[]string{"--milestone", "v1"}, "4567 89ab"},
		{[]string{"--due-before", "2026-11-01"}, "0123"},
		{[]string{"--milestone", "v1", "--status", "backlog"}, "89ab"},
		{[]string{"--status", "done"}, ""},
	} {
		if got := ids(c.args...); got != c.want {
			t.Errorf("ls %q = %q, want %q", c.args, got, c.want)
		}
	}
	tr.wantError(2, "usage", "ls", "--due-before", "soon")
	tr.wantError(2, "usage", "ls", "extra")
}

func TestVersionHelpAndUnknown(t *testing.T) {
	tr := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}}
	if code, data, _ := tr.json("version"); code != 0 || compact(t, data) != `{"version":"1.2.3"}` {
		t.Fatalf("version: exit %d, %s", code, data)
	}
	if code, out, _ := tr.run("version"); code != 0 || out != "cairn 1.2.3\n" {
		t.Fatalf("version text: exit %d, %q", code, out)
	}
	if code, out, _ := tr.run("help"); code != 0 || !strings.Contains(out, "Usage") {
		t.Fatalf("help: exit %d, %q", code, out)
	}
	tr.wantError(2, "usage", "nope")
	if code, out, errOut := tr.run("nope"); code != 2 || out != "" || errOut == "" {
		t.Fatalf("unknown text: exit %d, %q, %q", code, out, errOut)
	}
}

// The real binary passes no clock, randomness or environment: the defaults must work.
func TestNilEnvDefaults(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	env := Env{Version: "1.2.3", Dir: dir, Stdout: &out, Stderr: &errOut}
	if code := Run([]string{"init"}, env); code != 0 {
		t.Fatalf("init: exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), ".cairn") {
		t.Errorf("init text should name the folder: %q", out.String())
	}
	out.Reset()
	before := time.Now().UTC().Add(-time.Second)
	if code := Run([]string{"new", "a", "--as", "bob", "--json"}, env); code != 0 {
		t.Fatalf("new: exit %d: %s", code, errOut.String())
	}
	var e struct {
		Data issue `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	created, err := time.Parse(time.RFC3339, e.Data.Created)
	if err != nil || created.Before(before) || created.After(time.Now().Add(time.Second)) || len(e.Data.ID) != 4 {
		t.Fatalf("got %+v (%v)", e.Data, err)
	}
	out.Reset()
	if code := Run([]string{"new", "b"}, env); code != 2 {
		t.Fatalf("no Getenv means no CAIRN_ACTOR: exit %d", code)
	}
}

func TestNoArgsAndAliases(t *testing.T) {
	tr := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}}
	if code, out, errOut := tr.run(); code != 2 || out != "" || errOut == "" {
		t.Errorf("no args: exit %d, %q, %q", code, out, errOut)
	}
	for _, a := range []string{"--help", "-h"} {
		if code, out, _ := tr.run(a); code != 0 || !strings.Contains(out, "Usage") {
			t.Errorf("%s: exit %d, %q", a, code, out)
		}
	}
	if code, out, _ := tr.run("--version"); code != 0 || out != "cairn 1.2.3\n" {
		t.Errorf("--version: exit %d, %q", code, out)
	}
}

// Flags from later steps are usage errors until those steps build them, never silently ignored.
func TestLaterFlagsRejected(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", "a")
	tr.wantError(2, "usage", "show", "0123", "--branches")
	tr.wantError(2, "usage", "ls", "--branches")
}

// A tag can always be removed, even when its group has left the config.
func TestRemoveStaleTag(t *testing.T) {
	tr := newTracker(t)
	s, err := store.Find(tr.dir)
	if err != nil {
		t.Fatal(err)
	}
	stale := model.Issue{ID: "7f3k", Title: "a", Status: "open", Tags: []string{"colour:blue", "area:x"},
		Created: t0, Updated: t0}
	if err := s.SaveIssue(stale); err != nil {
		t.Fatal(err)
	}
	if i := tr.issue("edit", "7f3k", "--tag", "-colour:blue"); !reflect.DeepEqual(i.Tags, []string{"area:x"}) {
		t.Fatalf("got %v", i.Tags)
	}
	tr.wantError(2, "usage", "edit", "7f3k", "--tag", "-")
}

func TestNewDropsDuplicateTags(t *testing.T) {
	tr := newTracker(t)
	if i := tr.issue("new", "a", "--tag", "area:x", "--tag", "area:x"); !reflect.DeepEqual(i.Tags, []string{"area:x"}) {
		t.Fatalf("got %v", i.Tags)
	}
}

func TestNotFoundNamesTheID(t *testing.T) {
	tr := newTracker(t)
	if code, _, errOut := tr.run("show", "zzzz"); code != 4 || !strings.Contains(errOut, "zzzz") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestConfigWithoutStatuses(t *testing.T) {
	tr := newTracker(t)
	if err := os.WriteFile(filepath.Join(tr.dir, ".cairn", "config.yml"), []byte("---\nstatuses: []\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr.wantError(1, "error", "new", "a")
}
