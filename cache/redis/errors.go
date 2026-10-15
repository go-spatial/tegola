package redis

import (
	"fmt"
	"strings"
)

// ErrUnsupportedConfigKeys is returned when the cache config still sets
// connection keys that were removed in favour of the 'uri' key.
type ErrUnsupportedConfigKeys struct {
	// Keys holds the unsupported config keys found, in legacyConfigKeys order.
	Keys []string
}

func (e *ErrUnsupportedConfigKeys) Error() string {
	quoted := make([]string, len(e.Keys))
	for i, key := range e.Keys {
		quoted[i] = "'" + key + "'"
	}

	keyWord, isAre, pronoun := "key", "is", "it"
	if len(e.Keys) > 1 {
		keyWord, isAre, pronoun = "keys", "are", "them"
	}

	return fmt.Sprintf(
		"cache/redis: the %s %s %s no longer supported; replace %s with uri = \"redis://:<password>@<host>:<port>/<db>\", e.g. before: address = \"127.0.0.1:6379\", password = \"secret\", db = 3, ssl = false. after: uri = \"redis://:secret@127.0.0.1:6379/3\" (schemes: redis://, rediss://, unix://; see cache/redis/README.md)",
		strings.Join(quoted, ", "), keyWord, isAre, pronoun)
}

// ErrURIMissing is returned when the cache config has no usable 'uri' key.
type ErrURIMissing struct{}

func (e *ErrURIMissing) Error() string {
	return "cache/redis: the 'uri' config key is required and must not be empty, e.g. uri = \"redis://127.0.0.1:6379/0\" (schemes: redis://, rediss://, unix://; see cache/redis/README.md)"
}

// ErrInvalidURI wraps the error of a 'uri' that could not be parsed. It never
// echoes the configured URI, which may contain credentials.
type ErrInvalidURI struct {
	Err error
}

func (e *ErrInvalidURI) Error() string {
	return fmt.Sprintf(
		"cache/redis: could not parse the 'uri' config key (%v), expected e.g. uri = \"redis://:<password>@<host>:<port>/<db>\" (schemes: redis://, rediss://, unix://; see cache/redis/README.md)",
		e.Err)
}

func (e *ErrInvalidURI) Unwrap() error {
	return e.Err
}
