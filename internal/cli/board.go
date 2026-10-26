package cli

import (
	"path/filepath"

	"github.com/seriouslysean/cairn/internal/board"
	"github.com/seriouslysean/cairn/internal/store"
)

// pagePerm is the mode of the page board --html writes, the mode of the tracker's own files.
const pagePerm = 0o644

func runBoard(c *call) (output, error) {
	file, toFile := c.given(flagHTML)
	if toFile && file == "" {
		return output{}, usageError("--html needs a file name")
	}
	s, err := c.openStore()
	if err != nil {
		return output{}, err
	}
	cfg, err := s.Config()
	if err != nil {
		return output{}, err
	}
	issues, err := s.ListIssues()
	if err != nil {
		return output{}, err
	}
	b := board.New(cfg, issues)
	if !toFile {
		return output{boardData(b), b.Text()}, nil
	}
	// The tracker's name is the name of the folder that holds the .cairn folder.
	page, err := b.HTML(filepath.Base(filepath.Dir(s.Dir)), c.env.stamp())
	if err != nil {
		return output{}, err
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(c.env.Dir, file)
	}
	if err := store.WriteFileAtomic(file, page, pagePerm); err != nil {
		return output{}, err
	}
	return output{boardFileJSON{File: file, Issues: b.Total()}, "wrote " + file + "\n"}, nil
}

// boardData is board's data: each column with its count, and its issues unless it is collapsed.
func boardData(b board.Board) boardJSON {
	data := boardJSON{Columns: make([]boardColumnJSON, len(b.Columns))}
	for n, col := range b.Columns {
		issues := []issueSummaryJSON{}
		if !col.Collapsed {
			for _, i := range col.Issues {
				issues = append(issues, issueSummary(i))
			}
		}
		data.Columns[n] = boardColumnJSON{Status: col.Status, Collapsed: col.Collapsed, Count: len(col.Issues), Issues: issues}
	}
	return data
}
