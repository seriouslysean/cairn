package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// boardTracker has four issues: two in backlog (the second with a tag, updated last), one open and one done.
// It returns the tracker and the ids in the order made.
func boardTracker(t *testing.T) (*tracker, []string) {
	t.Helper()
	tr := newTracker(t)
	ids := tr.many(4, "issue")
	tr.now = t0.Add(time.Hour)
	tr.issue("edit", ids[1], "--status", "open")
	tr.now = t0.Add(2 * time.Hour)
	tr.issue("edit", ids[2], "--status", "done")
	tr.now = t0.Add(3 * time.Hour)
	tr.issue("edit", ids[3], "--tag", "area:export")
	return tr, ids
}

// board prints a section per status, in the config's order, newest update first, as the board package describes. The
// done column is collapsed, so it has its count and no cards.
func TestBoardText(t *testing.T) {
	tr, ids := boardTracker(t)
	want := "backlog (2)\n" +
		"\t" + ids[3] + "\t2026-10-18T12:30:00.123Z\tarea:export\tissue 4\n" +
		"\t" + ids[0] + "\t2026-10-18T09:30:01.123Z\t\tissue 1\n" +
		"\n" +
		"open (1)\n" +
		"\t" + ids[1] + "\t2026-10-18T10:30:00.123Z\t\tissue 2\n" +
		"\n" +
		"in-progress (0)\n" +
		"\n" +
		"done (1, collapsed)\n"
	if code, out, errOut := tr.run("board"); code != 0 || out != want || errOut != "" {
		t.Fatalf("exit %d, stderr %q\ngot\n%q\nwant\n%q", code, errOut, out, want)
	}
}

// With --json, board's data is {"columns": [...]}: each column has its status, whether it is collapsed, how many
// issues it holds, and "issues", the same rows as the text, as ls gives them. A collapsed column's list is empty.
func TestBoardJSON(t *testing.T) {
	tr, ids := boardTracker(t)
	env, _ := tr.envelope("board")
	if _, ok := env["total"]; ok {
		t.Error("board's envelope has total")
	}
	data := env["data"].(map[string]any)
	if len(data) != 1 {
		t.Errorf("data has keys %v, want only columns", data)
	}
	type col struct {
		status    string
		collapsed bool
		count     float64
		ids       []string
	}
	var got []col
	for _, c := range data["columns"].([]any) {
		m := c.(map[string]any)
		cc := col{m["status"].(string), m["collapsed"].(bool), m["count"].(float64), []string{}}
		for _, i := range m["issues"].([]any) {
			row := i.(map[string]any)
			cc.ids = append(cc.ids, row["id"].(string))
			for _, k := range []string{"title", "tags", "status", "updated", "created"} {
				if _, ok := row[k]; !ok {
					t.Errorf("row lacks %q", k)
				}
			}
			if _, ok := row["body"]; ok {
				t.Error("row has a body")
			}
		}
		got = append(got, cc)
	}
	want := []col{
		{"backlog", false, 2, []string{ids[3], ids[0]}},
		{"open", false, 1, []string{ids[1]}},
		{"in-progress", false, 0, []string{}},
		{"done", true, 1, []string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

// A status the config no longer lists keeps its issues on the board, in a column after the config's.
func TestBoardStatusLeftOutOfConfig(t *testing.T) {
	tr, _ := boardTracker(t)
	tr.writeConfig("---\nstatuses: [backlog, open]\n---\n")
	_, out, _ := tr.run("board")
	if want := "open (1)\n"; !strings.Contains(out, want) || !strings.Contains(out, "\n\ndone (1, collapsed)\n") ||
		strings.Contains(out, "in-progress") {
		t.Fatalf("got %q", out)
	}
}

// board only reads: it asks for no actor and, with commit on write, commits nothing and leaves nothing for git to see
// in the tracker.
func TestBoardReadsOnly(t *testing.T) {
	tr := commitTracker(t, "")
	tr.write("new", "first")
	tr.write("new", "second")
	delete(tr.env, "CAIRN_ACTOR")
	head, files := tr.headCommit(), snapshot(t, filepath.Join(tr.dir, ".cairn"))
	for _, args := range [][]string{{"board"}, {"board", "--json"}, {"board", "--html", "board.html"}} {
		if code, _, errOut := tr.run(args...); code != 0 || errOut != "" {
			t.Fatalf("%q: exit %d: %s", args, code, errOut)
		}
	}
	if got := tr.headCommit(); got != head {
		t.Errorf("HEAD moved from %s to %s", head, got)
	}
	if got := snapshot(t, filepath.Join(tr.dir, ".cairn")); !reflect.DeepEqual(got, files) {
		t.Errorf("the tracker's files changed\nbefore %v\nafter  %v", files, got)
	}
	if got, want := tr.status(), []string{"?? board.html"}; !reflect.DeepEqual(got, want) {
		t.Errorf("git status %q, want only the page, %q", got, want)
	}
}

// snapshot is every file under dir with its content.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		files[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// board --html writes the page to the file, replacing it: a relative name is relative to the folder Cairn runs in,
// which -C changes. The page holds every issue, the collapsed ones too, and the write leaves no temporary file.
func TestBoardHTMLFile(t *testing.T) {
	tr, ids := boardTracker(t)
	want := filepath.Join(tr.dir, "board.html")
	if code, out, errOut := tr.run("board", "--html", "board.html"); code != 0 || out != "wrote "+want+"\n" || errOut != "" {
		t.Fatalf("exit %d, %q, %q", code, out, errOut)
	}
	page, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"<!doctype html>", "issue 1", "issue 2", "issue 3", "issue 4", ids[2], "area:export",
		"2026-10-18 09:30 UTC"} {
		if !strings.Contains(string(page), s) {
			t.Errorf("page lacks %q", s)
		}
	}

	// Run again: the page is replaced, and an absolute name works.
	tr.issue("new", "A fifth issue")
	if code, _, errOut := tr.run("board", "--html="+want); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if page, _ = os.ReadFile(want); !strings.Contains(string(page), "A fifth issue") {
		t.Error("the page was not replaced")
	}
	entries, _ := os.ReadDir(tr.dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".cairn-tmp-") {
			t.Errorf("left %s behind", e.Name())
		}
	}

	// -C runs as if started in the folder, so a relative name lands there.
	other := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}}
	if code, _, errOut := other.run("-C", tr.dir, "board", "--html", "second.html"); code != 0 {
		t.Fatalf("-C: exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(tr.dir, "second.html")); err != nil {
		t.Error(err)
	}
}

