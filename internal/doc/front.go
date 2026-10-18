// Package doc reads and writes the frontmatter at the start of every Cairn file.
//
// Frontmatter is a strict subset of YAML. The file starts with a line "---", then has one "key: value" per line,
// then a line "---". A key is a lower-case letter followed by lower-case letters, digits, "_" or "-". The value
// follows the first ": " (or is empty when the line ends at "key:"). A value is plain text, a double-quoted Go
// string literal (it starts with `"`), or a flow list (it starts with "[" and ends with "]"), whose items are split
// on "," and trimmed. Blank lines inside the frontmatter are skipped; any other line is an error.
package doc

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Field is one frontmatter line. A scalar sets Value and leaves List nil. A list sets IsList and List; an empty
// list "[]" gives a non-nil empty List.
type Field struct {
	Key    string
	Value  string
	List   []string
	IsList bool
}

// Parse splits a file into its fields, in order, and the body: everything after the closing "---" line.
// It returns an error for a missing opening or closing line, a malformed line or key, a duplicate key, an empty
// list item, or a quoted value that does not unquote.
func Parse(b []byte) ([]Field, []byte, error) {
	first, rest, ok := bytes.Cut(b, []byte("\n"))
	if !ok || string(first) != "---" {
		return nil, nil, errors.New("doc: missing opening ---")
	}
	var fields []Field
	seen := map[string]bool{}
	for {
		if len(rest) == 0 {
			return nil, nil, errors.New("doc: missing closing ---")
		}
		line, after, _ := bytes.Cut(rest, []byte("\n"))
		rest = after
		if string(line) == "---" {
			return fields, rest, nil
		}
		s := string(line)
		if strings.TrimSpace(s) == "" {
			continue
		}
		f, err := parseLine(s)
		if err != nil {
			return nil, nil, err
		}
		if seen[f.Key] {
			return nil, nil, fmt.Errorf("doc: duplicate key %q", f.Key)
		}
		seen[f.Key] = true
		fields = append(fields, f)
	}
}

// Format writes fields and body back as "---\n", one "key: value\n" per field, "---\n", then body.
// A scalar is written with strconv.Quote when it is empty, has leading or trailing space, starts with `"`, "[" or
// "#", or contains ": " or a newline; otherwise it is written as is. A list is written as "[a, b]". It returns an
// error for a malformed or duplicate key, or for a list item that is empty, has leading or trailing space, or
// contains ",", "[", "]" or a newline.
// For any file Format wrote, Format(Parse(file)) gives the same bytes.
func Format(fields []Field, body []byte) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("---\n")
	seen := map[string]bool{}
	for _, f := range fields {
		if !validKey(f.Key) {
			return nil, fmt.Errorf("doc: bad key %q", f.Key)
		}
		if seen[f.Key] {
			return nil, fmt.Errorf("doc: duplicate key %q", f.Key)
		}
		seen[f.Key] = true
		out.WriteString(f.Key + ": ")
		if f.IsList {
			for _, it := range f.List {
				if it == "" || strings.TrimSpace(it) != it || strings.ContainsAny(it, ",[]\n") {
					return nil, fmt.Errorf("doc: bad list item %q in %q", it, f.Key)
				}
			}
			out.WriteString("[" + strings.Join(f.List, ", ") + "]")
		} else {
			v := f.Value
			if v == "" || strings.TrimSpace(v) != v || strings.ContainsAny(v[:1], `"[#`) ||
				strings.Contains(v, ": ") || strings.Contains(v, "\n") {
				v = strconv.Quote(v)
			}
			out.WriteString(v)
		}
		out.WriteString("\n")
	}
	out.WriteString("---\n")
	out.Write(body)
	return out.Bytes(), nil
}

// validKey reports whether k is a lower-case letter followed by lower-case letters, digits, "_" or "-".
func validKey(k string) bool {
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch {
		case c >= 'a' && c <= 'z':
		case i > 0 && (c >= '0' && c <= '9' || c == '_' || c == '-'):
		default:
			return false
		}
	}
	return k != ""
}

// parseLine parses one "key: value" line.
func parseLine(s string) (Field, error) {
	key, val, ok := strings.Cut(s, ":")
	if !ok || !validKey(key) {
		return Field{}, fmt.Errorf("doc: malformed line %q", s)
	}
	f := Field{Key: key}
	if val == "" {
		return f, nil
	}
	if val[0] != ' ' {
		return Field{}, fmt.Errorf("doc: malformed line %q", s)
	}
	val = val[1:]
	switch {
	case strings.HasPrefix(val, `"`):
		u, err := strconv.Unquote(val)
		if err != nil {
			return Field{}, fmt.Errorf("doc: bad quoted value for %q: %w", key, err)
		}
		f.Value = u
	case strings.HasPrefix(val, "["):
		if !strings.HasSuffix(val, "]") {
			return Field{}, fmt.Errorf("doc: unterminated list for %q", key)
		}
		f.IsList = true
		f.List = []string{}
		if inner := strings.TrimSpace(val[1 : len(val)-1]); inner != "" {
			for _, it := range strings.Split(inner, ",") {
				it = strings.TrimSpace(it)
				if it == "" {
					return Field{}, fmt.Errorf("doc: empty list item in %q", key)
				}
				f.List = append(f.List, it)
			}
		}
	default:
		f.Value = val
	}
	return f, nil
}
