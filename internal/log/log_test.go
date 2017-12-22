package log

import (
	"bytes"
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

func TestLogging(t *testing.T) {
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
			expected:    ".*•INFO•.*log_test.go•.*•Hello",
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
			expected:    ".*•TRACE•.*log_test.go•.*•Hello",
		},
		{
			loggerLevel: TRACE,
			msgLevel:    DEBUG,
			msg:         "Hello",
			expected:    ".*•DEBUG•.*log_test.go•.*•Hello",
		},
		{
			loggerLevel: TRACE,
			msgLevel:    INFO,
			msg:         "Hello",
			expected:    ".*•INFO•.*log_test.go•.*•Hello",
		},
		{
			loggerLevel: TRACE,
			msgLevel:    WARN,
			msg:         "Hello",
			expected:    ".*•WARN•.*log_test.go•.*•Hello",
		},
		{
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msg:         "Hello",
			expected:    ".*•ERROR•.*log_test.go•.*•Hello",
		},
		// Check use of formatting args.
		{
			loggerLevel: TRACE,
			msgLevel:    ERROR,
			msg:         "Hello #%v %v",
			msgArgs:     []interface{}{1, "Joe"},
			expected:    ".*•ERROR•.*log_test.go•.*•Hello",
		},
	}

	loggerCalls := map[Level]func(string, ...interface{}){
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
		loggerCalls[tc.msgLevel](tc.msg, tc.msgArgs...)

		resultMsg := testOut.String()
		matched, err := regexp.MatchString(tc.expected, resultMsg)
		if err != nil || !matched {
			fmt.Printf("TestCase[%v] failed, \n'%v'\ndoesn't match\n'%v'\n", i, tc.expected, resultMsg)
			t.Fail()
		}
	}
}
