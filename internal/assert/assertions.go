package assert

// Testing check shortcuts

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Example Usage:
// assert.Equal(t, rightAnswer, testAnswer)
// assert.Equal(t, rightAnswer, testAnswer, "Append this to failure message")
// assert.Equal(t, rightAnswer, testAnswer, "More usefully, append this: %v", someImportantValue)

// Log message and fail if actualValue doesn't match expectedValue
// *Note that DeepEqual will check the value pointed to by two pointers, ie don't use for checking
//	pointer equality.
func Equal(t *testing.T, expectedValue interface{}, actualValue interface{}, cm ...interface{}) bool {
	// 'cm', the custom message arguments are optional, and if provided will be appended to a fail
	//	message if generated.
	// If cm[0] is a string, the cm arguments will be treated like fmt.Printf() treats its params.
	// If cm[0] is not a string, the cm arguments will be treated like fmt.Print().
	if !reflect.DeepEqual(expectedValue, actualValue) {
		msg := fmt.Sprintf("%v != %v (expected)", actualValue, expectedValue)
		if len(cm) > 0 {
			switch cm[0].(type) {
			case string:
				msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
			default:
				msg = msg + " - " + fmt.Sprint(cm...)
			}
		}
		return Fail(t, msg)
	}
	return true
}

// Log message and fail if testValue is not true.
func True(t *testing.T, testValue bool, cm ...interface{}) bool {
	// 'cm', the custom message arguments are optional.
	// If cm[0] is a string, the cm arguments will be treated like fmt.Printf() treats its params.
	// If cm[0] is not a string, the cm arguments will be treated like fmt.Print().
	if !testValue {
		msg := fmt.Sprintf("%v is false", testValue)
		if len(cm) > 0 {
			switch cm[0].(type) {
			case string:
				msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
			default:
				msg = msg + " - " + fmt.Sprint(cm...)
			}
		}
		return Fail(t, msg)
	}
	return true
}

// Log message and fail if testValue is not nil.
func Nil(t *testing.T, testValue interface{}, cm ...interface{}) bool {
	// 'cm', the custom message arguments are optional.
	// If cm[0] is a string, the cm arguments will be treated like fmt.Printf() treats its params.
	// If cm[0] is not a string, the cm arguments will be treated like fmt.Print().
	if !isNil(testValue) {
		msg := fmt.Sprintf("%v (%T) is not nil", testValue, testValue)
		if len(cm) > 0 {
			switch cm[0].(type) {
			case string:
				msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
			default:
				msg = msg + " - " + fmt.Sprint(cm...)
			}
		}
		return Fail(t, msg)
	}
	return true
}

// Log message and fail if testValue is nil.
func NotNil(t *testing.T, testValue interface{}, cm ...interface{}) bool {
	// 'cm', the custom message arguments are optional.
	// If cm[0] is a string, the cm arguments will be treated like fmt.Printf() treats its params.
	// If cm[0] is not a string, the cm arguments will be treated like fmt.Print().
	if isNil(testValue) {
		msg := fmt.Sprintf("%v is nil", testValue)
		if len(cm) > 0 {
			switch cm[0].(type) {
			case string:
				msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
			default:
				msg = msg + " - " + fmt.Sprint(cm...)
			}
		}
		return Fail(t, msg)
	}
	return true
}

// Fail performs t.Errorf(), but modifies the message with the file and line of the caller.
func Fail(t *testing.T, failureMessage string) bool {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		panic("unable to get caller file:line")
	}
	fileParts := strings.Split(file, "/")
	file = fileParts[len(fileParts)-1]
	// The "\r" here is what eliminates the default file:line portion of the message.
	t.Errorf("\r\t%s:%v: %s", file, line, failureMessage)

	return false
}

func isNil(val interface{}) bool {
	if val == nil {
		return true
	}

	// A bit involved for checking a nil-valued object passed via interface{}
	value := reflect.ValueOf(val)
	kind := value.Kind()

	// Only for types that are nil-able, check if they are nil.
	if kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil() {
		return true
	}

	return false
}
