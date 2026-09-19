// Package validate holds every rule about what the server will accept.
// Handlers call into here before anything reaches Mongo or Redis, so the
// trust boundary lives in one readable file instead of being scattered.
package validate

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MinQuestion = 5
	MaxQuestion = 200
	MinOptions  = 2
	MaxOptions  = 10
	MaxOption   = 80
	MinPassword = 8
	MaxPassword = 128
	MaxName     = 60
)

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Clean strips control characters and collapses runs of whitespace. Client
// input reaches the database only after passing through here, which removes
// the easy tricks: invisible characters, newline injection, padded duplicates.
func Clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func Email(raw string) (string, error) {
	e := strings.ToLower(Clean(raw))
	if e == "" {
		return "", errors.New("email is required")
	}
	if len(e) > 254 || !emailRe.MatchString(e) {
		return "", errors.New("enter a valid email address")
	}
	return e, nil
}

func Name(raw string) (string, error) {
	n := Clean(raw)
	if utf8.RuneCountInString(n) < 2 {
		return "", errors.New("name must be at least 2 characters")
	}
	if utf8.RuneCountInString(n) > MaxName {
		return "", errors.New("name is too long")
	}
	return n, nil
}

func Password(p string) error {
	if len(p) < MinPassword {
		return errors.New("password must be at least 8 characters")
	}
	if len(p) > MaxPassword {
		return errors.New("password is too long")
	}
	var hasLetter, hasDigit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return errors.New("password needs at least one letter and one number")
	}
	return nil
}

func Question(raw string) (string, error) {
	q := Clean(raw)
	n := utf8.RuneCountInString(q)
	if n < MinQuestion {
		return "", errors.New("question must be at least 5 characters")
	}
	if n > MaxQuestion {
		return "", errors.New("question must be 200 characters or fewer")
	}
	return q, nil
}

// Options trims, drops blanks, rejects case-insensitive duplicates and
// enforces the 2-10 range. Returns the cleaned list in the submitted order.
func Options(raw []string) ([]string, error) {
	if len(raw) > 50 {
		return nil, errors.New("too many options submitted")
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		o := Clean(r)
		if o == "" {
			continue
		}
		if utf8.RuneCountInString(o) > MaxOption {
			return nil, errors.New("each option must be 80 characters or fewer")
		}
		key := strings.ToLower(o)
		if _, dup := seen[key]; dup {
			return nil, errors.New("options must be different from each other")
		}
		seen[key] = struct{}{}
		out = append(out, o)
	}
	if len(out) < MinOptions {
		return nil, errors.New("a poll needs at least 2 options")
	}
	if len(out) > MaxOptions {
		return nil, errors.New("a poll can have at most 10 options")
	}
	return out, nil
}
