package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/seriouslysean/cairn/internal/model"
	"github.com/seriouslysean/cairn/internal/store"
)

func runInit(c *call) (output, error) {
	s, err := store.Init(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	return output{initJSON{Dir: s.Dir}, "initialized " + s.Dir + "\n"}, nil
}

func runNew(c *call) (output, error) {
	title := c.args[0]
	if blank(title) {
		return output{}, usageError("title must not be blank")
	}
	actor, err := c.actor()
	if err != nil {
		return output{}, err
	}
	s, err := store.Find(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	cfg, err := s.Config()
	if err != nil {
		return output{}, err
	}
	status, err := firstStatus(cfg)
	if err != nil {
		return output{}, err
	}
	var tags []string
	for _, t := range c.values[flagTag] {
		if err := checkTag(t, cfg); err != nil {
			return output{}, err
		}
		if !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}
	milestone := c.value(flagMilestone)
	if err := checkIssueMilestone(milestone); err != nil {
		return output{}, err
	}
	due, hasDue := c.given(flagDue)
	if hasDue {
		if err := checkDate(due); err != nil {
			return output{}, err
		}
	}
	issueID, err := s.NewIssueID(c.env.Rand)
	if err != nil {
		return output{}, err
	}
	now := c.env.stamp()
	i := model.Issue{
		ID:        issueID,
		Title:     title,
		Status:    status,
		Tags:      tags,
		Milestone: milestone,
		Due:       due,
		By:        actor,
		Persona:   c.value(flagPersona),
		Model:     c.value(flagModel),
		Created:   now,
		Updated:   now,
		Body:      withNewline(c.value(flagBody)),
	}
	if err := s.SaveIssue(i); err != nil {
		return output{}, err
	}
	return output{issueData(i), i.ID + "\n"}, nil
}

func runEdit(c *call) (output, error) {
	issueID := c.args[0]
	if _, err := c.actor(); err != nil {
		return output{}, err
	}
	s, err := store.Find(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	i, _, err := s.LoadIssue(issueID)
	if err != nil {
		return output{}, noIssue(err, issueID)
	}
	cfg, err := s.Config()
	if err != nil {
		return output{}, err
	}
	if !c.anyGiven(flagTitle, flagBody, flagStatus, flagTag, flagMilestone, flagDue) {
		return output{}, usageError("edit requires at least one change")
	}
	if title, ok := c.given(flagTitle); ok {
		if blank(title) {
			return output{}, usageError("title must not be blank")
		}
		i.Title = title
	}
	if body, ok := c.given(flagBody); ok {
		i.Body = withNewline(body)
	}
	if status, ok := c.given(flagStatus); ok {
		if err := checkStatus(status, cfg); err != nil {
			return output{}, err
		}
		i.Status = status
	}
	if edits := c.values[flagTag]; len(edits) > 0 {
		for _, e := range edits {
			if err := checkEditTag(e, cfg); err != nil {
				return output{}, err
			}
		}
		i.Tags = editTags(i.Tags, edits)
	}
	if milestone, ok := c.given(flagMilestone); ok {
		if err := checkIssueMilestone(milestone); err != nil {
			return output{}, err
		}
		i.Milestone = milestone
	}
	if due, ok := c.given(flagDue); ok {
		if err := checkClearableDate(due); err != nil {
			return output{}, err
		}
		i.Due = due
	}
	i.Updated = c.env.stamp()
	if err := s.SaveIssue(i); err != nil {
		return output{}, err
	}
	return output{issueData(i), i.ID + "\n"}, nil
}

func runRm(c *call) (output, error) {
	issueID := c.args[0]
	if _, err := c.actor(); err != nil {
		return output{}, err
	}
	s, err := store.Find(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	if err := s.RemoveIssue(issueID); err != nil {
		return output{}, noIssue(err, issueID)
	}
	return output{rmJSON{ID: issueID}, "removed " + issueID + "\n"}, nil
}

func runShow(c *call) (output, error) {
	issueID := c.args[0]
	s, err := store.Find(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	i, comments, err := s.LoadIssue(issueID)
	if err != nil {
		return output{}, noIssue(err, issueID)
	}
	if !c.switches[flagAll] {
		comments = slices.DeleteFunc(comments, model.Comment.Retracted)
	}
	return output{showData(i, comments), showText(i, comments)}, nil
}

// showText is the issue and its comments for a person to read.
func showText(i model.Issue, comments []model.Comment) string {
	var b strings.Builder
	line := func(key, value string) { fmt.Fprintf(&b, "%s: %s\n", key, value) }
	optLine := func(key, value string) {
		if value != "" {
			line(key, value)
		}
	}
	line("id", i.ID)
	line("title", i.Title)
	line("status", i.Status)
	if len(i.Tags) > 0 {
		line("tags", strings.Join(i.Tags, ", "))
	}
	optLine("milestone", i.Milestone)
	optLine("due", i.Due)
	optLine("by", i.By)
	optLine("persona", i.Persona)
	optLine("model", i.Model)
	line("created", model.FormatTime(i.Created))
	line("updated", model.FormatTime(i.Updated))
	if i.Body != "" {
		b.WriteString("\n" + i.Body)
	}
	for _, cm := range comments {
		mark := ""
		if cm.Retracted() {
			mark = " [retracted]"
		}
		fmt.Fprintf(&b, "\n--- %s%s ---\n", cm.ID, mark)
		line("kind", cm.Kind)
		line("by", cm.By)
		optLine("persona", cm.Persona)
		optLine("model", cm.Model)
		line("at", model.FormatTime(cm.At))
		optLine("source", cm.Source)
		if cm.Retracted() {
			line("retracted_by", cm.RetractedBy)
			line("retracted_at", model.FormatTime(cm.RetractedAt))
			line("retracted_reason", cm.RetractedReason)
			optLine("disproved_by", cm.DisprovedBy)
		}
		if cm.Body != "" {
			b.WriteString("\n" + cm.Body)
		}
	}
	return b.String()
}

// Tag syntax: tagForbidden are the characters no tag may hold, tagGroupSep ends a tag's group, and an edit's
// --tag adds a tag (with or without tagAdd) or removes it (tagRemove).
const (
	tagForbidden = " ,[]"
	tagGroupSep  = ":"
	tagAdd       = "+"
	tagRemove    = "-"
)

// errNoStatuses is the error for a config that lists no statuses, which leaves new issues and milestones none.
var errNoStatuses = errors.New("config.yml lists no statuses")

// firstStatus is the status a new issue or milestone starts with.
func firstStatus(cfg model.Config) (string, error) {
	if len(cfg.Statuses) == 0 {
		return "", errNoStatuses
	}
	return cfg.Statuses[0], nil
}

func checkStatus(status string, cfg model.Config) error {
	if !slices.Contains(cfg.Statuses, status) {
		return usageError(fmt.Sprintf("status %q is not valid", status))
	}
	return nil
}

func checkTag(t string, cfg model.Config) error {
	if t == "" {
		return usageError("tag must not be empty")
	}
	if strings.ContainsAny(t, tagForbidden) {
		return usageError(fmt.Sprintf("tag %q contains an invalid character", t))
	}
	if group, _, ok := strings.Cut(t, tagGroupSep); ok && !slices.Contains(cfg.TagGroups, group) {
		return usageError(fmt.Sprintf("tag group %q is not in the config", group))
	}
	return nil
}

// checkEditTag checks one edit --tag value. A removal only needs a tag, so one whose group has left the config can
// still be removed.
func checkEditTag(e string, cfg model.Config) error {
	if t, ok := strings.CutPrefix(e, tagRemove); ok {
		if t == "" {
			return usageError("tag operation has an empty tag")
		}
		return nil
	}
	return checkTag(strings.TrimPrefix(e, tagAdd), cfg)
}

// editTags applies edit --tag values to tags, in order, and returns the result in a new slice.
func editTags(tags, edits []string) []string {
	tags = slices.Clone(tags)
	for _, e := range edits {
		if t, ok := strings.CutPrefix(e, tagRemove); ok {
			tags = slices.DeleteFunc(tags, func(have string) bool { return have == t })
		} else if t := strings.TrimPrefix(e, tagAdd); !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}
	return tags
}

// checkIssueMilestone checks the milestone an issue names, which need not have a file yet.
func checkIssueMilestone(m string) error {
	if strings.Contains(m, "/") {
		return usageError(`milestone must not contain "/"`)
	}
	return nil
}

func checkDate(d string) error {
	if _, err := time.Parse(model.DateLayout, d); err != nil {
		return usageError(fmt.Sprintf("invalid date %q", d))
	}
	return nil
}

// checkClearableDate is checkDate for a flag where "" clears the date.
func checkClearableDate(d string) error {
	if d == "" {
		return nil
	}
	return checkDate(d)
}

func blank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// withNewline is a body as it is stored: a non-empty one ends in "\n".
func withNewline(body string) string {
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body
}
