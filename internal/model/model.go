// Package model holds Cairn's issues, comments, milestones and config, and converts each to and from the
// frontmatter fields of its file.
//
// Parsing checks the shape of each value (ids, dates, times, numbers, lists) but not statuses, tags or kinds against
// the config: a config change must never stop an old file from loading. Keys the parser does not know are kept, in
// order, in Extra (nil when there are none), and written back after the known keys, so a file from a newer Cairn
// survives an edit.
//
// Dates are "2006-01-02". Times are RFC 3339: they parse with or without fractional seconds and with any offset,
// are converted to UTC, and are written with time.RFC3339Nano, so whole seconds print without a fraction.
package model

import (
	"time"

	"github.com/seriouslysean/cairn/internal/doc"
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
	DupeMeaning   float64
	Extra         []doc.Field
}

// DefaultConfig is the config used for absent keys: statuses [backlog, open, in-progress, done], tag groups
// [severity, effort, area], dupe_text 0.6 and dupe_meaning 0.85, with GitHub sync and embeddings off.
func DefaultConfig() Config {
	panic("todo: step 2")
}

// ParseIssue builds an Issue from a file's frontmatter and body. It returns an error for a missing required key, a
// bad id, date, time or number, or a scalar where a list belongs (or the reverse).
func ParseIssue(fields []doc.Field, body []byte) (Issue, error) {
	panic("todo: step 2")
}

// Fields returns the issue's frontmatter in the order id, title, status, tags, milestone, due, created, updated,
// github, then Extra.
func (i Issue) Fields() []doc.Field {
	panic("todo: step 2")
}

// ParseComment builds a Comment from a file's frontmatter and body, with the same errors as ParseIssue. The id must
// pass id.ValidComment, and disproved_by too when present.
func ParseComment(fields []doc.Field, body []byte) (Comment, error) {
	panic("todo: step 2")
}

// Fields returns the comment's frontmatter in the order id, kind, by, persona, model, at, source, retracted_by,
// retracted_at, retracted_reason, disproved_by, then Extra.
func (c Comment) Fields() []doc.Field {
	panic("todo: step 2")
}

// ParseMilestone builds a Milestone from a file's frontmatter and body, with the same errors as ParseIssue.
func ParseMilestone(fields []doc.Field, body []byte) (Milestone, error) {
	panic("todo: step 2")
}

// Fields returns the milestone's frontmatter in the order id, title, status, due, then Extra.
func (m Milestone) Fields() []doc.Field {
	panic("todo: step 2")
}

// ParseConfig builds a Config from config.yml's frontmatter, taking DefaultConfig's value for each absent key. It
// returns an error for a bad number or a scalar where a list belongs.
func ParseConfig(fields []doc.Field) (Config, error) {
	panic("todo: step 2")
}

// Fields returns the config's frontmatter in the order statuses, tag_groups, github_repo, embeddings_url, dupe_text,
// dupe_meaning, then Extra. github_repo and embeddings_url are left out when empty. Numbers are written with
// strconv.FormatFloat(f, 'g', -1, 64).
func (c Config) Fields() []doc.Field {
	panic("todo: step 2")
}
