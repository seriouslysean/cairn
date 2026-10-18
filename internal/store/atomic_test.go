package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWriteFileAtomicReplaces(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	if err := WriteFileAtomic(p, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "two" {
		t.Fatalf("got %q", b)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	assertOnly(t, dir, "a.md")
}

func TestWriteFileAtomicMissingFolder(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFileAtomic(filepath.Join(dir, "missing", "a.md"), []byte("x"), 0o644); err == nil {
		t.Fatal("want an error")
	}
	assertOnly(t, dir)
}

func TestWriteFileAtomicIntoFolderPathFails(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(sub, []byte("x"), 0o644); err == nil {
		t.Fatal("want an error when path is a folder")
	}
	assertOnly(t, dir, "sub")
}

func TestWriteFileAtomicConcurrentWritersNeverTear(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = WriteFileAtomic(p, bytes.Repeat([]byte(fmt.Sprint(i%10)), 4096), 0o644)
		}(i)
	}
	wg.Wait()
	b, _ := os.ReadFile(p)
	if len(b) != 4096 || strings.Count(string(b), string(b[:1])) != 4096 {
		t.Fatalf("torn write: %d bytes", len(b))
	}
	assertOnly(t, dir, "a.md")
}

func assertOnly(t *testing.T, dir string, names ...string) {
	t.Helper()
	es, _ := os.ReadDir(dir)
	var got []string
	for _, e := range es {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != strings.Join(names, ",") {
		t.Fatalf("folder holds %v, want %v", got, names)
	}
}
