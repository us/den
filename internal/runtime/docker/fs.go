package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"

	"github.com/us/den/internal/runtime"
)

// ReadFile reads a file from the container using Docker's CopyFromContainer API.
// This is significantly faster than exec-based cat approach.
func (r *DockerRuntime) ReadFile(ctx context.Context, id string, path string) ([]byte, error) {
	containerName := r.containerName(id)

	reader, _, err := r.cli.CopyFromContainer(ctx, containerName, path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s in %s: %w", path, id, err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	if _, err := tr.Next(); err != nil {
		return nil, fmt.Errorf("reading tar header for %s in %s: %w", path, id, err)
	}

	var buf bytes.Buffer
	limited := io.LimitReader(tr, maxReadFileSize+1)
	n, err := buf.ReadFrom(limited)
	if err != nil {
		return nil, fmt.Errorf("reading file content %s in %s: %w", path, id, err)
	}
	if n > maxReadFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", n, maxReadFileSize)
	}

	return buf.Bytes(), nil
}

// WriteFile writes a file to the container using Docker's CopyToContainer API.
// This is significantly faster than exec-based tee approach.
func (r *DockerRuntime) WriteFile(ctx context.Context, id string, path string, content []byte) error {
	containerName := r.containerName(id)
	dir := filepath.Dir(path)
	base := filepath.Base(path)

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

	// Create tar archive with the file
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: base,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("writing tar header for %s: %w", path, err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("writing tar content for %s: %w", path, err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar for %s: %w", path, err)
	}

	// Copy tar to container
	err = r.cli.CopyToContainer(ctx, containerName, dir, &buf, container.CopyToContainerOptions{})
	if err != nil {
		return fmt.Errorf("writing file %s in %s: %w", path, id, err)
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
