// Package cli runs cairn's commands. It never prompts and never opens an editor.
//
// Arguments: flags start with "--" and take "--name value" or "--name=value"; --json and --force take no value.
// Flags and positional arguments can come in any order, and "--" ends the flags. A flag that takes a value
// takes the next argument even when it starts with "-", so "--tag -t" works, but not one that starts with "--": that
// is a missing value, and "--name=--value" passes such a value. --tag can be repeated; for any other flag the last
// one wins. No arguments at all is a usage error. "--help" and "-h" mean help, and "--version" means version.
// Flags that later steps build (--all, --branches) are usage errors until then.
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
//   - 4: "not_found": no issue with that id. The message names the id.
//
// Writes (new, edit, rm) need an actor: --as, or else the CAIRN_ACTOR environment variable. They also take --persona
// and --model. Step 3 checks the actor but stores it nowhere; comments (step 4) record it.
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/seriouslysean/cairn/internal/model"
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

// Run runs one command and returns its exit code.
func Run(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintf(env.Stderr, "cairn: no command given\n")
		return 1
	}
	cmd := args[0]

	switch cmd {
	case "version":
		return runVersion(env, args[1:])
	case "help":
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
	default:
		jsonOn := scanJSON(args[1:])
		return fail(env, jsonOn, 2, "usage", fmt.Sprintf("unknown command %q", cmd))
	}
}

// ---------------------------------------------------------------------------
// JSON detection
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Flag parsing
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Command specs
// ---------------------------------------------------------------------------

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
		"json": {false, false}, "all": {false, false}, "branches": {false, false},
	}
}

func lsFlags() map[string]flagSpec {
	return map[string]flagSpec{
		"json": {false, false}, "status": {true, false}, "tag": {true, true},
		"milestone": {true, false}, "due-before": {true, false}, "branches": {false, false},
	}
}

