// Package cli runs cairn's commands. It never prompts and never opens an editor.
//
// Arguments: flags start with "--" and take "--name value" or "--name=value"; --json and --force take no value.
// Flags and positional arguments can come in any order, and "--" ends the flags. A flag that takes a value
// takes the next argument even when it starts with "-", so "--tag -t" works, but not one that starts with "--": that
// is a missing value, and "--name=--value" passes such a value. --tag can be repeated; for any other flag the last
// one wins. No arguments at all is a usage error. "--help" and "-h" mean help, and "--version" means version.
// --branches, which a later step builds, is a usage error until then, and so is --all anywhere but show.
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
//   - 1: "no_tracker" (no .cairn here or above), "exists" (init where .cairn exists), "error" (anything else).
//   - 2: "usage": an unknown command or flag, a missing or extra argument, a missing value, or a value in the wrong
//     form, including a bad date, a status not in the config, a tag whose group is not in the config, and a write
//     with no actor.
//   - 4: "not_found": no issue or comment with that id, including a comment id that is not valid. The message names
//     the id that was not found.
//
// Writes (new, edit, rm, comment, retract, purge) need an actor: --as, or else the CAIRN_ACTOR environment variable.
// They also take --persona and --model. new records them on the issue as by, persona and model, once: no later write
// changes them, and an issue without them keeps "". comment records them the same way on the comment, and retract
// records the actor as retracted_by; the other writes check the actor but store it nowhere.
//
// Commands:
//   - init: makes .cairn in Env.Dir (it does not look upward). Text: "initialized <the .cairn path>\n". Data:
//     {"dir": the .cairn path}.
//   - new "title" [--body text] [--tag t]... [--milestone m] [--due YYYY-MM-DD] [--force]: adds an issue with the
//     config's first status, created and updated set to Env.Now in UTC truncated to the millisecond, and a new id.
//     The title must not be blank. A non-empty body that does not end in "\n" gets one. --force is accepted (step 6
//     uses it). Repeated tags are kept once. A config with no statuses is an "error". Text: the id and "\n".
//     Data: the issue.
//   - edit <id> [--title t] [--body text] [--status s] [--tag +t] [--tag -t] [--milestone m] [--due d]: changes
//     only what is given, and at least one change is required. "+t" or "t" adds a tag (no duplicates), "-t" removes
//     it; a removed tag only has to be non-empty, so a tag whose group left the config can still go. An empty
//     --milestone or --due clears it. updated becomes Env.Now as in new. Text: the id and "\n". Data: the issue.
//   - rm <id>: deletes the issue folder, comments included. Text: "removed <id>\n". Data: {"id": id}.
//   - show <id> [--all]: Text: the issue for a person to read, with its by, persona and model when set, then its active comments in order, each with its id,
//     kind, by, persona and model when set, at, source when set, and body. --all adds the retracted comments in their
//     places, each marked "retracted" with its retracted_by, retracted_at, reason and disproved_by. Data: the issue
//     plus "comments", its active comments in order (all of them with --all), a list never null.
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
//   - version, help. help lists every command.
//
// A tag is non-empty with no space, ",", "[" or "]". When it has a ":", the part before the first ":" must be one of
// the config's tag_groups. A milestone has no "/".
//
// An issue in data is {"id", "title", "status", "tags", "milestone", "due", "by", "persona", "model", "created",
// "updated", "github", "body"}, every key always present: tags is a list (never null), times are RFC 3339 UTC strings, github is a number (0 when
// not synced), and empty text is "". Only show adds "comments".
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

	"github.com/seriouslysean/cairn/internal/model"
	"github.com/seriouslysean/cairn/internal/store"
	"github.com/seriouslysean/cairn/internal/words"
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

