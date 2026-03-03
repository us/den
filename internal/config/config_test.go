package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "docker", cfg.Runtime.Backend)
	assert.Equal(t, "den/default:latest", cfg.Sandbox.DefaultImage)
	assert.Equal(t, 30*time.Minute, cfg.Sandbox.DefaultTimeout)
	assert.Equal(t, 50, cfg.Sandbox.MaxSandboxes)
	assert.Equal(t, "den.db", cfg.Store.Path)
	assert.Equal(t, false, cfg.Auth.Enabled)
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
server:
  host: "127.0.0.1"
  port: 9090
runtime:
  backend: "docker"
sandbox:
  default_image: "custom:latest"
  max_sandboxes: 100
store:
  path: "/tmp/test.db"
auth:
  enabled: true
  api_keys:
    - "key-123"
    - "key-456"
log:
  level: "debug"
  format: "json"
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "custom:latest", cfg.Sandbox.DefaultImage)
	assert.Equal(t, 100, cfg.Sandbox.MaxSandboxes)
	assert.Equal(t, "/tmp/test.db", cfg.Store.Path)
	assert.True(t, cfg.Auth.Enabled)
	assert.Len(t, cfg.Auth.APIKeys, 2)
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("DEN_SERVER__PORT", "7070")
	t.Setenv("DEN_LOG__LEVEL", "warn")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, 7070, cfg.Server.Port)
	assert.Equal(t, "warn", cfg.Log.Level)
}

func TestLoadEmptyPath(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestLoadInvalidFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	assert.Error(t, err)
}
