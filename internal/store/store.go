package store

import (
	"errors"
	"io"

	"github.com/seriouslysean/cairn/internal/model"
)

// Errors that commands map to exit codes.
var (
	ErrNoTracker = errors.New("store: no .cairn folder here or above")
	ErrExists    = errors.New("store: .cairn folder already exists")
	ErrNotFound  = errors.New("store: no such issue")
)

// Store is an open .cairn folder. Dir is its path.
type Store struct {
	Dir string
}

// Find walks up from start, which must be an absolute path, and opens the first ".cairn" folder it finds. It
// returns ErrNoTracker when it reaches the root without finding one; a ".cairn" that is a file does not count.
func Find(start string) (*Store, error) {
	panic("todo: step 2")
}

// Init makes dir/.cairn with config.yml (DefaultConfig's fields, empty body), an issues and a milestones folder, and
// a .gitignore holding ".index/\n". It returns ErrExists when dir/.cairn already exists and changes nothing.
func Init(dir string) (*Store, error) {
	panic("todo: step 2")
}

// Config reads config.yml, or returns model.DefaultConfig when the file is missing.
func (s *Store) Config() (model.Config, error) {
	panic("todo: step 2")
}

// LoadIssue reads issues/<id>/issue.md and every c-*.md file beside it. Comments are sorted by At, then by ID. Other
// files in the folder, temporary ones included, are ignored. It returns ErrNotFound when the folder or issue.md is
// missing, and an error when a file's id does not match its folder or file name.
func (s *Store) LoadIssue(id string) (model.Issue, []model.Comment, error) {
	panic("todo: step 2")
}

// SaveIssue writes issues/<i.ID>/issue.md, making the folder when needed. It leaves comment files alone.
func (s *Store) SaveIssue(i model.Issue) error {
	panic("todo: step 2")
}

// SaveComment writes issues/<issueID>/<c.ID>.md. It returns ErrNotFound when the issue does not exist.
func (s *Store) SaveComment(issueID string, c model.Comment) error {
	panic("todo: step 2")
}

// ListIssues reads issue.md from every issue folder, sorted by Created, then by ID, without reading comments. Entries
// in issues/ whose name is not a valid id are ignored. A missing issues folder gives an empty list.
func (s *Store) ListIssues() ([]model.Issue, error) {
	panic("todo: step 2")
}

// NewIssueID returns an unused issue id. Its length is id.Length of the number of folders in issues/ named with a
// valid id, whether or not they hold issue.md. r is passed to id.New.
func (s *Store) NewIssueID(r io.Reader) (string, error) {
	panic("todo: step 2")
}

// NewCommentID returns "c-" plus 4 characters from id.New, unused among the issue's comments. It returns ErrNotFound
// when the issue does not exist.
func (s *Store) NewCommentID(issueID string, r io.Reader) (string, error) {
	panic("todo: step 2")
}
