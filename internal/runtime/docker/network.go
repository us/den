package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/getden/den/internal/runtime"
)

// PortForwarder manages dynamic TCP port forwarding for sandboxes.
type PortForwarder struct {
	mu        sync.Mutex
	listeners map[string]net.Listener // key: "sandboxID:sandboxPort"
	logger    *slog.Logger
}

// NewPortForwarder creates a new PortForwarder.
func NewPortForwarder(logger *slog.Logger) *PortForwarder {
	return &PortForwarder{
		listeners: make(map[string]net.Listener),
		logger:    logger,
	}
}

// Forward creates a TCP proxy from hostPort to sandboxIP:sandboxPort.
// If hostPort is 0, a random port is assigned.
func (pf *PortForwarder) Forward(ctx context.Context, sandboxID string, sandboxIP string, mapping runtime.PortMapping) (int, error) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	key := fmt.Sprintf("%s:%d", sandboxID, mapping.SandboxPort)
	if _, exists := pf.listeners[key]; exists {
		return 0, fmt.Errorf("port %d already forwarded for sandbox %s", mapping.SandboxPort, sandboxID)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", mapping.HostPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("listening on %s: %w", addr, err)
	}

	assignedPort := listener.Addr().(*net.TCPAddr).Port
	pf.listeners[key] = listener

	targetAddr := fmt.Sprintf("%s:%d", sandboxIP, mapping.SandboxPort)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // listener closed
			}
			go pf.proxyConn(conn, targetAddr)
		}
	}()

	pf.logger.Info("port forwarding established",
		"sandbox", sandboxID,
		"host_port", assignedPort,
		"sandbox_port", mapping.SandboxPort,
	)

	return assignedPort, nil
}

// Remove stops port forwarding for the given sandbox and port.
func (pf *PortForwarder) Remove(sandboxID string, sandboxPort int) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	key := fmt.Sprintf("%s:%d", sandboxID, sandboxPort)
	listener, exists := pf.listeners[key]
	if !exists {
		return fmt.Errorf("no forwarding for sandbox %s port %d", sandboxID, sandboxPort)
	}

	listener.Close()
	delete(pf.listeners, key)
	return nil
}

// RemoveAll stops all port forwarding for the given sandbox.
func (pf *PortForwarder) RemoveAll(sandboxID string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	prefix := sandboxID + ":"
	for key, listener := range pf.listeners {
		if strings.HasPrefix(key, prefix) {
			listener.Close()
			delete(pf.listeners, key)
		}
	}
}

func (pf *PortForwarder) proxyConn(client net.Conn, targetAddr string) {
	defer client.Close()

	target, err := net.Dial("tcp", targetAddr)
	if err != nil {
		pf.logger.Warn("failed to connect to target", "addr", targetAddr, "error", err)
		return
	}
	defer target.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(target, client)
	}()
	go func() {
		defer wg.Done()
		io.Copy(client, target)
	}()

	wg.Wait()
}
