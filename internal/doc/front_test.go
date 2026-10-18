package doc

import (
	"reflect"
	"testing"
)

const sample = "---\nid: 7f3k\ntitle: \"Export: drops the last row\"\ntags: [severity:high, area:export]\nnone: []\nstatus: open\n---\nBody line.\n"

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
