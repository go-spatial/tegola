package config

import "testing"

func TestWriteAuthFailsClosedBeforeValidation(t *testing.T) {
	for _, mode := range []string{"", "production", "prodution", "Production", " dev "} {
		if !(FeaturesWriteConfig{AuthMode: mode}).IsProductionAuth() {
			t.Errorf("mode %q permits anonymous writes before validation", mode)
		}
	}
	if (FeaturesWriteConfig{AuthMode: "dev"}).IsProductionAuth() {
		t.Fatal("explicit dev profile must remain available")
	}
}
