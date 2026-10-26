// Package board lays a tracker's issues out as a board: one column per status.
package board

import (
	"time"

	"github.com/seriouslysean/cairn/internal/model"
)

// Column is the issues of one status.
type Column struct {
	Status    string
	Collapsed bool
	Issues    []model.Issue
}

// Board is a tracker's issues in columns.
type Board struct {
	Columns []Column
}

// New lays issues out in columns.
func New(cfg model.Config, issues []model.Issue) Board {
	panic("todo")
}

// Text is the board for a terminal.
func (b Board) Text() string {
	panic("todo")
}

// HTML is the board as one self-contained page.
func (b Board) HTML(tracker string, at time.Time) ([]byte, error) {
	panic("todo")
}
