package store

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seriouslysean/cairn/internal/doc"
	"github.com/seriouslysean/cairn/internal/id"
	"github.com/seriouslysean/cairn/internal/model"
)

// DirName is the tracker's folder.
const DirName = ".cairn"

// ConfigFile is the config's file in the tracker's folder.
const ConfigFile = "config.yml"

// IssuesDir is the folder in the tracker's folder that holds one folder per issue.
const IssuesDir = "issues"

// The other names inside the tracker's folder.
const (
	milestonesDir = "milestones"
	issueFile     = "issue.md"
	ignoreFile    = ".gitignore"
	ignored       = ".index/\n"
	docExt        = ".md" // issue, comment and milestone files
	hiddenPrefix  = "."   // starts the names ListMilestones skips, temporary files among them
)

const (
	dirPerm  fs.FileMode = 0o755
	filePerm fs.FileMode = 0o644
)

// Errors that commands map to exit codes.
var (
	ErrNoTracker = errors.New("store: no " + DirName + " folder here or above")
	ErrExists    = errors.New("store: " + DirName + " folder already exists")
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
		candidate := filepath.Join(dir, DirName)
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
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
	s := &Store{Dir: filepath.Join(dir, DirName)}
	if _, err := os.Stat(s.Dir); err == nil {
		return nil, ErrExists
	}
	for _, sub := range []string{IssuesDir, milestonesDir} {
		if err := os.MkdirAll(filepath.Join(s.Dir, sub), dirPerm); err != nil {
			return nil, err
		}
	}
	if err := writeDoc(filepath.Join(s.Dir, ConfigFile), model.DefaultConfig().Fields(), ""); err != nil {
		return nil, err
	}
	if err := WriteFileAtomic(filepath.Join(s.Dir, ignoreFile), []byte(ignored), filePerm); err != nil {
		return nil, err
	}
	return s, nil
}

// Config reads config.yml, or returns model.DefaultConfig when the file is missing.
func (s *Store) Config() (model.Config, error) {
	fields, _, err := readDoc(filepath.Join(s.Dir, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return model.DefaultConfig(), nil
	}
	if err != nil {
		return model.Config{}, err
	}
	c, err := model.ParseConfig(fields)
	if err != nil {
		return model.Config{}, fileError(filepath.Join(s.Dir, ConfigFile), err)
	}
	return c, nil
}

// LoadIssue reads issues/<id> on disk, as ReadIssue reads a folder. A missing folder is ErrNotFound too.
func (s *Store) LoadIssue(issueID string) (model.Issue, []model.Comment, error) {
	if !id.Valid(issueID) {
		return model.Issue{}, nil, ErrNotFound
	}
	dir := s.issueDir(issueID)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return model.Issue{}, nil, ErrNotFound
	}
	if err != nil {
		return model.Issue{}, nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return ReadIssue(issueID, diskFolder(dir, names))
}

// Folder is an issue folder, wherever its files are: on disk, or in a git commit.
type Folder struct {
	Path  string                            // the folder, as errors name it
	Names []string                          // the files in it, not its subfolders
	Read  func(name string) ([]byte, error) // a missing file is fs.ErrNotExist
}

// diskFolder is the folder dir on disk, holding the files names.
func diskFolder(dir string, names []string) Folder {
	return Folder{Path: dir, Names: names, Read: func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, name))
	}}
}

// path is the file name in f, as errors name it.
func (f Folder) path(name string) string {
	return filepath.Join(f.Path, name)
}

// parse reads and parses the file name in f. A read error is f.Read's; a parse error is a fileError.
func (f Folder) parse(name string) ([]doc.Field, []byte, error) {
	data, err := f.Read(name)
	if err != nil {
		return nil, nil, err
	}
	return parseDoc(f.path(name), data)
}

// ReadIssue reads the issue issueID from its folder f: issue.md and every c-*.md file beside it. Comments are sorted
// by At, then by ID. Other files, temporary ones included, are ignored. It returns ErrNotFound when the id is not
// valid or issue.md is missing, and an error when a file does not parse or its id does not match its folder or file
// name.
func ReadIssue(issueID string, f Folder) (model.Issue, []model.Comment, error) {
	if !id.Valid(issueID) {
		return model.Issue{}, nil, ErrNotFound
	}
	issue, err := readIssue(issueID, f)
	if err != nil {
		return model.Issue{}, nil, err
	}
	var comments []model.Comment
	for _, name := range f.Names {
		commentID, ok := commentFileID(name)
		if !ok {
			continue
		}
		c, err := readComment(commentID, f)
		if err != nil {
			return model.Issue{}, nil, err
		}
		comments = append(comments, c)
	}
	slices.SortFunc(comments, func(a, b model.Comment) int {
		return cmp.Or(a.At.Compare(b.At), strings.Compare(a.ID, b.ID))
	})
	return issue, comments, nil
}