// Run runs one command and returns its exit code.
func Run(args []string, env Env) int {
	if len(args) == 0 {
		return fail(env, false, 2, "usage", "no command given (run `cairn help`)")
	}
	cmd := args[0]

	switch cmd {
	case "version", "--version":
		return runVersion(env, args[1:])
	case "help", "--help", "-h":
		return runHelp(env, args[1:])
	case "init":
		return runInit(env, args[1:])
	case "new":
		return runNew(env, args[1:])
	case "edit":
		return runEdit(env, args[1:])
	case "rm":
		return runRm(env, args[1:])
	case "show":
		return runShow(env, args[1:])
	case "ls":
		return runLs(env, args[1:])
	case "comment":
		return runComment(env, args[1:])
	case "retract":
		return runRetract(env, args[1:])
	case "purge":
		return runPurge(env, args[1:])
	default:
		jsonOn := scanJSON(args[1:])
		return fail(env, jsonOn, 2, "usage", fmt.Sprintf("unknown command %q", cmd))
	}
}

// scanJSON returns true when args contains a standalone "--json" before any "--".
func scanJSON(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "--json" {
			return true
		}
	}
	return false
}

// flagSpec declares whether a flag takes a value and whether it is repeatable.
type flagSpec struct {
	TakesValue bool
	Repeat     bool
}

// parseResult holds parsed flags and positionals.
type parseResult struct {
	values      map[string][]string
	bools       map[string]bool
	positionals []string
}

// parseFlags parses arguments after the command name according to spec.
func parseFlags(args []string, valid map[string]flagSpec) (*parseResult, error) {
	res := &parseResult{
		values: map[string][]string{},
		bools:  map[string]bool{},
	}
	noMoreFlags := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if noMoreFlags {
			res.positionals = append(res.positionals, a)
			continue
		}
		if a == "--" {
			noMoreFlags = true
			continue
		}
		if strings.HasPrefix(a, "--") {
			name := a[2:]
			inlineVal := ""
			hasInline := false
			if idx := strings.Index(name, "="); idx >= 0 {
				inlineVal = name[idx+1:]
				name = name[:idx]
				hasInline = true
			}

			spec, ok := valid[name]
			if !ok {
				return nil, fmt.Errorf("unknown flag --%s", name)
			}
			if spec.TakesValue {
				if hasInline {
					res.values[name] = append(res.values[name], inlineVal)
				} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					i++
					res.values[name] = append(res.values[name], args[i])
				} else {
					return nil, fmt.Errorf("flag --%s needs a value", name)
				}
			} else {
				if hasInline {
					return nil, fmt.Errorf("flag --%s does not take a value", name)
				}
				res.bools[name] = true
			}
		} else {
			res.positionals = append(res.positionals, a)
		}
	}
	return res, nil
}

func newFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "force": {false, false}, "as": {true, false},
		"persona": {true, false}, "model": {true, false},
		"body": {true, false}, "tag": {true, true}, "milestone": {true, false},
		"due": {true, false},
	}
}

func editFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "as": {true, false}, "persona": {true, false},
		"model": {true, false}, "title": {true, false}, "body": {true, false},
		"status": {true, false}, "tag": {true, true}, "milestone": {true, false},
		"due": {true, false},
	}
}

func rmFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "as": {true, false}, "persona": {true, false},
		"model": {true, false},
	}
}

func showFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "all": {false, false},
	}
}

func lsFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "status": {true, false}, "tag": {true, true},
		"milestone": {true, false}, "due-before": {true, false},
	}
}

func commentFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "as": {true, false}, "persona": {true, false},
		"model": {true, false}, "kind": {true, false}, "source": {true, false},
	}
}

func retractFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "as": {true, false}, "persona": {true, false},
		"model": {true, false}, "reason": {true, false}, "disproved-by": {true, false},
	}
}

func purgeFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "as": {true, false}, "persona": {true, false},
		"model": {true, false},
	}
}

func runVersion(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, map[string]flagSpec{"json": {false, false}})
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) > 0 {
		return fail(env, jsonOn, 2, "usage", "version takes no arguments")
	}
	if jsonOn {
		return successJSON(env, map[string]string{"version": env.Version})
	}
	fmt.Fprintf(env.Stdout, "cairn %s\n", env.Version)
	return 0
}

