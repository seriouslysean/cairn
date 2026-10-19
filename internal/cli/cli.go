// Package cli runs cairn's commands. It never prompts and never opens an editor.
//
// Arguments: flags start with "--" and take "--name value" or "--name=value"; --json and --all take no value.
// Flags and positional arguments can come in any order, and "--" ends the flags. A flag that takes a value
// takes the next argument even when it starts with "-", so "--tag -t" works, but not one that starts with "--": that
// is a missing value, and "--name=--value" passes such a value. --tag can be repeated; for any other flag the last
// one wins. No arguments at all is a usage error. "--help" and "-h" mean help, and "--version" means version.
// --branches and search --meaning, which later steps build, are usage errors until then, and so is --all anywhere but
// show.
//
// Output: --json anywhere before "--" turns on JSON, even when the other arguments are wrong. With --json, success
// prints {"ok": true, "data": …} and failure prints
// {"ok": false, "error": {"code": "…", "message": "…"}} on stdout, plus the message on one line of stderr. Without
// --json, success prints text on stdout and failure prints "cairn: <message>" on one line of stderr and nothing on
// stdout. A command that succeeds with warnings still exits 0, and prints each warning in both modes as one line
// "cairn: warning: <message>" on stderr.
//
// Exit codes and error codes:
//   - 0: ok.
//   - 1: "no_tracker" (no .cairn here or above), "exists" (init where .cairn exists, or milestone new for an id
//     that has a file), "error" (anything else).
//   - 2: "usage": an unknown command or flag, a missing or extra argument, a missing value, or a value in the wrong
//     form, including a bad date, a status not in the config, a tag whose group is not in the config, and a write
//     with no actor.
//   - 4: "not_found": no issue, comment or milestone with that id, including an id that is not valid. The message names
//     the id that was not found.
//
// Writes (new, edit, rm, comment, retract, purge, milestone new, milestone edit) need an actor: --as, or else the
// CAIRN_ACTOR environment variable. They also take --persona and --model. new records them on the issue as by, persona
// and model, once: no later write changes them, and an issue without them keeps "". comment records them the same way
// on the comment, and retract records the actor as retracted_by; the other writes check the actor but store it nowhere.
//
// Commands:
//   - init: makes .cairn in Env.Dir (it does not look upward). Text: "initialized <the .cairn path>\n". Data:
//     {"dir": the .cairn path}.
//   - new "title" [--body text] [--tag t]... [--milestone m] [--due YYYY-MM-DD]: adds an issue with the
//     config's first status, created and updated set to Env.Now in UTC truncated to the millisecond, and a new id.
//     The title must not be blank. A non-empty body that does not end in "\n" gets one. Repeated tags are kept once.
//     A config with no statuses is an "error". Text: the id and "\n". Data: the issue.
//   - edit <id> [--title t] [--body text] [--status s] [--tag +t] [--tag -t] [--milestone m] [--due d]: changes
//     only what is given, and at least one change is required. "+t" or "t" adds a tag (no duplicates), "-t" removes
//     it; a removed tag only has to be non-empty, so a tag whose group left the config can still go. An empty
//     --milestone or --due clears it. updated becomes Env.Now as in new. Text: the id and "\n". Data: the issue.
//   - rm <id>: deletes the issue folder, comments included. Text: "removed <id>\n". Data: {"id": id}.
//   - show <id> [--all]: Text: the issue for a person to read, with its by, persona and model when set, then its
//     active comments in order, each with its id, kind, by, persona and model when set, at, source when set, and
//     body. --all adds the retracted comments in their places, each marked "retracted" with its retracted_by,
//     retracted_at, reason and disproved_by. Data: the issue plus "comments", its active comments in order (all of
//     them with --all), a list never null.
//   - ls [--status s] [--tag t]... [--milestone m] [--due-before YYYY-MM-DD]: lists issues in store order. Every filter
//     given must match; --due-before keeps issues with a due date before d. Text: one "<id>\t<status>\t<title>\n"
//     line per issue. Data: a list of issues without "body" (an empty list, never null).
//   - comment <id> --kind evidence|decision|result [--source s] "text": adds a comment with a new id, by, persona and
//     model from the actor flags, and at set to Env.Now as in new. The text must not be blank, and gets a trailing
//     "\n" as a body does. evidence needs a --source that is not blank; any other kind, github included, is a usage
//     error. A comment never changes issue.md. When the text's words.Overlap with a retracted comment on the same
//     issue reaches the config's dupe_text, the comment is still added, with one warning per such comment in
//     comment order: "matches retracted comment <comment id> (reason <reason as %q>)". Text: the new comment id and
//     "\n". Data: {"comment": the comment, "warnings": [{"comment": id, "reason": reason, "score": overlap}, …]},
//     warnings an empty list when there are none.
//   - retract <id> <comment> --reason "…" [--disproved-by <comment>]: keeps the comment and sets retracted_by to the
//     actor, retracted_at to Env.Now as in new, retracted_reason, and disproved_by. The reason must not be blank.
//     --disproved-by names another comment on the same issue: one that does not exist is not_found, and the comment
//     itself is a usage error. A comment already retracted is an "error" and stays as it was. Text: "retracted
//     <comment>\n". Data: the comment.
//   - purge <id> <comment>: deletes the comment's file, retracted or not. Text: "purged <comment>\n". Data:
//     {"id": the issue id, "comment": the comment id}.
//   - tags: every tag on any issue, with the number of issues that have it (an issue counts once), sorted by tag in
//     byte order. Text: one "<tag>\t<count>\n" line per tag. Data: a list of {"tag", "count"} (an empty list, never
//     null).
//   - milestones: every milestone with a file in milestones/ and every milestone an issue names, with the number of
//     issues that name it. A milestone without a file has "" for title, status and due. Sorted by due, those without
//     one last, then by id. Text: one "<id>\t<due>\t<issues>\t<title>\n" line each. Data: a list of {"id", "title",
//     "status", "due", "issues"} (an empty list, never null).
//   - milestone new <id> "title" [--due YYYY-MM-DD]: writes milestones/<id>.md with the config's first status. The id
//     must not be empty, contain "/" or start with "." (a usage error). An id that already has a file is "exists",
//     and a file that does not parse is an "error"; either way the file is left as it was. The title must not be
//     blank. Text: the id and "\n". Data: the milestone.
//   - milestone edit <id> [--title t] [--status s] [--due YYYY-MM-DD]: changes only what is given, and at least one
//     change is required. The title must not be blank, the status must be one of the config's, and an empty --due
//     clears it. The file's other keys and its body are kept. A milestone without a file is not_found, and a file
//     that does not parse is an "error". Text: the id and "\n". Data: the milestone.
//   - milestone with no subcommand, or any other subcommand, is a usage error. The subcommand is the argument right
//     after milestone.
//   - search <words>...: the positional arguments joined by spaces are the query, and its words are words.Split of
//     it; a query with no words is a usage error. An issue's matches are the query words found among the words of
//     its title, its body, and the bodies and sources of its active comments; tags are not searched. Issues with at
//     least one match are listed by matches, most first, then in store order. Text: one
//     "<id>\t<matches>\t<title>\n" line each. Data: a list of issues without "body", each with "matches" added (an
//     empty list, never null, when nothing matches).
//   - version.
//   - help prints commandUsage, one usage line per command, then agentRules, the rules in docs/spec.md. Data:
//     {"usage": commandUsage, "rules": agentRules}.
//
// A tag is non-empty with no space, ",", "[" or "]". When it has a ":", the part before the first ":" must be one of
// the config's tag_groups. A milestone has no "/".
//
// A milestone in data is {"id", "title", "status", "due"}, every key always present ("" when due is not set).
//
// An issue in data is {"id", "title", "status", "tags", "milestone", "due", "by", "persona", "model", "created",
// "updated", "github", "body"}, every key always present: tags is a list (never null), times are strings in
// model.TimeLayout (RFC 3339 UTC with three fraction digits, in text output too), github is a number (0 when not
// synced), and empty text is "". Only show adds "comments".
//
// A comment in data is {"id", "kind", "by", "persona", "model", "at", "source", "body", "retracted"}, every key always
// present: retracted is null for an active comment, and otherwise {"by", "at", "reason", "disproved_by"}, with ""
// for no disproved_by.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/seriouslysean/cairn/internal/store"
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

