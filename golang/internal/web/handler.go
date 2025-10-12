package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"sniplicity/internal/config"
)

//go:embed ui.html
var uiHTML string

//go:embed pico.min.css
var picoCSS string

//go:embed custom.css
var customCSS string

// Handler manages the web interface for sniplicity configuration
type Handler struct {
	config       *config.Config
	onConfigSave func(*config.Config) error // Callback for when config is saved
}

// NewHandler creates a new web interface handler
func NewHandler(cfg *config.Config, onConfigSave func(*config.Config) error) *Handler {
	return &Handler{
		config:       cfg,
		onConfigSave: onConfigSave,
	}
}

// ServeHTTP handles all /sniplicity routes
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sniplicity")
	
	switch {
	case path == "" || path == "/":
		h.serveUI(w, r)
	case path == "/css":
		h.serveCSS(w, r)
	case path == "/custom.css":
		h.serveCustomCSS(w, r)
	case path == "/api/config" && r.Method == "GET":
		h.getConfig(w, r)
	case path == "/api/config" && r.Method == "POST":
		h.saveConfig(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveUI serves the configuration interface
func (h *Handler) serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(uiHTML))
}

// serveCSS serves the embedded Pico CSS
func (h *Handler) serveCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(picoCSS))
}

// serveCustomCSS serves the embedded custom CSS
func (h *Handler) serveCustomCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(customCSS))
}

// ConfigResponse represents the configuration data sent to the client
type ConfigResponse struct {
	InputDir  string `json:"input_dir"`
	OutputDir string `json:"output_dir"`
	Port      int    `json:"port"`
	Watch     bool   `json:"watch"`
	Serve     bool   `json:"serve"`
	Verbose   bool   `json:"verbose"`
	ImgSize   bool   `json:"imgsize"`
}

// getConfig returns the current configuration as JSON
func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	response := ConfigResponse{
		InputDir:  h.config.InputDir,
		OutputDir: h.config.OutputDir,
		Port:      h.config.Port,
		Watch:     h.config.Watch,
		Serve:     h.config.Serve,
		Verbose:   h.config.Verbose,
		ImgSize:   h.config.ImgSize,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ConfigRequest represents the configuration data received from the client
type ConfigRequest struct {
	InputDir  string `json:"input_dir"`
	OutputDir string `json:"output_dir"`
	Port      int    `json:"port"`
	Watch     bool   `json:"watch"`
	Serve     bool   `json:"serve"`
	Verbose   bool   `json:"verbose"`
	ImgSize   bool   `json:"imgsize"`
}

// saveConfig updates the configuration from the web interface
func (h *Handler) saveConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}
	
	// Validate port
	if req.Port < 1024 || req.Port > 65535 {
		http.Error(w, `{"error": "Port must be between 1024 and 65535"}`, http.StatusBadRequest)
		return
	}
	
	// Check what changed to determine if rebuild/restart is needed
	oldInputDir := h.config.InputDir
	oldOutputDir := h.config.OutputDir
	oldPort := h.config.Port
	
	// Update configuration
	h.config.InputDir = req.InputDir
	h.config.OutputDir = req.OutputDir
	h.config.Port = req.Port
	h.config.Watch = req.Watch
	h.config.Serve = req.Serve
	h.config.Verbose = req.Verbose
	h.config.ImgSize = req.ImgSize
	
	// Save to file
	if err := h.config.SaveConfigToFile(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to save config: %v"}`, err), http.StatusInternalServerError)
		return
	}
	
	// Determine if rebuild or restart is needed
	needsRestart := oldPort != req.Port || (!h.config.Serve && req.Serve)
	needsRebuild := oldInputDir != req.InputDir || oldOutputDir != req.OutputDir
	
	// Call the callback for rebuilds or restarts
	if (needsRestart || needsRebuild) && h.onConfigSave != nil {
		if err := h.onConfigSave(h.config); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to apply config: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}
	
	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Configuration saved successfully",
		"restart_needed": needsRestart,
		"rebuild_needed": needsRebuild,
	}
	json.NewEncoder(w).Encode(response)
}