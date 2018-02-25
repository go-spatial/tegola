package ttools

import (
	"os"
	"strings"
	"testing"
)

const YES = "yes"

func ShouldSkip(t *testing.T, env string) {

	if os.Getenv(env) != YES {
		msg := env
		if strings.HasPrefix(env, "RUN_") {
			msg = msg[4:]
		}
		t.Skipf("%v NOT ENABLED", strings.Replace(msg, "_", " ", -1))
	}

}
