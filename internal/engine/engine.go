package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rs/xid"

	"github.com/getden/den/internal/config"
	"github.com/getden/den/internal/runtime"
	"github.com/getden/den/internal/store"
)

var (
	ErrNotFound     = errors.New("sandbox not found")
	ErrNotRunning   = errors.New("sandbox is not running")
	ErrLimitReached = errors.New("maximum sandbox limit reached")
)

// Engine orchestrates sandbox lifecycle.
type Engine struct {
	runtime      runtime.Runtime
	store        store.Store
	config       config.SandboxConfig
	sandboxes    sync.Map // map[string]*Sandbox
	mu           sync.Mutex
	count        int
	logger       *slog.Logger
	stopReaper   chan struct{}
	shutdownOnce sync.Once
}

// NewEngine creates a new Engine.
func NewEngine(rt runtime.Runtime, st store.Store, cfg config.SandboxConfig, logger *slog.Logger) *Engine {
	e := &Engine{
		runtime:    rt,
		store:      st,
		config:     cfg,
		logger:     logger,
		stopReaper: make(chan struct{}),
	}

	// Restore sandboxes from store
	e.restoreFromStore()

	// Start reaper goroutine
	go e.reaper()

	return e
}

// CreateSandbox creates and starts a new sandbox.
func (e *Engine) CreateSandbox(ctx context.Context, cfg runtime.SandboxConfig) (*Sandbox, error) {
	e.mu.Lock()
	if e.count >= e.config.MaxSandboxes {
		e.mu.Unlock()
		return nil, ErrLimitReached
	}
	e.count++
	e.mu.Unlock()

	// Apply defaults
	if cfg.Image == "" {
		cfg.Image = e.config.DefaultImage
	}
	if cfg.CPU == 0 {
		cfg.CPU = e.config.DefaultCPU
	}
	if cfg.Memory == 0 {
		cfg.Memory = e.config.DefaultMemory
	}
	if cfg.PidLimit == 0 {
		cfg.PidLimit = e.config.DefaultPidLimit
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}

	id := xid.New().String()

	sandbox := &Sandbox{
		ID:        id,
		Image:     cfg.Image,
		status:    runtime.StatusCreating,
		Config:    cfg,
		CreatedAt: time.Now().UTC(),
	}
	if timeout > 0 {
		sandbox.ExpiresAt = sandbox.CreatedAt.Add(timeout)
	}

	// Create container
	if err := e.runtime.Create(ctx, id, cfg); err != nil {
		e.decrementCount()
		return nil, fmt.Errorf("creating sandbox: %w", err)
	}

	// Start container
	if err := e.runtime.Start(ctx, id); err != nil {
		e.runtime.Remove(ctx, id)
		e.decrementCount()
		return nil, fmt.Errorf("starting sandbox: %w", err)
	}

	sandbox.SetStatus(runtime.StatusRunning)

	// Save to store
	e.sandboxes.Store(id, sandbox)
	if err := e.saveSandbox(sandbox); err != nil {
		e.logger.Warn("failed to save sandbox to store", "id", id, "error", err)
	}

	e.logger.Info("sandbox created", "id", id, "image", cfg.Image)
	return sandbox, nil
}

// GetSandbox returns a sandbox by ID.
func (e *Engine) GetSandbox(id string) (*Sandbox, error) {
	v, ok := e.sandboxes.Load(id)
	if !ok {
		return nil, ErrNotFound
	}
	return v.(*Sandbox), nil
}

// ListSandboxes returns all tracked sandboxes.
func (e *Engine) ListSandboxes() []*Sandbox {
	var sandboxes []*Sandbox
	e.sandboxes.Range(func(_, v any) bool {
		sandboxes = append(sandboxes, v.(*Sandbox))
		return true
	})
	return sandboxes
}

// StopSandbox stops a sandbox without removing it.
func (e *Engine) StopSandbox(ctx context.Context, id string) error {
	sandbox, err := e.GetSandbox(id)
	if err != nil {
		return err
	}

	if err := e.runtime.Stop(ctx, id, 10*time.Second); err != nil {
		return fmt.Errorf("stopping sandbox: %w", err)
	}

	sandbox.SetStatus(runtime.StatusStopped)
	if err := e.saveSandbox(sandbox); err != nil {
		e.logger.Warn("failed to save sandbox state after stop", "id", id, "error", err)
	}
	e.logger.Info("sandbox stopped", "id", id)
	return nil
}

