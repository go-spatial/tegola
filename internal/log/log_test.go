package log

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"testing"
)

func TestSetLevel(t *testing.T) {
	// Check default level is INFO & Is* flags are set appropriately
	if level != INFO || !IsError || !IsWarn || !IsInfo || IsDebug || IsTrace {
		fmt.Println("Default level is not set as expected")
		t.Fail()
	}

	// Check TRACE
	SetLogLevel(TRACE)
	if level != TRACE || !IsError || !IsWarn || !IsInfo || !IsDebug || !IsTrace {
		fmt.Println("TRACE level is not set as expected")
		t.Fail()
	}

	// Check DEBUG
	SetLogLevel(DEBUG)
	if level != DEBUG || !IsError || !IsWarn || !IsInfo || !IsDebug || IsTrace {
		fmt.Println("DEBUG level is not set as expected")
		t.Fail()
	}

	// Check INFO
	SetLogLevel(INFO)
	if level != INFO || !IsError || !IsWarn || !IsInfo || IsDebug || IsTrace {
		fmt.Println("INFO level is not set as expected")
		t.Fail()
	}

	// Check WARN
	SetLogLevel(WARN)
	if level != WARN || !IsError || !IsWarn || IsInfo || IsDebug || IsTrace {
		fmt.Println("WARN level is not set as expected")
		t.Fail()
	}

	// Check ERROR
	SetLogLevel(ERROR)
	if level != ERROR || !IsError || IsWarn || IsInfo || IsDebug || IsTrace {
		fmt.Println("ERROR level is not set as expected")
		t.Fail()
	}

	// Check FATAL
	SetLogLevel(FATAL)
	if level != FATAL || IsError || IsWarn || IsInfo || IsDebug || IsTrace {
		fmt.Println("FATAL level is not set as expected")
		t.Fail()
	}
}

func TestLoggingF(t *testing.T) {
	// Tests Tracef(), Debugf(), Infof(), Warnf(), Errorf() logging methods.
	type TestCase struct {
		loggerLevel Level
		msgLevel    Level
		msg         string
		msgArgs     []interface{}
		expected    string // regex pattern
	}

	var testCases []TestCase = []TestCase{
		// These test cases use ".*" to avoid specifics of timestamp, file location, and line number.
		{
			loggerLevel: INFO,
			msgLevel:    INFO,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v•INFO•.*log_test.go•.*•Hello", TimestampRegex),
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
			expected:    fmt.Sprintf("%v•TRACE•.*log_test.go•.*•Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    DEBUG,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v•DEBUG•.*log_test.go•.*•Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    INFO,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v•INFO•.*log_test.go•.*•Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    WARN,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v•WARN•.*log_test.go•.*•Hello", TimestampRegex),
		},
		{
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msg:         "Hello",
			expected:    fmt.Sprintf("%v•ERROR•.*log_test.go•.*•Hello", TimestampRegex),
		},
		// Check use of formatting args.
		{
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msg:         "Hello #%v %v",
			msgArgs:     []interface{}{1, "Joe"},
			expected:    fmt.Sprintf("%v•ERROR•.*log_test.go•.*•Hello", TimestampRegex),
		},
	}

	loggerCalls := map[Level]func(string, ...interface{}){
		FATAL: Fatalf,
		ERROR: Errorf,
		WARN:  Warnf,
		INFO:  Infof,
		DEBUG: Debugf,
		TRACE: Tracef,
	}

	for i, tc := range testCases {
		testOut := bytes.NewBufferString("")
		SetOutput(testOut)
		SetLogLevel(tc.loggerLevel)
		loggerCalls[tc.msgLevel](tc.msg, tc.msgArgs...)

		resultMsg := testOut.String()
		matched, err := regexp.MatchString(tc.expected, resultMsg)
		if err != nil || !matched {
			fmt.Printf("TestCase[%v] failed, \n'%v'\ndoesn't match\n'%v'\n", i, tc.expected, resultMsg)
			t.Fail()
		}
	}
}

func TestLogging(t *testing.T) {
	// Tests Trace(), Debug(), Info(), Warn(), Error() logging methods.
	type TestCase struct {
		loggerLevel Level
		msgLevel    Level
		msgArgs     []interface{}
		expected    string // regex pattern
	}

	var testCases []TestCase = []TestCase{
		// These test cases use regex ".*" to avoid specifics of file location, and line number.
		{ // Check string as arg
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msgArgs:     []interface{}{"Hello"},
			expected:    fmt.Sprintf("%v•ERROR•.*log_test.go•.*•Hello", TimestampRegex),
		},
		{ // Check numbers as args
			loggerLevel: INFO,
			msgLevel:    INFO,
			msgArgs:     []interface{}{1, 2, 3.3, "joe"},
			expected:    fmt.Sprintf("%v•INFO•.*log_test.go•.*•1 2 3.3", TimestampRegex),
		},
		{ // Check error as arg
			loggerLevel: INFO,
			msgLevel:    INFO,
			msgArgs:     []interface{}{errors.New("Test error message")},
			expected:    fmt.Sprintf("%v•INFO•.*log_test.go•.*•Test error message", TimestampRegex),
		},
		{ // Check mix of numbers, errors, and strings as args
			loggerLevel: TRACE,
			msgLevel:    TRACE,
			msgArgs:     []interface{}{1.1, errors.New("Test error message"), 42, " is the answer"},
			expected:    fmt.Sprintf("%v•TRACE•.*log_test.go•.*•1.1 Test error message 42 is the answer", TimestampRegex),
		},
		{ // Check that a format string gets interpretted literally
			loggerLevel: TRACE,
			msgLevel:    TRACE,
			msgArgs:     []interface{}{"This %v could be a %v format string"},
			expected:    fmt.Sprintf("%v•TRACE•.*log_test.go•.*•This %%v could be a %%v format string", TimestampRegex),
		},
	}

	loggerCalls := map[Level]func(...interface{}){
		FATAL: Fatal,
		ERROR: Error,
		WARN:  Warn,
		INFO:  Info,
		DEBUG: Debug,
		TRACE: Trace,
	}

	for i, tc := range testCases {
		testOut := bytes.NewBufferString("")
		SetOutput(testOut)
		SetLogLevel(tc.loggerLevel)
		loggerCalls[tc.msgLevel](tc.msgArgs...)

		resultMsg := testOut.String()
		matched, err := regexp.MatchString(tc.expected, resultMsg)
		if err != nil || !matched {
			fmt.Printf("TestCase[%v] failed, \n'%v'\ndoesn't match\n'%v'\n", i, tc.expected, resultMsg)
			t.Fail()
		}
	}
}
