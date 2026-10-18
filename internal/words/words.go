// Package words scores how alike two texts are by the words they share.
package words

import (
	"strings"
	"unicode"
)

// Split returns the distinct words of s in the order they first appear. A word is a run of letters and digits
// (unicode.IsLetter or unicode.IsDigit), lowercased; everything else separates words.
func Split(s string) []string {
	var result []string
	seen := map[string]bool{}
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		w := strings.ToLower(cur.String())
		cur.Reset()
		if !seen[w] {
			seen[w] = true
			result = append(result, w)
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return result
}

// Overlap is the word-overlap score of a and b: the number of distinct words in both, divided as a float64 by the
// number of distinct words in either (Jaccard similarity), with words as Split gives them. It is 0 when neither has
// a word and 1 when both have the same words.
func Overlap(a, b string) float64 {
	wa := Split(a)
	wb := Split(b)
	setA := make(map[string]bool, len(wa))
	for _, w := range wa {
		setA[w] = true
	}
	setB := make(map[string]bool, len(wb))
	for _, w := range wb {
		setB[w] = true
	}
	shared := 0
	for w := range setA {
		if setB[w] {
			shared++
		}
	}
	union := len(setA) + len(setB) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}
