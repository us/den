package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Go SDK must round-trip port mappings unchanged through CreateSandbox:
// the request carries `ports`, and the decoded Sandbox echoes them back. This
// also pins the wire field names (sandbox_port/host_port/protocol) the server
// emits — a server-side rename would break this without touching SDK code.
func TestClient_CreateSandbox_PortsRoundTrip(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/api/v1/sandboxes", r.URL.Path)
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &gotBody))

		// Echo a server-shaped response with the canonical key names.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"id":"sb-1","image":"test:latest","status":"running",
			"created_at":"2026-05-18T00:00:00Z",
			"ports":[{"sandbox_port":8080,"host_port":49160,"protocol":"tcp"}]
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	// NetworkMode left empty so no /api/v1/version feature probe is triggered;
	// that path is covered by the feature-token tests.
	sb, err := c.CreateSandbox(context.Background(), SandboxConfig{
		Image: "test:latest",
		Ports: []PortMapping{{SandboxPort: 8080, HostPort: 49160, Protocol: "tcp"}},
	})
	require.NoError(t, err)

	// Request carried the ports under the documented keys.
	ports, ok := gotBody["ports"].([]any)
	require.True(t, ok, "request body must carry a ports array")
	require.Len(t, ports, 1)
	pm := ports[0].(map[string]any)
	assert.EqualValues(t, 8080, pm["sandbox_port"])
	assert.EqualValues(t, 49160, pm["host_port"])
	assert.Equal(t, "tcp", pm["protocol"])

	// Response decoded back into the typed Sandbox.
	require.Len(t, sb.Ports, 1)
	assert.Equal(t, 8080, sb.Ports[0].SandboxPort)
	assert.Equal(t, 49160, sb.Ports[0].HostPort)
	assert.Equal(t, "tcp", sb.Ports[0].Protocol)
}

// A sandbox with no published ports must decode to an empty slice (the server
// omits the key entirely); the SDK must not synthesize a phantom mapping.
func TestClient_CreateSandbox_NoPorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"sb-2","image":"test:latest","status":"running","created_at":"2026-05-18T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	sb, err := c.CreateSandbox(context.Background(), SandboxConfig{Image: "test:latest"})
	require.NoError(t, err)
	assert.Empty(t, sb.Ports, "absent ports key must decode to no port mappings")
}