func runHelp(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, map[string]flagSpec{"json": {false, false}})
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) > 0 {
		return fail(env, jsonOn, 2, "usage", "help takes no arguments")
	}
	if jsonOn {
		return successJSON(env, "cairn init | new | edit | rm | show | ls | comment | retract | purge | version | help")
	}
	fmt.Fprint(env.Stdout, "Usage: cairn <command> [args]\n\nCommands:\n  init   new   edit   rm   show   ls   comment   retract   purge   version   help\n")
	return 0
}

func runInit(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, map[string]flagSpec{"json": {false, false}})
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) > 0 {
		return fail(env, jsonOn, 2, "usage", "init takes no arguments")
	}
	s, initErr := store.Init(env.Dir)
	if initErr != nil {
		if errors.Is(initErr, store.ErrExists) {
			return fail(env, jsonOn, 1, "exists", initErr.Error())
		}
		return fail(env, jsonOn, 1, "error", initErr.Error())
	}
	if jsonOn {
		return successJSON(env, map[string]string{"dir": s.Dir})
	}
	fmt.Fprintf(env.Stdout, "initialized %s\n", s.Dir)
	return 0
}

func runNew(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, newFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) != 1 {
		return fail(env, jsonOn, 2, "usage", "new requires exactly one title argument")
	}
	title := res.positionals[0]
	if strings.TrimSpace(title) == "" {
		return fail(env, jsonOn, 2, "usage", "title must not be blank")
	}

	if _, actorErr := requireActor(env, res); actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}
	cfg, cfgErr := s.Config()
	if cfgErr != nil {
		return fail(env, jsonOn, 1, "error", cfgErr.Error())
	}

	if len(cfg.Statuses) == 0 {
		return fail(env, jsonOn, 1, "error", "config.yml lists no statuses")
	}
	tags := []string{}
	for _, t := range res.values["tag"] {
		if tagErr := validateTag(t, cfg); tagErr != "" {
			return fail(env, jsonOn, 2, "usage", tagErr)
		}
		if !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}

	milestone := ""
	if vals, ok := res.values["milestone"]; ok {
		milestone = vals[len(vals)-1]
		if strings.Contains(milestone, "/") {
			return fail(env, jsonOn, 2, "usage", "milestone must not contain \"/\"")
		}
	}

	due := ""
	if vals, ok := res.values["due"]; ok {
		due = vals[len(vals)-1]
		if errStr := validateDate(due); errStr != "" {
			return fail(env, jsonOn, 2, "usage", errStr)
		}
	}

	body := ""
	if vals, ok := res.values["body"]; ok {
		body = vals[len(vals)-1]
	}
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	newID, idErr := s.NewIssueID(env.Rand)
	if idErr != nil {
		return fail(env, jsonOn, 1, "error", idErr.Error())
	}

	now := env.stamp()

	issue := model.Issue{
		ID:        newID,
		Title:     title,
		Status:    cfg.Statuses[0],
		Tags:      tags,
		Milestone: milestone,
		Due:       due,
		Created:   now,
		Updated:   now,
		Body:      body,
	}
	if saveErr := s.SaveIssue(issue); saveErr != nil {
		return fail(env, jsonOn, 1, "error", saveErr.Error())
	}

	if jsonOn {
		return successJSON(env, issueToJSON(issue))
	}
	fmt.Fprintf(env.Stdout, "%s\n", newID)
	return 0
}

