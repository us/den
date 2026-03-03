package runtime

import (
	"context"
	"io"
	"time"
)

// SandboxStatus represents the current state of a sandbox.
type SandboxStatus string

const (
	StatusCreating SandboxStatus = "creating"
	StatusRunning  SandboxStatus = "running"
	StatusStopped  SandboxStatus = "stopped"
	StatusError    SandboxStatus = "error"
)

// PortMapping defines a port forwarding between host and sandbox.
type PortMapping struct {
	SandboxPort int    `json:"sandbox_port"`
	HostPort    int    `json:"host_port"`
	Protocol    string `json:"protocol,omitempty"` // "tcp" (default) or "udp"
}

// VolumeMount defines a named volume to mount into a sandbox.
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	ReadOnly  bool   `json:"read_only,omitempty"`
}

// TmpfsMount defines a tmpfs filesystem to mount inside a sandbox.
type TmpfsMount struct {
	Path    string `json:"path"`
	Size    string `json:"size"`             // "256m", "1g"
	Options string `json:"options,omitempty"` // "rw,noexec,nosuid"
}

// S3SyncMode determines how S3 synchronization is performed.
type S3SyncMode string

const (
	S3SyncModeHooks    S3SyncMode = "hooks"
	S3SyncModeFUSE     S3SyncMode = "fuse"
	S3SyncModeOnDemand S3SyncMode = "on_demand"
)

// S3SyncConfig holds S3 synchronization settings for a sandbox.
type S3SyncConfig struct {
	Endpoint  string     `json:"endpoint,omitempty"`
	Bucket    string     `json:"bucket"`
	Prefix    string     `json:"prefix,omitempty"`
	Region    string     `json:"region,omitempty"`
	AccessKey string     `json:"access_key,omitempty"`
	SecretKey string     `json:"secret_key,omitempty"`
	Mode      S3SyncMode `json:"mode"`
	MountPath string     `json:"mount_path,omitempty"` // FUSE: "/mnt/s3"
	SyncPath  string     `json:"sync_path,omitempty"`  // Hooks: local path to sync
}

// StorageConfig holds storage settings for a sandbox.
type StorageConfig struct {
	Volumes []VolumeMount `json:"volumes,omitempty"`
	Tmpfs   []TmpfsMount  `json:"tmpfs,omitempty"`
	S3      *S3SyncConfig `json:"s3,omitempty"`
}

// SandboxConfig holds configuration for creating a new sandbox.
type SandboxConfig struct {
	Image      string            `json:"image"`
	Env        map[string]string `json:"env,omitempty"`
	WorkDir    string            `json:"workdir,omitempty"`
	Cmd        []string          `json:"cmd,omitempty"`
	CPU        int64             `json:"cpu,omitempty"`        // NanoCPUs (1e9 = 1 core)
	Memory     int64             `json:"memory,omitempty"`     // bytes
	DiskLimit  int64             `json:"disk_limit,omitempty"` // bytes
	PidLimit   int64             `json:"pid_limit,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty"`
	Ports      []PortMapping     `json:"ports,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	NetworkID  string            `json:"-"`
	ReadOnlyFS bool              `json:"readonly_fs,omitempty"`
	Storage    *StorageConfig    `json:"storage,omitempty"`
	TmpfsMap   map[string]string `json:"-"` // Computed by engine from storage config + defaults
}

// SandboxInfo holds runtime information about a sandbox.
type SandboxInfo struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Status    SandboxStatus     `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	Ports     []PortMapping     `json:"ports,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Pid       int               `json:"pid,omitempty"`
}

// ExecOpts configures command execution inside a sandbox.
type ExecOpts struct {
	Cmd     []string          `json:"cmd"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"workdir,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`
	Stdin   io.Reader         `json:"-"`
}

// ExecResult holds the result of a synchronous command execution.
type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// ExecStreamMessage represents a single message from a streaming exec.
type ExecStreamMessage struct {
	Type string `json:"type"` // "stdout", "stderr", "exit"
	Data string `json:"data"`
}

// ExecStream allows reading streaming output from command execution.
type ExecStream interface {
	// Recv returns the next message from the stream.
	// Returns io.EOF when the stream is finished.
	Recv() (ExecStreamMessage, error)
	// Close terminates the stream.
	Close() error
}

// FileInfo holds metadata about a file inside a sandbox.
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

// SnapshotInfo holds metadata about a sandbox snapshot.
type SnapshotInfo struct {
	ID        string    `json:"id"`
	SandboxID string    `json:"sandbox_id"`
	Name      string    `json:"name"`
	ImageID   string    `json:"image_id"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

// SandboxStats holds resource usage statistics for a sandbox.
type SandboxStats struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsage   int64     `json:"memory_usage"`   // bytes
	MemoryLimit   int64     `json:"memory_limit"`   // bytes
	MemoryPercent float64   `json:"memory_percent"`
	NetworkRx     int64     `json:"network_rx"`     // bytes
	NetworkTx     int64     `json:"network_tx"`     // bytes
	DiskRead      int64     `json:"disk_read"`      // bytes
	DiskWrite     int64     `json:"disk_write"`     // bytes
	PidCount      int64     `json:"pid_count"`
	Timestamp     time.Time `json:"timestamp"`
}

// Runtime defines the interface for sandbox backend implementations.
type Runtime interface {
	// Ping verifies connectivity to the runtime backend.
	Ping(ctx context.Context) error

	// Lifecycle
	Create(ctx context.Context, id string, cfg SandboxConfig) error
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string, timeout time.Duration) error
	Remove(ctx context.Context, id string) error
	Info(ctx context.Context, id string) (*SandboxInfo, error)
	List(ctx context.Context) ([]SandboxInfo, error)

	// Execution
	Exec(ctx context.Context, id string, opts ExecOpts) (*ExecResult, error)
	ExecStream(ctx context.Context, id string, opts ExecOpts) (ExecStream, error)

	// File operations
	ReadFile(ctx context.Context, id string, path string) ([]byte, error)
	WriteFile(ctx context.Context, id string, path string, content []byte) error
	ListDir(ctx context.Context, id string, path string) ([]FileInfo, error)
	MkDir(ctx context.Context, id string, path string) error
	RemoveFile(ctx context.Context, id string, path string) error

	// Snapshots
	Snapshot(ctx context.Context, id string, name string) (*SnapshotInfo, error)
	Restore(ctx context.Context, snapshotID string) (string, error)
	ListSnapshots(ctx context.Context, sandboxID string) ([]SnapshotInfo, error)
	RemoveSnapshot(ctx context.Context, snapshotID string) error

	// Stats
	Stats(ctx context.Context, id string) (*SandboxStats, error)
}
