package config

import (
	"math"
	"testing"

	"github.com/alexeydott/tegola/internal/env"
)

func TestFeatureProtocolConfigBudgets(t *testing.T) {
	for _, tc := range []FeaturesConfig{{MaxResponseBytes: env.IntPtr(0)}, {MaxResponseBytes: env.IntPtr(1023)}, {QueryTimeoutMS: env.IntPtr(0)}, {QueryTimeoutMS: env.IntPtr(-1)}, {QueryTimeoutMS: env.IntPtr(env.Int(math.MaxInt64))}} {
		if tc.Validate() == nil {
			t.Fatalf("invalid configuration accepted %+v", tc)
		}
	}
	original := FeaturesConfig{}
	r := original.Resolved()
	if *r.MaxResponseBytes != 16<<20 || *r.QueryTimeoutMS != 30000 {
		t.Fatal("defaults")
	}
	*r.MaxResponseBytes = 1024
	*r.QueryTimeoutMS = 1
	if original.MaxResponseBytes != nil || original.QueryTimeoutMS != nil {
		t.Fatal("input mutated")
	}
	if (FeaturesConfig{MaxResponseBytes: env.IntPtr(1024), QueryTimeoutMS: env.IntPtr(1)}).Validate() != nil {
		t.Fatal("valid budget rejected")
	}
}