func runEdit(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, editFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) != 1 {
		return fail(env, jsonOn, 2, "usage", "edit requires exactly one issue id")
	}
	issueID := res.positionals[0]

	if _, actorErr := requireActor(env, res); actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}
	issue, _, loadErr := s.LoadIssue(issueID)
	if loadErr != nil {
		return storeFail(env, jsonOn, loadErr, issueID)
	}
	cfg, cfgErr := s.Config()
	if cfgErr != nil {
		return fail(env, jsonOn, 1, "error", cfgErr.Error())
	}

	_, hasTitle := res.values["title"]
	_, hasBody := res.values["body"]
	_, hasStatus := res.values["status"]
	_, hasTags := res.values["tag"]
	_, hasMilestone := res.values["milestone"]
	_, hasDue := res.values["due"]
	if !hasTitle && !hasBody && !hasStatus && !hasTags && !hasMilestone && !hasDue {
		return fail(env, jsonOn, 2, "usage", "edit requires at least one change")
	}

	if hasTitle {
		newTitle := res.values["title"][len(res.values["title"])-1]
		if strings.TrimSpace(newTitle) == "" {
			return fail(env, jsonOn, 2, "usage", "title must not be blank")
		}
		issue.Title = newTitle
	}

	if hasBody {
		newBody := res.values["body"][len(res.values["body"])-1]
		if newBody != "" && !strings.HasSuffix(newBody, "\n") {
			newBody += "\n"
		}
		issue.Body = newBody
	}

	if hasStatus {
		newStatus := res.values["status"][len(res.values["status"])-1]
		if !statusValid(newStatus, cfg) {
			return fail(env, jsonOn, 2, "usage", fmt.Sprintf("status %q is not valid", newStatus))
		}
		issue.Status = newStatus
	}

	if hasTags {
		edits := res.values["tag"]
		for _, e := range edits {
			if tagErr := validateEditTag(e, cfg); tagErr != "" {
				return fail(env, jsonOn, 2, "usage", tagErr)
			}
		}
		issue.Tags = applyTagEdits(issue.Tags, edits)
		if issue.Tags == nil {
			issue.Tags = []string{}
		}
	}

	if hasMilestone {
		newMilestone := res.values["milestone"][len(res.values["milestone"])-1]
		if strings.Contains(newMilestone, "/") {
			return fail(env, jsonOn, 2, "usage", "milestone must not contain \"/\"")
		}
		issue.Milestone = newMilestone
	}

	if hasDue {
		newDue := res.values["due"][len(res.values["due"])-1]
		if newDue != "" {
			if errStr := validateDate(newDue); errStr != "" {
				return fail(env, jsonOn, 2, "usage", errStr)
			}
		}
		issue.Due = newDue
	}

	issue.Updated = env.stamp()

	if saveErr := s.SaveIssue(issue); saveErr != nil {
		return fail(env, jsonOn, 1, "error", saveErr.Error())
	}

	if jsonOn {
		return successJSON(env, issueToJSON(issue))
	}
	fmt.Fprintf(env.Stdout, "%s\n", issue.ID)
	return 0
}

func runRm(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, rmFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) != 1 {
		return fail(env, jsonOn, 2, "usage", "rm requires exactly one issue id")
	}
	issueID := res.positionals[0]

	if _, actorErr := requireActor(env, res); actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}
	if rmErr := s.RemoveIssue(issueID); rmErr != nil {
		return storeFail(env, jsonOn, rmErr, issueID)
	}

	if jsonOn {
		return successJSON(env, map[string]string{"id": issueID})
	}
	fmt.Fprintf(env.Stdout, "removed %s\n", issueID)
	return 0
}

