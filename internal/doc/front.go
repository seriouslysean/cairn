// Package doc reads and writes the frontmatter at the start of every Cairn file.
//
// Frontmatter is a strict subset of YAML. The file starts with a line "---", then has one "key: value" per line,
// then a line "---". A key is a lower-case letter followed by lower-case letters, digits, "_" or "-". The value
// follows the first ": " (or is empty when the line ends at "key:"). A value is plain text, a double-quoted Go
// string literal (it starts with `"`), or a flow list (it starts with "[" and ends with "]"), whose items are split
// on "," and trimmed. Blank lines inside the frontmatter are skipped; any other line is an error.
package doc

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
	panic("todo: step 1")
}

// Format writes fields and body back as "---\n", one "key: value\n" per field, "---\n", then body.
// A scalar is written with strconv.Quote when it is empty, has leading or trailing space, starts with `"`, "[" or
// "#", or contains ": " or a newline; otherwise it is written as is. A list is written as "[a, b]". It returns an
// error for a malformed key or for a list item that is empty or contains ",", "[", "]" or a newline.
// For any file Format wrote, Format(Parse(file)) gives the same bytes.
func Format(fields []Field, body []byte) ([]byte, error) {
	panic("todo: step 1")
}
