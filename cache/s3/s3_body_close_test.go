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

// closeTrackingBody wraps a reader and records whether Close was ever called,
// so tests can detect a leaked response body.
type closeTrackingBody struct {
	reader io.Reader
	closed bool
}

func (b *closeTrackingBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

// stubRoundTripper returns a canned response whose body tracks Close calls.
type stubRoundTripper struct {
	body *closeTrackingBody
}

func (rt *stubRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       rt.body,
	}, nil
}

// TestGetClosesResponseBody guards against leaking the S3 GetObject response
// body: after Get returns (on both success and read-error paths) the body must
// have been closed.
func TestGetClosesResponseBody(t *testing.T) {
	const want = "tile-bytes"
	body := &closeTrackingBody{reader: strings.NewReader(want)}

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Credentials:      credentials.NewStaticCredentials("key", "secret", ""),
		Endpoint:         aws.String("http://localhost"),
		S3ForcePathStyle: aws.Bool(true),
		HTTPClient:       &http.Client{Transport: &stubRoundTripper{body: body}},
	})
	if err != nil {
		t.Fatalf("session.NewSession: %v", err)
	}

	cacher := &s3.Cache{
		Bucket: "test-bucket",
		Client: awss3.New(sess),
	}

	key := cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 2, Y: 3}
	got, hit, err := cacher.Get(context.Background(), &key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !hit {
		t.Fatalf("Get hit = false, want true")
	}
	if string(got) != want {
		t.Errorf("Get value = %q, want %q", got, want)
	}
	if !body.closed {
		t.Errorf("Get response body was not closed (connection leak)")
	}
}
