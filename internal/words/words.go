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
	for w := range strings.FieldsFuncSeq(s, notWordRune) {
		w = strings.ToLower(w)
		if !seen[w] {
			seen[w] = true
			result = append(result, w)
		}
	}
	return result
}

// Shared is the number of distinct words that a and b have in common, with words as Split gives them.
func Shared(a, b string) int {
	return shared(Split(a), Split(b))
}

// Overlap is the word-overlap score of a and b: the number of distinct words in both, divided as a float64 by the
// number of distinct words in either (Jaccard similarity), with words as Split gives them. It is 0 when neither has
// a word and 1 when both have the same words.
func Overlap(a, b string) float64 {
	wa, wb := Split(a), Split(b)
	both := shared(wa, wb)
	either := len(wa) + len(wb) - both
	if either == 0 {
		return 0
	}
	return float64(both) / float64(either)
}

// shared counts the words in both a and b, which each hold distinct words.
func shared(a, b []string) int {
	inA := make(map[string]bool, len(a))
	for _, w := range a {
		inA[w] = true
	}
	n := 0
	for _, w := range b {
		if inA[w] {
			n++
		}
	}
	return n
}

func notWordRune(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
