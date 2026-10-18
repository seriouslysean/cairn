package main

import (
	"bytes"
	"encoding/json"
	"runtime/debug"
	"testing"
)

func TestVersionJSON(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"version", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || !got.OK || got.Data["version"] != version() {
		t.Fatalf("got %q (%v)", out.String(), err)
	}
}

// The version is the one Go records for the main module, so go install …@v0.1.0 reports v0.1.0 without -ldflags.
func TestVersionFromBuildInfo(t *testing.T) {
	want := "(devel)"
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		want = bi.Main.Version
	}
	if got := version(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"nope"}, &out, &errb); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if errb.Len() == 0 {
		t.Fatal("no message on stderr")
	}
}