func runShow(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, showFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) != 1 {
		return fail(env, jsonOn, 2, "usage", "show requires exactly one issue id")
	}
	issueID := res.positionals[0]
	allComments := res.bools["all"]

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}
	issue, comments, loadErr := s.LoadIssue(issueID)
	if loadErr != nil {
		return storeFail(env, jsonOn, loadErr, issueID)
	}

	// Filter out retracted comments unless --all.
	var shown []model.Comment
	for _, c := range comments {
		if !allComments && c.RetractedBy != "" {
			continue
		}
		shown = append(shown, c)
	}

	if jsonOn {
		data := issueToJSONMap(issue, true)
		data["comments"] = commentsToJSON(shown)
		return successJSON(env, data)
	}

	fmt.Fprintf(env.Stdout, "id: %s\ntitle: %s\nstatus: %s\n", issue.ID, issue.Title, issue.Status)
	if len(issue.Tags) > 0 {
		fmt.Fprintf(env.Stdout, "tags: %s\n", strings.Join(issue.Tags, ", "))
	}
	if issue.Milestone != "" {
		fmt.Fprintf(env.Stdout, "milestone: %s\n", issue.Milestone)
	}
	if issue.Due != "" {
		fmt.Fprintf(env.Stdout, "due: %s\n", issue.Due)
	}
	fmt.Fprintf(env.Stdout, "created: %s\nupdated: %s\n",
		issue.Created.Format(time.RFC3339Nano), issue.Updated.Format(time.RFC3339Nano))
	if issue.Body != "" {
		fmt.Fprintf(env.Stdout, "\n%s", issue.Body)
	}
	for _, c := range shown {
		fmt.Fprintf(env.Stdout, "\n--- %s", c.ID)
		if c.RetractedBy != "" {
			fmt.Fprintf(env.Stdout, " [retracted]")
		}
		fmt.Fprintf(env.Stdout, " ---\n")
		fmt.Fprintf(env.Stdout, "kind: %s\nby: %s\n", c.Kind, c.By)
		if c.Persona != "" {
			fmt.Fprintf(env.Stdout, "persona: %s\n", c.Persona)
		}
		if c.Model != "" {
			fmt.Fprintf(env.Stdout, "model: %s\n", c.Model)
		}
		fmt.Fprintf(env.Stdout, "at: %s\n", c.At.Format(time.RFC3339Nano))
		if c.Source != "" {
			fmt.Fprintf(env.Stdout, "source: %s\n", c.Source)
		}
		if c.RetractedBy != "" {
			fmt.Fprintf(env.Stdout, "retracted_by: %s\n", c.RetractedBy)
			fmt.Fprintf(env.Stdout, "retracted_at: %s\n", c.RetractedAt.Format(time.RFC3339Nano))
			fmt.Fprintf(env.Stdout, "retracted_reason: %s\n", c.RetractedReason)
			if c.DisprovedBy != "" {
				fmt.Fprintf(env.Stdout, "disproved_by: %s\n", c.DisprovedBy)
			}
		}
		if c.Body != "" {
			fmt.Fprintf(env.Stdout, "\n%s", c.Body)
		}
	}
	return 0
}

func runLs(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, lsFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) > 0 {
		return fail(env, jsonOn, 2, "usage", "ls takes no positional arguments")
	}

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}
	issues, listErr := s.ListIssues()
	if listErr != nil {
		return fail(env, jsonOn, 1, "error", listErr.Error())
	}

	statusFilter := ""
	if vals, ok := res.values["status"]; ok {
		statusFilter = vals[len(vals)-1]
	}
	tagFilters := res.values["tag"]
	milestoneFilter := ""
	if vals, ok := res.values["milestone"]; ok {
		milestoneFilter = vals[len(vals)-1]
	}
	dueBefore := ""
	if vals, ok := res.values["due-before"]; ok {
		dueBefore = vals[len(vals)-1]
		if errStr := validateDate(dueBefore); errStr != "" {
			return fail(env, jsonOn, 2, "usage", errStr)
		}
	}

	var filtered []model.Issue
	for _, iss := range issues {
		if statusFilter != "" && iss.Status != statusFilter {
			continue
		}
		if !tagsMatch(iss.Tags, tagFilters) {
			continue
		}
		if milestoneFilter != "" && iss.Milestone != milestoneFilter {
			continue
		}
		if dueBefore != "" {
			if iss.Due == "" {
				continue
			}
			if iss.Due >= dueBefore {
				continue
			}
		}
		filtered = append(filtered, iss)
	}

	if jsonOn {
		items := make([]map[string]any, len(filtered))
		for idx, iss := range filtered {
			items[idx] = issueToJSONMap(iss, false)
		}
		return successJSON(env, items)
	}
	for _, iss := range filtered {
		fmt.Fprintf(env.Stdout, "%s\t%s\t%s\n", iss.ID, iss.Status, iss.Title)
	}
	return 0
}

