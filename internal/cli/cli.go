// Package cli runs cairn's commands. It never prompts and never opens an editor.
//
// Arguments: flags start with "--" and take "--name value" or "--name=value"; --json, --force and --all take no
// value. Flags and positional arguments can come in any order, and "--" ends the flags. A flag that takes a value
// takes the next argument whatever it looks like, so "--tag -t" works. --tag can be repeated; for any other flag the
// last one wins.
//
// Output: --json anywhere before "--" turns on JSON, even when the other arguments are wrong. With --json, success
// prints {"ok": true, "data": …} and failure prints
// {"ok": false, "error": {"code": "…", "message": "…"}} on stdout, plus the message on one line of stderr. Without
// --json, success prints text on stdout and failure prints "cairn: <message>" on one line of stderr and nothing on
// stdout.
//
// Exit codes and error codes:
//   - 0: ok.
//   - 1: "no_tracker" (no .cairn here or above), "exists" (init where .cairn exists), "error" (anything else).
//   - 2: "usage": an unknown command or flag, a missing or extra argument, a missing value, or a value in the wrong
//     form, including a bad date, a status not in the config, a tag whose group is not in the config, and a write
//     with no actor.
//   - 4: "not_found": no issue with that id.
//
// Writes (new, edit, rm) need an actor: --as, or else the CAIRN_ACTOR environment variable. They also take --persona
// and --model. Step 3 checks the actor but stores it nowhere; comments (step 4) record it.
//
// Commands:
//   - init: makes .cairn in Env.Dir (it does not look upward). Data: {"dir": the .cairn path}.
//   - new "title" [--body text] [--tag t]... [--milestone m] [--due YYYY-MM-DD] [--force]: adds an issue with the
//     config's first status, created and updated set to Env.Now in UTC truncated to the millisecond, and a new id.
//     The title must not be blank. A non-empty body that does not end in "\n" gets one. --force is accepted (step 6
//     uses it). Text: the id and "\n". Data: the issue.
//   - edit <id> [--title t] [--body text] [--status s] [--tag +t] [--tag -t] [--milestone m] [--due d]: changes
//     only what is given, and at least one change is required. "+t" or "t" adds a tag (no duplicates), "-t" removes
//     it. An empty --milestone or --due clears it. updated becomes Env.Now as in new. Text: the id and "\n". Data:
//     the issue.
//   - rm <id>: deletes the issue folder, comments included. Text: "removed <id>\n". Data: {"id": id}.
//   - show <id>: Text: the issue for a person to read. Data: the issue.
//   - ls [--status s] [--tag t]... [--milestone m] [--due-before YYYY-MM-DD]: lists issues in store order. Every filter
//     given must match; --due-before keeps issues with a due date before d. Text: one "<id>\t<status>\t<title>\n"
//     line per issue. Data: a list of issues without "body" (an empty list, never null).
//   - version, help.
//
// A tag is non-empty with no space, ",", "[" or "]". When it has a ":", the part before the first ":" must be one of
// the config's tag_groups. A milestone has no "/".
//
// An issue in data is {"id", "title", "status", "tags", "milestone", "due", "created", "updated", "github", "body"},
// every key always present: tags is a list (never null), times are RFC 3339 UTC strings, github is a number (0 when
// not synced), and empty text is "".
package cli

import (
	"io"
	"time"
)

// Env is everything a command touches outside its arguments. A nil Now means time.Now, a nil Rand means
// crypto/rand, and a nil Getenv means no environment variables.
type Env struct {
	Version string
	Dir     string
	Stdout  io.Writer
	Stderr  io.Writer
	Getenv  func(string) string
	Now     func() time.Time
	Rand    io.Reader
}

// Run runs one command and returns its exit code.
func Run(args []string, env Env) int {
	panic("todo: step 3")
}