// ---------------------------------------------------------------------------
// Command implementations
// ---------------------------------------------------------------------------

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
		return successJSON(env, "cairn init | new | edit | rm | show | ls | version | help")
	}
	fmt.Fprint(env.Stdout, "Usage: cairn <command> [args]\n\nCommands:\n  init   new   edit   rm   show   ls   version   help\n")
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

	actor, actorErr := requireActor(env, res)
	if actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}
	_ = actor

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr)
	}
	cfg, cfgErr := s.Config()
	if cfgErr != nil {
		return fail(env, jsonOn, 1, "error", cfgErr.Error())
	}

	// Validate tags.
	tags := res.values["tag"]
	for _, t := range tags {
		if tagErr := validateTag(t, cfg); tagErr != "" {
			return fail(env, jsonOn, 2, "usage", tagErr)
		}
	}

	// Validate milestone.
	milestone := ""
	if vals, ok := res.values["milestone"]; ok {
		milestone = vals[len(vals)-1]
		if strings.Contains(milestone, "/") {
			return fail(env, jsonOn, 2, "usage", "milestone must not contain \"/\"")
		}
	}

	// Validate due.
	due := ""
	if vals, ok := res.values["due"]; ok {
		due = vals[len(vals)-1]
		if errStr := validateDate(due); errStr != "" {
			return fail(env, jsonOn, 2, "usage", errStr)
		}
	}

	// Body.
	body := ""
	if vals, ok := res.values["body"]; ok {
		body = vals[len(vals)-1]
	}
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	// Generate id.
	newID, idErr := s.NewIssueID(env.Rand)
	if idErr != nil {
		return fail(env, jsonOn, 1, "error", idErr.Error())
	}

	now := env.Now()
	if now.IsZero() && env.Now == nil {
		now = time.Now()
	}
	now = now.UTC().Truncate(time.Millisecond)

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
	if issue.Tags == nil {
		issue.Tags = []string{}
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

	actor, actorErr := requireActor(env, res)
	if actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}
	_ = actor

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr)
	}
	issue, _, loadErr := s.LoadIssue(issueID)
	if loadErr != nil {
		return storeFail(env, jsonOn, loadErr)
	}
	cfg, cfgErr := s.Config()
	if cfgErr != nil {
		return fail(env, jsonOn, 1, "error", cfgErr.Error())
	}

	// Check that at least one change is specified.
	_, hasTitle := res.values["title"]
	_, hasBody := res.values["body"]
	_, hasStatus := res.values["status"]
	_, hasTags := res.values["tag"]
	_, hasMilestone := res.values["milestone"]
	_, hasDue := res.values["due"]
	if !hasTitle && !hasBody && !hasStatus && !hasTags && !hasMilestone && !hasDue {
		return fail(env, jsonOn, 2, "usage", "edit requires at least one change")
	}

	// Apply title.
	if hasTitle {
		newTitle := res.values["title"][len(res.values["title"])-1]
		if strings.TrimSpace(newTitle) == "" {
			return fail(env, jsonOn, 2, "usage", "title must not be blank")
		}
		issue.Title = newTitle
	}

	// Apply body.
	if hasBody {
		newBody := res.values["body"][len(res.values["body"])-1]
		if newBody != "" && !strings.HasSuffix(newBody, "\n") {
			newBody += "\n"
		}
		issue.Body = newBody
	}

	// Apply status.
	if hasStatus {
		newStatus := res.values["status"][len(res.values["status"])-1]
		if !statusValid(newStatus, cfg) {
			return fail(env, jsonOn, 2, "usage", fmt.Sprintf("status %q is not valid", newStatus))
		}
		issue.Status = newStatus
	}

	// Apply tags.
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

	// Apply milestone.
	if hasMilestone {
		newMilestone := res.values["milestone"][len(res.values["milestone"])-1]
		if strings.Contains(newMilestone, "/") {
			return fail(env, jsonOn, 2, "usage", "milestone must not contain \"/\"")
		}
		issue.Milestone = newMilestone
	}

	// Apply due.
	if hasDue {
		newDue := res.values["due"][len(res.values["due"])-1]
		if newDue != "" {
			if errStr := validateDate(newDue); errStr != "" {
				return fail(env, jsonOn, 2, "usage", errStr)
			}
		}
		issue.Due = newDue
	}

	// Update time.
	now := env.Now()
	if now.IsZero() && env.Now == nil {
		now = time.Now()
	}
	issue.Updated = now.UTC().Truncate(time.Millisecond)

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

	actor, actorErr := requireActor(env, res)
	if actorErr != "" {
		return fail(env, jsonOn, 2, "usage", actorErr)
	}
	_ = actor

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr)
	}
	if rmErr := s.RemoveIssue(issueID); rmErr != nil {
		return storeFail(env, jsonOn, rmErr)
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

	s, sErr := store.Find(env.Dir)
	if sErr != nil {
		return storeFail(env, jsonOn, sErr)
	}
	issue, _, loadErr := s.LoadIssue(issueID)
	if loadErr != nil {
		return storeFail(env, jsonOn, loadErr)
	}

	if jsonOn {
		return successJSON(env, issueToJSON(issue))
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
		return storeFail(env, jsonOn, sErr)
	}
	issues, listErr := s.ListIssues()
	if listErr != nil {
		return fail(env, jsonOn, 1, "error", listErr.Error())
	}

	// Filters.
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

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

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

func validateEditTag(e string, cfg model.Config) string {
	var tag string
	if strings.HasPrefix(e, "+") {
		tag = e[1:]
	} else if strings.HasPrefix(e, "-") {
		tag = e[1:]
	} else {
		tag = e
	}
	if tag == "" {
		return "tag operation has an empty tag"
	}
	return validateTag(tag, cfg)
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

// ---------------------------------------------------------------------------
// JSON output helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Output and failure helpers
// ---------------------------------------------------------------------------

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

func storeFail(env Env, jsonOn bool, err error) int {
	switch {
	case errors.Is(err, store.ErrNoTracker):
		return fail(env, jsonOn, 1, "no_tracker", "no .cairn folder found")
	case errors.Is(err, store.ErrNotFound):
		return fail(env, jsonOn, 4, "not_found", "issue not found")
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