func runComment(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, commentFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) != 2 {
		return fail(env, jsonOn, 2, "usage", "comment requires exactly one issue id and one text")
	}
	issueID := res.positionals[0]
	text := res.positionals[1]

	// Validate kind.
	kind := ""
	if vals, ok := res.values["kind"]; ok {
		kind = vals[len(vals)-1]
	}
	if kind != "evidence" && kind != "decision" && kind != "result" {
		return fail(env, jsonOn, 2, "usage", "kind must be evidence, decision or result")
	}

	// Validate source for evidence.
	source := ""
	if vals, ok := res.values["source"]; ok {
		source = vals[len(vals)-1]
	}
	if kind == "evidence" && strings.TrimSpace(source) == "" {
		return fail(env, jsonOn, 2, "usage", "evidence requires --source")
	}

	// Validate text.
	if strings.TrimSpace(text) == "" {
		return fail(env, jsonOn, 2, "usage", "comment text must not be blank")
	}

	actor, actorErr := requireActor(env, res)
	if actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}

	_, comments, loadErr := s.LoadIssue(issueID)
	if loadErr != nil {
		return storeFail(env, jsonOn, loadErr, issueID)
	}

	cfg, cfgErr := s.Config()
	if cfgErr != nil {
		return fail(env, jsonOn, 1, "error", cfgErr.Error())
	}

	// Check retracted comments for overlap warnings.
	body := text
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	type warning struct {
		Comment string  `json:"comment"`
		Reason  string  `json:"reason"`
		Score   float64 `json:"score"`
	}
	var warnings []warning
	for _, c := range comments {
		if c.RetractedBy == "" {
			continue
		}
		score := words.Overlap(text, c.Body)
		if score >= cfg.DupeText {
			warnings = append(warnings, warning{
				Comment: c.ID,
				Reason:  c.RetractedReason,
				Score:   score,
			})
		}
	}

	// Generate new comment id.
	commentID, idErr := s.NewCommentID(issueID, env.Rand)
	if idErr != nil {
		return storeFail(env, jsonOn, idErr, issueID)
	}

	// Get persona and model.
	persona := ""
	if vals, ok := res.values["persona"]; ok {
		persona = vals[len(vals)-1]
	}
	modelName := ""
	if vals, ok := res.values["model"]; ok {
		modelName = vals[len(vals)-1]
	}

	now := env.stamp()
	c := model.Comment{
		ID:      commentID,
		Kind:    kind,
		By:      actor,
		Persona: persona,
		Model:   modelName,
		At:      now,
		Source:  source,
		Body:    body,
	}

	if saveErr := s.SaveComment(issueID, c); saveErr != nil {
		return fail(env, jsonOn, 1, "error", saveErr.Error())
	}

	// Print warnings.
	for _, w := range warnings {
		fmt.Fprintf(env.Stderr, "cairn: warning: matches retracted comment %s (reason %q)\n", w.Comment, w.Reason)
	}

	if jsonOn {
		data := map[string]any{
			"comment":  commentToJSON(c),
			"warnings": warnings,
		}
		if warnings == nil {
			data["warnings"] = []warning{}
		}
		return successJSON(env, data)
	}
	fmt.Fprintf(env.Stdout, "%s\n", commentID)
	return 0
}

