// Package id makes short random ids for issues and comments.
package id

import "io"

// Alphabet is Crockford's base32 in lower case: the digits and the letters without i, l, o and u.
const Alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// New returns an id of n characters (4 to 8) that is not in taken. Each character is Alphabet[b&31] for the next
// byte b read from r, and r is crypto/rand.Reader when nil. It tries at most 100 candidates. It returns an error
// when n is out of range, when r fails or runs out, or when every candidate is taken.
func New(n int, taken map[string]bool, r io.Reader) (string, error) {
	panic("todo: step 1")
}

// Valid reports whether s has 4 to 8 characters, all from Alphabet.
func Valid(s string) bool {
	panic("todo: step 1")
}

// Length is the id length for a tracker that already holds count issues: 4 below 1,000, 5 below 32,000, and 6
// from there on.
func Length(count int) int {
	panic("todo: step 1")
}
