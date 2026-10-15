package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/alexeydott/tegola/internal/env"
)

func TestFeaturesDefaultsAndOwnership(t *testing.T) {
	f := FeaturesConfig{Collections: []FeatureCollectionConfig{{ID: "public", ProviderLayer: "source.layer"}}}
	r := f.Resolved()
	if r.Enabled || r.BasePath != "/features" || *r.DefaultLimit != 100 || *r.MaxLimit != 10000 {
		t.Fatalf("defaults: %+v", r)
	}
	*r.DefaultLimit = 1
	r.Collections[0].ID = "changed"
	if f.DefaultLimit != nil || f.Collections[0].ID != "public" {
		t.Fatal("input mutated")
	}
	value := env.Int(20)
	f.DefaultLimit = &value
	r = f.Resolved()
	*r.DefaultLimit = 3
	if value != 20 {
		t.Fatal("limit retained")
	}
}

func TestFeatureGrammar(t *testing.T) {
	for _, path := range []string{"/features", "/ogc/v1", "/a-._~9"} {
		if err := ValidateFeatureBasePath(path); err != nil {
			t.Fatalf("valid %q: %v", path, err)
		}
	}
	for _, path := range []string{"", "/", "features", "/features/", "//features", "/a//b", "/a/.", "/../a", "/%66eatures", "/a?x", "/a#x", "/:id", "/a*", "/é", "/maps/a", "/capabilities", "/metrics"} {
		if err := ValidateFeatureBasePath(path); err == nil {
			t.Fatalf("invalid path %q accepted", path)
		}
	}
	for _, id := range []string{"roads", "Roads_9-~.x"} {
		if err := ValidateFeatureCollectionID(id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"", ".", "..", "a/b", "a%2Fb", "a b", "é", "a?b", "a#b", ":id"} {
		if err := ValidateFeatureCollectionID(id); err == nil {
			t.Fatalf("invalid ID %q accepted", id)
		}
	}
}

func TestFeaturesValidation(t *testing.T) {
	valid := FeatureCollectionConfig{ID: "public", ProviderLayer: "source.layer"}
	cases := map[string]FeaturesConfig{
		"enabled empty":         {Enabled: true},
		"zero default":          {DefaultLimit: env.IntPtr(0)},
		"negative max":          {MaxLimit: env.IntPtr(-1)},
		"default exceeds max":   {DefaultLimit: env.IntPtr(3), MaxLimit: env.IntPtr(2)},
		"disabled invalid path": {BasePath: "/maps"},
		"duplicate":             {Collections: []FeatureCollectionConfig{valid, valid}},
		"binding":               {Collections: []FeatureCollectionConfig{{ID: "public", ProviderLayer: "a.b.c"}}},
		"empty provider":        {Collections: []FeatureCollectionConfig{{ID: "public", ProviderLayer: ".layer"}}},
		"empty layer":           {Collections: []FeatureCollectionConfig{{ID: "public", ProviderLayer: "provider."}}},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	if err := (FeaturesConfig{Collections: []FeatureCollectionConfig{valid}}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (&Config{Features: cases["disabled invalid path"]}).Validate(); err == nil {
		t.Fatal("Config.Validate missed features")
	}
}

func TestFeaturesTOMLAndEnvironment(t *testing.T) {
	t.Setenv("FEATURE_BASE", "/ogc/v1")
	t.Setenv("FEATURE_LIMIT", "25")
	var cfg Config
	_, err := toml.Decode(`[features]
enabled = true
basepath = "${FEATURE_BASE}"
default_limit = "${FEATURE_LIMIT}"
max_limit = 50
title = "Features"
description = "Published data"
[[features.collections]]
id = "roads"
provider_layer = "data.roads"
title = "Roads"
description = "Road data"
`, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Features.Validate(); err != nil {
		t.Fatal(err)
	}
	f := cfg.Features.Resolved()
	if f.BasePath != "/ogc/v1" || *f.DefaultLimit != 25 || len(f.Collections) != 1 || f.Collections[0].Title != "Roads" {
		t.Fatalf("parse: %+v", f)
	}
	var invalid Config
	if _, err := toml.Decode("[features]\ndefault_limit = 0", &invalid); err != nil {
		t.Fatal(err)
	}
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "limits") {
		t.Fatalf("explicit zero lost: %v", err)
	}
}
