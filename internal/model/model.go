// Package model holds Cairn's issues, comments, milestones and config, and converts each to and from the
// frontmatter fields of its file.
//
// Parsing checks the shape of each value (ids, dates, times, numbers, and scalar or list) but not statuses, tags or kinds against
// the config: a config change must never stop an old file from loading. Keys the parser does not know are kept, in
// order, in Extra (nil when there are none), and written back after the known keys, so a file from a newer Cairn
// survives an edit.
//
// Dates are "2006-01-02". Times are RFC 3339: they parse with or without fractional seconds and with any offset,
// are converted to UTC, and are written with time.RFC3339Nano, so whole seconds print without a fraction.
package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/seriouslysean/cairn/internal/doc"
	"github.com/seriouslysean/cairn/internal/id"
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

// issueKnownKeys are the frontmatter keys ParseIssue handles.
var issueKnownKeys = map[string]bool{
	"id": true, "title": true, "status": true, "tags": true,
	"milestone": true, "due": true, "by": true, "persona": true, "model": true,
	"created": true, "updated": true, "github": true,
}

// ParseIssue builds an Issue from a file's frontmatter and body. It returns an error for a missing required key, a
// bad id, date, time or number, or a scalar where a list belongs (or the reverse).
func ParseIssue(fields []doc.Field, body []byte) (Issue, error) {
	i := Issue{Body: string(body)}
	var extra []doc.Field
	for _, f := range fields {
		if !issueKnownKeys[f.Key] {
			extra = append(extra, f)
			continue
		}
		if err := checkShape(f, f.Key == "tags"); err != nil {
			return Issue{}, err
		}
		switch f.Key {
		case "id":
			i.ID = f.Value
		case "title":
			i.Title = f.Value
		case "status":
			i.Status = f.Value
		case "tags":
			i.Tags = f.List
		case "milestone":
			i.Milestone = f.Value
		case "due":
			_, err := time.Parse("2006-01-02", f.Value)
			if err != nil {
				return Issue{}, fmt.Errorf("model: bad due date %q: %w", f.Value, err)
			}
			i.Due = f.Value
		case "by":
			i.By = f.Value
		case "persona":
			i.Persona = f.Value
		case "model":
			i.Model = f.Value
		case "created":
			t, err := time.Parse(time.RFC3339, f.Value)
			if err != nil {
				return Issue{}, fmt.Errorf("model: bad created time %q: %w", f.Value, err)
			}
			i.Created = t.UTC()
		case "updated":
			t, err := time.Parse(time.RFC3339, f.Value)
			if err != nil {
				return Issue{}, fmt.Errorf("model: bad updated time %q: %w", f.Value, err)
			}
			i.Updated = t.UTC()
		case "github":
			n, err := strconv.Atoi(f.Value)
			if err != nil {
				return Issue{}, fmt.Errorf("model: bad github number %q: %w", f.Value, err)
			}
			i.GitHub = n
		}
	}
	if i.ID == "" {
		return Issue{}, fmt.Errorf("model: missing required key \"id\"")
	}
	if !id.Valid(i.ID) {
		return Issue{}, fmt.Errorf("model: bad issue id %q", i.ID)
	}
	if i.Title == "" {
		return Issue{}, fmt.Errorf("model: missing required key \"title\"")
	}
	if i.Status == "" {
		return Issue{}, fmt.Errorf("model: missing required key \"status\"")
	}
	if i.Created.IsZero() {
		return Issue{}, fmt.Errorf("model: missing required key \"created\"")
	}
	if i.Updated.IsZero() {
		return Issue{}, fmt.Errorf("model: missing required key \"updated\"")
	}
	i.Extra = extra
	return i, nil
}

// Fields returns the issue's frontmatter in the order id, title, status, tags, milestone, due, created, updated,
// github, then Extra.
func (i Issue) Fields() []doc.Field {
	fields := []doc.Field{
		{Key: "id", Value: i.ID},
		{Key: "title", Value: i.Title},
		{Key: "status", Value: i.Status},
	}
	if len(i.Tags) > 0 {
		fields = append(fields, doc.Field{Key: "tags", List: i.Tags, IsList: true})
	}
	if i.Milestone != "" {
		fields = append(fields, doc.Field{Key: "milestone", Value: i.Milestone})
	}
	if i.Due != "" {
		fields = append(fields, doc.Field{Key: "due", Value: i.Due})
	}
	for _, f := range []doc.Field{{Key: "by", Value: i.By}, {Key: "persona", Value: i.Persona},
		{Key: "model", Value: i.Model}} {
		if f.Value != "" {
			fields = append(fields, f)
		}
	}
	fields = append(fields,
		doc.Field{Key: "created", Value: i.Created.Format(time.RFC3339Nano)},
		doc.Field{Key: "updated", Value: i.Updated.Format(time.RFC3339Nano)},
	)
	if i.GitHub != 0 {
		fields = append(fields, doc.Field{Key: "github", Value: strconv.Itoa(i.GitHub)})
	}
	fields = append(fields, i.Extra...)
	return fields
}

