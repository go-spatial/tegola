package symbol

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/geom/internal/parsing"
)

func TestSymbol(t *testing.T) {

	type tc struct {
		input    string
		expected [][]byte
		err      error
	}
	fn := func(idx int, test tc) {
		r := strings.NewReader(test.input)
		s := parsing.NewScanner(r, SplitFn)
		i := 0
		for s.Scan() {
			if i >= len(test.expected) {
				t.Errorf("[%v] Got(%v) more entries then expected (%v).", idx, i, len(test.expected))
			}
			b := s.RawBytes()
			if !reflect.DeepEqual(test.expected[i], b) {
				t.Errorf("[%v] Bytes Expected: %v got: %v", idx, test.expected[i], b)
			}
			f, m := s.RawPeek()
			i++
			if m {
				if i >= len(test.expected) {
					t.Errorf("[%v] Got(%v) more entries then expected (%v).", idx, i, len(test.expected))
					break
				}
				if !reflect.DeepEqual(test.expected[i], f) {
					t.Errorf("[%v] M Expected: %v got: %v", idx, test.expected[i], f)
				}
			} else {
				if !reflect.DeepEqual([]byte{parsing.EOF}, f) {
					t.Errorf("[%v] !M Expected: %v got: %v", idx, []byte{parsing.EOF}, f)
				}
			}
		}
		if i < len(test.expected) {
			t.Errorf("[%v] Got(%v) entries fewer then expected (%v).", idx, i, len(test.expected))
			for j := i; j < len(test.expected); j++ {
				t.Errorf("[%v] Missing: %v:%v:%v", idx, test.expected[j][0], string(test.expected[j][1:]), test.expected[j])
			}
		}

	}

	tbltest.Cases(
		tc{
			input: ` this is a test.`,
			expected: [][]byte{
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 4, []byte{'t', 'h', 'i', 's'}),
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 2, []byte{'i', 's'}),
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 1, []byte{'a'}),
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 4, []byte{'t', 'e', 's', 't'}),
				parsing.EncodeSymbol(Dot, 1, []byte{'.'}),
			},
		},
		tc{
			input: ` { this is a test.}`,
			expected: [][]byte{
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Brace, 1, []byte{'{'}),
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 4, []byte{'t', 'h', 'i', 's'}),
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 2, []byte{'i', 's'}),
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 1, []byte{'a'}),
				parsing.EncodeSymbol(Space, 1, []byte{' '}),
				parsing.EncodeSymbol(Letter, 4, []byte{'t', 'e', 's', 't'}),
				parsing.EncodeSymbol(Dot, 1, []byte{'.'}),
				parsing.EncodeSymbol(Cbrace, 1, []byte{'}'}),
			},
		},
		tc{
			input: `}`,
			expected: [][]byte{
				parsing.EncodeSymbol(Cbrace, 1, []byte{'}'}),
			},
		},
		tc{
			input: `{`,
			expected: [][]byte{
				parsing.EncodeSymbol(Brace, 1, []byte{'{'}),
			},
		},
		tc{
			input: `[`,
			expected: [][]byte{
				parsing.EncodeSymbol(Bracket, 1, []byte{'['}),
			},
		},
		tc{
			input: `}}`,
			expected: [][]byte{
				parsing.EncodeSymbol(Cdbrace, 1, []byte{'}', '}'}),
			},
		},
		tc{
			input: `-12.000`,
			expected: [][]byte{
				parsing.EncodeSymbol(Dash, 1, []byte{'-'}),
				parsing.EncodeSymbol(Digit, 2, []byte{'1', '2'}),
				parsing.EncodeSymbol(Dot, 1, []byte{'.'}),
				parsing.EncodeSymbol(Digit, 3, []byte{'0', '0', '0'}),
			},
		},
	).Run(fn)

}
