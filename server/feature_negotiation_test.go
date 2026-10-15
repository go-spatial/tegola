package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeatureAcceptQuality(t *testing.T) {
	for _, tc := range []struct {
		values []string
		want   bool
	}{
		{nil, true}, {[]string{"*/*"}, true}, {[]string{"application/*"}, true}, {[]string{"application/geo+json"}, true},
		{[]string{"application/json"}, false}, {[]string{"text/html"}, false},
		{[]string{"application/geo+json;q=0, */*;q=1"}, false},
		{[]string{"application/*;q=0, */*;q=1"}, false},
		{[]string{"application/geo+json;q=0", "application/geo+json;q=0.5"}, true},
		{[]string{"application/geo+json;q=0.001"}, true},
		{[]string{"application/geo+json;q=0.0001"}, false},
		{[]string{"application/geo+json;q=NaN"}, false},
		{[]string{"application/geo+json;q=1.1"}, false},
		{[]string{"application/geo+json;q=-1"}, false},
		{[]string{"application/geo+json;q=1;extension=\"a,b\""}, true},
		{[]string{"application/geo+json;profile=\"a,b\";q=1"}, false},
		{[]string{"application/geo+json;q=1,"}, true}, {[]string{",,application/geo+json,,"}, true},
		{[]string{""}, false},
	} {
		if got := featureAccepts(tc.values, "application/geo+json"); got != tc.want {
			t.Fatalf("Accept=%v got=%v want=%v", tc.values, got, tc.want)
		}
	}
}

func TestFeatureNegotiationBeforeQuery(t *testing.T) {
	api := discoveryAPI(t)
	for _, method := range []string{"GET", "HEAD"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/features/collections/alpha/items", nil)
		request.Header.Add("Accept", "*/*;q=1")
		request.Header.Add("Accept", "application/geo+json;q=0, text/html;q=0")
		// The discovery querier errors if called, so a 406 also proves early rejection.
		api.negotiate(http.HandlerFunc(api.serveItems), "application/geo+json").ServeHTTP(response, request)
		if response.Code != 406 {
			t.Fatalf("status %d", response.Code)
		}
		if method == "HEAD" && response.Body.Len() != 0 {
			t.Fatal("HEAD has body")
		}
	}
}

func TestFeatureOpenAPIParameterNegotiation(t *testing.T) {
	for _, tc := range []struct {
		accept string
		want   bool
	}{
		{"application/vnd.oai.openapi+json", true},
		{"application/vnd.oai.openapi+json;version=3.0", true},
		{"application/vnd.oai.openapi+json;version=3.1", false},
		{"application/vnd.oai.openapi+json;version=3.0;q=0, application/vnd.oai.openapi+json;q=1", false},
		{"application/vnd.oai.openapi+json;version=3.1;q=0, */*;q=1", true},
	} {
		if got := featureAccepts([]string{tc.accept}, "application/vnd.oai.openapi+json;version=3.0"); got != tc.want {
			t.Fatalf("Accept %s: got %v", tc.accept, got)
		}
	}
}

func TestFeatureAcceptRejectsQuotedAndDuplicateWeights(t *testing.T) {
	for _, accept := range []string{
		`application/geo+json;q="0.5"`,
		`application/geo+json;q="0"`,
		`application/geo+json;q=0.5;q=0.5`,
		`application/geo+json;q=0.5;Q=0.5`,
		`application/geo+json;q=0.5;q=1`,
		`application/geo+json;q=1;note="a,b";q=1`,
	} {
		if featureAccepts([]string{accept}, "application/geo+json") {
			t.Fatalf("invalid raw weight accepted: %s", accept)
		}
	}
	if !featureAccepts([]string{`application/geo+json;q=0.5,application/geo+json;q=1`}, "application/geo+json") {
		t.Fatal("separate ranges incorrectly treated as duplicate weights")
	}
}
