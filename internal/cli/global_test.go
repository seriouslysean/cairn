package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// globalSetup is a global tracker with one issue, a home folder whose user config file names it, and a working
// folder with no tracker of its own, from which commands run.
type globalSetup struct {
	global *tracker // the global tracker's folder, made with init
	home   string
	work   *tracker // runs in a folder with no .cairn, with HOME set to home
}

func newGlobalSetup(t *testing.T) *globalSetup {
	t.Helper()
	g := &globalSetup{global: newTracker(t), home: t.TempDir()}
	if code, _, errOut := g.global.run("new", "Filed in the global tracker"); code != 0 {
		t.Fatalf("new: exit %d: %s", code, errOut)
	}
	writeUserConfig(t, filepath.Join(g.home, ".config"), "---\ntracker: "+g.global.dir+"\n---\n")
	g.work = &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{},
		env: map[string]string{"CAIRN_ACTOR": "test-agent", "HOME": g.home}}
	return g
}

// writeUserConfig writes cairn/config.yml in configDir.
func writeUserConfig(t *testing.T, configDir, content string) string {
	t.Helper()
	p := filepath.Join(configDir, "cairn", "config.yml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Outside any .cairn, every command reads and writes the tracker that the user config file names.
func TestGlobalTrackerOutsideAnyCairn(t *testing.T) {
	g := newGlobalSetup(t)
	got := g.work.issue("new", "Filed from another folder")
	if _, err := os.Stat(filepath.Join(g.global.dir, ".cairn", "issues", got.ID, "issue.md")); err != nil {
		t.Fatalf("new did not write to the global tracker: %v", err)
	}
	if code, out, errOut := g.work.run("ls"); code != 0 || strings.Count(out, "\n") != 2 {
		t.Fatalf("ls: exit %d, %q, %q", code, out, errOut)
	}
	if code, out, _ := g.work.run("search", "global"); code != 0 || !strings.HasPrefix(out, "0123\t") {
		t.Fatalf("search: exit %d, %q", code, out)
	}
	if code, _, errOut := g.work.run("comment", got.ID, "--kind", "result", "done"); code != 0 {
		t.Fatalf("comment: exit %d: %s", code, errOut)
	}
	if code, _, errOut := g.work.run("milestone", "new", "v1", "One"); code != 0 {
		t.Fatalf("milestone new: exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(g.global.dir, ".cairn", "milestones", "v1.md")); err != nil {
		t.Fatalf("milestone new did not write to the global tracker: %v", err)
	}
	if code, out, _ := g.work.run("tags"); code != 0 || out != "" {
		t.Fatalf("tags: exit %d, %q", code, out)
	}
}

// A repository's own tracker wins: a folder with a .cairn, or under one, keeps using it.
func TestOwnTrackerWinsOverGlobal(t *testing.T) {
	g := newGlobalSetup(t)
	local := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}, env: g.work.env}
	if code, _, errOut := local.run("init"); code != 0 {
		t.Fatalf("init: exit %d: %s", code, errOut)
	}
	sub := filepath.Join(local.dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{local.dir, sub} {
		local.dir = dir
		if code, out, errOut := local.run("ls"); code != 0 || out != "" {
			t.Fatalf("ls in %s: exit %d, %q, %q", dir, code, out, errOut)
		}
	}
}

// The global tracker is read only when no .cairn is found, so a broken user config file never breaks a repository's
// own tracker.
func TestBrokenUserConfigLeavesOwnTracker(t *testing.T) {
	home := t.TempDir()
	writeUserConfig(t, filepath.Join(home, ".config"), "not frontmatter\n")
	tr := newTracker(t)
	tr.env["HOME"] = home
	if code, _, errOut := tr.run("ls"); code != 0 {
		t.Fatalf("ls: exit %d: %s", code, errOut)
	}
}

// init makes .cairn where it runs, even when a global tracker is set.
func TestInitIgnoresGlobal(t *testing.T) {
	g := newGlobalSetup(t)
	if code, _, errOut := g.work.run("init"); code != 0 {
		t.Fatalf("init: exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(g.work.dir, ".cairn", "config.yml")); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := g.work.run("ls"); code != 0 || out != "" {
		t.Fatalf("ls after init: exit %d, %q", code, out)
	}
}

// With no user config file, or one with no tracker key, lookup is as it was: no_tracker outside any .cairn.
func TestNoGlobalTracker(t *testing.T) {
	home := t.TempDir()
	tr := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{},
		env: map[string]string{"CAIRN_ACTOR": "test-agent", "HOME": home}}
	tr.wantError(1, "no_tracker", "ls")
	writeUserConfig(t, filepath.Join(home, ".config"), "---\nsomething_else: x\n---\n")
	tr.wantError(1, "no_tracker", "ls")
	tr.wantError(1, "no_tracker", "new", "title")
	code, _, errOut := tr.run("ls")
	if code != 1 || errOut != "cairn: no .cairn folder here or above\n" {
		t.Fatalf("exit %d, %q", code, errOut)
	}
}

// $XDG_CONFIG_HOME, when it is an absolute path, is the config folder instead of $HOME/.config. A relative one is
// ignored, as the XDG Base Directory spec says.
func TestXDGConfigHome(t *testing.T) {
	g := newGlobalSetup(t)
	xdg := t.TempDir()
	g.work.env["XDG_CONFIG_HOME"] = xdg
	g.work.wantError(1, "no_tracker", "ls")
	writeUserConfig(t, xdg, "---\ntracker: "+g.global.dir+"\n---\n")
	if code, out, errOut := g.work.run("ls"); code != 0 || !strings.HasPrefix(out, "0123\t") {
		t.Fatalf("ls: exit %d, %q, %q", code, out, errOut)
	}
	g.work.env["XDG_CONFIG_HOME"] = "relative/config"
	if code, out, errOut := g.work.run("ls"); code != 0 || !strings.HasPrefix(out, "0123\t") {
		t.Fatalf("ls with a relative XDG_CONFIG_HOME: exit %d, %q, %q", code, out, errOut)
	}
}

// A user config file that is set but wrong is an error that names the file.
func TestBadUserConfig(t *testing.T) {
	noCairn := t.TempDir()
	notDir := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(notDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notDir, ".cairn"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"not frontmatter":   "tracker: /x\n",
		"relative tracker":  "---\ntracker: notes\n---\n",
		"empty tracker":     "---\ntracker: \"\"\n---\n",
		"no .cairn there":   "---\ntracker: " + noCairn + "\n---\n",
		".cairn is a file":  "---\ntracker: " + notDir + "\n---\n",
		"tracker is a list": "---\ntracker: [" + noCairn + "]\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			p := writeUserConfig(t, filepath.Join(home, ".config"), content)
			tr := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{},
				env: map[string]string{"CAIRN_ACTOR": "test-agent", "HOME": home}}
			tr.wantError(1, "error", "ls")
			if code, _, errOut := tr.run("ls"); code != 1 || !strings.Contains(errOut, p) {
				t.Fatalf("exit %d, %q does not name %s", code, errOut, p)
			}
		})
	}
	t.Run("unreadable", func(t *testing.T) {
		home := t.TempDir()
		p := filepath.Join(home, ".config", "cairn", "config.yml")
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		tr := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}, env: map[string]string{"HOME": home}}
		if code, _, errOut := tr.run("ls"); code != 1 || !strings.Contains(errOut, p) {
			t.Fatalf("exit %d, %q does not name %s", code, errOut, p)
		}
	})
}