// stamp is the time a write records: Env.Now, or time.Now when it is nil, in UTC and truncated to the millisecond.
func (e Env) stamp() time.Time {
	now := time.Now
	if e.Now != nil {
		now = e.Now
	}
	return now().UTC().Truncate(time.Millisecond)
}

// actorEnv is the environment variable that names the actor when --as is not given.
const actorEnv = "CAIRN_ACTOR"

// Flag names, without the leading "--".
const (
	flagJSON        = "json"
	flagAll         = "all"
	flagAs          = "as"
	flagPersona     = "persona"
	flagModel       = "model"
	flagTitle       = "title"
	flagBody        = "body"
	flagStatus      = "status"
	flagTag         = "tag"
	flagMilestone   = "milestone"
	flagDue         = "due"
	flagDueBefore   = "due-before"
	flagKind        = "kind"
	flagSource      = "source"
	flagReason      = "reason"
	flagDisprovedBy = "disproved-by"
)

const (
	flagPrefix = "--"
	endOfFlags = "--"
)

// switches are the flags that take no value. Every other flag takes one.
var switches = map[string]bool{flagJSON: true, flagAll: true}

// actorFlags are the flags every write takes.
var actorFlags = []string{flagAs, flagPersona, flagModel}

// command is one cairn command.
type command struct {
	flags   []string // the flags it takes besides --json, and actorFlags when it writes
	writes  bool
	args    int    // how many positional arguments it takes, or oneOrMore
	argsErr string // the usage error for any other number
	run     func(c *call) (output, error)
}

