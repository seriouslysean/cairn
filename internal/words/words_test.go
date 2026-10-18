package words

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ,.;!  ", nil},
		{"The last row", []string{"the", "last", "row"}},
		{"row ROW Row", []string{"row"}},
		{"export_test.go:42 fails", []string{"export", "test", "go", "42", "fails"}},
		{"a-b\nc\td", []string{"a", "b", "c", "d"}},
		{"Naïve CAFÉ", []string{"naïve", "café"}},
		{"b a b c a", []string{"b", "a", "c"}},
	} {
		got := Split(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Split(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOverlap(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want float64
	}{
		{"a b c d", "a b c e", 3.0 / 5},
		{"a b c d", "a b e f", 2.0 / 6},
		{"a b", "c d", 0},
		{"The last row is dropped", "the LAST row, is dropped!", 1},
		{"Row row", "row", 1},
		{"", "a", 0},
		{"", "", 0},
		{"...", "!!!", 0},
	} {
		if got := Overlap(c.a, c.b); got != c.want {
			t.Errorf("Overlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := Overlap(c.b, c.a); got != c.want {
			t.Errorf("Overlap(%q, %q) = %v, want %v", c.b, c.a, got, c.want)
		}
	}
}
