package privacy

import (
	"fmt"
	"regexp"
)

const redactedPlaceholder = "[REDACTED]"

// Compile compiles a list of regex pattern strings into compiled regexps.
// Returns an error if any pattern is invalid.
func Compile(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("compile redact pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// Apply replaces all matches of the compiled patterns in text with [REDACTED].
func Apply(text string, patterns []*regexp.Regexp) string {
	for _, re := range patterns {
		text = re.ReplaceAllString(text, redactedPlaceholder)
	}
	return text
}
