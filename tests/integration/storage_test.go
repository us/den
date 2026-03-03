//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getden/den/internal/config"
	"github.com/getden/den/internal/engine"
	"github.com/getden/den/internal/runtime"
	"github.com/getden/den/internal/runtime/docker"
	"github.com/getden/den/internal/storage"
	"github.com/getden/den/internal/store"
)

func getMinIOEndpoint() string {
	if ep := os.Getenv("MINIO_ENDPOINT"); ep != "" {
		return ep
	}
	return "http://localhost:9000"
}

func getTestS3Config() config.S3Config {
	return config.S3Config{
		Endpoint:  getMinIOEndpoint(),
		Region:    "us-east-1",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
	}
}

func newIntegrationEngine(t *testing.T) *engine.Engine {
	t.Helper()

	dir := t.TempDir()
	st, err := store.NewBoltStore(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rt, err := docker.New(docker.WithLogger(logger))
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, rt.Ping(ctx))
	rt.EnsureNetwork(ctx)

	cfg := config.SandboxConfig{
		DefaultImage:         "alpine:latest",
		DefaultTimeout:       5 * time.Minute,
		MaxSandboxes:         10,
		DefaultCPU:           1_000_000_000,
		DefaultMemory:        256 * 1024 * 1024,
		DefaultPidLimit:      256,
		AllowVolumes:         true,
		AllowSharedVolumes:   true,
		AllowS3:              true,
		MaxVolumesPerSandbox: 5,
		DefaultTmpfs: []config.TmpfsDefault{
			{Path: "/tmp", Size: "64m"},
			{Path: "/home/sandbox", Size: "64m"},
			{Path: "/run", Size: "32m"},
			{Path: "/var/tmp", Size: "32m"},
		},
	}

	eng := engine.NewEngine(rt, st, cfg, getTestS3Config(), logger)
	t.Cleanup(func() { eng.Shutdown(context.Background()) })

	return eng
}

