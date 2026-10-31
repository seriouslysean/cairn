package doc

import (
	"reflect"
	"strings"
	"testing"
)

const sample = "---\nid: 7f3k\ntitle: \"Export: drops the last row\"\ntags: [severity:high, area:export]\nnone: []\n" +
	"status: open\n---\nBody line.\n"

func TestParse(t *testing.T) {
	f, body, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	want := []Field{
		{Key: "id", Value: "7f3k"},
		{Key: "title", Value: "Export: drops the last row"},
		{Key: "tags", List: []string{"severity:high", "area:export"}, IsList: true},
		{Key: "none", List: []string{}, IsList: true},
		{Key: "status", Value: "open"},
	}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("fields %#v", f)
	}
	if string(body) != "Body line.\n" {
		t.Fatalf("body %q", body)
	}
}

// A file checked out with CRLF line endings, as git does on Windows, has the same fields. The body comes back as the
// file has it, CRs included.
func TestParseCRLF(t *testing.T) {
	want, lfBody, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	f, body, err := Parse([]byte(strings.ReplaceAll(sample, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("fields %#v", f)
	}
	if want := strings.ReplaceAll(string(lfBody), "\n", "\r\n"); string(body) != want {
		t.Fatalf("body %q, want %q", body, want)
	}
	if _, _, err := Parse([]byte("---\r\nid: a\r\n\r\ntitle:\r\n---\r\n")); err != nil {
		t.Fatalf("a blank line and an empty value: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	f, body, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Format(f, body)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != sample {
		t.Fatalf("got\n%s", out)
	}
}

func TestParseUnquotedColon(t *testing.T) {
	f, _, err := Parse([]byte("---\ntitle: a: b\n---\n"))
	if err != nil || len(f) != 1 || f[0].Value != "a: b" {
		t.Fatalf("%#v %v", f, err)
	}
}

func TestParseEmptyValuesBlankLinesAndNoBody(t *testing.T) {
	f, body, err := Parse([]byte("---\n\nkey:\nq: \"\"\n---"))
	if err != nil || len(f) != 2 || f[0].Value != "" || f[1].Value != "" || len(body) != 0 {
		t.Fatalf("%#v %q %v", f, body, err)
	}
}

func TestParseErrors(t *testing.T) {
	for name, in := range map[string]string{
		"no opening line": "id: 7f3k\n---\n",
		"no closing line": "---\nid: 7f3k\n",
		"upper-case key":  "---\nId: 7f3k\n---\n",
		"no colon":        "---\njust text\n---\n",
		"duplicate key":   "---\nid: a\nid: b\n---\n",
		"bad quote":       "---\nid: \"open\n---\n",
		"empty list item": "---\ntags: [a, , b]\n---\n",
		"comment line":    "---\n# note\n---\n",
	} {
		if _, _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestFormatQuoting(t *testing.T) {
	out, err := Format([]Field{
		{Key: "a", Value: ""},
		{Key: "b", Value: " x"},
		{Key: "c", Value: "[x]"},
		{Key: "d", Value: "#x"},
		{Key: "e", Value: "two\nlines"},
		{Key: "f", Value: "k: v"},
		{Key: "g", Value: "plain text"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\na: \"\"\nb: \" x\"\nc: \"[x]\"\nd: \"#x\"\ne: \"two\\nlines\"\nf: \"k: v\"\ng: plain text\n---\n"
	if string(out) != want {
		t.Fatalf("got\n%s", out)
	}
}

func TestFormatRejectsBadListItems(t *testing.T) {
	for _, item := range []string{"", "a,b", "a]", "[a", "a\nb"} {
		if _, err := Format([]Field{{Key: "t", List: []string{item}, IsList: true}}, nil); err == nil {
			t.Errorf("%q: want an error", item)
		}
	}
}

func TestFormatRejectsBadKey(t *testing.T) {
	if _, err := Format([]Field{{Key: "Bad", Value: "x"}}, nil); err == nil {
		t.Fatal("want an error")
	}
}

// Parse trims list items, so an item with outer space would not survive the round trip.
func TestFormatRejectsSpacedListItems(t *testing.T) {
	for _, item := range []string{" a", "a ", " ", "\ta"} {
		if _, err := Format([]Field{{Key: "t", List: []string{item}, IsList: true}}, nil); err == nil {
			t.Errorf("%q: want an error", item)
		}
	}
}

// Parse rejects a duplicate key, so Format must not write one.
func TestFormatRejectsDuplicateKey(t *testing.T) {
	if _, err := Format([]Field{{Key: "a", Value: "1"}, {Key: "a", Value: "2"}}, nil); err == nil {
		t.Fatal("want an error")
	}
}

// YAML drops the spaces and tabs after a plain value, so a hand edit that leaves them reads as the value without them.
func TestParsePlainValueTrailingSpace(t *testing.T) {
	f, _, err := Parse([]byte("---\nstatus: done \t\n---\n"))
	if err != nil || len(f) != 1 || f[0].Value != "done" {
		t.Fatalf("%#v %v", f, err)
	}
}
