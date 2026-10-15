// Package preferences implements W43: OGC API Features Part 4 draft.3
// preference classes as an optional package.
//
// Preferences are client hints sent via the Prefer header (RFC 7240).
// Servers may honor them or ignore them; they never cause errors.
package preferences

// Preference is a single client preference.
type Preference struct {
	// Name is the preference token (e.g. "return", "handling").
	Name string
	// Value is the preference value (e.g. "minimal", "strict").
	Value string
	// Params carries optional parameters.
	Params map[string]string
}

// Defined preference names (Part 4 draft.3, RFC 7240).
const (
	// PreferReturn controls response content for mutations.
	// Values: "minimal" (default), "representation".
	PreferReturn = "return"
	// PreferHandling controls error handling strictness.
	// Values: "strict", "lenient".
	PreferHandling = "handling"
	// PreferWait controls synchronous vs asynchronous processing.
	// Value is seconds as integer string.
	PreferWait = "wait"
)

// Return values.
const (
	ReturnMinimal        = "minimal"
	ReturnRepresentation = "representation"
)

// Handling values.
const (
	HandlingStrict  = "strict"
	HandlingLenient = "lenient"
)

// Parse parses a Prefer header value into preferences.
// Example: `return=representation, handling=strict`
func Parse(header string) []Preference {
	var out []Preference
	for _, part := range splitHeader(header) {
		part = trimSpace(part)
		if part == "" {
			continue
		}
		name, value, params := parsePreference(part)
		out = append(out, Preference{Name: name, Value: value, Params: params})
	}
	return out
}

// Get returns the first preference with the given name, or nil.
func Get(prefs []Preference, name string) *Preference {
	for i := range prefs {
		if prefs[i].Name == name {
			return &prefs[i]
		}
	}
	return nil
}

// WantRepresentation reports whether the client prefers full representations.
func WantRepresentation(prefs []Preference) bool {
	if p := Get(prefs, PreferReturn); p != nil {
		return p.Value == ReturnRepresentation
	}
	return false
}

// IsStrictHandling reports whether the client prefers strict error handling.
func IsStrictHandling(prefs []Preference) bool {
	if p := Get(prefs, PreferHandling); p != nil {
		return p.Value == HandlingStrict
	}
	return false
}

func splitHeader(h string) []string {
	var out []string
	depth := 0
	start := 0
	for i, c := range h {
		switch c {
		case '"':
			// toggle quote - simplified, doesn't handle escapes
		case ',':
			if depth == 0 {
				out = append(out, h[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, h[start:])
	return out
}

func trimSpace(s string) string {
	// simple trim
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func parsePreference(s string) (name, value string, params map[string]string) {
	params = map[string]string{}
	// Split on ; for params
	parts := splitOn(s, ';')
	nameValue := trimSpace(parts[0])
	if idx := indexOf(nameValue, '='); idx >= 0 {
		name = trimSpace(nameValue[:idx])
		value = trimSpace(nameValue[idx+1:])
		// Strip quotes
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
	} else {
		name = trimSpace(nameValue)
	}
	for _, p := range parts[1:] {
		p = trimSpace(p)
		if idx := indexOf(p, '='); idx >= 0 {
			params[trimSpace(p[:idx])] = trimSpace(p[idx+1:])
		}
	}
	return name, value, params
}

func splitOn(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
