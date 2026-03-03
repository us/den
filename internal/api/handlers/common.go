package handlers

import (
	"encoding/json"
	"net/http"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// SetVersion sets the build version info.
func SetVersion(v, c, b string) {
	version = v
	commit = c
	buildDate = b
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

const maxJSONBodySize = 1 * 1024 * 1024 // 1MB

func readJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxJSONBodySize)
	return json.NewDecoder(r.Body).Decode(v)
}

// Health returns server health status.
func Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Version returns server version info.
func Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":    version,
		"commit":     commit,
		"build_date": buildDate,
	})
}
