package git

import "regexp"

// subjectRe matches a conventional-commits subject line:
//
//	type(scope)!: description
//
// Group 1: type (lowercase), Group 2: scope (optional),
// Group 3: "!" breaking marker (optional), Group 4: description.
var subjectRe = regexp.MustCompile(`^([a-z]+)(?:\(([a-z][a-z0-9-]*)\))?(!)?: (.+)$`)

// Subject represents a parsed conventional-commits subject line.
type Subject struct {
	Type        string // e.g., "feat", "fix", "refactor"
	Scope       string // e.g., "auth", "payments" (empty if no scope)
	Breaking    bool   // true if "!" marker present
	Description string // the description after ": "
	Raw         string // the original unparsed subject line
}

// ParseSubject parses a conventional-commits subject line into
// its components. Returns ok=false if the subject does not match
// the conventional-commits format.
//
//	ParseSubject("feat(auth): add login") → Subject{Type:"feat", Scope:"auth", Description:"add login"}, true
//	ParseSubject("fix!: critical bug")    → Subject{Type:"fix", Breaking:true, Description:"critical bug"}, true
//	ParseSubject("random message")        → Subject{}, false
func ParseSubject(line string) (Subject, bool) {
	m := subjectRe.FindStringSubmatch(line)
	if m == nil {
		return Subject{}, false
	}
	return Subject{
		Type:        m[1],
		Scope:       m[2],
		Breaking:    m[3] == "!",
		Description: m[4],
		Raw:         line,
	}, true
}
