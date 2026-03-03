package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/us/den/internal/runtime"
	"github.com/us/den/internal/storage"

	dockermount "github.com/docker/docker/api/types/mount"
)

const (
	labelPrefix  = "den."
	labelID      = labelPrefix + "id"
	labelCreated = labelPrefix + "created"
)

// DockerRuntime implements runtime.Runtime using Docker containers.
type DockerRuntime struct {
	cli       *client.Client
	networkID string
	logger    *slog.Logger
}

// Option configures the DockerRuntime.
type Option func(*DockerRuntime)

// WithNetworkID sets the Docker network to attach sandboxes to.
func WithNetworkID(id string) Option {
	return func(r *DockerRuntime) {
		r.networkID = id
	}
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option {
	return func(r *DockerRuntime) {
		r.logger = l
	}
}

// New creates a new DockerRuntime.
func New(opts ...Option) (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	r := &DockerRuntime{
		cli:       cli,
		networkID: "den-net",
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Ping verifies Docker daemon connectivity.
func (r *DockerRuntime) Ping(ctx context.Context) error {
	_, err := r.cli.Ping(ctx)
	return err
}

// EnsureNetwork creates the den Docker network if it doesn't exist.
func (r *DockerRuntime) EnsureNetwork(ctx context.Context) error {
	networks, err := r.cli.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", r.networkID)),
	})
	if err != nil {
		return fmt.Errorf("listing networks: %w", err)
	}
	for _, n := range networks {
		if n.Name == r.networkID {
			return nil
		}
	}

	_, err = r.cli.NetworkCreate(ctx, r.networkID, network.CreateOptions{
		Driver:   "bridge",
		Internal: true,
	})
	if err != nil {
		return fmt.Errorf("creating network %s: %w", r.networkID, err)
	}
	r.logger.Info("created docker network", "network", r.networkID)
	return nil
}

// Create creates a new Docker container for the sandbox.
func (r *DockerRuntime) Create(ctx context.Context, id string, cfg runtime.SandboxConfig) error {
	var envList []string
	for k, v := range cfg.Env {
		envList = append(envList, k+"="+v)
	}

	labels := map[string]string{
		labelID:      id,
		labelCreated: time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, pm := range cfg.Ports {
		containerPort := nat.Port(fmt.Sprintf("%d/tcp", pm.SandboxPort))
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []nat.PortBinding{
			{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", pm.HostPort)},
		}
	}

	containerCfg := &container.Config{
		Image:        cfg.Image,
		Env:          envList,
		Labels:       labels,
		ExposedPorts: exposedPorts,
		Tty:          true,
		OpenStdin:    true,
	}
	if cfg.WorkDir != "" {
		containerCfg.WorkingDir = cfg.WorkDir
	}
	if len(cfg.Cmd) > 0 {
		containerCfg.Cmd = cfg.Cmd
	}

	// Build tmpfs map from storage config (computed by engine)
	tmpfsMap := cfg.TmpfsMap

	// Build Docker volume mounts
	var mounts []dockermount.Mount
	if cfg.Storage != nil {
		for _, vol := range cfg.Storage.Volumes {
			mounts = append(mounts, dockermount.Mount{
				Type:     dockermount.TypeVolume,
				Source:   storage.NamespacedVolumeName(vol.Name),
				Target:   vol.MountPath,
				ReadOnly: vol.ReadOnly,
			})
		}
	}

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Resources: container.Resources{
			NanoCPUs: cfg.CPU,
			Memory:   cfg.Memory,
		},
		SecurityOpt:    []string{"no-new-privileges"},
		CapDrop:        []string{"ALL"},
		CapAdd:         []string{"NET_BIND_SERVICE", "CHOWN", "SETUID", "SETGID", "DAC_OVERRIDE", "FOWNER"},
		ReadonlyRootfs: true,
		Tmpfs:          tmpfsMap,
		Mounts:         mounts,
	}
	if cfg.PidLimit > 0 {
		hostCfg.PidsLimit = &cfg.PidLimit
	} else {
		defaultPidLimit := int64(256)
		hostCfg.PidsLimit = &defaultPidLimit
	}

	networkCfg := &network.NetworkingConfig{}
	netID := cfg.NetworkID
	if netID == "" {
		netID = r.networkID
	}
	if netID != "" {
		networkCfg.EndpointsConfig = map[string]*network.EndpointSettings{
			netID: {},
		}
	}

	containerName := "den-" + id

	_, err := r.cli.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, containerName)
	if err != nil {
		return fmt.Errorf("creating container %s: %w", id, err)
	}
	return nil
}

// Start starts the container.
func (r *DockerRuntime) Start(ctx context.Context, id string) error {
	return r.cli.ContainerStart(ctx, r.containerName(id), container.StartOptions{})
}

// Stop stops the container with the given timeout.
func (r *DockerRuntime) Stop(ctx context.Context, id string, timeout time.Duration) error {
	timeoutSec := int(timeout.Seconds())
	opts := container.StopOptions{Timeout: &timeoutSec}
	return r.cli.ContainerStop(ctx, r.containerName(id), opts)
}

// Remove forcefully removes the container.
func (r *DockerRuntime) Remove(ctx context.Context, id string) error {
	return r.cli.ContainerRemove(ctx, r.containerName(id), container.RemoveOptions{
		Force:         true,
		RemoveVolumes: false, // Preserve named volumes (may be shared between sandboxes)
	})
}

// Info returns information about the sandbox container.
func (r *DockerRuntime) Info(ctx context.Context, id string) (*runtime.SandboxInfo, error) {
	inspect, err := r.cli.ContainerInspect(ctx, r.containerName(id))
	if err != nil {
		return nil, fmt.Errorf("inspecting container %s: %w", id, err)
	}

	status := mapContainerStatus(inspect.State)
	createdAt, _ := time.Parse(time.RFC3339Nano, inspect.Created)

	var ports []runtime.PortMapping
	for containerPort, bindings := range inspect.NetworkSettings.Ports {
		for _, b := range bindings {
			hp := 0
			fmt.Sscanf(b.HostPort, "%d", &hp)
			ports = append(ports, runtime.PortMapping{
				SandboxPort: containerPort.Int(),
				HostPort:    hp,
				Protocol:    containerPort.Proto(),
			})
		}
	}

	return &runtime.SandboxInfo{
		ID:        id,
		Name:      strings.TrimPrefix(inspect.Name, "/"),
		Image:     inspect.Config.Image,
		Status:    status,
		CreatedAt: createdAt,
		Ports:     ports,
		Labels:    inspect.Config.Labels,
		Pid:       inspect.State.Pid,
	}, nil
}

// List returns all den-managed containers.
func (r *DockerRuntime) List(ctx context.Context) ([]runtime.SandboxInfo, error) {
	containers, err := r.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", labelID)),
	})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	infos := make([]runtime.SandboxInfo, 0, len(containers))
	for _, c := range containers {
		id := c.Labels[labelID]
		status := runtime.StatusStopped
		if c.State == "running" {
			status = runtime.StatusRunning
		}

		var ports []runtime.PortMapping
		for _, p := range c.Ports {
			ports = append(ports, runtime.PortMapping{
				SandboxPort: int(p.PrivatePort),
				HostPort:    int(p.PublicPort),
				Protocol:    p.Type,
			})
		}

		infos = append(infos, runtime.SandboxInfo{
			ID:        id,
			Name:      strings.TrimPrefix(c.Names[0], "/"),
			Image:     c.Image,
			Status:    status,
			CreatedAt: time.Unix(c.Created, 0),
			Ports:     ports,
			Labels:    c.Labels,
		})
	}
	return infos, nil
}

// Stats returns resource usage stats for the sandbox.
func (r *DockerRuntime) Stats(ctx context.Context, id string) (*runtime.SandboxStats, error) {
	resp, err := r.cli.ContainerStatsOneShot(ctx, r.containerName(id))
	if err != nil {
		return nil, fmt.Errorf("getting stats for %s: %w", id, err)
	}
	defer resp.Body.Close()

	var statsResp container.StatsResponse
	if err := decodeStats(resp.Body, &statsResp); err != nil {
		return nil, err
	}

	return mapStats(&statsResp), nil
}

func (r *DockerRuntime) containerName(id string) string {
	return "den-" + id
}

func mapContainerStatus(state *container.State) runtime.SandboxStatus {
	if state == nil {
		return runtime.StatusError
	}
	switch {
	case state.Running:
		return runtime.StatusRunning
	case state.Paused:
		return runtime.StatusStopped
	case state.Dead, state.OOMKilled:
		return runtime.StatusError
	default:
		return runtime.StatusStopped
	}
}
