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
	// ErrNoComment is a missing comment on an issue that exists. errors.Is(ErrNoComment, ErrNotFound) is true.
	ErrNoComment error = noComment{}
)

type noComment struct{}

func (noComment) Error() string { return "store: no such comment" }

func (noComment) Is(target error) bool { return target == ErrNotFound }

// Store is an open .cairn folder. Dir is its path.
//
// Ids become paths, so every method checks them first. An issue id that fails id.Valid is ErrNotFound for reads and
// removals and an error for saves. A comment id that fails id.ValidComment is ErrNotFound for removals and an error
// for saves.
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
	if !id.Valid(idStr) {
		return model.Issue{}, nil, ErrNotFound
	}
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
	if !id.Valid(i.ID) {
		return fmt.Errorf("store: bad issue id %q", i.ID)
	}
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
	if !id.Valid(issueID) {
		return ErrNotFound
	}
	if !id.ValidComment(c.ID) {
		return fmt.Errorf("store: bad comment id %q", c.ID)
	}
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
		path := filepath.Join(issuesDir, e.Name(), "issue.md")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		fields, body, err := doc.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", path, err)
		}
		issue, err := model.ParseIssue(fields, body)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", path, err)
		}
		if issue.ID != e.Name() {
			return nil, fmt.Errorf("store: issue id %q does not match folder %q", issue.ID, e.Name())
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
	if !id.Valid(issueID) {
		return "", ErrNotFound
	}
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

// RemoveIssue deletes issues/<id> and everything in it. It returns ErrNotFound when the id is not valid or the issue
// does not exist.
func (s *Store) RemoveIssue(issueID string) error {
	if !id.Valid(issueID) {
		return ErrNotFound
	}
	folder := filepath.Join(s.Dir, "issues", issueID)
	if _, err := os.Stat(folder); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return os.RemoveAll(folder)
}

// RemoveComment deletes issues/<issueID>/<commentID>.md and nothing else, without reading it, so a file that does not
// parse can still go. It returns ErrNotFound when the issue id is not valid or the issue does not exist, and
// ErrNoComment when the comment id is not valid or the comment file does not exist.
func (s *Store) RemoveComment(issueID, commentID string) error {
	if !id.Valid(issueID) {
		return ErrNotFound
	}
	folder := filepath.Join(s.Dir, "issues", issueID)
	if _, err := os.Stat(filepath.Join(folder, "issue.md")); err != nil {
		return ErrNotFound
	}
	if !id.ValidComment(commentID) {
		return ErrNoComment
	}
	err := os.Remove(filepath.Join(folder, commentID+".md"))
	if os.IsNotExist(err) {
		return ErrNoComment
	}
	return err
}

// ListMilestones reads every <id>.md file in milestones/, sorted by ID. Folders, names that do not end in ".md", and
// names that start with "." (temporary files included) are ignored. A file that does not parse, or whose id does not
// match its file name, is an error. A missing milestones folder gives an empty list.
func (s *Store) ListMilestones() ([]model.Milestone, error) {
	dir := filepath.Join(s.Dir, "milestones")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var milestones []model.Milestone
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		idStr := strings.TrimSuffix(name, ".md")
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		fields, body, err := doc.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", name, err)
		}
		m, err := model.ParseMilestone(fields, body)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", name, err)
		}
		if m.ID != idStr {
			return nil, fmt.Errorf("store: milestone id %q does not match file name %q", m.ID, name)
		}
		milestones = append(milestones, m)
	}
	sort.Slice(milestones, func(a, b int) bool {
		return milestones[a].ID < milestones[b].ID
	})
	return milestones, nil
}

// validMilestoneID reports whether id can name a milestone file: non-empty, no "/", not starting with ".".
func validMilestoneID(idStr string) bool {
	return idStr != "" && !strings.Contains(idStr, "/") && !strings.HasPrefix(idStr, ".")
}

// LoadMilestone reads milestones/<id>.md. It returns ErrNotFound when the file is missing or the id cannot name one
// (empty, with a "/", or starting with "."), and an error when the file does not parse or its id does not match its
// file name.
func (s *Store) LoadMilestone(idStr string) (model.Milestone, error) {
	if !validMilestoneID(idStr) {
		return model.Milestone{}, ErrNotFound
	}
	path := filepath.Join(s.Dir, "milestones", idStr+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Milestone{}, ErrNotFound
		}
		return model.Milestone{}, err
	}
	fields, body, err := doc.Parse(data)
	if err != nil {
		return model.Milestone{}, fmt.Errorf("store: %s: %w", idStr+".md", err)
	}
	m, err := model.ParseMilestone(fields, body)
	if err != nil {
		return model.Milestone{}, fmt.Errorf("store: %s: %w", idStr+".md", err)
	}
	if m.ID != idStr {
		return model.Milestone{}, fmt.Errorf("store: milestone id %q does not match file name %q", m.ID, idStr+".md")
	}
	return m, nil
}

// SaveMilestone writes milestones/<m.ID>.md, making the folder when needed. An id that is empty, has a "/" or starts
// with "." is an error.
func (s *Store) SaveMilestone(m model.Milestone) error {
	if !validMilestoneID(m.ID) {
		return fmt.Errorf("store: bad milestone id %q", m.ID)
	}
	dir := filepath.Join(s.Dir, "milestones")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := doc.Format(m.Fields(), []byte(m.Body))
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(dir, m.ID+".md"), data, 0o644)
}
