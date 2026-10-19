// Package model holds Cairn's issues, comments, milestones and config, and converts each to and from the
// frontmatter fields of its file.
//
// Parsing checks the shape of each value (ids, dates, times, numbers, and scalar or list) but not statuses, tags or
// kinds against the config: a config change must never stop an old file from loading. Keys the parser does not know
// are kept, in order, in Extra (nil when there are none), and written back after the known keys, so a file from a
// newer Cairn survives an edit.
//
// Dates are DateLayout. Times are RFC 3339: they parse with or without fractional seconds and with any offset, and
// are converted to UTC. FormatTime writes them, always with three fraction digits.
package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/seriouslysean/cairn/internal/doc"
	"github.com/seriouslysean/cairn/internal/id"
)

// DateLayout is how a date is written: "2006-01-02".
const DateLayout = time.DateOnly

// TimeLayout is how every time is written: UTC, with three fraction digits, so every time is 24 characters and times
// compare the same as text as they do as times.
const TimeLayout = "2006-01-02T15:04:05.000Z07:00"

// FormatTime writes t in TimeLayout, in UTC.
func FormatTime(t time.Time) string {
	return t.UTC().Format(TimeLayout)
}

// The comment kinds a writer may add.
const (
	KindEvidence = "evidence"
	KindDecision = "decision"
	KindResult   = "result"
)

// Issue is the issue.md file of an issue folder. Required keys: id, title, status, created, updated. The others are
// left out of the file when empty (GitHub when 0).
type Issue struct {
	ID        string
	Title     string
	Status    string
	Tags      []string
	Milestone string
	Due       string
	By        string // who created the issue; set once, never changed by an edit
	Persona   string
	Model     string
	Created   time.Time
	Updated   time.Time
	GitHub    int
	Extra     []doc.Field
	Body      string
}

// Comment is one c-<id>.md file in an issue folder. Required keys: id, kind, by, at. The others are left out of the
// file when empty.
type Comment struct {
	ID              string
	Kind            string
	By              string
	Persona         string
	Model           string
	At              time.Time
	Source          string
	RetractedBy     string
	RetractedAt     time.Time
	RetractedReason string
	DisprovedBy     string
	Extra           []doc.Field
	Body            string
}

// Retracted reports whether the comment has been retracted.
func (c Comment) Retracted() bool {
	return c.RetractedBy != ""
}

// Milestone is one file in milestones/. Its id is its file name without ".md" and is any non-empty name without "/"
// (for example "v0.4"). Required keys: id, title, status. Due is left out of the file when empty.
type Milestone struct {
	ID     string
	Title  string
	Status string
	Due    string
	Extra  []doc.Field
	Body   string
}

// Config is config.yml: frontmatter only, with an empty body. Every key is optional.
type Config struct {
	Statuses      []string
	TagGroups     []string
	GitHubRepo    string
	EmbeddingsURL string
	DupeText      float64
	Extra         []doc.Field
}

// DefaultConfig is the config used for absent keys: statuses [backlog, open, in-progress, done], tag groups
// [severity, effort, area] and dupe_text 0.6, with GitHub sync and embeddings off.
func DefaultConfig() Config {
	return Config{
		Statuses:  []string{"backlog", "open", "in-progress", "done"},
		TagGroups: []string{"severity", "effort", "area"},
		DupeText:  0.6,
	}
}

// ParseIssue builds an Issue from a file's frontmatter and body. It returns an error for a missing required key, a
// bad id, date, time or number, or a scalar where a list belongs (or the reverse).
func ParseIssue(fields []doc.Field, body []byte) (Issue, error) {
	r := newReader(fields)
	i := Issue{
		ID:        r.text("id"),
		Title:     r.text("title"),
		Status:    r.text("status"),
		Tags:      r.list("tags", nil),
		Milestone: r.text("milestone"),
		Due:       r.date("due"),
		By:        r.text("by"),
		Persona:   r.text("persona"),
		Model:     r.text("model"),
		Created:   r.time("created"),
		Updated:   r.time("updated"),
		GitHub:    r.number("github"),
		Body:      string(body),
	}
	if err := r.done("id", "title", "status", "created", "updated"); err != nil {
		return Issue{}, err
	}
	if !id.Valid(i.ID) {
		return Issue{}, fmt.Errorf("model: bad issue id %q", i.ID)
	}
	i.Extra = r.extra()
	return i, nil
}

