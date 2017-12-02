package log

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"
)

func TestSetLevel(t *testing.T) {
	// Check default level is INFO & Is* flags are set appropriately
	if !(logLevel == INFO && IsError == true && IsWarn == true && IsInfo == true && IsDebug == false &&
		IsTrace == false) {
		fmt.Println("Default level is not set as expected")
		t.Fail()
	}

	// Check TRACE
	SetLogLevel(TRACE)
	if !(logLevel == TRACE && IsError == true && IsWarn == true && IsInfo == true && IsDebug == true &&
		IsTrace == true) {
		fmt.Println("TRACE level is not set as expected")
		t.Fail()
	}

	// Check DEBUG
	SetLogLevel(DEBUG)
	if !(logLevel == DEBUG && IsError == true && IsWarn == true && IsInfo == true && IsDebug == true &&
		IsTrace == false) {
		fmt.Println("DEBUG level is not set as expected")
		t.Fail()
	}

	// Check INFO
	SetLogLevel(INFO)
	if !(logLevel == INFO && IsError == true && IsWarn == true && IsInfo == true && IsDebug == false &&
		IsTrace == false) {
		fmt.Println("INFO level is not set as expected")
		t.Fail()
	}

	// Check WARN
	SetLogLevel(WARN)
	if !(logLevel == WARN && IsError == true && IsWarn == true && IsInfo == false && IsDebug == false &&
		IsTrace == false) {
		fmt.Println("WARN level is not set as expected")
		t.Fail()
	}

	// Check ERROR
	SetLogLevel(ERROR)
	if !(logLevel == ERROR && IsError == true && IsWarn == false && IsInfo == false &&
		IsDebug == false && IsTrace == false) {
		fmt.Println("ERROR level is not set as expected")
		t.Fail()
	}

	// Check FATAL
	SetLogLevel(FATAL)
	if !(logLevel == FATAL && IsError == false && IsWarn == false && IsInfo == false &&
		IsDebug == false && IsTrace == false) {
		fmt.Println("FATAL level is not set as expected")
		t.Fail()
	}
}

func TestLogging(t *testing.T) {
	type TestCase struct {
		loggerLevel int
		msgLevel    int
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

	loggerCalls := map[int]func(string, ...interface{}){
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
