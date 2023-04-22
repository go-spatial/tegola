package provider

import "testing"

func TestReplaceParams(t *testing.T) {
	type tcase struct {
		params       Params
		sql          string
		expectedSql  string
		expectedArgs []interface{}
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			args := make([]interface{}, 0)
			out := tc.params.ReplaceParams(tc.sql, &args)

			if out != tc.expectedSql {
				t.Errorf("expected \n \t%v\n out \n \t%v", tc.expectedSql, out)
				return
			}

			if len(tc.expectedArgs) != len(args) {
				t.Errorf("expected \n \t%v\n out \n \t%v", tc.expectedArgs, args)
				return
			}
			for i, arg := range tc.expectedArgs {
				if arg != args[i] {
					t.Errorf("expected \n \t%v\n out \n \t%v", tc.expectedArgs, args)
					return
				}
			}
		}
	}

	tests := map[string]tcase{
		"nil params": {
			params:       nil,
			sql:          "SELECT * FROM table",
			expectedSql:  "SELECT * FROM table",
			expectedArgs: []interface{}{},
		},
		"int replacement": {
			params: Params{
				"!PARAM!": {
					Token: "!PARAM!",
					SQL:   "?",
					Value: 1,
				},
			},
			sql:          "SELECT * FROM table WHERE param = !PARAM!",
			expectedSql:  "SELECT * FROM table WHERE param = $1",
			expectedArgs: []interface{}{1},
		},
		"string replacement": {
			params: Params{
				"!PARAM!": {
					Token: "!PARAM!",
					SQL:   "?",
					Value: "test",
				},
			},
			sql:          "SELECT * FROM table WHERE param = !PARAM!",
			expectedSql:  "SELECT * FROM table WHERE param = $1",
			expectedArgs: []interface{}{"test"},
		},
		"null replacement": {
			params: Params{
				"!PARAM!": {
					Token: "!PARAM!",
					SQL:   "?",
					Value: nil,
				},
			},
			sql:          "SELECT * FROM table WHERE param = !PARAM!",
			expectedSql:  "SELECT * FROM table WHERE param = $1",
			expectedArgs: []interface{}{nil},
		},
		"complex sql replacement": {
			params: Params{
				"!PARAM!": {
					Token: "!PARAM!",
					SQL:   "WHERE param=?",
					Value: 1,
				},
			},
			sql:          "SELECT * FROM table !PARAM!",
			expectedSql:  "SELECT * FROM table WHERE param=$1",
			expectedArgs: []interface{}{1},
		},
		"subquery removal": {
			params: Params{
				"!PARAM!": {
					Token: "!PARAM!",
					SQL:   "",
					Value: nil,
				},
			},
			sql:          "SELECT * FROM table !PARAM!",
			expectedSql:  "SELECT * FROM table ",
			expectedArgs: []interface{}{},
		},
		"multiple params": {
			params: Params{
				"!PARAM!": {
					Token: "!PARAM!",
					SQL:   "? ? ?",
					Value: 1,
				},
				"!PARAM2!": {
					Token: "!PARAM2!",
					SQL:   "???",
					Value: 2,
				},
			},
			sql:          "!PARAM! !PARAM2! !PARAM!",
			expectedSql:  "$1 $1 $1 $2$2$2 $1 $1 $1",
			expectedArgs: []interface{}{1, 2},
		},
		"unknown token": {
			params: Params{
				"!PARAM!": {
					Token: "!PARAM!",
					SQL:   "?",
					Value: 1,
				},
			},
			sql:          "!NOT_PARAM! !PARAM! !NOT_PARAM!",
			expectedSql:  "!NOT_PARAM! $1 !NOT_PARAM!",
			expectedArgs: []interface{}{1},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
