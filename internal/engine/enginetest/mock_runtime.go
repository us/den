package enginetest

import (
	"context"
	"sync"
	"time"

	"github.com/us/den/internal/runtime"
)

// MockRuntime implements runtime.Runtime for testing.
type MockRuntime struct {
	mu      sync.Mutex
	Created map[string]runtime.SandboxConfig
	Started map[string]bool
	Stopped map[string]bool
	Removed map[string]bool
	ExecFn  func(ctx context.Context, id string, opts runtime.ExecOpts) (*runtime.ExecResult, error)
}

// NewMockRuntime creates a new MockRuntime.
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		Created: make(map[string]runtime.SandboxConfig),
		Started: make(map[string]bool),
		Stopped: make(map[string]bool),
		Removed: make(map[string]bool),
	}
}

func (m *MockRuntime) Ping(ctx context.Context) error { return nil }

func (m *MockRuntime) Create(ctx context.Context, id string, cfg runtime.SandboxConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Created[id] = cfg
	return nil
}

func (m *MockRuntime) Start(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Started[id] = true
	return nil
}

func (m *MockRuntime) Stop(ctx context.Context, id string, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Stopped[id] = true
	return nil
}

func (m *MockRuntime) Remove(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Removed[id] = true
	return nil
}

func (m *MockRuntime) Info(ctx context.Context, id string) (*runtime.SandboxInfo, error) {
	return &runtime.SandboxInfo{ID: id, Status: runtime.StatusRunning}, nil
}

func (m *MockRuntime) List(ctx context.Context) ([]runtime.SandboxInfo, error) {
	return nil, nil
}

func (m *MockRuntime) Exec(ctx context.Context, id string, opts runtime.ExecOpts) (*runtime.ExecResult, error) {
	if m.ExecFn != nil {
		return m.ExecFn(ctx, id, opts)
	}
	return &runtime.ExecResult{ExitCode: 0, Stdout: "ok\n"}, nil
}

func (m *MockRuntime) ExecStream(ctx context.Context, id string, opts runtime.ExecOpts) (runtime.ExecStream, error) {
	return nil, nil
}

func (m *MockRuntime) ReadFile(ctx context.Context, id string, path string) ([]byte, error) {
	return []byte("content"), nil
}

func (m *MockRuntime) WriteFile(ctx context.Context, id string, path string, content []byte) error {
	return nil
}

func (m *MockRuntime) ListDir(ctx context.Context, id string, path string) ([]runtime.FileInfo, error) {
	return nil, nil
}

func (m *MockRuntime) MkDir(ctx context.Context, id string, path string) error {
	return nil
}

func (m *MockRuntime) RemoveFile(ctx context.Context, id string, path string) error {
	return nil
}

func (m *MockRuntime) Snapshot(ctx context.Context, id string, name string) (*runtime.SnapshotInfo, error) {
	return &runtime.SnapshotInfo{ID: "snap-1", SandboxID: id, Name: name}, nil
}

func (m *MockRuntime) Restore(ctx context.Context, snapshotID string) (string, error) {
	return "restored-id", nil
}

func (m *MockRuntime) ListSnapshots(ctx context.Context, sandboxID string) ([]runtime.SnapshotInfo, error) {
	return nil, nil
}

func (m *MockRuntime) RemoveSnapshot(ctx context.Context, snapshotID string) error {
	return nil
}

func (m *MockRuntime) Stats(ctx context.Context, id string) (*runtime.SandboxStats, error) {
	return &runtime.SandboxStats{CPUPercent: 5.0, MemoryUsage: 1024}, nil
}

func (m *MockRuntime) UpdateMemoryLimit(ctx context.Context, id string, memoryBytes int64) error {
	return nil
}
