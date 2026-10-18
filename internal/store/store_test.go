package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/seriouslysean/cairn/internal/doc"
	"github.com/seriouslysean/cairn/internal/id"
	"github.com/seriouslysean/cairn/internal/model"
)

var t0 = time.Date(2026, 10, 17, 19, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func issue(i string, created time.Time) model.Issue {
	return model.Issue{ID: i, Title: "title " + i, Status: "open", Created: created, Updated: created, Body: "body\n"}
}

func comment(c string, at time.Time) model.Comment {
	return model.Comment{ID: c, Kind: "evidence", By: "alice", At: at, Body: "note " + c + "\n"}
}

func save(t *testing.T, s *Store, i model.Issue, cs ...model.Comment) {
	t.Helper()
	if err := s.SaveIssue(i); err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if err := s.SaveComment(i.ID, c); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Dir != filepath.Join(dir, ".cairn") {
		t.Fatalf("Dir %q", s.Dir)
	}
	cfg, err := doc.Format(model.DefaultConfig().Fields(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"config.yml": string(cfg), ".gitignore": ".index/\n"} {
		if got, err := os.ReadFile(filepath.Join(s.Dir, name)); err != nil || string(got) != want {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
	for _, name := range []string{"issues", "milestones"} {
		if fi, err := os.Stat(filepath.Join(s.Dir, name)); err != nil || !fi.IsDir() {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := Init(dir); !errors.Is(err, ErrExists) {
		t.Fatalf("second Init: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(s.Dir, "config.yml")); string(got) != string(cfg) {
		t.Fatal("second Init changed config.yml")
	}
}

func TestFind(t *testing.T) {
	s := newStore(t)
	deep := filepath.Join(filepath.Dir(s.Dir), "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{filepath.Dir(s.Dir), deep, s.Dir} {
		got, err := Find(start)
		if err != nil || got.Dir != s.Dir {
			t.Errorf("Find(%q) = %v, %v", start, got, err)
		}
	}
}

func TestFindNone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cairn"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(dir); !errors.Is(err, ErrNoTracker) {
		t.Fatalf("got %v", err)
	}
}

func TestConfig(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(filepath.Join(s.Dir, "config.yml"), []byte("---\nstatuses: [todo, done]\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := s.Config()
	if err != nil || !reflect.DeepEqual(c.Statuses, []string{"todo", "done"}) || c.DupeText != 0.6 {
		t.Fatalf("got %+v, %v", c, err)
	}
	if err := os.Remove(filepath.Join(s.Dir, "config.yml")); err != nil {
		t.Fatal(err)
	}
	if c, err := s.Config(); err != nil || !reflect.DeepEqual(c, model.DefaultConfig()) {
		t.Fatalf("missing file: %+v, %v", c, err)
	}
}

func TestSaveAndLoadIssue(t *testing.T) {
	s := newStore(t)
	i := issue("7f3k", t0)
	// c-0002 is older; c-0001 and c-0003 share a time, so the id breaks the tie.
	c1, c2, c3 := comment("c-0001", t0.Add(time.Minute)), comment("c-0002", t0), comment("c-0003", t0.Add(time.Minute))
	save(t, s, i, c3, c1, c2)
	folder := filepath.Join(s.Dir, "issues", "7f3k")
	for _, junk := range []string{".cairn-tmp-123", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(folder, junk), []byte("junk"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, cs, err := s.LoadIssue("7f3k")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, i) {
		t.Fatalf("issue\ngot  %+v\nwant %+v", got, i)
	}
	if want := []model.Comment{c2, c1, c3}; !reflect.DeepEqual(cs, want) {
		t.Fatalf("comments\ngot  %+v\nwant %+v", cs, want)
	}
	i.Title = "renamed"
	if err := s.SaveIssue(i); err != nil {
		t.Fatal(err)
	}
	if _, cs, err := s.LoadIssue("7f3k"); err != nil || len(cs) != 3 {
		t.Fatalf("SaveIssue must leave comments alone: %d, %v", len(cs), err)
	}
}

func TestLoadIssueNotFound(t *testing.T) {
	s := newStore(t)
	if _, _, err := s.LoadIssue("zzzz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("LoadIssue: %v", err)
	}
	if err := os.Mkdir(filepath.Join(s.Dir, "issues", "zzzz"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.LoadIssue("zzzz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("LoadIssue without issue.md: %v", err)
	}
	if err := s.SaveComment("yyyy", comment("c-0001", t0)); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveComment: %v", err)
	}
	if _, err := s.NewCommentID("yyyy", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("NewCommentID: %v", err)
	}
}

func TestLoadIssueIDMismatch(t *testing.T) {
	s := newStore(t)
	save(t, s, issue("aaaa", t0))
	issues := filepath.Join(s.Dir, "issues")
	if err := os.Rename(filepath.Join(issues, "aaaa"), filepath.Join(issues, "bbbb")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.LoadIssue("bbbb"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("issue id mismatch: %v", err)
	}

	save(t, s, issue("cccc", t0), comment("c-1111", t0))
	folder := filepath.Join(issues, "cccc")
	if err := os.Rename(filepath.Join(folder, "c-1111.md"), filepath.Join(folder, "c-2222.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.LoadIssue("cccc"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("comment id mismatch: %v", err)
	}
}

func TestListIssues(t *testing.T) {
	s := newStore(t)
	a, b, c := issue("aaaa", t0.Add(time.Hour)), issue("bbbb", t0), issue("cccc", t0.Add(time.Hour))
	save(t, s, c, comment("c-0001", t0))
	save(t, s, a)
	save(t, s, b)
	issues := filepath.Join(s.Dir, "issues")
	if err := os.Mkdir(filepath.Join(issues, "Not-An-Id"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issues, "README"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListIssues()
	if err != nil {
		t.Fatal(err)
	}
	if want := []model.Issue{b, a, c}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	empty := &Store{Dir: t.TempDir()}
	if got, err := empty.ListIssues(); err != nil || len(got) != 0 {
		t.Fatalf("no issues folder: %v, %v", got, err)
	}
}

func TestNewIssueID(t *testing.T) {
	s := newStore(t)
	got, err := s.NewIssueID(bytes.NewReader([]byte{0, 0, 0, 0}))
	if err != nil || got != "0000" {
		t.Fatalf("empty tracker: %q, %v", got, err)
	}
	save(t, s, issue("0000", t0))
	got, err = s.NewIssueID(bytes.NewReader([]byte{0, 0, 0, 0, 1, 1, 1, 1}))
	if err != nil || got != "1111" {
		t.Fatalf("taken id: %q, %v", got, err)
	}
	// 1,000 issue folders make ids 5 characters long. Folders are enough: NewIssueID only counts them.
	issues := filepath.Join(s.Dir, "issues")
	for n := 1; n < 1000; n++ {
		name := string([]byte{'1', id.Alphabet[n/32/32%32], id.Alphabet[n/32%32], id.Alphabet[n%32]})
		if err := os.Mkdir(filepath.Join(issues, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.NewIssueID(bytes.NewReader([]byte{2, 2, 2, 2, 2}))
	if err != nil || got != "22222" {
		t.Fatalf("1,000 issues: %q, %v", got, err)
	}
}

func TestNewCommentID(t *testing.T) {
	s := newStore(t)
	save(t, s, issue("7f3k", t0), comment("c-0000", t0))
	got, err := s.NewCommentID("7f3k", bytes.NewReader([]byte{0, 0, 0, 0, 1, 1, 1, 1}))
	if err != nil || got != "c-1111" {
		t.Fatalf("got %q, %v", got, err)
	}
}

// A broken issue file must surface, not vanish from the list. A folder without issue.md is skipped.
func TestListIssuesErrors(t *testing.T) {
	s := newStore(t)
	save(t, s, issue("aaaa", t0))
	if err := os.Mkdir(filepath.Join(s.Dir, "issues", "bbbb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ListIssues(); err != nil || len(got) != 1 {
		t.Fatalf("folder without issue.md: %v, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "issues", "bbbb", "issue.md"), []byte("not frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListIssues(); err == nil {
		t.Fatal("broken issue.md: want an error")
	}
	if err := os.Rename(filepath.Join(s.Dir, "issues", "aaaa"), filepath.Join(s.Dir, "issues", "cccc")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(s.Dir, "issues", "bbbb")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListIssues(); err == nil {
		t.Fatal("id that does not match its folder: want an error")
	}
}

// Ids become paths, so an id that is not valid must never reach the file system.
func TestBadIDsNeverBecomePaths(t *testing.T) {
	s := newStore(t)
	save(t, s, issue("7f3k", t0))
	for _, bad := range []string{"../7f3k", "7f3k/..", "", "."} {
		if _, _, err := s.LoadIssue(bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("LoadIssue(%q): %v", bad, err)
		}
		if _, err := s.NewCommentID(bad, nil); !errors.Is(err, ErrNotFound) {
			t.Errorf("NewCommentID(%q): %v", bad, err)
		}
		if err := s.SaveComment(bad, comment("c-0001", t0)); err == nil {
			t.Errorf("SaveComment(%q): want an error", bad)
		}
	}
	if err := s.SaveIssue(issue("../x", t0)); err == nil {
		t.Error("SaveIssue with a bad id: want an error")
	}
	if err := s.SaveComment("7f3k", comment("c-../../x", t0)); err == nil {
		t.Error("SaveComment with a bad comment id: want an error")
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "x")); !os.IsNotExist(err) {
		t.Error("a bad id wrote outside issues/")
	}
}

func TestRemoveIssue(t *testing.T) {
	s := newStore(t)
	save(t, s, issue("7f3k", t0), comment("c-0001", t0))
	if err := s.RemoveIssue("7f3k"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "issues", "7f3k")); !os.IsNotExist(err) {
		t.Fatalf("folder still there: %v", err)
	}
	for _, bad := range []string{"7f3k", "../7f3k", "", "."} {
		if err := s.RemoveIssue(bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("RemoveIssue(%q): %v", bad, err)
		}
	}
	if _, err := os.Stat(s.Dir); err != nil {
		t.Fatalf("RemoveIssue removed too much: %v", err)
	}
}
