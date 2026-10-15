package core_test

import (
	"testing"

	"github.com/go-spatial/proj/core"
	// register the projection operations NewSystem looks up
	_ "github.com/go-spatial/proj/operations"
	"github.com/go-spatial/proj/support"
)

func newTestSystem(t *testing.T, proj4 string) (*core.System, core.IOperation, error) {
	t.Helper()
	ps, err := support.NewProjString(proj4)
	if err != nil {
		t.Fatalf("NewProjString(%q) returned error: %v", proj4, err)
	}
	return core.NewSystem(ps)
}

// TestNewSystemRejectsUnparseableParameters guards P6-8: when a parameter key
// is present but its value does not parse as a finite float, NewSystem must
// return an error instead of silently treating the parameter as zero. A comma
// decimal separator ("+lon_0=39,5") previously produced lon_0=0 and a
// geometrically wrong but apparently valid system.
func TestNewSystemRejectsUnparseableParameters(t *testing.T) {
	tests := []struct {
		name  string
		proj4 string
	}{
		{"comma decimal separator", "+proj=merc +lon_0=39,5 +ellps=WGS84"},
		{"lat_0 not a number", "+proj=merc +lat_0=abc +ellps=WGS84"},
		{"x_0 not a number", "+proj=merc +x_0=1d30 +ellps=WGS84"},
		{"y_0 not a number", "+proj=merc +y_0=12,0 +ellps=WGS84"},
		{"z_0 not a number", "+proj=merc +z_0=? +ellps=WGS84"},
		{"t_0 not a number", "+proj=merc +t_0=x +ellps=WGS84"},
		{"lon_wrap not a number", "+proj=merc +lon_wrap=east +ellps=WGS84"},
		{"k_0 not a number", "+proj=merc +k_0=big +ellps=WGS84"},
		{"k not a number", "+proj=merc +k=big +ellps=WGS84"},
		{"k_0 NaN", "+proj=merc +k_0=nan +ellps=WGS84"},
		{"lon_0 Inf", "+proj=merc +lon_0=inf +ellps=WGS84"},
		{"y_0 out of range", "+proj=merc +y_0=1e320 +ellps=WGS84"},
	}
	for _, tc := range tests {
		sys, _, err := newTestSystem(t, tc.proj4)
		if err == nil {
			t.Errorf("%s: NewSystem(%q) accepted an unparseable parameter (Lam0=%v Phi0=%v K0=%v); want error",
				tc.name, tc.proj4, sys.Lam0, sys.Phi0, sys.K0)
		}
	}
}

// TestNewSystemAcceptsValidParameters ensures strict parsing does not reject
// well-formed definitions.
func TestNewSystemAcceptsValidParameters(t *testing.T) {
	tests := []string{
		"+proj=merc +lon_0=10 +lat_0=5 +k_0=0.9 +x_0=100 +y_0=-200 +ellps=WGS84",
		"+proj=merc +k=1 +ellps=WGS84",
		"+proj=merc +lon_wrap=3.14 +ellps=WGS84",
		"+proj=merc +lon_0=0 +lat_0=0 +ellps=WGS84",
	}
	for _, proj4 := range tests {
		if _, _, err := newTestSystem(t, proj4); err != nil {
			t.Errorf("NewSystem(%q) returned error: %v", proj4, err)
		}
	}
}
