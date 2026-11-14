package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type DiskStore struct {
	root      string
	publicURL string
}

func NewDiskStore(root, publicURL string) (*DiskStore, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}

	return &DiskStore{
		root:      root,
		publicURL: strings.TrimRight(publicURL, "/"),
	}, nil
}

func (s *DiskStore) Put(_ context.Context, key, contentType string, body io.Reader, size int64) error {
	if err := validateKey(key); err != nil {
		return err
	}

	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	written, err := io.Copy(tmp, io.LimitReader(body, size+1))
	if err != nil {
		return err
	}
	if written != size {
		return fmt.Errorf("unexpected size")
	}
	if err = tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), path)
}

func (s *DiskStore) Delete(_ context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	path := filepath.Join(s.root, filepath.FromSlash(key))
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (s *DiskStore) URL(key string) string {
	if key == "" || s.publicURL == "" {
		return ""
	}

	return s.publicURL + "/" + strings.TrimPrefix(key, "/")
}

func validateKey(key string) error {
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return fmt.Errorf("invalid media key")
	}

	return nil
}