// DestroySandbox stops and removes a sandbox.
func (e *Engine) DestroySandbox(ctx context.Context, id string) error {
	_, err := e.GetSandbox(id)
	if err != nil {
		return err
	}

	// Stop (ignore error if already stopped)
	e.runtime.Stop(ctx, id, 5*time.Second)

	// Remove
	if err := e.runtime.Remove(ctx, id); err != nil {
		return fmt.Errorf("removing sandbox: %w", err)
	}

	e.sandboxes.Delete(id)
	if err := e.store.DeleteSandbox(id); err != nil {
		e.logger.Warn("failed to delete sandbox from store", "id", id, "error", err)
	}
	e.decrementCount()

	e.logger.Info("sandbox destroyed", "id", id)
	return nil
}

// getRunning returns the sandbox if it exists and is running.
func (e *Engine) getRunning(id string) (*Sandbox, error) {
	sb, err := e.GetSandbox(id)
	if err != nil {
		return nil, err
	}
	if sb.GetStatus() != runtime.StatusRunning {
		return nil, ErrNotRunning
	}
	return sb, nil
}

// Exec runs a command in a sandbox.
func (e *Engine) Exec(ctx context.Context, id string, opts runtime.ExecOpts) (*runtime.ExecResult, error) {
	if _, err := e.getRunning(id); err != nil {
		return nil, err
	}
	return e.runtime.Exec(ctx, id, opts)
}

// ExecStream runs a streaming command in a sandbox.
func (e *Engine) ExecStream(ctx context.Context, id string, opts runtime.ExecOpts) (runtime.ExecStream, error) {
	if _, err := e.getRunning(id); err != nil {
		return nil, err
	}
	return e.runtime.ExecStream(ctx, id, opts)
}

// ReadFile reads a file from a sandbox.
func (e *Engine) ReadFile(ctx context.Context, id string, path string) ([]byte, error) {
	if _, err := e.getRunning(id); err != nil {
		return nil, err
	}
	return e.runtime.ReadFile(ctx, id, path)
}

// WriteFile writes a file to a sandbox.
func (e *Engine) WriteFile(ctx context.Context, id string, path string, content []byte) error {
	if _, err := e.getRunning(id); err != nil {
		return err
	}
	return e.runtime.WriteFile(ctx, id, path, content)
}

// ListDir lists a directory in a sandbox.
func (e *Engine) ListDir(ctx context.Context, id string, path string) ([]runtime.FileInfo, error) {
	if _, err := e.getRunning(id); err != nil {
		return nil, err
	}
	return e.runtime.ListDir(ctx, id, path)
}

// MkDir creates a directory in a sandbox.
func (e *Engine) MkDir(ctx context.Context, id string, path string) error {
	if _, err := e.getRunning(id); err != nil {
		return err
	}
	return e.runtime.MkDir(ctx, id, path)
}

// RemoveFile removes a file from a sandbox.
func (e *Engine) RemoveFile(ctx context.Context, id string, path string) error {
	if _, err := e.getRunning(id); err != nil {
		return err
	}
	return e.runtime.RemoveFile(ctx, id, path)
}

// Snapshot creates a snapshot of a sandbox.
func (e *Engine) Snapshot(ctx context.Context, id string, name string) (*runtime.SnapshotInfo, error) {
	if _, err := e.GetSandbox(id); err != nil {
		return nil, err
	}
	snap, err := e.runtime.Snapshot(ctx, id, name)
	if err != nil {
		return nil, err
	}
	// Save to store
	if err := e.store.SaveSnapshot(&store.SnapshotRecord{
		ID:        snap.ID,
		SandboxID: snap.SandboxID,
		Name:      snap.Name,
		ImageID:   snap.ImageID,
		CreatedAt: snap.CreatedAt,
	}); err != nil {
		e.logger.Warn("failed to save snapshot to store", "id", snap.ID, "error", err)
	}
	return snap, nil
}

// RestoreSnapshot restores a sandbox from a snapshot.
func (e *Engine) RestoreSnapshot(ctx context.Context, snapshotID string) (*Sandbox, error) {
	// Enforce sandbox limit
	e.mu.Lock()
	if e.count >= e.config.MaxSandboxes {
		e.mu.Unlock()
		return nil, ErrLimitReached
	}
	e.count++
	e.mu.Unlock()

	snapRecord, err := e.store.GetSnapshot(snapshotID)
	if err != nil {
		// Try runtime directly
		newID, restoreErr := e.runtime.Restore(ctx, snapshotID)
		if restoreErr != nil {
			e.decrementCount()
			return nil, restoreErr
		}
		sandbox := &Sandbox{
			ID:        newID,
			status:    runtime.StatusRunning,
			CreatedAt: time.Now().UTC(),
		}
		e.sandboxes.Store(newID, sandbox)
		return sandbox, nil
	}

	newID, err := e.runtime.Restore(ctx, snapshotID)
	if err != nil {
		e.decrementCount()
		return nil, err
	}

	sandbox := &Sandbox{
		ID:        newID,
		Image:     snapRecord.ImageID,
		status:    runtime.StatusRunning,
		CreatedAt: time.Now().UTC(),
	}

	e.sandboxes.Store(newID, sandbox)
	if err := e.saveSandbox(sandbox); err != nil {
		e.logger.Warn("failed to save restored sandbox to store", "id", newID, "error", err)
	}

	e.logger.Info("sandbox restored from snapshot", "id", newID, "snapshot", snapshotID)
	return sandbox, nil
}

