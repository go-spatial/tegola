package ttools

import (
	"os"
)

// GetEnvDefault looks up an environment variable, and return a default value
// when it is not present
func GetEnvDefault(key, dvalue string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return dvalue
	}

	return v
}
