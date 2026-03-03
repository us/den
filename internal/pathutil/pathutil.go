package pathutil

import (
	"errors"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversal = errors.New("path traversal not allowed")
	ErrNotAbsolute   = errors.New("absolute path required")
	ErrNullByte      = errors.New("path contains null byte")
)

// ValidatePath checks that a path is absolute and does not escape the filesystem root.
// It rejects path traversal attempts, null bytes, and relative paths.
func ValidatePath(path string) error {
	// Reject null bytes which can truncate paths in C-based systems
	if strings.ContainsRune(path, 0) {
		return ErrNullByte
	}

	// Must be absolute
	if !filepath.IsAbs(path) {
		return ErrNotAbsolute
	}

	// Clean and resolve the path
	cleaned := filepath.Clean(path)

	// After cleaning, verify it's still absolute (Clean preserves absolute paths,
	// but we double-check for safety)
	if !filepath.IsAbs(cleaned) {
		return ErrPathTraversal
	}

	// Split into components and reject any ".." that survived cleaning
	// (filepath.Clean resolves ".." but we check anyway for defense in depth)
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return ErrPathTraversal
		}
	}

	// Verify the cleaned path doesn't escape root by checking that
	// it can be made relative to "/" without going up
	rel, err := filepath.Rel("/", cleaned)
	if err != nil {
		return ErrPathTraversal
	}
	if strings.HasPrefix(rel, "..") {
		return ErrPathTraversal
	}

	return nil
}