// -C <dir> before the command runs as if Cairn started in dir: a relative dir is read from the current folder, the
// walk up starts there, and init makes .cairn there.
func TestDirFlag(t *testing.T) {
	g := newGlobalSetup(t)
	other := newTracker(t)
	if code, out, errOut := g.work.run("-C", other.dir, "ls"); code != 0 || out != "" {
		t.Fatalf("-C ls: exit %d, %q, %q", code, out, errOut)
	}
	if code, _, errOut := g.work.run("-C", g.global.dir, "new", "Through -C"); code != 0 {
		t.Fatalf("-C new: exit %d: %s", code, errOut)
	}
	if code, out, _ := g.global.run("ls"); code != 0 || strings.Count(out, "\n") != 2 {
		t.Fatalf("-C new did not write to the named tracker: %q", out)
	}
	sub := filepath.Join(other.dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := other.run("-C", filepath.Join("a", "b"), "ls"); code != 0 || out != "" {
		t.Fatalf("-C relative ls: exit %d, %q, %q", code, out, errOut)
	}
	fresh := filepath.Join(g.work.dir, "fresh")
	if err := os.Mkdir(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := g.work.run("-C", "fresh", "init"); code != 0 {
		t.Fatalf("-C init: exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(fresh, ".cairn", "config.yml")); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := g.work.run("-C", other.dir, "ls", "--json"); code != 0 || !strings.HasPrefix(out, `{"ok":true`) {
		t.Fatalf("-C ls --json: exit %d, %q", code, out)
	}
	if code, out, _ := g.work.run("-C", other.dir, "help"); code != 0 || !strings.HasPrefix(out, "Usage: ") {
		t.Fatalf("-C help: exit %d, %q", code, out)
	}
}

// -C forces its folder: with no .cairn there or above, it is no_tracker even when a global tracker is set.
func TestDirFlagSkipsGlobal(t *testing.T) {
	g := newGlobalSetup(t)
	g.work.wantError(1, "no_tracker", "-C", g.work.dir, "ls")
}

func TestDirFlagErrors(t *testing.T) {
	g := newGlobalSetup(t)
	missing := filepath.Join(g.work.dir, "missing")
	file := filepath.Join(g.work.dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	g.work.wantError(1, "error", "-C", missing, "ls")
	g.work.wantError(1, "error", "-C", file, "ls")
	g.work.wantError(2, "usage", "-C")
	g.work.wantError(2, "usage", "-C", "--json", "ls")
	// As with no -C, a command line with no command prints text, even with --json after it.
	if code, _, errOut := g.work.run("-C", g.global.dir); code != 2 || errOut == "" {
		t.Fatalf("-C with no command: exit %d, %q", code, errOut)
	}
	g.work.wantError(2, "usage", "-C", g.global.dir, "-C", g.global.dir, "ls")
	// -C goes before the command: after it, even where a positional argument could be, it is a usage error.
	g.work.wantError(2, "usage", "ls", "-C", g.global.dir)
	g.work.wantError(2, "usage", "search", "words", "-C", g.global.dir)
	// After "--", -C is a plain argument.
	if code, out, errOut := g.work.run("search", "--", "-C"); code != 0 || out != "" {
		t.Fatalf("search -- -C: exit %d, %q, %q", code, out, errOut)
	}
}
