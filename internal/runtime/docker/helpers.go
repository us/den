package docker

import (
	"encoding/json"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/getden/den/internal/runtime"
)

func decodeStats(r io.Reader, stats *container.StatsResponse) error {
	return json.NewDecoder(r).Decode(stats)
}

func mapStats(s *container.StatsResponse) *runtime.SandboxStats {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(s.CPUStats.OnlineCPUs) * 100.0
	}

	memPercent := 0.0
	if s.MemoryStats.Limit > 0 {
		memPercent = float64(s.MemoryStats.Usage) / float64(s.MemoryStats.Limit) * 100.0
	}

	var netRx, netTx int64
	for _, n := range s.Networks {
		netRx += int64(n.RxBytes)
		netTx += int64(n.TxBytes)
	}

	var diskRead, diskWrite int64
	for _, bio := range s.BlkioStats.IoServiceBytesRecursive {
		switch bio.Op {
		case "read", "Read":
			diskRead += int64(bio.Value)
		case "write", "Write":
			diskWrite += int64(bio.Value)
		}
	}

	return &runtime.SandboxStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:  int64(s.MemoryStats.Usage),
		MemoryLimit:  int64(s.MemoryStats.Limit),
		MemoryPercent: memPercent,
		NetworkRx:    netRx,
		NetworkTx:    netTx,
		DiskRead:     diskRead,
		DiskWrite:    diskWrite,
		PidCount:     int64(s.PidsStats.Current),
		Timestamp:    time.Now().UTC(),
	}
}
