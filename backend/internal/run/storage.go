package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"testforge/internal/platform/apperr"
)

// Storage keeps run artifacts. Local disk today; S3 later behind the same interface.
type Storage interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// LocalStorage stores artifacts under a directory.
type LocalStorage struct {
	Dir string
}

func (l LocalStorage) path(key string) (string, error) {
	clean := filepath.Clean("/" + key)
	if strings.Contains(key, "..") || clean == "/" {
		return "", fmt.Errorf("invalid artifact key %q", key)
	}
	return filepath.Join(l.Dir, clean), nil
}

// Put writes an artifact atomically.
func (l LocalStorage) Put(_ context.Context, key string, data []byte) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("create artifact dir: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}
	return os.Rename(tmp, p)
}

// Get reads an artifact.
func (l LocalStorage) Get(_ context.Context, key string) ([]byte, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, apperr.NotFound("Audio not found.")
	}
	return data, err
}
