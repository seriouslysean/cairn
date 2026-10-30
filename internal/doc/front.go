// Package doc reads and writes the frontmatter at the start of every Cairn file.
//
// Frontmatter is a strict subset of YAML. The file starts with a line "---", then has one "key: value" per line,
// then a line "---". A key is a lower-case letter followed by lower-case letters, digits, "_" or "-". The value
// follows the first ": " (or is empty when the line ends at "key:"). A value is plain text (spaces and tabs at its end
// are dropped, as YAML does), a double-quoted Go string literal (it starts with `"`), or a flow list (it starts with
// "[" and ends with "]"), whose items are split on "," and trimmed. Blank lines inside the frontmatter are skipped;
// any other line is an error. A frontmatter line can end in "\r\n" as well as "\n", as files checked out on Windows
// do; Format writes "\n".
package doc

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The frontmatter's syntax.
const (
	delimiter  = "---"        // the line that opens and closes the frontmatter
	keyEnd     = ":"          // ends a key; "key:" alone has an empty value
	separator  = keyEnd + " " // between a key and a value
	quote      = `"`          // starts a quoted value
	comment    = "#"          // starts a YAML comment, so a plain value can't
	plainSpace = " \t"        // dropped from the end of a plain value, as YAML does
	listOpen   = "["
	listClose  = "]"
	listSep    = ","
	listJoin   = listSep + " " // between the items Format writes
	newline    = "\n"
	cr         = "\r" // before newline in a CRLF line ending
)

// Field is one frontmatter line. A scalar sets Value and leaves List nil. A list sets IsList and List; an empty
// list "[]" gives a non-nil empty List.
type Field struct {
	Key    string
	Value  string
	List   []string
	IsList bool
}

// Parse splits a file into its fields, in order, and the body: everything after the closing "---" line, as the file
// has it. It returns an error for a missing opening or closing line, a malformed line or key, a duplicate key, an
// empty list item, or a quoted value that does not unquote.
func Parse(b []byte) ([]Field, []byte, error) {
	first, rest, ok := cutLine(b)
	if !ok || first != delimiter {
		return nil, nil, errors.New("doc: missing opening ---")
	}
	var fields []Field
	seen := map[string]bool{}
	for {
		if len(rest) == 0 {
			return nil, nil, errors.New("doc: missing closing ---")
		}
		s, after, _ := cutLine(rest)
		rest = after
		if s == delimiter {
			return fields, rest, nil
		}
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

// cutLine cuts b at its first newline into the line before it, without the CR of a CRLF ending, and the rest.
func cutLine(b []byte) (line string, rest []byte, ok bool) {
	l, rest, ok := bytes.Cut(b, []byte(newline))
	return string(bytes.TrimSuffix(l, []byte(cr))), rest, ok
}

// Format writes fields and body back as "---\n", one "key: value\n" per field, "---\n", then body.
// A scalar is written with strconv.Quote when it is empty, has leading or trailing space, starts with `"`, "[" or
// "#", or contains ": " or a newline; otherwise it is written as is. A list is written as "[a, b]". It returns an
// error for a malformed or duplicate key, or for a list item that is empty, has leading or trailing space, or
// contains ",", "[", "]" or a newline.
// For any file Format wrote, Format(Parse(file)) gives the same bytes.
func Format(fields []Field, body []byte) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString(delimiter + newline)
	seen := map[string]bool{}
	for _, f := range fields {
		if !validKey(f.Key) {
			return nil, fmt.Errorf("doc: bad key %q", f.Key)
		}
		if seen[f.Key] {
			return nil, fmt.Errorf("doc: duplicate key %q", f.Key)
		}
		seen[f.Key] = true
		out.WriteString(f.Key + separator)
		if f.IsList {
			if i := slices.IndexFunc(f.List, badItem); i >= 0 {
				return nil, fmt.Errorf("doc: bad list item %q in %q", f.List[i], f.Key)
			}
			out.WriteString(listOpen + strings.Join(f.List, listJoin) + listClose)
		} else if needsQuote(f.Value) {
			out.WriteString(strconv.Quote(f.Value))
		} else {
			out.WriteString(f.Value)
		}
		out.WriteString(newline)
	}
	out.WriteString(delimiter + newline)
	out.Write(body)
	return out.Bytes(), nil
}

// needsQuote reports whether a scalar must be quoted to parse back as itself: it is empty, has leading or trailing
// space, starts like a quoted value, a list or a comment, or holds a separator or a newline.
func needsQuote(v string) bool {
	return v == "" || strings.TrimSpace(v) != v || strings.ContainsAny(v[:1], quote+listOpen+comment) ||
		strings.Contains(v, separator) || strings.Contains(v, newline)
}

// badItem reports whether a list item cannot be written: it is empty, has leading or trailing space, or holds a
// list character or a newline.
func badItem(it string) bool {
	return it == "" || strings.TrimSpace(it) != it || strings.ContainsAny(it, listSep+listOpen+listClose+newline)
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
	if key, ok := strings.CutSuffix(s, keyEnd); ok && validKey(key) {
		return Field{Key: key}, nil
	}
	key, val, ok := strings.Cut(s, separator)
	if !ok || !validKey(key) {
		return Field{}, fmt.Errorf("doc: malformed line %q", s)
	}
	f := Field{Key: key}
	switch {
	case strings.HasPrefix(val, quote):
		u, err := strconv.Unquote(val)
		if err != nil {
			return Field{}, fmt.Errorf("doc: bad quoted value for %q: %w", key, err)
		}
		f.Value = u
	case strings.HasPrefix(val, listOpen):
		inner, ok := strings.CutSuffix(strings.TrimPrefix(val, listOpen), listClose)
		if !ok {
			return Field{}, fmt.Errorf("doc: unterminated list for %q", key)
		}
		f.IsList = true
		f.List = []string{}
		if inner = strings.TrimSpace(inner); inner != "" {
			for it := range strings.SplitSeq(inner, listSep) {
				it = strings.TrimSpace(it)
				if it == "" {
					return Field{}, fmt.Errorf("doc: empty list item in %q", key)
				}
				f.List = append(f.List, it)
			}
		}
	default:
		f.Value = strings.TrimRight(val, plainSpace)
	}
	return f, nil
}