// SaveIssue writes issues/<i.ID>/issue.md, making the folder when needed. It leaves comment files alone.
func (s *Store) SaveIssue(i model.Issue) error {
	if !id.Valid(i.ID) {
		return fmt.Errorf("store: bad issue id %q", i.ID)
	}
	dir := s.issueDir(i.ID)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	return writeDoc(filepath.Join(dir, issueFile), i.Fields(), i.Body)
}

// SaveComment writes issues/<issueID>/<c.ID>.md. It returns ErrNotFound when the issue does not exist.
func (s *Store) SaveComment(issueID string, c model.Comment) error {
	if !s.hasIssue(issueID) {
		return ErrNotFound
	}
	if !id.ValidComment(c.ID) {
		return fmt.Errorf("store: bad comment id %q", c.ID)
	}
	return writeDoc(filepath.Join(s.issueDir(issueID), c.ID+docExt), c.Fields(), c.Body)
}

// ListIssues reads issue.md from every issue folder, sorted by Created, then by ID, without reading comments. Entries
// in issues/ whose name is not a valid id, and folders without issue.md, are ignored. A file that does not parse, or
// whose id does not match its folder, is an error. A missing issues folder gives an empty list.
func (s *Store) ListIssues() ([]model.Issue, error) {
	issueIDs, err := s.issueIDs()
	if err != nil {
		return nil, err
	}
	var issues []model.Issue
	for _, issueID := range issueIDs {
		// Only issue.md is read, so the folder's names are not needed.
		issue, err := readIssue(issueID, diskFolder(s.issueDir(issueID), nil))
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	slices.SortFunc(issues, CompareIssues)
	return issues, nil
}

// CompareIssues orders issues as the store lists them: by Created, then by ID.
func CompareIssues(a, b model.Issue) int {
	return cmp.Or(a.Created.Compare(b.Created), strings.Compare(a.ID, b.ID))
}

// NewIssueID returns an unused issue id. Its length is id.Length of the number of folders in issues/ named with a
// valid id, whether or not they hold issue.md. r is passed to id.New.
func (s *Store) NewIssueID(r io.Reader) (string, error) {
	issueIDs, err := s.issueIDs()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, issueID := range issueIDs {
		taken[issueID] = true
	}
	return id.New(id.Length(len(taken)), taken, r)
}

// NewCommentID returns a comment id from id.NewComment, unused among the issue's comments. It returns ErrNotFound
// when the issue does not exist.
func (s *Store) NewCommentID(issueID string, r io.Reader) (string, error) {
	if !s.hasIssue(issueID) {
		return "", ErrNotFound
	}
	entries, err := os.ReadDir(s.issueDir(issueID))
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, e := range entries {
		if commentID, ok := commentFileID(e.Name()); ok && !e.IsDir() {
			taken[commentID] = true
		}
	}
	return id.NewComment(taken, r)
}

// RemoveIssue deletes issues/<id> and everything in it. It returns ErrNotFound when the id is not valid or the issue
// does not exist.
func (s *Store) RemoveIssue(issueID string) error {
	if !id.Valid(issueID) {
		return ErrNotFound
	}
	dir := s.issueDir(issueID)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return os.RemoveAll(dir)
}

// RemoveComment deletes issues/<issueID>/<commentID>.md and nothing else, without reading it, so a file that does not
// parse can still go. It returns ErrNotFound when the issue id is not valid or the issue does not exist, and
// ErrNoComment when the comment id is not valid or the comment file does not exist.
func (s *Store) RemoveComment(issueID, commentID string) error {
	if !s.hasIssue(issueID) {
		return ErrNotFound
	}
	if !id.ValidComment(commentID) {
		return ErrNoComment
	}
	err := os.Remove(filepath.Join(s.issueDir(issueID), commentID+docExt))
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNoComment
	}
	return err
}