// oneOrMore as a command's args means it takes one positional argument or more.
const oneOrMore = -1

// milestoneCommand is the command whose first argument names a subcommand in milestoneCommands.
const milestoneCommand = "milestone"

var commands = map[string]command{
	"version": {argsErr: "version takes no arguments", run: runVersion},
	"help":    {argsErr: "help takes no arguments", run: runHelp},
	"init":    {argsErr: "init takes no arguments", run: runInit},
	"new": {flags: []string{flagBody, flagTag, flagMilestone, flagDue}, writes: true,
		args: 1, argsErr: "new requires exactly one title argument", run: runNew},
	"edit": {flags: []string{flagTitle, flagBody, flagStatus, flagTag, flagMilestone, flagDue}, writes: true,
		args: 1, argsErr: "edit requires exactly one issue id", run: runEdit},
	"rm":   {writes: true, args: 1, argsErr: "rm requires exactly one issue id", run: runRm},
	"show": {flags: []string{flagAll}, args: 1, argsErr: "show requires exactly one issue id", run: runShow},
	"ls": {flags: []string{flagStatus, flagTag, flagMilestone, flagDueBefore},
		argsErr: "ls takes no positional arguments", run: runLs},
	"comment": {flags: []string{flagKind, flagSource}, writes: true,
		args: 2, argsErr: "comment requires exactly one issue id and one text", run: runComment},
	"retract": {flags: []string{flagReason, flagDisprovedBy}, writes: true,
		args: 2, argsErr: "retract requires exactly one issue id and one comment id", run: runRetract},
	"purge": {writes: true,
		args: 2, argsErr: "purge requires exactly one issue id and one comment id", run: runPurge},
	"tags":       {argsErr: "tags takes no positional arguments", run: runTags},
	"milestones": {argsErr: "milestones takes no positional arguments", run: runMilestones},
	"search":     {args: oneOrMore, argsErr: "search requires at least one word", run: runSearch},
}

