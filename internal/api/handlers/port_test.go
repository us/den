package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both dynamic-port endpoints are permanently 501 in v1: port mappings are
// fixed at creation and published Docker-natively only in bridge mode. The
// dead in-process PortForwarder was removed in v9. These tests pin the
// contract so a future reintroduction is a deliberate, test-breaking change.

func TestPortHandler_Add_NotImplemented(t *testing.T) {
	eng := newTestEngine(t)
	h := NewPortHandler(eng, slog.Default())

	req := httptest.NewRequest("POST", "/api/v1/sandboxes/sb-1/ports", nil)
	w := httptest.NewRecorder()
	h.Add(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, portsUnsupportedMsg, body["error"])
}

func TestPortHandler_Remove_NotImplemented(t *testing.T) {
	eng := newTestEngine(t)
	h := NewPortHandler(eng, slog.Default())

	req := httptest.NewRequest("DELETE", "/api/v1/sandboxes/sb-1/ports/8080", nil)
	w := httptest.NewRecorder()
	h.Remove(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, portsUnsupportedMsg, body["error"])
}
