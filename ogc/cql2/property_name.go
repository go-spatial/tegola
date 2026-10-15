package cql2

import "unicode/utf8"

// CanReferenceProperty reports whether name can be referenced as a CQL2 text
// property, quoting keywords when necessary. It validates the decoded alias;
// quote delimiters and escape sequences are not part of name.
func CanReferenceProperty(name string) bool {
	if name == "" || len(name) > maxPropertyBytes || !utf8.ValidString(name) {
		return false
	}
	first := true
	for _, r := range name {
		if first {
			if !nameStart(r) {
				return false
			}
			first = false
		} else if !namePart(r) {
			return false
		}
	}
	return true
}