func runRetract(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, retractFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) != 2 {
		return fail(env, jsonOn, 2, "usage", "retract requires exactly one issue id and one comment id")
	}
	issueID := res.positionals[0]
	commentID := res.positionals[1]

	reason := ""
	if vals, ok := res.values["reason"]; ok {
		reason = vals[len(vals)-1]
	}
	if strings.TrimSpace(reason) == "" {
		return fail(env, jsonOn, 2, "usage", "retract requires a non-blank --reason")
	}

	disprovedBy := ""
	if vals, ok := res.values["disproved-by"]; ok {
		disprovedBy = vals[len(vals)-1]
	}
	if disprovedBy != "" && disprovedBy == commentID {
		return fail(env, jsonOn, 2, "usage", "disproved-by must not be the same comment as the one being retracted")
	}

	actor, actorErr := requireActor(env, res)
	if actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}

	_, comments, loadErr := s.LoadIssue(issueID)
	if loadErr != nil {
		return storeFail(env, jsonOn, loadErr, issueID)
	}

	// Find the comment to retract.
	var target *model.Comment
	for idx := range comments {
		if comments[idx].ID == commentID {
			target = &comments[idx]
			break
		}
	}
	if target == nil {
		return fail(env, jsonOn, 4, "not_found", fmt.Sprintf("no comment %q on issue %q", commentID, issueID))
	}

	// Check disproved-by exists if given.
	if disprovedBy != "" {
		found := false
		for _, c := range comments {
			if c.ID == disprovedBy {
				found = true
				break
			}
		}
		if !found {
			return fail(env, jsonOn, 4, "not_found", fmt.Sprintf("no comment %q on issue %q", disprovedBy, issueID))
		}
	}

	// Check not already retracted.
	if target.RetractedBy != "" {
		return fail(env, jsonOn, 1, "error", fmt.Sprintf("comment %q is already retracted", commentID))
	}

	// Apply retraction.
	target.RetractedBy = actor
	target.RetractedAt = env.stamp()
	target.RetractedReason = reason
	target.DisprovedBy = disprovedBy

	if saveErr := s.SaveComment(issueID, *target); saveErr != nil {
		return fail(env, jsonOn, 1, "error", saveErr.Error())
	}

	if jsonOn {
		return successJSON(env, commentToJSON(*target))
	}
	fmt.Fprintf(env.Stdout, "retracted %s\n", commentID)
	return 0
}

func runPurge(env Env, args []string) int {
	jsonOn := scanJSON(args)
	res, err := parseFlags(args, purgeFlags())
	if err != nil {
		return fail(env, jsonOn, 2, "usage", err.Error())
	}
	if len(res.positionals) != 2 {
		return fail(env, jsonOn, 2, "usage", "purge requires exactly one issue id and one comment id")
	}
	issueID := res.positionals[0]
	commentID := res.positionals[1]

	if _, actorErr := requireActor(env, res); actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr, "")
	}

	if rmErr := s.RemoveComment(issueID, commentID); rmErr != nil {
		if errors.Is(rmErr, store.ErrNoComment) {
			return fail(env, jsonOn, 4, "not_found", fmt.Sprintf("no comment %q on issue %q", commentID, issueID))
		}
		return storeFail(env, jsonOn, rmErr, issueID)
	}

	if jsonOn {
		return successJSON(env, map[string]string{"id": issueID, "comment": commentID})
	}
	fmt.Fprintf(env.Stdout, "purged %s\n", commentID)
	return 0
}

func validateTag(t string, cfg model.Config) string {
	if t == "" {
		return "tag must not be empty"
	}
	if strings.ContainsAny(t, " ,[]") {
		return fmt.Sprintf("tag %q contains an invalid character", t)
	}
	if before, _, ok := strings.Cut(t, ":"); ok {
		group := before
		found := slices.Contains(cfg.TagGroups, group)
		if !found {
			return fmt.Sprintf("tag group %q is not in the config", group)
		}
	}
	return ""
}

// validateEditTag checks one edit --tag value. A removal only needs a tag, so one whose group has left the config
// can still be removed.
func validateEditTag(e string, cfg model.Config) string {
	if tag, ok := strings.CutPrefix(e, "-"); ok {
		if tag == "" {
			return "tag operation has an empty tag"
		}
		return ""
	}
	return validateTag(strings.TrimPrefix(e, "+"), cfg)
}

func applyTagEdits(existing []string, edits []string) []string {
	result := make([]string, len(existing))
	copy(result, existing)
	for _, e := range edits {
		if strings.HasPrefix(e, "-") {
			tag := e[1:]
			out := make([]string, 0, len(result))
			for _, t := range result {
				if t != tag {
					out = append(out, t)
				}
			}
			result = out
		} else {
			tag := strings.TrimPrefix(e, "+")
			found := slices.Contains(result, tag)
			if !found {
				result = append(result, tag)
			}
		}
	}
	return result
}

