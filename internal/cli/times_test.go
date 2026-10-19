package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every time Cairn writes, in files, text and JSON, has three fraction digits, so times compare the same as text as
// they do as times.
func TestTimesHaveOneWidth(t *testing.T) {
	tr := newTracker(t)
	tr.now = time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)
	i := tr.issue("new", "a")
	tr.now = time.Date(2026, 10, 18, 9, 30, 0, 700_000_000, time.UTC)
	c := tr.comment("comment", i.ID, "--kind", "result", "done").Comment
	tr.now = tr.now.Add(10 * time.Millisecond)
	r := tr.retract(i.ID, c.ID, "--reason", "wrong")

	if i.Created != "2026-10-18T09:30:00.000Z" || i.Updated != "2026-10-18T09:30:00.000Z" {
		t.Errorf("issue times: %q, %q", i.Created, i.Updated)
	}
	if c.At != "2026-10-18T09:30:00.700Z" {
		t.Errorf("comment at: %q", c.At)
	}
	if r.Retracted == nil || r.Retracted.At != "2026-10-18T09:30:00.710Z" {
		t.Errorf("retracted: %+v", r.Retracted)
	}
	folder := filepath.Join(tr.dir, ".cairn", "issues", i.ID)
	issueFile, err := os.ReadFile(filepath.Join(folder, "issue.md"))
	if err != nil {
		t.Fatal(err)
	}
	commentFile, err := os.ReadFile(filepath.Join(folder, c.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	_, show, _ := tr.run("show", i.ID, "--retracted")
	for name, text := range map[string]string{"issue.md": string(issueFile), "show": show} {
		for _, line := range []string{"created: 2026-10-18T09:30:00.000Z\n", "updated: 2026-10-18T09:30:00.000Z\n"} {
			if !strings.Contains(text, line) {
				t.Errorf("%s: no %q in\n%s", name, line, text)
			}
		}
	}
	for name, text := range map[string]string{c.ID + ".md": string(commentFile), "show": show} {
		for _, line := range []string{"at: 2026-10-18T09:30:00.700Z\n", "retracted_at: 2026-10-18T09:30:00.710Z\n"} {
			if !strings.Contains(text, line) {
				t.Errorf("%s: no %q in\n%s", name, line, text)
			}
		}
	}
}
