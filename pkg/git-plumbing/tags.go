package git

import "strings"

// MatchTagPrefix reports whether candidate is equal to or a descendant of
// query using segment-wise prefix matching. Segments are separated by dots.
// Query segments must be a complete prefix of candidate segments.
//
//	MatchTagPrefix("security", "security.auth.oauth") → true
//	MatchTagPrefix("security.auth", "security.auth.oauth") → true
//	MatchTagPrefix("sec", "security") → false
func MatchTagPrefix(query, candidate string) bool {
	if query == "" || candidate == "" {
		return false
	}
	qParts := strings.Split(query, ".")
	cParts := strings.Split(candidate, ".")
	if len(qParts) > len(cParts) {
		return false
	}
	for i, q := range qParts {
		if q != cParts[i] {
			return false
		}
	}
	return true
}

// MatchTagInList reports whether any tag in the comma-separated list matches
// the query using MatchTagPrefix.
//
//	MatchTagInList("security", "auth, security.mfa, payments") → true
//	MatchTagInList("billing", "auth, security.mfa, payments") → false
func MatchTagInList(query, tagList string) bool {
	for _, tag := range ParseTagList(tagList) {
		if MatchTagPrefix(query, tag) {
			return true
		}
	}
	return false
}

// ParseTagList splits a comma-separated tag trailer value into individual
// trimmed tags. Returns nil for empty input.
//
//	ParseTagList("auth, security.mfa, payments") → ["auth", "security.mfa", "payments"]
//	ParseTagList("") → nil
func ParseTagList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	var result []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
