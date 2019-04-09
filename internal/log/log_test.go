package log

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const ExpectedFilename = "log_test.go"

func TestSetLevel(t *testing.T) {

	type tcase struct {
		isDefault bool
		lvl       Level
		tst       func() bool
	}
	fn := func(tc tcase) (string, func(*testing.T)) {

		name := "Default"
		if !tc.isDefault {
			name = tc.lvl.String()
		}

		return name, func(t *testing.T) {

			if !tc.isDefault {
				l := level
				SetLogLevel(tc.lvl)
				// Restore the level back
				defer SetLogLevel(l)
			}

			if tc.tst() {
				t.Errorf("%v , expected level to be set correctly, got %v", name, level)
			}

		}
	}

	// Order is import as we are dealing with setting a global.
	tests := [...]tcase{
		{
			isDefault: true,
			tst: func() bool {
				return level != INFO || !IsError || !IsWarn || !IsInfo || IsDebug || IsTrace
			},
		},
		{
			lvl: TRACE,
			tst: func() bool {
				return level != TRACE || !IsError || !IsWarn || !IsInfo || !IsDebug || !IsTrace
			},
		},
		{
			lvl: DEBUG,
			tst: func() bool {
				return level != DEBUG || !IsError || !IsWarn || !IsInfo || !IsDebug || IsTrace
			},
		},
		{
			lvl: INFO,
			tst: func() bool {
				return level != INFO || !IsError || !IsWarn || !IsInfo || IsDebug || IsTrace
			},
		},
		{
			lvl: WARN,
			tst: func() bool {
				return level != WARN || !IsError || !IsWarn || IsInfo || IsDebug || IsTrace
			},
		},
		{
			lvl: ERROR,
			tst: func() bool {
				return level != ERROR || !IsError || IsWarn || IsInfo || IsDebug || IsTrace
			},
		},
		{
			lvl: FATAL,
			tst: func() bool {
				return level != FATAL || IsError || IsWarn || IsInfo || IsDebug || IsTrace
			},
		},
	}

	for _, tc := range tests {
		t.Run(fn(tc))
	}
}

type testLoggingFTCase struct {
	loggerLevel Level
	msgLevel    Level
	msg         string
	msgArgs     []interface{}
	expected    string // regex pattern
}

//go:noinline
func testLoggingF(tc testLoggingFTCase) (string, func(*testing.T)) {

	loggerCalls := map[Level]func(string, ...interface{}){
		FATAL: Fatalf,
		ERROR: Errorf,
		WARN:  Warnf,
		INFO:  Infof,
		DEBUG: Debugf,
		TRACE: Tracef,
	}

	msg := tc.msg

	name := fmt.Sprintf("%s %s %s", tc.loggerLevel, tc.msgLevel, msg)
	return name, func(t *testing.T) {
		testOut := bytes.NewBufferString("")
		SetOutput(testOut)
		SetLogLevel(tc.loggerLevel)

		loggerCalls[tc.msgLevel](tc.msg, tc.msgArgs...)

		resultMsg := testOut.String()

		matched, err := regexp.MatchString(tc.expected, resultMsg)
		if err != nil || !matched {
			t.Errorf("failed, expected:\n %v \ngot\n %v\n", tc.expected, resultMsg)
		}
	}
}

func TestLoggingF(t *testing.T) {
	// Tests Tracef(), Debugf(), Infof(), Warnf(), Errorf() logging methods.
	type tcase = testLoggingFTCase

	tests := [...]tcase{
		// These test cases use ".*" to avoid specifics of timestamp, file location, and line number.
		{
			loggerLevel: INFO,
			msgLevel:    INFO,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v.*[INFO].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
		// Logging with logger's level set higher than message should result in no output.
		{
			loggerLevel: WARN,
			msgLevel:    INFO,
			msg:         "Hello",
			expected:    "",
		},
		{
			loggerLevel: TRACE,
			msgLevel:    TRACE,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v.*[TRACE].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    DEBUG,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v.*[DEBUG].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    INFO,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v.*[INFO].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    WARN,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v.*[WARN].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v.*[ERROR].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
		// Check use of formatting args.
		{
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msg:         "Hello #%v %v",
			msgArgs:     []interface{}{1, "Joe"},
			expected:    fmt.Sprintf("%v.*[ERROR].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
	}

	for _, tc := range tests {
		t.Run(testLoggingF(tc))
	}
}

//go:noinline
func testLogging(tc testLoggingFTCase) (string, func(*testing.T)) {

	loggerCalls := map[Level]func(...interface{}){
		FATAL: Fatal,
		ERROR: Error,
		WARN:  Warn,
		INFO:  Info,
		DEBUG: Debug,
		TRACE: Trace,
	}

	var msgs = make([]string, len(tc.msgArgs))
	for _, a := range tc.msgArgs {
		msgs = append(msgs, fmt.Sprintf("%v", a))
	}
	msg := strings.Join(msgs, "_")

	name := fmt.Sprintf("%s %s %s", tc.loggerLevel, tc.msgLevel, msg)
	return name, func(t *testing.T) {
		testOut := bytes.NewBufferString("")
		SetOutput(testOut)
		SetLogLevel(tc.loggerLevel)

		loggerCalls[tc.msgLevel](tc.msgArgs...)

		resultMsg := testOut.String()

		matched, err := regexp.MatchString(tc.expected, resultMsg)
		if err != nil || !matched {
			t.Errorf("failed, expected:\n %v \ngot\n %v\n", tc.expected, resultMsg)
		}
	}
}

func TestLogging(t *testing.T) {
	// Tests Trace(), Debug(), Info(), Warn(), Error() logging methods.
	type tcase = testLoggingFTCase

	tests := [...]tcase{
		// These test cases use regex ".*" to avoid specifics of file location, and line number.
		{ // Check string as arg
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msgArgs:     []interface{}{"Hello"},
			expected:    fmt.Sprintf("%v.*[ERROR].*"+ExpectedFilename+".*Hello", TimestampRegex),
		},
		{ // Check numbers as args
			loggerLevel: INFO,
			msgLevel:    INFO,
			msgArgs:     []interface{}{1, 2, 3.3, "joe"},
			expected:    fmt.Sprintf("%v.*[INFO].*"+ExpectedFilename+".*1 2 3.3", TimestampRegex),
		},
		{ // Check error as arg
			loggerLevel: INFO,
			msgLevel:    INFO,
			msgArgs:     []interface{}{errors.New("Test error message")},
			expected:    fmt.Sprintf("%v.*[INFO].*"+ExpectedFilename+".*Test error message", TimestampRegex),
		},
		{ // Check mix of numbers, errors, and strings as args
			loggerLevel: TRACE,
			msgLevel:    TRACE,
			msgArgs:     []interface{}{1.1, errors.New("Test error message"), 42, " is the answer"},
			expected:    fmt.Sprintf("%v.*[TRACE].*"+ExpectedFilename+".*1.1 Test error message 42 is the answer", TimestampRegex),
		},
		{ // Check that a format string gets interpretted literally
			loggerLevel: TRACE,
			msgLevel:    TRACE,
			msgArgs:     []interface{}{"This %v could be a %v format string"},
			expected:    fmt.Sprintf("%v.*[TRACE].*"+ExpectedFilename+".*This %%v could be a %%v format string", TimestampRegex),
		},
	}

	for _, tc := range tests {
		t.Run(testLogging(tc))
	}

}
