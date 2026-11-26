package media

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestR2Storage runs against any S3-compatible server. Locally:
//
//	docker run -d -p 9090:9090 -e COM_ADOBE_TESTING_S3MOCK_STORE_INITIAL_BUCKETS=photos adobe/s3mock
//	S3_TEST_ENDPOINT=localhost:9090 go test ./internal/media -run R2
func TestR2Storage(t *testing.T) {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set S3_TEST_ENDPOINT to an S3-compatible server to run this test")
	}
	ctx := context.Background()
	s, err := NewR2Storage(R2Config{
		AccessKeyID: "test", SecretAccessKey: "test", Bucket: "photos",
		PublicURL: "https://media.example.test/", Endpoint: endpoint, Insecure: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Check(ctx); err != nil {
		t.Fatal(err)
	}
	missing, _ := NewR2Storage(R2Config{AccessKeyID: "test", SecretAccessKey: "test", Bucket: "no-such-bucket", Endpoint: endpoint, Insecure: true})
	if err := missing.Check(ctx); err == nil {
		t.Fatal("a missing bucket passed the check")
	}

	key := "products/1/test-" + time.Now().Format("150405") + "-800.jpg"
	if err := s.Put(ctx, key, strings.NewReader("jpeg bytes")); err != nil {
		t.Fatal(err)
	}
	// Read it back straight from the server.
	res, err := http.Get("http://" + endpoint + "/photos/" + key)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != "jpeg bytes" || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("stored %q as %q", body, res.Header.Get("Content-Type"))
	}

	// /media/<key> goes to the public address; bad keys don't.
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+key, nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://media.example.test/"+key {
		t.Fatalf("redirect: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/../secret", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bad key: %d", rec.Code)
	}
	if err := s.Put(ctx, "../escape.jpg", strings.NewReader("x")); err == nil {
		t.Fatal("a key with .. was stored")
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	res, err = http.Get("http://" + endpoint + "/photos/" + key)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("after delete: %d", res.StatusCode)
	}
}