var milestoneCommands = map[string]command{
	"new": {flags: []string{flagDue}, writes: true,
		args: 2, argsErr: "milestone new requires exactly one id and one title", run: runMilestoneNew},
	"edit": {flags: []string{flagTitle, flagStatus, flagDue}, writes: true,
		args: 1, argsErr: "milestone edit requires exactly one milestone id", run: runMilestoneEdit},
}

// aliases are other names for commands.
var aliases = map[string]string{"--version": "version", "--help": "help", "-h": "help"}

// call is one run of a command: its environment, its parsed arguments, and the warnings it gives.
type call struct {
	env      Env
	args     []string            // the positional arguments
	values   map[string][]string // each value flag's values, in the order given
	switches map[string]bool     // the switches given
	warnings []string
}

// output is what a command prints when it succeeds: data with --json, text without.
type output struct {
	data any
	text string
}

// Run runs one command and returns its exit code.
func Run(args []string, env Env) int {
	jsonOn := len(args) > 0 && scanJSON(args[1:])
	c := &call{env: env}
	out, err := c.dispatch(args)
	if err == nil {
		err = c.print(out, jsonOn)
	}
	if err != nil {
		return c.fail(err, jsonOn)
	}
	for _, w := range c.warnings {
		fmt.Fprintf(env.Stderr, "cairn: warning: %s\n", w)
	}
	return ExitOK
}

// scanJSON reports whether args holds a standalone "--json" before any "--".
func scanJSON(args []string) bool {
	for _, a := range args {
		if a == endOfFlags {
			return false
		}
		if a == flagPrefix+flagJSON {
			return true
		}
	}
	return false
}

func (c *call) dispatch(args []string) (output, error) {
	if len(args) == 0 {
		return output{}, usageError("no command given (run `cairn help`)")
	}
	name, args := args[0], args[1:]
	if name == milestoneCommand {
		if len(args) == 0 {
			return output{}, usageError("milestone requires a subcommand: new or edit")
		}
		cmd, ok := milestoneCommands[args[0]]
		if !ok {
			return output{}, usageError(fmt.Sprintf("unknown milestone subcommand %q", args[0]))
		}
		return c.exec(cmd, args[1:])
	}
	if alias, ok := aliases[name]; ok {
		name = alias
	}
	cmd, ok := commands[name]
	if !ok {
		return output{}, usageError(fmt.Sprintf("unknown command %q", name))
	}
	return c.exec(cmd, args)
}

func (c *call) exec(cmd command, args []string) (output, error) {
	allowed := append([]string{flagJSON}, cmd.flags...)
	if cmd.writes {
		allowed = append(allowed, actorFlags...)
	}
	if err := c.parse(args, allowed); err != nil {
		return output{}, err
	}
	if !cmd.takes(len(c.args)) {
		return output{}, usageError(cmd.argsErr)
	}
	return cmd.run(c)
}

// takes reports whether n positional arguments are what cmd takes.
func (cmd command) takes(n int) bool {
	if cmd.args == oneOrMore {
		return n > 0
	}
	return n == cmd.args
}

// parse splits args into the positional arguments and the flags in allowed, as the package doc describes.
func (c *call) parse(args, allowed []string) error {
	c.values, c.switches = map[string][]string{}, map[string]bool{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == endOfFlags {
			c.args = append(c.args, args[i+1:]...)
			break
		}
		name, ok := strings.CutPrefix(a, flagPrefix)
		if !ok {
			c.args = append(c.args, a)
			continue
		}
		name, inline, hasInline := strings.Cut(name, "=")
		switch {
		case !slices.Contains(allowed, name):
			return usageError(fmt.Sprintf("unknown flag --%s", name))
		case switches[name] && hasInline:
			return usageError(fmt.Sprintf("flag --%s does not take a value", name))
		case switches[name]:
			c.switches[name] = true
		case hasInline:
			c.values[name] = append(c.values[name], inline)
		case i+1 < len(args) && !strings.HasPrefix(args[i+1], flagPrefix):
			i++
			c.values[name] = append(c.values[name], args[i])
		default:
			return usageError(fmt.Sprintf("flag --%s needs a value", name))
		}
	}
	return nil
}

