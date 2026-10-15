package wfs

import (
	"context"
	"testing"

	"github.com/alexeydott/tegola/ogc/cql2"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type typedFESSource struct {
	reviewQuerySource
	kind     provider.QueryableType
	received *provider.FilterExpression
}

func TestPR203RejectsInvalidBeforeProviderQuery(t *testing.T) {
	for _, tc := range []struct {
		kind           provider.QueryableType
		property, text string
	}{
		{provider.QueryableBoolean, "v", "yes"},
		{provider.QueryableDate, "v", "2026-02-30"},
		{provider.QueryableTimestamp, "v", "tomorrow"},
		{provider.QueryableNumber, "v", "NaN"},
		{provider.QueryableNumber, "v", "123abc"},
		{provider.QueryableString, "missing", "123"},
	} {
		source := &typedFESSource{kind: tc.kind}
		service, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: source, Querier: source}})
		if err != nil {
			t.Fatal(err)
		}
		req, ex := ParseGetFeatureKVP(V202, map[string]string{"typename": "sites", "filter": `<Filter><PropertyIsEqualTo><ValueReference>` + tc.property + `</ValueReference><Literal>` + tc.text + `</Literal></PropertyIsEqualTo></Filter>`})
		if len(ex) > 0 {
			t.Fatal(ex)
		}
		if _, ex = ExecuteGetFeature(context.Background(), service, req); len(ex) == 0 || ex[0].Code != ExceptionInvalidParameterValue {
			t.Fatalf("accepted incompatible %v %q: %v", tc.kind, tc.text, ex)
		}
		if source.received != nil {
			t.Fatal("invalid filter reached provider")
		}
	}
}

func TestPR203MixedTypesAndBetween(t *testing.T) {
	raw := `<Filter><And><PropertyIsEqualTo><ValueReference>code</ValueReference><Literal>00123</Literal></PropertyIsEqualTo><PropertyIsEqualTo><ValueReference>active</ValueReference><Literal>1</Literal></PropertyIsEqualTo><PropertyIsBetween><ValueReference>amount</ValueReference><LowerBoundary><Literal>12345678901234567890.1234567890</Literal></LowerBoundary><UpperBoundary><Literal>12345678901234567890.1234567891</Literal></UpperBoundary></PropertyIsBetween></And></Filter>`
	parsed, err := ParseFESFilter([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := provider.NewFeatureQueryables([]provider.FeatureQueryable{{Name: "code", Type: provider.QueryableString}, {Name: "active", Type: provider.QueryableBoolean}, {Name: "amount", Type: provider.QueryableNumber}})
	if err != nil {
		t.Fatal(err)
	}
	bound, err := parsed.Resolve(catalog)
	if err != nil {
		t.Fatal(err)
	}
	children := bound.Root().Children
	if children[0].Literal.Text() != "00123" || children[0].Literal.Type() != provider.FilterString || children[1].Literal.Text() != "true" || children[1].Literal.Type() != provider.FilterBoolean {
		t.Fatalf("bad mixed literals %#v", children)
	}
	lo, hi := children[2].Children[0].Literal, children[2].Children[1].Literal
	lower, ok := lo.Number()
	if !ok {
		t.Fatal("lower not number")
	}
	upper, ok := hi.Number()
	if !ok || lower.Cmp(upper) >= 0 {
		t.Fatal("decimal precision lost")
	}
	// Resolving is repeatable and never mutates the lexical expression.
	if parsed.Root().Children[1].Literal.Text() != "1" {
		t.Fatal("resolver mutated raw lexical text")
	}
	if _, err := parsed.Resolve(catalog); err != nil {
		t.Fatal(err)
	}
}

func TestPR203CQL2TypingRemainsStrict(t *testing.T) {
	catalog, err := provider.NewFeatureQueryables([]provider.FeatureQueryable{{Name: "code", Type: provider.QueryableString}})
	if err != nil {
		t.Fatal(err)
	}
	numeric, err := cql2.Parse("code = 123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ResolveFeatureFilter(numeric, catalog); err == nil {
		t.Fatal("CQL2 numeric literal implicitly coerced to string")
	}
	text, err := cql2.Parse("code = '00123'")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ResolveFeatureFilter(text, catalog); err != nil {
		t.Fatal(err)
	}
}

func (s *typedFESSource) FeatureQueryables() (provider.FeatureQueryables, error) {
	return provider.NewFeatureQueryables([]provider.FeatureQueryable{{Name: "v", Type: s.kind}})
}

func (s *typedFESSource) DescribeSchema(context.Context, string) (provider.SchemaDescriptor, error) {
	return provider.SchemaDescriptor{IDColumn: "fid", Columns: []provider.ColumnDescriptor{{Name: "v", Type: "TEXT"}}, Geometry: provider.GeometryColumnDescriptor{Name: "geom", SRID: 4326}}, nil
}

func (s *typedFESSource) QueryFeatures(_ context.Context, _ string, q provider.FeatureQuery, _ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	s.received = q.Filter
	return provider.FeatureQueryResult{}, nil
}

func TestPR203LiteralBindsToQueryable(t *testing.T) {
	cases := []struct {
		name, text  string
		queryType   provider.QueryableType
		literalType provider.FilterScalarType
	}{
		{"text-control", "abc", provider.QueryableString, provider.FilterString},
		{"numeric-text", "123", provider.QueryableString, provider.FilterString},
		{"leading-zero-text", "00123", provider.QueryableString, provider.FilterString},
		{"exponent-text", "1e3", provider.QueryableString, provider.FilterString},
		{"whitespace-text", "  123  ", provider.QueryableString, provider.FilterString},
		{"integer", "9007199254740993", provider.QueryableInteger, provider.FilterNumber},
		{"decimal", "12345678901234567890.12345678901234567890", provider.QueryableNumber, provider.FilterNumber},
		{"boolean-true", "true", provider.QueryableBoolean, provider.FilterBoolean},
		{"boolean-false", "false", provider.QueryableBoolean, provider.FilterBoolean},
		{"date", "2026-10-04", provider.QueryableDate, provider.FilterDate},
		{"timestamp", "2026-10-04T08:00:00.123456789123Z", provider.QueryableTimestamp, provider.FilterTimestamp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := &typedFESSource{kind: tc.queryType}
			service, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: source, Querier: source}})
			if err != nil {
				t.Fatal(err)
			}
			params := map[string]string{"typename": "sites", "filter": `<Filter><PropertyIsEqualTo><ValueReference>v</ValueReference><Literal>` + xmlEscape(tc.text) + `</Literal></PropertyIsEqualTo></Filter>`}
			req, ex := ParseGetFeatureKVP(V202, params)
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			if _, ex = ExecuteGetFeature(context.Background(), service, req); len(ex) > 0 {
				t.Fatal(ex)
			}
			if source.received == nil {
				t.Fatal("provider did not receive filter")
			}
			literal := source.received.Root().Literal
			if literal.Type() != tc.literalType || literal.Text() != tc.text {
				t.Fatalf("received type=%v text=%q", literal.Type(), literal.Text())
			}
			// GetPropertyValue must use the same typed path.
			source.received = nil
			params["valuereference"] = "v"
			propertyReq, ex := ParseGetPropertyValueKVP(V202, params)
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			if _, ex = ExecuteGetPropertyValue(context.Background(), service, propertyReq); len(ex) > 0 {
				t.Fatal(ex)
			}
			if source.received == nil || source.received.Root().Literal.Type() != tc.literalType {
				t.Fatal("GetPropertyValue lost typed filter")
			}
		})
	}
}
