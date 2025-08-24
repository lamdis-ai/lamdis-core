package problems

import (
	"os"
	"strings"
)

// Base returns the base URL for problem type identifiers.
// Order of precedence:
// 1. PROBLEM_BASE_URL (exact base, e.g. https://mydomain.com/problems)
// 2. BASE_PUBLIC_URL + "/problems" (if set)
// 3. https://example.com/problems (fallback)
func Base() string {
	if b := os.Getenv("PROBLEM_BASE_URL"); b != "" {
		return strings.TrimRight(b, "/")
	}
	if b := os.Getenv("BASE_PUBLIC_URL"); b != "" {
		return strings.TrimRight(b, "/") + "/problems"
	}
	return "https://example.com/problems"
}

// Type builds a full problem type URL for the given slug.
func Type(slug string) string { return Base() + "/" + slug }
