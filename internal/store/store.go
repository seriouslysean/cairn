// Package store reads and writes the .cairn folder.
package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/seriouslysean/cairn/internal/doc"
	"github.com/seriouslysean/cairn/internal/id"
	"github.com/seriouslysean/cairn/internal/model"
)

// Errors that commands map to exit codes.
var (
	ErrNoTracker = errors.New("store: no .cairn folder here or above")
	ErrExists    = errors.New("store: .cairn folder already exists")
	ErrNotFound  = errors.New("store: no such issue")
)

// Store is an open .cairn folder. Dir is its path.
//
// Ids become paths, so every method checks them first: an issue id that fails id.Valid is ErrNotFound for reads
// and an error for writes, and a comment id that fails id.ValidComment is an error.
type Store struct {
	Dir string
}

// Find walks up from start, which must be an absolute path, and opens the first ".cairn" folder it finds. It
// returns ErrNoTracker when it reaches the root without finding one; a ".cairn" that is a file does not count.
func Find(start string) (*Store, error) {
	dir := start
	for {
		candidate := filepath.Join(dir, ".cairn")
		fi, err := os.Stat(candidate)
		if err == nil && fi.IsDir() {
			return &Store{Dir: candidate}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNoTracker
		}
		dir = parent
	}
}

// Init makes dir/.cairn with config.yml (DefaultConfig's fields, empty body), an issues and a milestones folder, and
// a .gitignore holding ".index/\n". It returns ErrExists when dir/.cairn already exists and changes nothing.
func Init(dir string) (*Store, error) {
	cairnDir := filepath.Join(dir, ".cairn")
	if _, err := os.Stat(cairnDir); err == nil {
		return nil, ErrExists
	}
	if err := os.MkdirAll(filepath.Join(cairnDir, "issues"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(cairnDir, "milestones"), 0o755); err != nil {
		return nil, err
	}
	cfgData, err := doc.Format(model.DefaultConfig().Fields(), nil)
	if err != nil {
		return nil, err
	}
	if err := WriteFileAtomic(filepath.Join(cairnDir, "config.yml"), cfgData, 0o644); err != nil {
		return nil, err
	}
	if err := WriteFileAtomic(filepath.Join(cairnDir, ".gitignore"), []byte(".index/\n"), 0o644); err != nil {
		return nil, err
	}
	return &Store{Dir: cairnDir}, nil
}

// Config reads config.yml, or returns model.DefaultConfig when the file is missing.
func (s *Store) Config() (model.Config, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, "config.yml"))
	if err != nil {
		if os.IsNotExist(err) {
			return model.DefaultConfig(), nil
		}
		return model.Config{}, err
	}
	fields, _, err := doc.Parse(data)
	if err != nil {
		return model.Config{}, err
	}
	return model.ParseConfig(fields)
}

// LoadIssue reads issues/<id>/issue.md and every c-*.md file beside it. Comments are sorted by At, then by ID. Other
// files in the folder, temporary ones included, are ignored. It returns ErrNotFound when the folder or issue.md is
// missing, and an error when a file's id does not match its folder or file name.
func (s *Store) LoadIssue(idStr string) (model.Issue, []model.Comment, error) {
	folder := filepath.Join(s.Dir, "issues", idStr)
	entries, err := os.ReadDir(folder)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Issue{}, nil, ErrNotFound
		}
		return model.Issue{}, nil, err
	}
	issueData, err := os.ReadFile(filepath.Join(folder, "issue.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return model.Issue{}, nil, ErrNotFound
		}
		return model.Issue{}, nil, err
	}
	fields, body, err := doc.Parse(issueData)
	if err != nil {
		return model.Issue{}, nil, err
	}
	issue, err := model.ParseIssue(fields, body)
	if err != nil {
		return model.Issue{}, nil, err
	}
	if issue.ID != idStr {
		return model.Issue{}, nil, fmt.Errorf("store: issue id %q does not match folder %q", issue.ID, idStr)
	}
	var comments []model.Comment
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "c-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		commentID := strings.TrimSuffix(name, ".md")
		if !id.ValidComment(commentID) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(folder, name))
		if err != nil {
			return model.Issue{}, nil, err
		}
		cFields, cBody, err := doc.Parse(data)
		if err != nil {
			return model.Issue{}, nil, err
		}
		c, err := model.ParseComment(cFields, cBody)
		if err != nil {
			return model.Issue{}, nil, err
		}
		if c.ID != commentID {
			return model.Issue{}, nil, fmt.Errorf("store: comment id %q does not match file %q", c.ID, name)
		}
		comments = append(comments, c)
	}
	sort.Slice(comments, func(a, b int) bool {
		if comments[a].At.Equal(comments[b].At) {
			return comments[a].ID < comments[b].ID
		}
		return comments[a].At.Before(comments[b].At)
	})
	return issue, comments, nil
}