func tagsMatch(issueTags, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		found := slices.Contains(issueTags, f)
		if !found {
			return false
		}
	}
	return true
}

func validateDate(d string) string {
	if _, err := time.Parse("2006-01-02", d); err != nil {
		return fmt.Sprintf("invalid date %q", d)
	}
	return ""
}

func statusValid(status string, cfg model.Config) bool {
	return slices.Contains(cfg.Statuses, status)
}

func requireActor(env Env, res *parseResult) (string, string) {
	as := ""
	if vals, ok := res.values["as"]; ok {
		as = vals[len(vals)-1]
	}
	if as == "" && env.Getenv != nil {
		as = env.Getenv("CAIRN_ACTOR")
	}
	if as == "" {
		return "", "no actor: use --as or set CAIRN_ACTOR"
	}
	return as, ""
}

func issueToJSON(i model.Issue) map[string]any {
	return issueToJSONMap(i, true)
}

func issueToJSONMap(i model.Issue, withBody bool) map[string]any {
	tags := i.Tags
	if tags == nil {
		tags = []string{}
	}
	m := map[string]any{
		"id":        i.ID,
		"title":     i.Title,
		"status":    i.Status,
		"tags":      tags,
		"milestone": i.Milestone,
		"due":       i.Due,
		"created":   i.Created.Format(time.RFC3339Nano),
		"updated":   i.Updated.Format(time.RFC3339Nano),
		"github":    i.GitHub,
	}
	if withBody {
		m["body"] = i.Body
	}
	return m
}

func commentToJSON(c model.Comment) map[string]any {
	m := map[string]any{
		"id":      c.ID,
		"kind":    c.Kind,
		"by":      c.By,
		"persona": c.Persona,
		"model":   c.Model,
		"at":      c.At.Format(time.RFC3339Nano),
		"source":  c.Source,
		"body":    c.Body,
	}
	if c.RetractedBy != "" {
		m["retracted"] = map[string]any{
			"by":           c.RetractedBy,
			"at":           c.RetractedAt.Format(time.RFC3339Nano),
			"reason":       c.RetractedReason,
			"disproved_by": c.DisprovedBy,
		}
	} else {
		m["retracted"] = nil
	}
	return m
}

func commentsToJSON(cs []model.Comment) []map[string]any {
	if cs == nil {
		return []map[string]any{}
	}
	result := make([]map[string]any, len(cs))
	for i, c := range cs {
		result[i] = commentToJSON(c)
	}
	return result
}

func successJSON(env Env, data any) int {
	writeJSON(env.Stdout, struct {
		OK   bool `json:"ok"`
		Data any  `json:"data"`
	}{OK: true, Data: data})
	return 0
}

func fail(env Env, jsonOn bool, exitCode int, code, message string) int {
	if jsonOn {
		writeJSON(env.Stdout, struct {
			OK    bool      `json:"ok"`
			Error errorBody `json:"error"`
		}{OK: false, Error: errorBody{Code: code, Message: message}})
	}
	fmt.Fprintf(env.Stderr, "cairn: %s\n", message)
	return exitCode
}

// storeFail maps a store error to its exit and error codes. issueID names the issue in a not-found message.
func storeFail(env Env, jsonOn bool, err error, issueID string) int {
	switch {
	case errors.Is(err, store.ErrNoTracker):
		return fail(env, jsonOn, 1, "no_tracker", "no .cairn folder here or above")
	case errors.Is(err, store.ErrNotFound):
		return fail(env, jsonOn, 4, "not_found", fmt.Sprintf("no issue %q", issueID))
	default:
		return fail(env, jsonOn, 1, "error", err.Error())
	}
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w io.Writer, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	w.Write(data)
	w.Write([]byte("\n"))
}
