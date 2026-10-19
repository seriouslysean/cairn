package cli

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/seriouslysean/cairn/internal/model"
	"github.com/seriouslysean/cairn/internal/store"
	"github.com/seriouslysean/cairn/internal/words"
)

func runLs(c *call) (output, error) {
	s, err := store.Find(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	issues, err := s.ListIssues()
	if err != nil {
		return output{}, err
	}
	dueBefore, byDue := c.given(flagDueBefore)
	if byDue {
		if err := checkDate(dueBefore); err != nil {
			return output{}, err
		}
	}
	rows := make([]branchVersion, len(issues))
	for n, i := range issues {
		rows[n] = branchVersion{issue: i}
	}
	withBranches := c.switches[flagBranches]
	if withBranches {
		versions, err := c.branchIssues(s, "")
		if err != nil {
			return output{}, err
		}
		rows = append(rows, versions...)
	}
	status, milestone, tags := c.value(flagStatus), c.value(flagMilestone), c.values[flagTag]
	rows = slices.DeleteFunc(rows, func(v branchVersion) bool {
		i := v.issue
		return status != "" && i.Status != status ||
			milestone != "" && i.Milestone != milestone ||
			slices.ContainsFunc(tags, func(t string) bool { return !slices.Contains(i.Tags, t) }) ||
			byDue && (i.Due == "" || i.Due >= dueBefore)
	})
	data := make([]lsJSON, len(rows))
	var text strings.Builder
	for n, v := range rows {
		data[n] = lsJSON{issueSummaryJSON: issueSummary(v.issue)}
		if withBranches {
			data[n].Branch = &v.branch
		}
		fmt.Fprintf(&text, "%s\t%s\t%s", v.issue.ID, v.issue.Status, v.issue.Title)
		if v.branch != "" {
			fmt.Fprintf(&text, "\t%s", v.branch)
		}
		text.WriteString("\n")
	}
	return output{data, text.String()}, nil
}

func runTags(c *call) (output, error) {
	s, err := store.Find(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	issues, err := s.ListIssues()
	if err != nil {
		return output{}, err
	}
	counts := map[string]int{}
	for _, i := range issues {
		for _, t := range slices.Compact(slices.Sorted(slices.Values(i.Tags))) {
			counts[t]++
		}
	}
	data := make([]tagJSON, 0, len(counts))
	var text strings.Builder
	for _, t := range slices.Sorted(maps.Keys(counts)) {
		data = append(data, tagJSON{Tag: t, Count: counts[t]})
		fmt.Fprintf(&text, "%s\t%d\n", t, counts[t])
	}
	return output{data, text.String()}, nil
}

func runSearch(c *call) (output, error) {
	query := strings.Join(c.args, " ")
	if len(words.Split(query)) == 0 {
		return output{}, usageError("search query has no words")
	}
	s, err := store.Find(c.env.Dir)
	if err != nil {
		return output{}, err
	}
	issues, err := s.ListIssues()
	if err != nil {
		return output{}, err
	}
	data := []searchJSON{}
	for _, i := range issues {
		comments, err := s.LoadComments(i.ID)
		if err != nil {
			return output{}, err
		}
		if n := words.Shared(query, searchText(i, comments)); n > 0 {
			data = append(data, searchJSON{issueSummary(i), n})
		}
	}
	slices.SortStableFunc(data, func(a, b searchJSON) int { return cmp.Compare(b.Matches, a.Matches) })
	var text strings.Builder
	for _, hit := range data {
		fmt.Fprintf(&text, "%s\t%d\t%s\n", hit.ID, hit.Matches, hit.Title)
	}
	return output{data, text.String()}, nil
}

// searchText is the text search looks in: the issue's title and body, and the bodies and sources of its active
// comments.
func searchText(i model.Issue, comments []model.Comment) string {
	parts := []string{i.Title, i.Body}
	for _, cm := range comments {
		if !cm.Retracted() {
			parts = append(parts, cm.Body, cm.Source)
		}
	}
	return strings.Join(parts, " ")
}
