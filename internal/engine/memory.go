package engine

// MemoryBackend abstracts platform-specific memory reading.
// Linux uses /proc/meminfo + cgroup v2, macOS uses sysctl + Docker API.
type MemoryBackend interface {
	// HostMemory returns total, used, and free host memory in bytes.
	HostMemory() (total, used, free uint64, err error)

	// ContainerMemory returns memory usage in bytes for a specific container.
	ContainerMemory(containerID string) (uint64, error)
}
