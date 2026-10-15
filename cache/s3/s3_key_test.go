package s3_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"

	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/s3"
)

// ctxRoundTripper honors request context cancellation the way http.Transport
// does, and records the URL paths of the requests it actually services.
type ctxRoundTripper struct {
	paths []string
}

func (rt *ctxRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	rt.paths = append(rt.paths, req.URL.Path)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func newKeyTestCache(t *testing.T, transport *ctxRoundTripper) (*s3.Cache, *ctxRoundTripper) {
	t.Helper()

	rt := transport
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Credentials:      credentials.NewStaticCredentials("key", "secret", ""),
		Endpoint:         aws.String("http://localhost"),
		S3ForcePathStyle: aws.Bool(true),
		HTTPClient:       &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("session.NewSession: %v", err)
	}

	return &s3.Cache{
		Bucket:   "test-bucket",
		Basepath: "mybase",
		Client:   awss3.New(sess),
	}, rt
}

// P6-33 regression: S3 object keys must be built with forward slashes on
// every OS. filepath.Join produced backslash keys on Windows, corrupting the
// object namespace shared with non-windows writers and readers.
func TestObjectKeysUseForwardSlashes(t *testing.T) {
	cacher, rt := newKeyTestCache(t, &ctxRoundTripper{})
	key := cache.Key{MapName: "map", LayerName: "layer", Z: 0, X: 1, Y: 2}

	ctx := context.Background()
	if err := cacher.Set(ctx, &key, []byte("tile")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, _, err := cacher.Get(ctx, &key); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := cacher.Purge(ctx, &key); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	if len(rt.paths) != 3 {
		t.Fatalf("issued %d requests, want 3 (Set, Get, Purge); paths: %v", len(rt.paths), rt.paths)
	}
	for i, p := range rt.paths {
		if strings.Contains(p, `\`) {
			t.Errorf("request %d key %q contains backslashes", i, p)
		}
		if !strings.HasSuffix(p, "/mybase/map/layer/0/1/2") {
			t.Errorf("request %d key = %q, want suffix %q", i, p, "/mybase/map/layer/0/1/2")
		}
	}
}

// P6-33 regression: Set must honor context cancellation. Pre-fix Set called
// PutObject without a context, so a cancelled caller context was ignored and
// the upload proceeded against a dead request.
func TestSetHonorsContextCancellation(t *testing.T) {
	cacher, rt := newKeyTestCache(t, &ctxRoundTripper{})
	key := cache.Key{MapName: "map", LayerName: "layer", Z: 0, X: 1, Y: 2}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := cacher.Set(ctx, &key, []byte("tile")); err == nil {
		t.Error("Set with canceled context returned nil, want error")
	}
	if len(rt.paths) != 0 {
		t.Errorf("Set with canceled context serviced %d requests, want 0", len(rt.paths))
	}
}
