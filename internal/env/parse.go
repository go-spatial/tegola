package env

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/internal/p"
)

func ParseString(v interface{}) (*string, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}
		return &val, nil
	default:
		return nil, ErrType{v}
	}
}

func ParseStringSlice(val string) ([]string, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// split
	vals := strings.Split(str, ",")

	// trim space
	for i, v := range vals {
		vals[i] = strings.TrimSpace(v)
	}

	return vals, nil
}

func ParseBool(v interface{}) (*bool, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case bool:
		return &val, nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		b, err := strconv.ParseBool(val)
		return &b, err
	default:
		return nil, ErrType{v}
	}
}

func ParseBoolSlice(val string) ([]bool, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	bools := make([]bool, 0, len(vals))
	for i := range vals {
		// trim space and parse
		b, err := strconv.ParseBool(strings.TrimSpace(vals[i]))
		if err != nil {
			return bools, err
		}

		bools = append(bools, b)
	}

	return bools, nil
}

func ParseInt(v interface{}) (*int, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case int:
		return &val, nil
	case int64:
		i := int(val)
		return &i, nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		i, err := strconv.Atoi(val)
		return &i, err
	default:
		return nil, ErrType{v}
	}
}

func ParseIntSlice(val string) ([]int, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	ints := make([]int, 0, len(vals))
	for i := range vals {
		// trim space and parse
		b, err := strconv.Atoi(strings.TrimSpace(vals[i]))
		if err != nil {
			return ints, err
		}

		ints = append(ints, b)
	}

	return ints, nil
}

func ParseUint(v interface{}) (*uint, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case uint:
		return &val, nil
	case uint64:
		ui := uint(val)
		return &ui, nil
	case int:
		if val < 0 {
			return nil, ErrType{v}
		}
		return p.Uint(uint(val)), nil
	case int64:
		if val < 0 {
			return nil, ErrType{v}
		}
		return p.Uint(uint(val)), nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		ui64, err := strconv.ParseUint(val, 10, 64)
		ui := uint(ui64)
		return &ui, err
	default:
		return nil, ErrType{v}
	}
}

func ParseUintSlice(val string) ([]uint, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	uints := make([]uint, 0, len(vals))
	for i := range vals {
		// trim space and parse
		u, err := strconv.ParseUint(strings.TrimSpace(vals[i]), 10, 64)
		if err != nil {
			return uints, err
		}

		uints = append(uints, uint(u))
	}

	return uints, nil
}

func ParseFloat(v interface{}) (*float64, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case float64:
		return &val, nil
	case float32:
		f := float64(val)
		return &f, nil
	case int:
		f := float64(val)
		return &f, nil
	case int8:
		f := float64(val)
		return &f, nil
	case int16:
		f := float64(val)
		return &f, nil
	case int32:
		f := float64(val)
		return &f, nil
	case int64:
		f := float64(val)
		return &f, nil
	case uint:
		f := float64(val)
		return &f, nil
	case uint8:
		f := float64(val)
		return &f, nil
	case uint16:
		f := float64(val)
		return &f, nil
	case uint32:
		f := float64(val)
		return &f, nil
	case uint64:
		f := float64(val)
		return &f, nil
	case uintptr:
		f := float64(val)
		return &f, nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		flt, err := strconv.ParseFloat(val, 64)
		return &flt, err
	default:
		return nil, ErrType{v}
	}
}

func ParseFloatSlice(val string) ([]float64, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	floats := make([]float64, 0, len(vals))
	for i := range vals {
		// trim space and parse
		f, err := strconv.ParseFloat(strings.TrimSpace(vals[i]), 64)
		if err != nil {
			return floats, err
		}

		floats = append(floats, f)
	}

	return floats, nil
}

func ParseDict(v interface{}) (*Dict, error) {
	if v == nil {
		return nil, nil
	}

	var d = make(Dict)

	switch val := v.(type) {
	case map[string]interface{}:
		for k, v := range val {
			switch v.(type) {
			case string:
				s, err := ParseString(v)
				if err != nil {
					return nil, err
				}

				d[k] = *s
			case map[string]interface{}:
				i, err := ParseDict(v)
				if err != nil {
					return nil, err
				}
				d[k] = *i
			default:
				d[k] = v
			}
		}
	default:
		return nil, ErrType{v}
	}

	return &d, nil
}

// ParseURL parses a webserver hostname from its TOML representation. The value
// must be either a bare host[:port] (e.g. "example.com", "cdn.example.com:443")
// or a URL with a scheme (e.g. "https://example.com"). A bare host[:port] is
// normalized to a URL with only the Host component set.
//
// This is intentionally strict: a malformed webserver.hostname is reported as a
// fatal configuration error rather than being silently ignored. In particular,
// values such as "cdn.example.com:443" must not fall through to net/url's
// scheme:opaque parsing (which would leave Host empty and quietly drop the
// setting).
//
// An empty value ("", "   ", or an env placeholder that expands to an empty
// string) is not a configuration error: it matches upstream's behavior of an
// empty URL, where the server derives the host from the incoming request
// (see cmd/tegola/cmd/server.go, which only sets server.HostName when the
// parsed URL has a host). ParseURL returns (nil, nil) for it.
func ParseURL(v any) (*url.URL, error) {
	if v == nil {
		return nil, nil
	}

	val, ok := v.(string)
	if !ok {
		return nil, ErrType{v}
	}

	val, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// An empty hostname means "use the request host" (upstream behavior), not
	// a fatal config error. This keeps templated configs such as
	// `hostname = "${TEGOLA_HOSTNAME}"` working when the variable is set to an
	// empty string (common in Docker/Helm templates). Unset variables still
	// fail above with ErrEnvVar, matching upstream.
	if strings.TrimSpace(val) == "" {
		return nil, nil
	}

	return parseHostName(val)
}

// parseHostName validates a hostname of the form host[:port], or a URL with a
// scheme, and returns the equivalent *url.URL. It rejects anything that is
// neither so callers get a clear error instead of a silently dropped value.
func parseHostName(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("hostname: value is empty")
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return nil, fmt.Errorf("hostname %q: contains whitespace", s)
	}

	// A value carrying "://" is unambiguously a URL with a scheme; parse it with
	// the standard parser and require a host component.
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("hostname %q: %w", s, err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("hostname %q: URL has no host", s)
		}
		return u, nil
	}

	// Otherwise the value must be a bare host[:port]. Reject anything carrying
	// URL-only delimiters (path/query/fragment/userinfo) that cannot appear in a
	// bare host.
	if strings.ContainsAny(s, "/?#@") {
		return nil, fmt.Errorf("hostname %q: expected host[:port] or a URL with a scheme", s)
	}

	// Delegate host/port validation to net/url by parsing a synthetic authority.
	u, err := url.Parse("//" + s)
	if err != nil {
		return nil, fmt.Errorf("hostname %q: %w", s, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("hostname %q: expected host[:port]", s)
	}
	// Keep only the host (and port); discard anything else url.Parse inferred.
	return &url.URL{Host: u.Host}, nil
}