// SaveIssue writes issues/<i.ID>/issue.md, making the folder when needed. It leaves comment files alone.
func (s *Store) SaveIssue(i model.Issue) error {
	folder := filepath.Join(s.Dir, "issues", i.ID)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	data, err := doc.Format(i.Fields(), []byte(i.Body))
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(folder, "issue.md"), data, 0o644)
}

// SaveComment writes issues/<issueID>/<c.ID>.md. It returns ErrNotFound when the issue does not exist.
func (s *Store) SaveComment(issueID string, c model.Comment) error {
	folder := filepath.Join(s.Dir, "issues", issueID)
	if _, err := os.Stat(filepath.Join(folder, "issue.md")); err != nil {
		return ErrNotFound
	}
	data, err := doc.Format(c.Fields(), []byte(c.Body))
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(folder, c.ID+".md"), data, 0o644)
}

// ListIssues reads issue.md from every issue folder, sorted by Created, then by ID, without reading comments. Entries
// in issues/ whose name is not a valid id, and folders without issue.md, are ignored. A file that does not parse, or
// whose id does not match its folder, is an error. A missing issues folder gives an empty list.
func (s *Store) ListIssues() ([]model.Issue, error) {
	issuesDir := filepath.Join(s.Dir, "issues")
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var issues []model.Issue
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !id.Valid(e.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(issuesDir, e.Name(), "issue.md"))
		if err != nil {
			continue
		}
		fields, body, err := doc.Parse(data)
		if err != nil {
			continue
		}
		issue, err := model.ParseIssue(fields, body)
		if err != nil {
			continue
		}
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(a, b int) bool {
		if issues[a].Created.Equal(issues[b].Created) {
			return issues[a].ID < issues[b].ID
		}
		return issues[a].Created.Before(issues[b].Created)
	})
	return issues, nil
}

// NewIssueID returns an unused issue id. Its length is id.Length of the number of folders in issues/ named with a
// valid id, whether or not they hold issue.md. r is passed to id.New.
func (s *Store) NewIssueID(r io.Reader) (string, error) {
	issuesDir := filepath.Join(s.Dir, "issues")
	entries, err := os.ReadDir(issuesDir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	count := 0
	taken := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() && id.Valid(e.Name()) {
			count++
			taken[e.Name()] = true
		}
	}
	n := id.Length(count)
	return id.New(n, taken, r)
}

// NewCommentID returns "c-" plus 4 characters from id.New, unused among the issue's comments. It returns ErrNotFound
// when the issue does not exist.
func (s *Store) NewCommentID(issueID string, r io.Reader) (string, error) {
	folder := filepath.Join(s.Dir, "issues", issueID)
	if _, err := os.Stat(filepath.Join(folder, "issue.md")); err != nil {
		return "", ErrNotFound
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			name := e.Name()
			if strings.HasPrefix(name, "c-") && strings.HasSuffix(name, ".md") {
				commentID := strings.TrimSuffix(name, ".md")
				if id.ValidComment(commentID) {
					taken[commentID[2:]] = true
				}
			}
		}
	}
	s2, err := id.New(4, taken, r)
	if err != nil {
		return "", err
	}
	return "c-" + s2, nil
}