// commentKnownKeys are the frontmatter keys ParseComment handles.
var commentKnownKeys = map[string]bool{
	"id": true, "kind": true, "by": true, "persona": true, "model": true,
	"at": true, "source": true, "retracted_by": true, "retracted_at": true,
	"retracted_reason": true, "disproved_by": true,
}

// ParseComment builds a Comment from a file's frontmatter and body, with the same errors as ParseIssue. The id must
// pass id.ValidComment, and disproved_by too when present.
func ParseComment(fields []doc.Field, body []byte) (Comment, error) {
	c := Comment{Body: string(body)}
	var extra []doc.Field
	for _, f := range fields {
		if !commentKnownKeys[f.Key] {
			extra = append(extra, f)
			continue
		}
		if err := checkShape(f, false); err != nil {
			return Comment{}, err
		}
		switch f.Key {
		case "id":
			c.ID = f.Value
		case "kind":
			c.Kind = f.Value
		case "by":
			c.By = f.Value
		case "persona":
			c.Persona = f.Value
		case "model":
			c.Model = f.Value
		case "at":
			t, err := time.Parse(time.RFC3339, f.Value)
			if err != nil {
				return Comment{}, fmt.Errorf("model: bad at time %q: %w", f.Value, err)
			}
			c.At = t.UTC()
		case "source":
			c.Source = f.Value
		case "retracted_by":
			c.RetractedBy = f.Value
		case "retracted_at":
			t, err := time.Parse(time.RFC3339, f.Value)
			if err != nil {
				return Comment{}, fmt.Errorf("model: bad retracted_at time %q: %w", f.Value, err)
			}
			c.RetractedAt = t.UTC()
		case "retracted_reason":
			c.RetractedReason = f.Value
		case "disproved_by":
			c.DisprovedBy = f.Value
		}
	}
	if c.ID == "" {
		return Comment{}, fmt.Errorf("model: missing required key \"id\"")
	}
	if !id.ValidComment(c.ID) {
		return Comment{}, fmt.Errorf("model: bad comment id %q", c.ID)
	}
	if c.Kind == "" {
		return Comment{}, fmt.Errorf("model: missing required key \"kind\"")
	}
	if c.By == "" {
		return Comment{}, fmt.Errorf("model: missing required key \"by\"")
	}
	if c.At.IsZero() {
		return Comment{}, fmt.Errorf("model: missing required key \"at\"")
	}
	if c.DisprovedBy != "" && !id.ValidComment(c.DisprovedBy) {
		return Comment{}, fmt.Errorf("model: bad disproved_by id %q", c.DisprovedBy)
	}
	c.Extra = extra
	return c, nil
}

// Fields returns the comment's frontmatter in the order id, kind, by, persona, model, at, source, retracted_by,
// retracted_at, retracted_reason, disproved_by, then Extra.
func (c Comment) Fields() []doc.Field {
	fields := []doc.Field{
		{Key: "id", Value: c.ID},
		{Key: "kind", Value: c.Kind},
		{Key: "by", Value: c.By},
	}
	if c.Persona != "" {
		fields = append(fields, doc.Field{Key: "persona", Value: c.Persona})
	}
	if c.Model != "" {
		fields = append(fields, doc.Field{Key: "model", Value: c.Model})
	}
	fields = append(fields, doc.Field{Key: "at", Value: c.At.Format(time.RFC3339Nano)})
	if c.Source != "" {
		fields = append(fields, doc.Field{Key: "source", Value: c.Source})
	}
	if c.RetractedBy != "" {
		fields = append(fields, doc.Field{Key: "retracted_by", Value: c.RetractedBy})
	}
	if !c.RetractedAt.IsZero() {
		fields = append(fields, doc.Field{Key: "retracted_at", Value: c.RetractedAt.Format(time.RFC3339Nano)})
	}
	if c.RetractedReason != "" {
		fields = append(fields, doc.Field{Key: "retracted_reason", Value: c.RetractedReason})
	}
	if c.DisprovedBy != "" {
		fields = append(fields, doc.Field{Key: "disproved_by", Value: c.DisprovedBy})
	}
	fields = append(fields, c.Extra...)
	return fields
}

