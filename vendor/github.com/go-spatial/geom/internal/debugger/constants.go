package debugger

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (

	// CategoryGot is for got values of a testcase
	CategoryGot = "got"
	// CategoryExpected is the expected values of a testcase
	CategoryExpected = "expected"
	// CategoryInput is the input values of a testcase
	CategoryInput = "input"
)

// CategoryFormatter is a helper category that will build a category
// made up of a set of values given with the With function.
// The category should be a template of where the With values will go.
// For example CategoryFormatter("%v_triangle_%v").With(1,10) will result
// in a string of `1_triangle_10` this can be used to create common
// category names.
type CategoryFormatter string

func (f CategoryFormatter) With(data ...interface{}) string { return fmt.Sprintf(string(f), data...) }
func (f CategoryFormatter) String() string                  { return string(f) }

// CategoryJoiner is a helper category that will build a category
// made up of a set of values given with the With function seperated
// the the last character of the category.
// For example CategoryJoiner("triangle:").With(1,10) will result
// in a string of `triangle:1:10` this can be used to create common
// category names.
type CategoryJoiner string

func (f CategoryJoiner) With(data ...interface{}) string {

	var (
		s    strings.Builder
		addc bool
		join string
	)

	if len(string(f)) > 0 {
		r, _ := utf8.DecodeLastRune([]byte(f))
		join = string([]rune{r})
	}

	s.WriteString(string(f))
	for _, v := range data {
		if addc {
			s.WriteString(join)
		}
		fmt.Fprintf(&s, "%v", v)
		addc = true
	}
	return s.String()
}

func (f CategoryJoiner) String() string { return string(f) }
