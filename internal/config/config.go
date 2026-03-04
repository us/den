package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds all configuration for the den server.
type Config struct {
	Server  ServerConfig  `koanf:"server"`
	Runtime RuntimeConfig `koanf:"runtime"`
	Sandbox SandboxConfig `koanf:"sandbox"`
	Store   StoreConfig   `koanf:"store"`
	Auth    AuthConfig    `koanf:"auth"`
	Log     LogConfig     `koanf:"log"`
	S3      S3Config      `koanf:"s3"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host           string   `koanf:"host"`
	Port           int      `koanf:"port"`
	AllowedOrigins []string `koanf:"allowed_origins"`
	RateLimitRPS   float64  `koanf:"rate_limit_rps"`
	RateLimitBurst int      `koanf:"rate_limit_burst"`
	TLS            struct {
		Enabled  bool   `koanf:"enabled"`
		CertFile string `koanf:"cert_file"`
		KeyFile  string `koanf:"key_file"`
	} `koanf:"tls"`
}

// RuntimeConfig holds runtime backend settings.
type RuntimeConfig struct {
	Backend    string `koanf:"backend"` // "docker"
	DockerHost string `koanf:"docker_host"`
	NetworkID  string `koanf:"network_id"`
}

// TmpfsDefault defines a default tmpfs mount.
type TmpfsDefault struct {
	Path string `koanf:"path"`
	Size string `koanf:"size"`
}

// SandboxConfig holds default sandbox settings.
type SandboxConfig struct {
	DefaultImage       string        `koanf:"default_image"`
	DefaultTimeout     time.Duration `koanf:"default_timeout"`
	MaxSandboxes       int           `koanf:"max_sandboxes"`
	DefaultCPU         int64         `koanf:"default_cpu"`     // NanoCPUs
	DefaultMemory      int64         `koanf:"default_memory"`  // bytes
	DefaultPidLimit    int64         `koanf:"default_pid_limit"`
	WarmPoolSize       int           `koanf:"warm_pool_size"`
	AllowVolumes       bool          `koanf:"allow_volumes"`
	AllowSharedVolumes bool          `koanf:"allow_shared_volumes"`
	AllowS3            bool          `koanf:"allow_s3"`
	AllowS3FUSE        bool          `koanf:"allow_s3_fuse"`
	AllowHostBinds     bool          `koanf:"allow_host_binds"`
	MaxVolumesPerSandbox int         `koanf:"max_volumes_per_sandbox"`
	DefaultTmpfs       []TmpfsDefault `koanf:"default_tmpfs"`
}

// S3Config holds server-wide S3 defaults.
type S3Config struct {
	Endpoint  string `koanf:"endpoint"`
	Region    string `koanf:"region"`
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
}

// String returns a safe representation of S3Config that masks the secret key.
func (c S3Config) String() string {
	masked := "***"
	if c.SecretKey == "" {
		masked = "(empty)"
	}
	return fmt.Sprintf("S3Config{Endpoint:%s Region:%s AccessKey:%s SecretKey:%s}", c.Endpoint, c.Region, c.AccessKey, masked)
}

// StoreConfig holds state persistence settings.
type StoreConfig struct {
	Path string `koanf:"path"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	APIKeys []string `koanf:"api_keys"`
	Enabled bool     `koanf:"enabled"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `koanf:"level"` // "debug", "info", "warn", "error"
	Format string `koanf:"format"` // "text", "json"
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			AllowedOrigins: []string{"http://localhost:8080", "http://127.0.0.1:8080"},
			RateLimitRPS:   10,
			RateLimitBurst: 20,
		},
		Runtime: RuntimeConfig{
			Backend:   "docker",
			NetworkID: "den-net",
		},
		Sandbox: SandboxConfig{
			DefaultImage:         "den/default:latest",
			DefaultTimeout:       30 * time.Minute,
			MaxSandboxes:         50,
			DefaultCPU:           1_000_000_000, // 1 core
			DefaultMemory:        512 * 1024 * 1024, // 512MB
			DefaultPidLimit:      256,
			WarmPoolSize:         0,
			AllowVolumes:         true,
			AllowSharedVolumes:   true,
			AllowS3:              true,
			AllowS3FUSE:          false,
			AllowHostBinds:       false,
			MaxVolumesPerSandbox: 5,
			DefaultTmpfs: []TmpfsDefault{
				{Path: "/tmp", Size: "256m"},
				{Path: "/home/sandbox", Size: "512m"},
				{Path: "/run", Size: "64m"},
				{Path: "/var/tmp", Size: "128m"},
			},
		},
		S3: S3Config{
			Region: "us-east-1",
		},
		Store: StoreConfig{
			Path: "den.db",
		},
		Auth: AuthConfig{
			Enabled: false,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Validate checks the config for obvious misconfigurations.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Sandbox.MaxSandboxes <= 0 {
		return fmt.Errorf("max_sandboxes must be positive")
	}
	if c.Sandbox.DefaultTimeout <= 0 {
		return fmt.Errorf("default_timeout must be positive")
	}
	if c.Sandbox.DefaultImage == "" {
		return fmt.Errorf("default_image is required")
	}
	if c.Sandbox.MaxVolumesPerSandbox < 0 {
		return fmt.Errorf("max_volumes_per_sandbox must be non-negative")
	}
	if c.Sandbox.WarmPoolSize < 0 {
		return fmt.Errorf("warm_pool_size must be non-negative")
	}
	if c.Auth.Enabled && len(c.Auth.APIKeys) == 0 {
		return fmt.Errorf("auth is enabled but no api_keys configured")
	}
	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS is enabled but cert_file or key_file is missing")
		}
	}
	if c.Sandbox.DefaultMemory > 0 && c.Sandbox.DefaultMemory < 4*1024*1024 {
		return fmt.Errorf("default_memory must be at least 4MB")
	}
	return nil
}

// Load reads configuration from a YAML file and environment variables.
// Environment variables are prefixed with DEN_ and use __ as separator.
// For example: DEN_SERVER__PORT=9090
func Load(path string) (*Config, error) {
	k := koanf.New(".")
	cfg := DefaultConfig()

	// Load from YAML file if provided
	if path != "" {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("loading config file %s: %w", path, err)
		}
	}

	// Load from environment variables (override file settings)
	if err := k.Load(env.Provider("DEN_", ".", func(s string) string {
		return strings.ReplaceAll(
			strings.ToLower(strings.TrimPrefix(s, "DEN_")),
			"__", ".",
		)
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env config: %w", err)
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return cfg, nil
}