// milestoneKnownKeys are the frontmatter keys ParseMilestone handles.
var milestoneKnownKeys = map[string]bool{
	"id": true, "title": true, "status": true, "due": true,
}

// ParseMilestone builds a Milestone from a file's frontmatter and body, with the same errors as ParseIssue.
func ParseMilestone(fields []doc.Field, body []byte) (Milestone, error) {
	m := Milestone{Body: string(body)}
	var extra []doc.Field
	for _, f := range fields {
		if !milestoneKnownKeys[f.Key] {
			extra = append(extra, f)
			continue
		}
		if err := checkShape(f, false); err != nil {
			return Milestone{}, err
		}
		switch f.Key {
		case "id":
			m.ID = f.Value
		case "title":
			m.Title = f.Value
		case "status":
			m.Status = f.Value
		case "due":
			_, err := time.Parse("2006-01-02", f.Value)
			if err != nil {
				return Milestone{}, fmt.Errorf("model: bad due date %q: %w", f.Value, err)
			}
			m.Due = f.Value
		}
	}
	if m.ID == "" {
		return Milestone{}, fmt.Errorf("model: missing required key \"id\"")
	}
	if strings.Contains(m.ID, "/") {
		return Milestone{}, fmt.Errorf("model: milestone id %q must not contain \"/\"", m.ID)
	}
	if m.Title == "" {
		return Milestone{}, fmt.Errorf("model: missing required key \"title\"")
	}
	if m.Status == "" {
		return Milestone{}, fmt.Errorf("model: missing required key \"status\"")
	}
	m.Extra = extra
	return m, nil
}

// Fields returns the milestone's frontmatter in the order id, title, status, due, then Extra.
func (m Milestone) Fields() []doc.Field {
	fields := []doc.Field{
		{Key: "id", Value: m.ID},
		{Key: "title", Value: m.Title},
		{Key: "status", Value: m.Status},
	}
	if m.Due != "" {
		fields = append(fields, doc.Field{Key: "due", Value: m.Due})
	}
	fields = append(fields, m.Extra...)
	return fields
}

// configKnownKeys are the frontmatter keys ParseConfig handles.
var configKnownKeys = map[string]bool{
	"statuses": true, "tag_groups": true, "github_repo": true,
	"embeddings_url": true, "dupe_text": true,
}

// ParseConfig builds a Config from config.yml's frontmatter, taking DefaultConfig's value for each absent key. It
// returns an error for a bad number or a scalar where a list belongs.
func ParseConfig(fields []doc.Field) (Config, error) {
	c := DefaultConfig()
	var extra []doc.Field
	for _, f := range fields {
		if !configKnownKeys[f.Key] {
			extra = append(extra, f)
			continue
		}
		if err := checkShape(f, f.Key == "statuses" || f.Key == "tag_groups"); err != nil {
			return Config{}, err
		}
		switch f.Key {
		case "statuses":
			c.Statuses = f.List
		case "tag_groups":
			c.TagGroups = f.List
		case "github_repo":
			c.GitHubRepo = f.Value
		case "embeddings_url":
			c.EmbeddingsURL = f.Value
		case "dupe_text":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				return Config{}, fmt.Errorf("model: bad dupe_text %q: %w", f.Value, err)
			}
			c.DupeText = v
		}
	}
	c.Extra = extra
	return c, nil
}

// Fields returns the config's frontmatter in the order statuses, tag_groups, github_repo, embeddings_url, dupe_text,
// then Extra. github_repo and embeddings_url are left out when empty. Numbers are written with
// strconv.FormatFloat(f, 'g', -1, 64).
func (c Config) Fields() []doc.Field {
	fields := []doc.Field{
		{Key: "statuses", List: c.Statuses, IsList: true},
		{Key: "tag_groups", List: c.TagGroups, IsList: true},
	}
	if c.GitHubRepo != "" {
		fields = append(fields, doc.Field{Key: "github_repo", Value: c.GitHubRepo})
	}
	if c.EmbeddingsURL != "" {
		fields = append(fields, doc.Field{Key: "embeddings_url", Value: c.EmbeddingsURL})
	}
	fields = append(fields, doc.Field{Key: "dupe_text", Value: strconv.FormatFloat(c.DupeText, 'g', -1, 64)})
	fields = append(fields, c.Extra...)
	return fields
}

// checkShape returns an error when f is a list and list is false, or the reverse.
func checkShape(f doc.Field, list bool) error {
	switch {
	case list && !f.IsList:
		return fmt.Errorf("model: %s must be a list", f.Key)
	case !list && f.IsList:
		return fmt.Errorf("model: %s must be a scalar, not a list", f.Key)
	}
	return nil
}
