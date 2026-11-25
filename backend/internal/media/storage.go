// Package media stores product photos and turns uploads into square, resized JPEGs.
package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Storage saves and removes files by key, e.g. "products/12/k3j2-800.jpg".
// LocalStorage is used in development; an S3-compatible store (Cloudflare R2) is added for launch (M7).
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Delete(ctx context.Context, key string) error
	// Handler serves stored files at /media/<key>.
	Handler() http.Handler
}

// LocalStorage keeps files in a folder on disk.
type LocalStorage struct {
	dir string
}

func NewLocalStorage(dir string) (*LocalStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create media folder %s: %w", dir, err)
	}
	return &LocalStorage{dir: dir}, nil
}

func (s *LocalStorage) path(key string) (string, error) {
	clean := path.Clean("/" + key)
	if clean == "/" || strings.Contains(key, "..") || strings.ContainsAny(key, `\:`) {
		return "", fmt.Errorf("invalid media key %q", key)
	}
	return filepath.Join(s.dir, filepath.FromSlash(clean)), nil
}

func (s *LocalStorage) Put(_ context.Context, key string, r io.Reader) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	// Write to a temp file and rename, so a half-written photo is never served.
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalStorage) Handler() http.Handler {
	fs := http.StripPrefix("/media/", http.FileServer(noDirListing{http.Dir(s.dir)}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Photo files never change (a new upload gets a new key), so browsers can cache them for a long time.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fs.ServeHTTP(w, r)
	})
}

// noDirListing hides folder listings: /media/products/ returns 404 instead of a file list.
type noDirListing struct{ fs http.FileSystem }

func (n noDirListing) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err == nil && st.IsDir() {
		f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}