// Fields returns the issue's frontmatter in the order id, title, status, tags, milestone, due, by, persona, model,
// created, updated, github, then Extra.
func (i Issue) Fields() []doc.Field {
	var w writer
	w.text("id", i.ID)
	w.text("title", i.Title)
	w.text("status", i.Status)
	w.optList("tags", i.Tags)
	w.optText("milestone", i.Milestone)
	w.optText("due", i.Due)
	w.optText("by", i.By)
	w.optText("persona", i.Persona)
	w.optText("model", i.Model)
	w.time("created", i.Created)
	w.time("updated", i.Updated)
	w.optNumber("github", i.GitHub)
	return w.with(i.Extra)
}

// ParseComment builds a Comment from a file's frontmatter and body, with the same errors as ParseIssue. The id must
// pass id.ValidComment, and disproved_by too when present.
func ParseComment(fields []doc.Field, body []byte) (Comment, error) {
	r := newReader(fields)
	c := Comment{
		ID:              r.text("id"),
		Kind:            r.text("kind"),
		By:              r.text("by"),
		Persona:         r.text("persona"),
		Model:           r.text("model"),
		At:              r.time("at"),
		Source:          r.text("source"),
		RetractedBy:     r.text("retracted_by"),
		RetractedAt:     r.time("retracted_at"),
		RetractedReason: r.text("retracted_reason"),
		DisprovedBy:     r.text("disproved_by"),
		Body:            string(body),
	}
	if err := r.done("id", "kind", "by", "at"); err != nil {
		return Comment{}, err
	}
	if !id.ValidComment(c.ID) {
		return Comment{}, fmt.Errorf("model: bad comment id %q", c.ID)
	}
	if c.DisprovedBy != "" && !id.ValidComment(c.DisprovedBy) {
		return Comment{}, fmt.Errorf("model: bad disproved_by id %q", c.DisprovedBy)
	}
	c.Extra = r.extra()
	return c, nil
}

// Fields returns the comment's frontmatter in the order id, kind, by, persona, model, at, source, retracted_by,
// retracted_at, retracted_reason, disproved_by, then Extra.
func (c Comment) Fields() []doc.Field {
	var w writer
	w.text("id", c.ID)
	w.text("kind", c.Kind)
	w.text("by", c.By)
	w.optText("persona", c.Persona)
	w.optText("model", c.Model)
	w.time("at", c.At)
	w.optText("source", c.Source)
	w.optText("retracted_by", c.RetractedBy)
	w.optTime("retracted_at", c.RetractedAt)
	w.optText("retracted_reason", c.RetractedReason)
	w.optText("disproved_by", c.DisprovedBy)
	return w.with(c.Extra)
}

// ParseMilestone builds a Milestone from a file's frontmatter and body, with the same errors as ParseIssue.
func ParseMilestone(fields []doc.Field, body []byte) (Milestone, error) {
	r := newReader(fields)
	m := Milestone{
		ID:     r.text("id"),
		Title:  r.text("title"),
		Status: r.text("status"),
		Due:    r.date("due"),
		Body:   string(body),
	}
	if err := r.done("id", "title", "status"); err != nil {
		return Milestone{}, err
	}
	if strings.Contains(m.ID, "/") {
		return Milestone{}, fmt.Errorf("model: milestone id %q must not contain \"/\"", m.ID)
	}
	m.Extra = r.extra()
	return m, nil
}

// Fields returns the milestone's frontmatter in the order id, title, status, due, then Extra.
func (m Milestone) Fields() []doc.Field {
	var w writer
	w.text("id", m.ID)
	w.text("title", m.Title)
	w.text("status", m.Status)
	w.optText("due", m.Due)
	return w.with(m.Extra)
}

// ParseConfig builds a Config from config.yml's frontmatter, taking DefaultConfig's value for each absent key. It
// returns an error for a bad number or a scalar where a list belongs.
func ParseConfig(fields []doc.Field) (Config, error) {
	def := DefaultConfig()
	r := newReader(fields)
	c := Config{
		Statuses:      r.list("statuses", def.Statuses),
		TagGroups:     r.list("tag_groups", def.TagGroups),
		GitHubRepo:    r.text("github_repo"),
		EmbeddingsURL: r.text("embeddings_url"),
		DupeText:      r.float("dupe_text", def.DupeText),
	}
	if err := r.done(); err != nil {
		return Config{}, err
	}
	c.Extra = r.extra()
	return c, nil
}

// Fields returns the config's frontmatter in the order statuses, tag_groups, github_repo, embeddings_url, dupe_text,
// then Extra. github_repo and embeddings_url are left out when empty. Numbers are written with
// strconv.FormatFloat(f, 'g', -1, 64).
func (c Config) Fields() []doc.Field {
	var w writer
	w.list("statuses", c.Statuses)
	w.list("tag_groups", c.TagGroups)
	w.optText("github_repo", c.GitHubRepo)
	w.optText("embeddings_url", c.EmbeddingsURL)
	w.float("dupe_text", c.DupeText)
	return w.with(c.Extra)
}
