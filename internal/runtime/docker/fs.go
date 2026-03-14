package docker

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/us/den/internal/runtime"
)

// ReadFile reads a file from the container using exec-based cat approach.
// Uses exec instead of CopyFromContainer because the latter fails with read-only rootfs.
func (r *DockerRuntime) ReadFile(ctx context.Context, id string, path string) ([]byte, error) {
	result, err := r.Exec(ctx, id, runtime.ExecOpts{
		Cmd: []string{"cat", path},
	})
	if err != nil {
		return nil, fmt.Errorf("reading file %s in %s: %w", path, id, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("reading file %s in %s: %s", path, id, result.Stderr)
	}

	content := []byte(result.Stdout)
	if int64(len(content)) > maxReadFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(content), maxReadFileSize)
	}

	return content, nil
}

// WriteFile writes a file to the container using exec-based approach.
// Uses exec (base64 + tee) instead of CopyToContainer because the latter
// fails with read-only rootfs. Exec works through the container's mount
// namespace and handles tmpfs correctly.
func (r *DockerRuntime) WriteFile(ctx context.Context, id string, path string, content []byte) error {
	dir := filepath.Dir(path)

	// Ensure parent directory exists
	mkdirResult, err := r.Exec(ctx, id, runtime.ExecOpts{
		Cmd: []string{"mkdir", "-p", dir},
	})
	if err != nil {
		return fmt.Errorf("creating dir for %s in %s: %w", path, id, err)
	}
	if mkdirResult.ExitCode != 0 {
		return fmt.Errorf("creating dir for %s in %s: %s", path, id, mkdirResult.Stderr)
	}

	// Write via base64-encoded exec to handle binary content safely
	encoded := base64.StdEncoding.EncodeToString(content)
	result, err := r.Exec(ctx, id, runtime.ExecOpts{
		Cmd: []string{"sh", "-c", fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, path)},
	})
	if err != nil {
		return fmt.Errorf("writing file %s in %s: %w", path, id, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("writing file %s in %s: %s", path, id, result.Stderr)
	}
	return nil
}

// ListDir lists the contents of a directory in the container.
func (r *DockerRuntime) ListDir(ctx context.Context, id string, path string) ([]runtime.FileInfo, error) {
	result, err := r.Exec(ctx, id, runtime.ExecOpts{
		Cmd: []string{"ls", "-la", "--time-style=full-iso", path},
	})
	if err != nil {
		return nil, fmt.Errorf("listing dir %s in %s: %w", path, id, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("listing dir %s in %s: %s", path, id, result.Stderr)
	}

	return parseLsOutput(result.Stdout, path), nil
}

// MkDir creates a directory in the container.
func (r *DockerRuntime) MkDir(ctx context.Context, id string, path string) error {
	result, err := r.Exec(ctx, id, runtime.ExecOpts{
		Cmd: []string{"mkdir", "-p", path},
	})
	if err != nil {
		return fmt.Errorf("mkdir %s in %s: %w", path, id, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("mkdir %s in %s: %s", path, id, result.Stderr)
	}
	return nil
}

// RemoveFile removes a file or directory from the container.
func (r *DockerRuntime) RemoveFile(ctx context.Context, id string, path string) error {
	result, err := r.Exec(ctx, id, runtime.ExecOpts{
		Cmd: []string{"rm", "-rf", path},
	})
	if err != nil {
		return fmt.Errorf("remove %s in %s: %w", path, id, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remove %s in %s: %s", path, id, result.Stderr)
	}
	return nil
}

const maxReadFileSize = 100 * 1024 * 1024 // 100MB

func parseLsOutput(output string, basePath string) []runtime.FileInfo {
	var files []runtime.FileInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "total") || strings.HasSuffix(line, " .") || strings.HasSuffix(line, " ..") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		name := strings.Join(fields[8:], " ")
		isDir := strings.HasPrefix(fields[0], "d")
		mode := fields[0]

		var size int64
		fmt.Sscanf(fields[4], "%d", &size)

		files = append(files, runtime.FileInfo{
			Name:  name,
			Path:  filepath.Join(basePath, name),
			Size:  size,
			Mode:  mode,
			IsDir: isDir,
		})
	}
	return files
}
