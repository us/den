package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/us/den/internal/engine"
)

// PortHandler handles port forwarding operations.
type PortHandler struct {
	engine *engine.Engine
	logger *slog.Logger
}

// NewPortHandler creates a new PortHandler.
func NewPortHandler(eng *engine.Engine, logger *slog.Logger) *PortHandler {
	return &PortHandler{engine: eng, logger: logger}
}

// List handles GET /api/v1/sandboxes/{id}/ports.
func (h *PortHandler) List(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sandbox, err := h.engine.GetSandbox(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "sandbox not found")
		return
	}
	writeJSON(w, http.StatusOK, sandbox.Ports)
}

// Add handles POST /api/v1/sandboxes/{id}/ports.
func (h *PortHandler) Add(w http.ResponseWriter, r *http.Request) {
	// Dynamic port forwarding would require the PortForwarder
	// For now, ports are specified at sandbox creation time
	writeError(w, http.StatusNotImplemented, "dynamic port forwarding not yet implemented")
}

// Remove handles DELETE /api/v1/sandboxes/{id}/ports/{port}.
func (h *PortHandler) Remove(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "dynamic port removal not yet implemented")
}
