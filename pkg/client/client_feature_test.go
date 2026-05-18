package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SDK feature-token probe is a lazy, scoped capability hint (NOT auth):
//   - it fires ONLY when network_mode is actually set on the request,
//   - it hits GET /api/v1/version exactly once and caches the result,
//   - a server that does not advertise "network_mode" fails fast BEFORE the
//     sandbox create POST is ever sent.

type featureServer struct {
	versionHits int64
	createHits  int64
	features    string // raw JSON array body for the "features" field
	srv         *httptest.Server
}

func newFeatureServer(t *testing.T, features string) *featureServer {
	t.Helper()
	fs := &featureServer{features: features}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			atomic.AddInt64(&fs.versionHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test","features":` + fs.features + `}`))
		case "/api/v1/sandboxes":
			atomic.AddInt64(&fs.createHits, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"sb-1","image":"x","status":"running","created_at":"2026-05-18T00:00:00Z"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

func TestClient_FeatureProbe_SupportedProceeds(t *testing.T) {
	fs := newFeatureServer(t, `["network_mode"]`)
	c := New(fs.srv.URL)

	_, err := c.CreateSandbox(context.Background(), SandboxConfig{
		Image: "x", NetworkMode: "none",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, atomic.LoadInt64(&fs.versionHits), "version probed exactly once")
	assert.EqualValues(t, 1, atomic.LoadInt64(&fs.createHits), "create proceeds when feature is advertised")
}

func TestClient_FeatureProbe_UnsupportedFailsFast(t *testing.T) {
	fs := newFeatureServer(t, `[]`)
	c := New(fs.srv.URL)

	_, err := c.CreateSandbox(context.Background(), SandboxConfig{
		Image: "x", NetworkMode: "none",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not advertise")
	assert.EqualValues(t, 1, atomic.LoadInt64(&fs.versionHits))
	assert.EqualValues(t, 0, atomic.LoadInt64(&fs.createHits),
		"create POST must NOT be sent when the server lacks the feature")
}

func TestClient_FeatureProbe_SkippedWhenModeUnset(t *testing.T) {
	fs := newFeatureServer(t, `["network_mode"]`)
	c := New(fs.srv.URL)

	_, err := c.CreateSandbox(context.Background(), SandboxConfig{Image: "x"})
	require.NoError(t, err)
	assert.EqualValues(t, 0, atomic.LoadInt64(&fs.versionHits),
		"no network_mode ⇒ no capability probe at all")
	assert.EqualValues(t, 1, atomic.LoadInt64(&fs.createHits))
}

func TestClient_FeatureProbe_CachedAcrossCalls(t *testing.T) {
	fs := newFeatureServer(t, `["network_mode"]`)
	c := New(fs.srv.URL)

	for i := 0; i < 3; i++ {
		_, err := c.CreateSandbox(context.Background(), SandboxConfig{
			Image: "x", NetworkMode: "none",
		})
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, atomic.LoadInt64(&fs.versionHits),
		"the version probe result is cached; it must not be re-fetched per create")
	assert.EqualValues(t, 3, atomic.LoadInt64(&fs.createHits))
}
