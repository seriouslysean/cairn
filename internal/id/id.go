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

// CommentPrefix starts every comment id. The rest of a comment id is 4 characters from Alphabet.
const CommentPrefix = "c-"

const (
	minLength     = 4
	maxLength     = 8
	commentLength = 4
	maxTries      = 100
)

// New returns an id of n characters (4 to 8) that is not in taken. Each character is Alphabet[b&31] for the next
// byte b read from r, and r is crypto/rand.Reader when nil. It tries at most 100 candidates. It returns an error
// when n is out of range, when r fails or runs out, or when every candidate is taken.
func New(n int, taken map[string]bool, r io.Reader) (string, error) {
	if n < minLength || n > maxLength {
		return "", errors.New("id: length must be 4 to 8")
	}
	if r == nil {
		r = rand.Reader
	}
	buf := make([]byte, n)
	for range maxTries {
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

// NewComment returns a comment id that is not in taken, a set of comment ids. It draws from r as New does.
func NewComment(taken map[string]bool, r io.Reader) (string, error) {
	rest := make(map[string]bool, len(taken))
	for c := range taken {
		rest[strings.TrimPrefix(c, CommentPrefix)] = true
	}
	s, err := New(commentLength, rest, r)
	if err != nil {
		return "", err
	}
	return CommentPrefix + s, nil
}

// Valid reports whether s has 4 to 8 characters, all from Alphabet.
func Valid(s string) bool {
	return len(s) >= minLength && len(s) <= maxLength && inAlphabet(s)
}

// ValidComment reports whether s is a comment id: CommentPrefix followed by 4 characters from Alphabet.
func ValidComment(s string) bool {
	rest, ok := strings.CutPrefix(s, CommentPrefix)
	return ok && len(rest) == commentLength && inAlphabet(rest)
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

// inAlphabet reports whether every character of s is in Alphabet.
func inAlphabet(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune(Alphabet, r) {
			return false
		}
	}
	return true
}