// With --json, board --html says which file it wrote and for how many issues.
func TestBoardHTMLJSON(t *testing.T) {
	tr, _ := boardTracker(t)
	env, _ := tr.envelope("board", "--html", "board.html")
	want := map[string]any{"file": filepath.Join(tr.dir, "board.html"), "issues": float64(4)}
	if !reflect.DeepEqual(env["data"], want) {
		t.Fatalf("got %v, want %v", env["data"], want)
	}
}

// Titles are escaped in the page the command writes.
func TestBoardHTMLEscapes(t *testing.T) {
	tr := newTracker(t)
	tr.issue("new", `<script>alert(1)</script> & "q"`)
	if code, _, errOut := tr.run("board", "--html", "board.html"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	page, _ := os.ReadFile(filepath.Join(tr.dir, "board.html"))
	if strings.Contains(string(page), "<script") || !strings.Contains(string(page), "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("the title is not escaped")
	}
}

func TestBoardUsageErrors(t *testing.T) {
	tr, _ := boardTracker(t)
	for _, args := range [][]string{
		{"board", "extra"},
		{"board", "somewhere/else", "--html", "board.html"},
		{"board", "--html"},
		{"board", "--html="},
		{"board", "--html", ""},
		{"board", "--limit", "3"},
		{"board", "--all"},
		{"ls", "--html", "board.html"},
		{"search", "issue", "--html", "board.html"},
	} {
		tr.wantError(2, "usage", args...)
	}
	if _, err := os.Stat(filepath.Join(tr.dir, "board.html")); err == nil {
		t.Error("a usage error wrote a file")
	}
}

// A file that cannot be written is an error, and nothing is left behind: not for a folder that is missing, and not for
// a name that is a folder, which the message says.
func TestBoardHTMLCannotWrite(t *testing.T) {
	tr, _ := boardTracker(t)
	tr.wantError(1, "error", "board", "--html", filepath.Join("no", "such", "folder", "board.html"))
	if entries, _ := os.ReadDir(tr.dir); len(entries) != 1 {
		t.Errorf("folder holds %d entries, want only .cairn", len(entries))
	}

	if err := os.Mkdir(filepath.Join(tr.dir, "page"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"page", "page" + string(filepath.Separator)} {
		code, out, errOut := tr.run("board", "--html", name)
		if code != 1 || out != "" || !strings.Contains(errOut, "page"+strings.TrimPrefix(name, "page")+" is a folder") {
			t.Errorf("--html %q: exit %d, stdout %q, stderr %q", name, code, out, errOut)
		}
	}
	if entries, _ := os.ReadDir(tr.dir); len(entries) != 2 {
		t.Errorf("folder holds %d entries, want .cairn and page", len(entries))
	}
}

// A tab in a title shifts no field before it: the title is last, as in ls.
func TestBoardTextTabInTitle(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "one\ttwo", "--tag", "area:export").ID
	_, out, _ := tr.run("board")
	if want := "\t" + id + "\t2026-10-18T09:30:00.123Z\tarea:export\tone\ttwo\n"; !strings.Contains(out, want) {
		t.Fatalf("got %q, want it to contain %q", out, want)
	}
}

func TestBoardNoTracker(t *testing.T) {
	tr := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}}
	tr.wantError(1, "no_tracker", "board")
	tr.wantError(1, "no_tracker", "board", "--html", "board.html")
	if _, err := os.Stat(filepath.Join(tr.dir, "board.html")); err == nil {
		t.Error("wrote a file with no tracker")
	}
}

// The global tracker serves board as it serves every other command.
func TestBoardGlobalTracker(t *testing.T) {
	g := newGlobalSetup(t)
	if code, out, _ := g.work.run("board"); code != 0 || !strings.Contains(out, "Filed in the global tracker") {
		t.Fatalf("exit %d, %q", code, out)
	}
}
