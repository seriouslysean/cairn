package id

import (
	"bytes"
	"strings"
	"testing"
)

func TestAlphabet(t *testing.T) {
	if len(Alphabet) != 32 {
		t.Fatalf("len %d", len(Alphabet))
	}
	seen := map[rune]bool{}
	for _, c := range Alphabet {
		if seen[c] || strings.ContainsRune("ilou", c) {
			t.Fatalf("bad character %q", c)
		}
		seen[c] = true
	}
}

func TestNewMapsBytes(t *testing.T) {
	got, err := New(4, nil, bytes.NewReader([]byte{0, 1, 31, 32}))
	if err != nil || got != "01z0" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestNewSkipsTaken(t *testing.T) {
	got, err := New(4, map[string]bool{"0000": true}, bytes.NewReader([]byte{0, 0, 0, 0, 1, 1, 1, 1}))
	if err != nil || got != "1111" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestNewFailsWhenReaderRunsOut(t *testing.T) {
	if _, err := New(4, nil, bytes.NewReader([]byte{1, 2})); err == nil {
		t.Fatal("want an error")
	}
}

func TestNewGivesUpAfter100Tries(t *testing.T) {
	r := bytes.NewReader(make([]byte, 4*200)) // every candidate is "0000"
	if _, err := New(4, map[string]bool{"0000": true}, r); err == nil {
		t.Fatal("want an error")
	}
}

func TestNewRejectsLength(t *testing.T) {
	for _, n := range []int{0, 3, 9} {
		if _, err := New(n, nil, nil); err == nil {
			t.Errorf("n=%d: want an error", n)
		}
	}
}

func TestNewDefaultReader(t *testing.T) {
	got, err := New(5, nil, nil)
	if err != nil || len(got) != 5 || !Valid(got) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestValid(t *testing.T) {
	for s, want := range map[string]bool{
		"7f3k": true, "7f3kab12": true,
		"7F3K": false, "7f3i": false, "7f-k": false, "abc": false, "7f3kab123": false, "": false,
	} {
		if Valid(s) != want {
			t.Errorf("Valid(%q) = %v", s, !want)
		}
	}
}

func TestLength(t *testing.T) {
	for n, want := range map[int]int{0: 4, 999: 4, 1000: 5, 31999: 5, 32000: 6, 1 << 20: 6} {
		if got := Length(n); got != want {
			t.Errorf("Length(%d) = %d, want %d", n, got, want)
		}
	}
}
