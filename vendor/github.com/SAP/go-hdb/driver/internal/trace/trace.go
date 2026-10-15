// Package trace provides helpers to format values for protocol trace output.
package trace

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"sync/atomic"
)

const (
	// limit is the maximum number of units printed in full: runes for strings,
	// bytes for byte slices. Values longer than limit are cut.
	limit = 64
	// keep is the number of units printed when a value exceeds limit.
	keep = 25
)

var redact atomic.Bool

func init() { redact.Store(true) } // masked by default

// Redact reports whether credential redaction in trace output is enabled.
func Redact() bool { return redact.Load() }

// SetRedact enables or disables credential redaction in trace output.
func SetRedact(on bool) { redact.Store(on) }

// RedactedText is the placeholder text replacing credentials in redacted
// output. Kept consistent with net/url.URL.Redacted, whose "xxxxx" literal is
// unexported.
const RedactedText = "xxxxx"

// Redacted returns RedactedText, or the value rendered via Cut when redaction
// is disabled.
func Redacted(v any) string {
	if redact.Load() {
		return RedactedText
	}
	return Cut(v)
}

// Cut formats value for a trace output. Strings are cut by runes and byte
// slices by bytes once they exceed limit. The "(n)" suffix reports the length
// of the value before cutting.
func Cut(v any) string {
	switch v := v.(type) {
	case string:
		return cutString(v)
	case []byte:
		return cutBytes(v)
	default:
		return cutString(fmt.Sprintf("%v", v))
	}
}

func cutString(s string) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:keep]) + " ... (" + strconv.Itoa(len(r)) + ")"
}

func cutBytes(b []byte) string {
	if len(b) <= limit {
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b[:keep]) + " ... (" + strconv.Itoa(len(b)) + ")"
}