// value is the last value given for a flag, or "" when it was not given.
func (c *call) value(name string) string {
	v, _ := c.given(name)
	return v
}

// given is the last value given for a flag, and whether it was given.
func (c *call) given(name string) (string, bool) {
	vs := c.values[name]
	if len(vs) == 0 {
		return "", false
	}
	return vs[len(vs)-1], true
}

// anyGiven reports whether any of the flags was given.
func (c *call) anyGiven(names ...string) bool {
	return slices.ContainsFunc(names, func(name string) bool { return len(c.values[name]) > 0 })
}

// actor is --as, or else the actorEnv environment variable. It is a usage error when both are empty.
func (c *call) actor() (string, error) {
	as := c.value(flagAs)
	if as == "" && c.env.Getenv != nil {
		as = c.env.Getenv(actorEnv)
	}
	if as == "" {
		return "", usageError("no actor: use --as or set " + actorEnv)
	}
	return as, nil
}

// warn adds a warning, which Run prints when the command succeeds.
func (c *call) warn(msg string) {
	c.warnings = append(c.warnings, msg)
}

// print writes out to stdout: its data in the success envelope with --json, else its text.
func (c *call) print(out output, jsonOn bool) error {
	if jsonOn {
		return json.NewEncoder(c.env.Stdout).Encode(successJSON{OK: true, Data: out.data})
	}
	_, err := io.WriteString(c.env.Stdout, out.text)
	return err
}

// fail reports err as the package doc describes and returns its exit code.
func (c *call) fail(err error, jsonOn bool) int {
	f := failureOf(err)
	if jsonOn {
		// Stdout has already failed if this does; the message still goes to stderr.
		_ = json.NewEncoder(c.env.Stdout).Encode(failureJSON{Error: errorJSON{Code: f.code, Message: f.msg}})
	}
	fmt.Fprintf(c.env.Stderr, "cairn: %s\n", f.msg)
	return f.exit
}

// Exit codes, as the package doc gives them.
const (
	ExitOK       = 0
	ExitError    = 1
	ExitUsage    = 2
	ExitNotFound = 4
)

// Error codes.
const (
	codeError     = "error"
	codeUsage     = "usage"
	codeNotFound  = "not_found"
	codeExists    = "exists"
	codeNoTracker = "no_tracker"
)

// failure is an error with its exit code and error code.
type failure struct {
	exit int
	code string
	msg  string
}

func (f *failure) Error() string { return f.msg }

func usageError(msg string) error    { return &failure{ExitUsage, codeUsage, msg} }
func notFoundError(msg string) error { return &failure{ExitNotFound, codeNotFound, msg} }
func existsError(msg string) error   { return &failure{ExitError, codeExists, msg} }

// noIssue turns store.ErrNotFound into the not_found failure for issueID, and returns any other error as it is.
func noIssue(err error, issueID string) error {
	if errors.Is(err, store.ErrNotFound) {
		return notFoundError(fmt.Sprintf("no issue %q", issueID))
	}
	return err
}

// noComment is the not_found failure for a comment that issueID does not have.
func noComment(commentID, issueID string) error {
	return notFoundError(fmt.Sprintf("no comment %q on issue %q", commentID, issueID))
}

// failureOf is err as a failure: itself when it is one, no_tracker or exists for those store errors, else "error".
func failureOf(err error) *failure {
	var f *failure
	switch {
	case errors.As(err, &f):
		return f
	case errors.Is(err, store.ErrNoTracker):
		return &failure{ExitError, codeNoTracker, "no " + store.DirName + " folder here or above"}
	case errors.Is(err, store.ErrExists):
		return &failure{ExitError, codeExists, err.Error()}
	}
	return &failure{ExitError, codeError, err.Error()}
}
