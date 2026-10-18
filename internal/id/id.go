// Package id makes short random ids for issues and comments.
package id

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
)

// Alphabet is Crockford's base32 in lower case: the digits and the letters without i, l, o and u.
const Alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// New returns an id of n characters (4 to 8) that is not in taken. Each character is Alphabet[b&31] for the next
// byte b read from r, and r is crypto/rand.Reader when nil. It tries at most 100 candidates. It returns an error
// when n is out of range, when r fails or runs out, or when every candidate is taken.
func New(n int, taken map[string]bool, r io.Reader) (string, error) {
	if n < 4 || n > 8 {
		return "", errors.New("id: length must be 4 to 8")
	}
	if r == nil {
		r = rand.Reader
	}
	buf := make([]byte, n)
	for try := 0; try < 100; try++ {
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		for i, b := range buf {
			buf[i] = Alphabet[b&31]
		}
		if s := string(buf); !taken[s] {
			return s, nil
		}
	}
	return "", errors.New("id: no free id after 100 tries")
}

// Valid reports whether s has 4 to 8 characters, all from Alphabet.
func Valid(s string) bool {
	if len(s) < 4 || len(s) > 8 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(Alphabet, rune(s[i])) {
			return false
		}
	}
	return true
}

// Length is the id length for a tracker that already holds count issues: 4 below 1,000, 5 below 32,000, and 6
// from there on.
func Length(count int) int {
	switch {
	case count < 1000:
		return 4
	case count < 32000:
		return 5
	}
	return 6
}
