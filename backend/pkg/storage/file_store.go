package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileStore saves uploaded files to disk under a base directory.
type FileStore struct {
	basePath string
}

// NewFileStore creates the base directory if missing.
func NewFileStore(basePath string) (*FileStore, error) {
	if strings.TrimSpace(basePath) == "" {
		return nil, fmt.Errorf("storage base path is required")
	}
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &FileStore{basePath: basePath}, nil
}

// Save writes an uploaded file under a book-specific folder.
func (f *FileStore) Save(bookID, filename string, r io.Reader) error {
	targetDir := filepath.Join(f.basePath, bookID)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create book dir: %w", err)
	}
	target := filepath.Join(targetDir, safeFilename(filename))

	out, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// Delete removes all files for a book.
func (f *FileStore) Delete(bookID string) error {
	targetDir := filepath.Join(f.basePath, bookID)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(targetDir)
}

func safeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "book"
	}
	return name
}