func TestIntegration_ConfigurableTmpfs(t *testing.T) {
	eng := newIntegrationEngine(t)
	ctx := context.Background()

	// Create sandbox with custom tmpfs size
	sb, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{
		Storage: &runtime.StorageConfig{
			Tmpfs: []runtime.TmpfsMount{
				{Path: "/tmp", Size: "128m"},
			},
		},
	})
	require.NoError(t, err)

	// Check tmpfs size inside container
	result, err := eng.Exec(ctx, sb.ID, runtime.ExecOpts{
		Cmd: []string{"df", "-h", "/tmp"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "128")
}

func TestIntegration_VolumePersistence(t *testing.T) {
	eng := newIntegrationEngine(t)
	ctx := context.Background()

	volumeName := "integration-test-persist"

	// Sandbox A: write data to volume
	sbA, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{
		Storage: &runtime.StorageConfig{
			Volumes: []runtime.VolumeMount{
				{Name: volumeName, MountPath: "/data"},
			},
		},
	})
	require.NoError(t, err)

	result, err := eng.Exec(ctx, sbA.ID, runtime.ExecOpts{
		Cmd: []string{"sh", "-c", "echo 'persistent data' > /data/test.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)

	// Destroy sandbox A
	require.NoError(t, eng.DestroySandbox(ctx, sbA.ID))

	// Sandbox B: read data from same volume
	sbB, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{
		Storage: &runtime.StorageConfig{
			Volumes: []runtime.VolumeMount{
				{Name: volumeName, MountPath: "/data"},
			},
		},
	})
	require.NoError(t, err)

	result, err = eng.Exec(ctx, sbB.ID, runtime.ExecOpts{
		Cmd: []string{"cat", "/data/test.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "persistent data")
}

func TestIntegration_SharedVolume(t *testing.T) {
	eng := newIntegrationEngine(t)
	ctx := context.Background()

	volumeName := "integration-test-shared"

	// Create two sandboxes with the same volume
	sbA, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{
		Storage: &runtime.StorageConfig{
			Volumes: []runtime.VolumeMount{
				{Name: volumeName, MountPath: "/shared"},
			},
		},
	})
	require.NoError(t, err)

	sbB, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{
		Storage: &runtime.StorageConfig{
			Volumes: []runtime.VolumeMount{
				{Name: volumeName, MountPath: "/shared", ReadOnly: true},
			},
		},
	})
	require.NoError(t, err)

	// Write from sandbox A
	result, err := eng.Exec(ctx, sbA.ID, runtime.ExecOpts{
		Cmd: []string{"sh", "-c", "echo 'shared data' > /shared/test.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)

	// Read from sandbox B
	result, err = eng.Exec(ctx, sbB.ID, runtime.ExecOpts{
		Cmd: []string{"cat", "/shared/test.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "shared data")
}

func TestIntegration_ReadOnlyVolume(t *testing.T) {
	eng := newIntegrationEngine(t)
	ctx := context.Background()

	volumeName := "integration-test-readonly"

	// Create sandbox with read-only volume
	sb, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{
		Storage: &runtime.StorageConfig{
			Volumes: []runtime.VolumeMount{
				{Name: volumeName, MountPath: "/rodata", ReadOnly: true},
			},
		},
	})
	require.NoError(t, err)

	// Try to write — should fail
	result, err := eng.Exec(ctx, sb.ID, runtime.ExecOpts{
		Cmd: []string{"sh", "-c", "echo test > /rodata/test.txt"},
	})
	require.NoError(t, err)
	assert.NotEqual(t, 0, result.ExitCode)
}

func TestIntegration_S3OnDemandImport(t *testing.T) {
	eng := newIntegrationEngine(t)
	ctx := context.Background()
	logger := slog.Default()

	sb, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{})
	require.NoError(t, err)

	// Create S3 client
	s3Cfg := getTestS3Config()
	creds, err := storage.ResolveS3Credentials(&runtime.S3SyncConfig{
		Endpoint:  s3Cfg.Endpoint,
		Bucket:    "test-bucket",
		AccessKey: s3Cfg.AccessKey,
		SecretKey: s3Cfg.SecretKey,
	}, s3Cfg)
	require.NoError(t, err)

	client, err := storage.NewS3Client(ctx, creds, logger)
	require.NoError(t, err)

	// Download from MinIO
	body, _, err := client.Download(ctx, "test-bucket", "test-data/hello.txt")
	require.NoError(t, err)

	buf, err := io.ReadAll(body)
	body.Close()
	require.NoError(t, err)

	// Write to sandbox
	require.NoError(t, eng.WriteFile(ctx, sb.ID, "/tmp/hello.txt", buf))

	// Verify file exists in sandbox
	result, err := eng.Exec(ctx, sb.ID, runtime.ExecOpts{
		Cmd: []string{"cat", "/tmp/hello.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello from s3")
}

func TestIntegration_S3Export(t *testing.T) {
	eng := newIntegrationEngine(t)
	ctx := context.Background()
	logger := slog.Default()

	sb, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{})
	require.NoError(t, err)

	// Write file in sandbox
	testContent := []byte("exported data from sandbox")
	require.NoError(t, eng.WriteFile(ctx, sb.ID, "/tmp/export-test.txt", testContent))

	// Upload to MinIO
	s3Cfg := getTestS3Config()
	creds, err := storage.ResolveS3Credentials(&runtime.S3SyncConfig{
		Endpoint:  s3Cfg.Endpoint,
		Bucket:    "test-bucket",
		AccessKey: s3Cfg.AccessKey,
		SecretKey: s3Cfg.SecretKey,
	}, s3Cfg)
	require.NoError(t, err)

	client, err := storage.NewS3Client(ctx, creds, logger)
	require.NoError(t, err)

	// Read from sandbox and upload
	data, err := eng.ReadFile(ctx, sb.ID, "/tmp/export-test.txt")
	require.NoError(t, err)

	err = client.Upload(ctx, "test-bucket", "exports/test-output.txt", bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	// Verify by downloading
	body, _, err := client.Download(ctx, "test-bucket", "exports/test-output.txt")
	require.NoError(t, err)
	defer body.Close()

	downloaded, err := io.ReadAll(body)
	require.NoError(t, err)

	assert.Equal(t, string(testContent), string(downloaded))
}

func TestIntegration_S3Hooks(t *testing.T) {
	eng := newIntegrationEngine(t)
	ctx := context.Background()

	s3Cfg := getTestS3Config()

	// Create sandbox with S3 hooks mode
	sb, err := eng.CreateSandbox(ctx, runtime.SandboxConfig{
		Storage: &runtime.StorageConfig{
			S3: &runtime.S3SyncConfig{
				Endpoint:  s3Cfg.Endpoint,
				Bucket:    "test-bucket",
				Prefix:    "test-data/",
				AccessKey: s3Cfg.AccessKey,
				SecretKey: s3Cfg.SecretKey,
				Mode:      runtime.S3SyncModeHooks,
				SyncPath:  "/home/sandbox",
			},
		},
	})
	require.NoError(t, err)

	// Check if file was downloaded by hook
	result, err := eng.Exec(ctx, sb.ID, runtime.ExecOpts{
		Cmd: []string{"cat", "/home/sandbox/hello.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello from s3")
}
