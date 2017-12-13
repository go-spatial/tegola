package assert

// Testing check shortcuts

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Log message and fail if actualValue doesn't match expectedValue
// *Note that DeepEqual will check the value pointed to by two pointers, ie don't use for checking
//	pointer equality.
func Equal(t *testing.T, expectedValue interface{}, actualValue interface{}, cm ...interface{}) bool {
	if !reflect.DeepEqual(expectedValue, actualValue) {
		msg := fmt.Sprintf("%v != %v (expected)", actualValue, expectedValue)
		if len(cm) > 0 {
			msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
		}
		return Fail(t, msg)
	}
	return true
}

// Log message and fail if testValue is not true.
func True(t *testing.T, testValue bool, cm ...interface{}) bool {
	if !testValue {
		msg := fmt.Sprintf("%v is false", testValue)
		if len(cm) > 0 {
			msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
		}
		return Fail(t, msg)
	}
	return true
}

// Log message and fail if testValue is not nil.
func Nil(t *testing.T, testValue interface{}, cm ...interface{}) bool {
	if !isNil(testValue) {
		msg := fmt.Sprintf("%v (%T) is not nil", testValue, testValue)
		if len(cm) > 0 {
			msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
		}
		return Fail(t, msg)
	}
	return true
}

// Log message and fail if testValue is nil.
func NotNil(t *testing.T, testValue interface{}, cm ...interface{}) bool {
	if isNil(testValue) {
		msg := fmt.Sprintf("%v is nil", testValue)
		if len(cm) > 0 {
			msg = msg + " - " + fmt.Sprintf(cm[0].(string), cm[1:]...)
		}
		return Fail(t, msg)
	}
	return true
}

// Fail performs t.Errorf(), but modifies the message with the file and line of the caller.
func Fail(t *testing.T, failureMessage string) bool {
	//	content := [][2]string{
	//		{"Error Trace", strings.Join(CallerInfo(), "\n\r\t\t\t")},
	//		{"Error", failureMessage},
	//	}
	_, file, line, _ := runtime.Caller(2)
	fileParts := strings.Split(file, "/")
	file = fileParts[len(fileParts)-1]
	// The "\r" here is what eliminates the default file:line portion of the message.
	t.Errorf("\r\t%s:%v: %s", file, line, failureMessage)

	return false
}

func isNil(object interface{}) bool {
	if object == nil {
		return true
	}

	// A little more involved for a nil-valued object passed via interface{}
	value := reflect.ValueOf(object)
	kind := value.Kind()
	// Only for types that are nil-able, check if they are nil.
	if kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil() {
		return true
	}

	return false
}
