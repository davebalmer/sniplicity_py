package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sniplicity/internal/config"
	"sniplicity/internal/projects"
)

//go:embed ui.html
var uiHTML string

//go:embed project_selector.html
var projectSelectorHTML string

//go:embed pico.min.css
var picoCSS string

//go:embed custom.css
var customCSS string

// Handler manages the web interface for sniplicity configuration
type Handler struct {
	config         *config.Config
	recentProjects *projects.RecentProjects
	onConfigSave   func(*config.Config) error     // Callback for when config is saved
	onProjectSwitch func(string) error            // Callback for when project is switched
}

// NewHandler creates a new web interface handler
func NewHandler(cfg *config.Config, onConfigSave func(*config.Config) error, onProjectSwitch func(string) error) (*Handler, error) {
	rp, err := projects.NewRecentProjects()
	if err != nil {
		return nil, fmt.Errorf("initializing recent projects: %w", err)
	}
	
	return &Handler{
		config:          cfg,
		recentProjects:  rp,
		onConfigSave:    onConfigSave,
		onProjectSwitch: onProjectSwitch,
	}, nil
}

// ServeHTTP handles all /sniplicity routes
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sniplicity")
	
	switch {
	case path == "" || path == "/":
		h.serveProjectSelector(w, r)
	case path == "-project" || path == "-project/":
		h.serveUI(w, r)
	case path == "/css":
		h.serveCSS(w, r)
	case path == "/custom.css":
		h.serveCustomCSS(w, r)
	case path == "/api/network" && r.Method == "GET":
		h.getNetworkInfo(w, r)
	case path == "/api/config" && r.Method == "GET":
		h.getConfig(w, r)
	case path == "/api/config" && r.Method == "POST":
		h.saveConfig(w, r)
	case path == "/api/projects" && r.Method == "GET":
		h.getProjects(w, r)
	case path == "/api/projects/switch" && r.Method == "POST":
		h.switchProject(w, r)
	case path == "/api/projects/add" && r.Method == "POST":
		h.addProject(w, r)
	case path == "/api/projects/remove" && r.Method == "POST":
		h.removeProject(w, r)
	case path == "/api/projects/validate" && r.Method == "POST":
		h.validateProject(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveProjectSelector serves the project selection interface
func (h *Handler) serveProjectSelector(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(projectSelectorHTML))
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

// getLocalIP returns the local IP address of the machine
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback to finding local IP through network interfaces
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return ""
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
		return ""
	}
	defer conn.Close()
	
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// NetworkInfoResponse represents the network information sent to the client
type NetworkInfoResponse struct {
	LocalhostURL string `json:"localhost_url"`
	NetworkURL   string `json:"network_url"`
	LocalIP      string `json:"local_ip"`
	Port         int    `json:"port"`
}

// getNetworkInfo returns network access information
func (h *Handler) getNetworkInfo(w http.ResponseWriter, r *http.Request) {
	localIP := getLocalIP()
	port := h.config.Port
	
	// Default to HTTP for local development
	protocol := "http"
	// Only use HTTPS if explicitly detected from request
	if r.TLS != nil {
		protocol = "https"
	}
	
	response := NetworkInfoResponse{
		LocalhostURL: fmt.Sprintf("%s://127.0.0.1:%d", protocol, port),
		NetworkURL:   "",
		LocalIP:      localIP,
		Port:         port,
	}
	
	if localIP != "" {
		response.NetworkURL = fmt.Sprintf("%s://%s:%d", protocol, localIP, port)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ConfigResponse represents the configuration data sent to the client
type ConfigResponse struct {
	Name       string `json:"name"`
	ProjectDir string `json:"project_dir"`
	InputDir   string `json:"input_dir"`
	OutputDir  string `json:"output_dir"`
	Port       int    `json:"port"`
	Watch      bool   `json:"watch"`
	Serve      bool   `json:"serve"`
	Verbose    bool   `json:"verbose"`
	ImgSize    bool   `json:"imgsize"`
}

// getConfig returns the current configuration as JSON
func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	response := ConfigResponse{
		Name:       h.config.Name,
		ProjectDir: h.config.ProjectDir,
		InputDir:   h.config.InputDir,
		OutputDir:  h.config.OutputDir,
		Port:       h.config.Port,
		Watch:      h.config.Watch,
		Serve:      h.config.Serve,
		Verbose:    h.config.Verbose,
		ImgSize:    h.config.ImgSize,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ConfigRequest represents the configuration data received from the client
type ConfigRequest struct {
	Name      string `json:"name"`
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
	h.config.Name = req.Name
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

// ProjectsResponse represents the projects data sent to the client
type ProjectsResponse struct {
	CurrentProject     *ProjectInfo  `json:"current_project,omitempty"`
	RecentProjects     []ProjectInfo `json:"recent_projects"`
	DefaultProjectPath string        `json:"default_project_path,omitempty"`
}

// ProjectInfo represents project information for the client
type ProjectInfo struct {
	Path        string `json:"path"`
	DisplayName string `json:"display_name"`
	LastUsed    string `json:"last_used"`
}

// getProjects returns the current and recent projects
func (h *Handler) getProjects(w http.ResponseWriter, r *http.Request) {
	var currentProject *ProjectInfo
	var defaultProjectPath string
	
	// Check if we have a valid project directory with config
	if h.config.ProjectDir != "" {
		// Config file exists - show as current project if it exists in recent projects
		isInRecentProjects := h.recentProjects.ProjectExists(h.config.ProjectDir)
		
		// Show as current project if it's in recent projects OR if there are no recent projects
		allRecentProjects := h.recentProjects.GetProjects()
		if isInRecentProjects || len(allRecentProjects) == 0 {
			displayName := h.config.Name
			if displayName == "" {
				displayName = "Current Project"
			}
			currentProject = &ProjectInfo{
				Path:        h.config.ProjectDir,
				DisplayName: displayName,
			}
		}
	} else {
		// No valid project - get current working directory for the input field
		if wd, err := os.Getwd(); err == nil {
			defaultProjectPath = wd
		}
	}
	
	recentProjects := h.recentProjects.GetProjectsExcluding(h.config.ProjectDir)
	var recentProjectsInfo []ProjectInfo
	for _, project := range recentProjects {
		displayName := project.DisplayName // Default to the stored display name
		
		// Try to load the project's config to get the friendly name
		if projectConfig, err := config.LoadConfigFromFile(project.Path); err == nil && projectConfig.Name != "" {
			displayName = projectConfig.Name
		}
		
		recentProjectsInfo = append(recentProjectsInfo, ProjectInfo{
			Path:        project.Path,
			DisplayName: displayName,
			LastUsed:    project.LastUsed.Format("2006-01-02 15:04:05"),
		})
	}
	
	response := ProjectsResponse{
		CurrentProject:     currentProject,
		RecentProjects:     recentProjectsInfo,
		DefaultProjectPath: defaultProjectPath,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ProjectRequest represents project operations from the client
type ProjectRequest struct {
	ProjectPath string `json:"project_path"`
}

// switchProject switches to a different project
func (h *Handler) switchProject(w http.ResponseWriter, r *http.Request) {
	var req ProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}
	
	if req.ProjectPath == "" {
		http.Error(w, `{"error": "Project path is required"}`, http.StatusBadRequest)
		return
	}
	
	// Check if project directory exists
	if !h.recentProjects.ProjectExists(req.ProjectPath) {
		http.Error(w, `{"error": "Project directory does not exist"}`, http.StatusBadRequest)
		return
	}
	
	// Add to recent projects
	if err := h.recentProjects.AddProject(req.ProjectPath); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to update recent projects: %v"}`, err), http.StatusInternalServerError)
		return
	}
	
	// Call the project switch callback
	if h.onProjectSwitch != nil {
		if err := h.onProjectSwitch(req.ProjectPath); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to switch project: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}
	
	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Project switched successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// addProject adds a new project to the recent projects list
func (h *Handler) addProject(w http.ResponseWriter, r *http.Request) {
	var req ProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}
	
	if req.ProjectPath == "" {
		http.Error(w, `{"error": "Project path is required"}`, http.StatusBadRequest)
		return
	}
	
	// Check if project directory exists
	if !h.recentProjects.ProjectExists(req.ProjectPath) {
		http.Error(w, `{"error": "Project directory does not exist"}`, http.StatusBadRequest)
		return
	}
	
	// Add to recent projects
	if err := h.recentProjects.AddProject(req.ProjectPath); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to add project: %v"}`, err), http.StatusInternalServerError)
		return
	}
	
	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Project added successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// removeProject removes a project from the recent projects list
func (h *Handler) removeProject(w http.ResponseWriter, r *http.Request) {
	var req ProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}
	
	if req.ProjectPath == "" {
		http.Error(w, `{"error": "Project path is required"}`, http.StatusBadRequest)
		return
	}
	
	// Remove from recent projects
	if err := h.recentProjects.RemoveProject(req.ProjectPath); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to remove project: %v"}`, err), http.StatusInternalServerError)
		return
	}
	
	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Project removed successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// AddCurrentProjectToRecent adds the current project to the recent projects list
func (h *Handler) AddCurrentProjectToRecent() error {
	if h.config.ProjectDir != "" {
		// Only add to recent projects if there's actually a config file in the directory
		configPath := filepath.Join(h.config.ProjectDir, "sniplicity.yaml")
		if _, err := os.Stat(configPath); err == nil {
			// Config file exists, so this is a valid project
			return h.recentProjects.AddProject(h.config.ProjectDir)
		}
	}
	return nil
}

// validateProject checks if a directory contains a valid sniplicity project
func (h *Handler) validateProject(w http.ResponseWriter, r *http.Request) {
	var req ProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}
	
	if req.ProjectPath == "" {
		http.Error(w, `{"error": "Project path is required"}`, http.StatusBadRequest)
		return
	}
	
	// Check if project directory exists
	if !h.recentProjects.ProjectExists(req.ProjectPath) {
		http.Error(w, `{"error": "Project directory does not exist"}`, http.StatusBadRequest)
		return
	}
	
	// Check if there's a sniplicity.yaml config file
	configPath := filepath.Join(req.ProjectPath, "sniplicity.yaml")
	hasConfig := false
	if _, err := os.Stat(configPath); err == nil {
		hasConfig = true
	}
	
	// Return validation result
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"valid":      true,
		"has_config": hasConfig,
		"path":       req.ProjectPath,
	}
	json.NewEncoder(w).Encode(response)
}