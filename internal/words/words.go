// Package words scores how alike two texts are by the words they share.
package words

// Split returns the distinct words of s in the order they first appear. A word is a run of letters and digits
// (unicode.IsLetter or unicode.IsDigit), lowercased; everything else separates words.
func Split(s string) []string {
	panic("todo: step 4")
}

// Overlap is the word-overlap score of a and b: the number of distinct words in both, divided as a float64 by the
// number of distinct words in either (Jaccard similarity), with words as Split gives them. It is 0 when neither has
// a word and 1 when both have the same words.
func Overlap(a, b string) float64 {
	panic("todo: step 4")
}