// ListMilestones reads every <id>.md file in milestones/, sorted by ID. Folders, names that do not end in ".md", and
// names that start with "." (temporary files included) are ignored. A file that does not parse, or whose id does not
// match its file name, is an error. A missing milestones folder gives an empty list.
func (s *Store) ListMilestones() ([]model.Milestone, error) {
	entries, err := os.ReadDir(filepath.Join(s.Dir, milestonesDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var milestones []model.Milestone
	for _, e := range entries {
		milestoneID, ok := strings.CutSuffix(e.Name(), docExt)
		if e.IsDir() || !ok || !ValidMilestoneID(milestoneID) {
			continue
		}
		m, err := s.LoadMilestone(milestoneID)
		if err != nil {
			return nil, err
		}
		milestones = append(milestones, m)
	}
	slices.SortFunc(milestones, func(a, b model.Milestone) int { return strings.Compare(a.ID, b.ID) })
	return milestones, nil
}

// ValidMilestoneID reports whether id can name a milestone file: it is not empty, has no "/" and does not start with
// ".", which ListMilestones would skip.
func ValidMilestoneID(milestoneID string) bool {
	return milestoneID != "" && model.ValidMilestoneName(milestoneID) && !strings.HasPrefix(milestoneID, hiddenPrefix)
}

// LoadMilestone reads milestones/<id>.md. It returns ErrNotFound when the file is missing or the id cannot name one
// (empty, with a "/", or starting with "."), and an error when the file does not parse or its id does not match its
// file name.
func (s *Store) LoadMilestone(milestoneID string) (model.Milestone, error) {
	if !ValidMilestoneID(milestoneID) {
		return model.Milestone{}, ErrNotFound
	}
	path := s.milestonePath(milestoneID)
	fields, body, err := readDoc(path)
	if errors.Is(err, fs.ErrNotExist) {
		return model.Milestone{}, ErrNotFound
	}
	if err != nil {
		return model.Milestone{}, err
	}
	m, err := model.ParseMilestone(fields, body)
	if err != nil {
		return model.Milestone{}, fileError(path, err)
	}
	if m.ID != milestoneID {
		return model.Milestone{}, fileError(path, fmt.Errorf("milestone id %q does not match the file name", m.ID))
	}
	return m, nil
}

// SaveMilestone writes milestones/<m.ID>.md, making the folder when needed. An id that is empty, has a "/" or starts
// with "." is an error.
func (s *Store) SaveMilestone(m model.Milestone) error {
	if !ValidMilestoneID(m.ID) {
		return fmt.Errorf("store: bad milestone id %q", m.ID)
	}
	if err := os.MkdirAll(filepath.Join(s.Dir, milestonesDir), dirPerm); err != nil {
		return err
	}
	return writeDoc(s.milestonePath(m.ID), m.Fields(), m.Body)
}

func (s *Store) issueDir(issueID string) string {
	return filepath.Join(s.Dir, IssuesDir, issueID)
}

func (s *Store) milestonePath(milestoneID string) string {
	return filepath.Join(s.Dir, milestonesDir, milestoneID+docExt)
}

// hasIssue reports whether issueID is valid and its folder holds issue.md.
func (s *Store) hasIssue(issueID string) bool {
	if !id.Valid(issueID) {
		return false
	}
	_, err := os.Stat(filepath.Join(s.issueDir(issueID), issueFile))
	return err == nil
}

// issueIDs returns the names of the folders in issues/ that are valid issue ids, whether or not they hold issue.md.
// A missing issues folder holds none.
func (s *Store) issueIDs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.Dir, IssuesDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() && id.Valid(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

// commentFileID returns the comment id that a file in an issue folder is named for, if it is a comment file.
func commentFileID(name string) (string, bool) {
	commentID, ok := strings.CutSuffix(name, docExt)
	return commentID, ok && id.ValidComment(commentID)
}

// readIssue reads issue.md in f for the issue issueID. It returns ErrNotFound when the file is missing.
func readIssue(issueID string, f Folder) (model.Issue, error) {
	fields, body, err := f.parse(issueFile)
	if errors.Is(err, fs.ErrNotExist) {
		return model.Issue{}, ErrNotFound
	}
	if err != nil {
		return model.Issue{}, err
	}
	issue, err := model.ParseIssue(fields, body)
	if err != nil {
		return model.Issue{}, fileError(f.path(issueFile), err)
	}
	if issue.ID != issueID {
		err := fmt.Errorf("issue id %q does not match the folder", issue.ID)
		return model.Issue{}, fileError(f.path(issueFile), err)
	}
	return issue, nil
}

// readComment reads <commentID>.md in f.
func readComment(commentID string, f Folder) (model.Comment, error) {
	name := commentID + docExt
	fields, body, err := f.parse(name)
	if err != nil {
		return model.Comment{}, err
	}
	c, err := model.ParseComment(fields, body)
	if err != nil {
		return model.Comment{}, fileError(f.path(name), err)
	}
	if c.ID != commentID {
		err := fmt.Errorf("comment id %q does not match the file name", c.ID)
		return model.Comment{}, fileError(f.path(name), err)
	}
	return c, nil
}

// readDoc reads and parses one file on disk. A read error is os.ReadFile's, which names the file.
func readDoc(path string) ([]doc.Field, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return parseDoc(path, data)
}

// parseDoc parses data, the contents of the file at path. A parse error is a fileError.
func parseDoc(path string, data []byte) ([]doc.Field, []byte, error) {
	fields, body, err := doc.Parse(data)
	if err != nil {
		return nil, nil, fileError(path, err)
	}
	return fields, body, nil
}

// fileError is err about the file at path, named in its message.
func fileError(path string, err error) error {
	return fmt.Errorf("store: %s: %w", path, err)
}

// writeDoc formats fields and body and writes them to path through WriteFileAtomic.
func writeDoc(path string, fields []doc.Field, body string) error {
	data, err := doc.Format(fields, []byte(body))
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data, filePerm)
}
