package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// R2Storage keeps photos in a Cloudflare R2 bucket (S3-compatible). The bucket is public
// read-only at PublicURL (an r2.dev address or a custom domain such as media.yetibyrash.com),
// so photos load straight from Cloudflare's network instead of through the app.
type R2Storage struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

// R2Config describes the bucket. Endpoint is optional (default: Cloudflare's for the account);
// tests point it at another S3-compatible server.
type R2Config struct {
	AccountID, AccessKeyID, SecretAccessKey, Bucket, PublicURL string
	Endpoint                                                   string
	Insecure                                                   bool // plain http, local tests only
}

func NewR2Storage(c R2Config) (*R2Storage, error) {
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = c.AccountID + ".r2.cloudflarestorage.com"
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.AccessKeyID, c.SecretAccessKey, ""),
		Secure: !c.Insecure,
		Region: "auto",
	})
	if err != nil {
		return nil, fmt.Errorf("r2 client: %w", err)
	}
	return &R2Storage{client: client, bucket: c.Bucket, publicURL: strings.TrimRight(c.PublicURL, "/")}, nil
}

// Check confirms the bucket exists and the keys work, so a bad setting fails at start-up
// instead of on the first photo upload.
func (s *R2Storage) Check(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("r2: can't reach bucket %q: %w", s.bucket, err)
	}
	if !ok {
		return fmt.Errorf("r2: bucket %q doesn't exist", s.bucket)
	}
	return nil
}

func validKey(key string) bool {
	clean := path.Clean("/" + key)
	return clean != "/" && !strings.Contains(key, "..") && !strings.ContainsAny(key, `\:`)
}

func (s *R2Storage) Put(ctx context.Context, key string, r io.Reader) error {
	if !validKey(key) {
		return fmt.Errorf("invalid media key %q", key)
	}
	contentType := "application/octet-stream"
	if strings.HasSuffix(key, ".jpg") {
		contentType = "image/jpeg"
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, r, -1, minio.PutObjectOptions{
		ContentType: contentType,
		// Every photo gets a new random name, so a stored file never changes.
		CacheControl: "public, max-age=31536000, immutable",
	})
	if err != nil {
		return fmt.Errorf("r2 put %s: %w", key, err)
	}
	return nil
}

func (s *R2Storage) Delete(ctx context.Context, key string) error {
	if !validKey(key) {
		return fmt.Errorf("invalid media key %q", key)
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// Handler sends /media/<key> requests to the bucket's public address, so photo links
// saved anywhere (emails, old pages) keep working.
func (s *R2Storage) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/media/")
		if !validKey(key) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.Redirect(w, r, s.publicURL+"/"+key, http.StatusFound)
	})
}

// PublicURL is where photos are served from (for the Content-Security-Policy).
func (s *R2Storage) PublicURL() string { return s.publicURL }
