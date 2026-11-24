package schema

import (
	"fmt"
	"github.com/ygelfand/libcountertop/pkg/say"
	"strings"
	"time"
	"unicode"
)

type DateFormat string

func DateFormats() []DateFormat {
	return []DateFormat{"", "dd.MM.yyyy", "MM/dd/yyyy", "yyyy-MM-dd", "d MMM yyyy", "EEEE, d MMMM yyyy", "custom"}
}
func (f DateFormat) Label() string {
	if f == "" {
		return say.T("clock.date_format.default")
	}
	if f == "custom" {
		return say.T("clock.date_format.custom")
	}
	return string(f)
}

// PatternDate uses Unicode-style calendar tokens, not Go reference-date layouts.
// Single quotes delimit literals; doubled quotes produce one literal quote.
func PatternDate(at time.Time, pattern string) (string, error) {
	if len(pattern) > 128 {
		return "", fmt.Errorf("date pattern exceeds 128 bytes")
	}
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("date pattern is empty")
	}
	var out strings.Builder
	field := false
	tokens := []string{"yyyy", "MMMM", "EEEE", "MMM", "EEE", "yy", "MM", "dd", "M", "d"}
	layouts := map[string]string{"yyyy": "2006", "yy": "06", "MMMM": "January", "MMM": "Jan", "MM": "01", "M": "1", "dd": "02", "d": "2", "EEEE": "Monday", "EEE": "Mon"}
	for len(pattern) > 0 {
		if pattern[0] == '\'' {
			pattern = pattern[1:]
			if strings.HasPrefix(pattern, "'") {
				out.WriteByte('\'')
				pattern = pattern[1:]
				continue
			}
			closed := false
			for len(pattern) > 0 {
				if pattern[0] == '\'' {
					pattern = pattern[1:]
					if strings.HasPrefix(pattern, "'") {
						out.WriteByte('\'')
						pattern = pattern[1:]
						continue
					}
					closed = true
					break
				}
				out.WriteByte(pattern[0])
				pattern = pattern[1:]
			}
			if !closed {
				return "", fmt.Errorf("unclosed literal in date pattern")
			}
			continue
		}
		matched := false
		for _, token := range tokens {
			if strings.HasPrefix(pattern, token) {
				out.WriteString(say.Date(at, layouts[token]))
				pattern = pattern[len(token):]
				field = true
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		r := []rune(pattern)[0]
		if unicode.IsLetter(r) {
			return "", fmt.Errorf("unsupported date token; use yyyy, yy, MMMM, MMM, MM, M, dd, d, EEEE or EEE; quote literal text")
		}
		out.WriteRune(r)
		pattern = pattern[len(string(r)):]
	}
	if !field {
		return "", fmt.Errorf("date pattern needs a calendar field")
	}
	return out.String(), nil
}
func (c Clock) FormatDate(at time.Time) string {
	if c.DateFormat != "" {
		pattern := string(c.DateFormat)
		if c.DateFormat == "custom" {
			pattern = c.DatePattern
		}
		if text, err := PatternDate(at, pattern); err == nil {
			return text
		}
	}
	return say.Date(at, say.T("date.layout"))
}