// ListSnapshots returns all snapshots for a sandbox.
func (e *Engine) ListSnapshots(ctx context.Context, sandboxID string) ([]runtime.SnapshotInfo, error) {
	return e.runtime.ListSnapshots(ctx, sandboxID)
}

// DeleteSnapshot removes a snapshot.
func (e *Engine) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	if err := e.runtime.RemoveSnapshot(ctx, snapshotID); err != nil {
		return err
	}
	if err := e.store.DeleteSnapshot(snapshotID); err != nil {
		e.logger.Warn("failed to delete snapshot from store", "id", snapshotID, "error", err)
	}
	return nil
}

// Stats returns stats for a sandbox.
func (e *Engine) Stats(ctx context.Context, id string) (*runtime.SandboxStats, error) {
	if _, err := e.GetSandbox(id); err != nil {
		return nil, err
	}
	return e.runtime.Stats(ctx, id)
}

// Shutdown gracefully stops all sandboxes.
func (e *Engine) Shutdown(ctx context.Context) {
	e.shutdownOnce.Do(func() {
		close(e.stopReaper)

		// Collect IDs first to avoid concurrent modification during Range
		var ids []string
		e.sandboxes.Range(func(key, _ any) bool {
			ids = append(ids, key.(string))
			return true
		})

		for _, id := range ids {
			if err := e.DestroySandbox(ctx, id); err != nil {
				e.logger.Warn("failed to destroy sandbox on shutdown", "id", id, "error", err)
			}
		}
	})
}

func (e *Engine) reaper() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.reapExpired()
		case <-e.stopReaper:
			return
		}
	}
}

func (e *Engine) reapExpired() {
	ctx := context.Background()
	e.sandboxes.Range(func(key, value any) bool {
		sandbox := value.(*Sandbox)
		if sandbox.IsExpired() {
			e.logger.Info("reaping expired sandbox", "id", sandbox.ID)
			if err := e.DestroySandbox(ctx, sandbox.ID); err != nil {
				e.logger.Warn("failed to reap sandbox", "id", sandbox.ID, "error", err)
			}
		}
		return true
	})
}

func (e *Engine) restoreFromStore() {
	records, err := e.store.ListSandboxes()
	if err != nil {
		e.logger.Warn("failed to list sandboxes from store", "error", err)
		return
	}

	ctx := context.Background()
	restored := 0

	e.mu.Lock()
	for _, r := range records {
		// Verify container still exists in Docker
		info, err := e.runtime.Info(ctx, r.ID)
		if err != nil {
			e.logger.Warn("removing stale sandbox from store (container not found)", "id", r.ID)
			if delErr := e.store.DeleteSandbox(r.ID); delErr != nil {
				e.logger.Warn("failed to delete stale sandbox from store", "id", r.ID, "error", delErr)
			}
			continue
		}

		sandbox := &Sandbox{
			ID:        r.ID,
			Image:     r.Image,
			status:    info.Status,
			CreatedAt: r.CreatedAt,
			ExpiresAt: r.ExpiresAt,
		}
		e.sandboxes.Store(r.ID, sandbox)
		e.count++
		restored++
	}
	e.mu.Unlock()

	if restored > 0 {
		e.logger.Info("restored sandboxes from store", "count", restored)
	}
}

func (e *Engine) saveSandbox(sandbox *Sandbox) error {
	return e.store.SaveSandbox(&store.SandboxRecord{
		ID:        sandbox.ID,
		Image:     sandbox.Image,
		Status:    string(sandbox.GetStatus()),
		CreatedAt: sandbox.CreatedAt,
		ExpiresAt: sandbox.ExpiresAt,
	})
}

func (e *Engine) decrementCount() {
	e.mu.Lock()
	e.count--
	if e.count < 0 {
		e.count = 0
	}
	e.mu.Unlock()
}
