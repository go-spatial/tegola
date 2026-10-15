package cql2

import (
	"strings"
	"testing"
)

func TestLexerTokenBoundaries(t *testing.T) {
	tokens, err := lex(strings.Repeat("(", maxTokens))
	if err != nil || len(tokens) != maxTokens+1 {
		t.Fatal("within token bound rejected", err)
	}
	if _, err := lex(strings.Repeat("(", maxTokens+1)); err == nil {
		t.Fatal("over token bound accepted")
	}
	for _, input := range []string{"n='a\\'b'", "n='a''b'", "n='\\\\'", "n='\\a\\b\\t\\n\\v\\f\\r'"} {
		if _, err := lex(input); err != nil {
			t.Fatal(err)
		}
	}
}
func TestParseActualBinaryDepthAndUTF8Budget(t *testing.T) {
	within := strings.Repeat("TRUE AND ", 31) + "TRUE"
	if _, err := Parse(within); err != nil {
		t.Fatal("binary depth32 rejected", err)
	}
	if _, err := Parse("TRUE AND " + within); err == nil {
		t.Fatal("binary depth33 accepted")
	}
	if _, err := Parse(strings.Repeat("Ж", 512) + "=1"); err != nil {
		t.Fatal("UTF8 bytes bound rejected", err)
	}
	if _, err := Parse(strings.Repeat("Ж", 513) + "=1"); err == nil {
		t.Fatal("UTF8 bytes overflow accepted")
	}
}
