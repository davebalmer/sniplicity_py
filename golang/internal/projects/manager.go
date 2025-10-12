package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kirsle/configdir"
	"sniplicity/internal/config"
)

// Project represents a recent project entry
type Project struct {
	Path        string    `json:"path"`
	LastUsed    time.Time `json:"last_used"`
	DisplayName string    `json:"display_name,omitempty"` // Optional friendly name
}

// RecentProjects manages the list of recently used project directories
type RecentProjects struct {
	configPath string
	projects   []Project
}

// NewRecentProjects creates a new RecentProjects manager
func NewRecentProjects() (*RecentProjects, error) {
	configPath := configdir.LocalConfig("sniplicity")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}
	
	rp := &RecentProjects{
		configPath: filepath.Join(configPath, "recent_projects.json"),
		projects:   []Project{},
	}
	
	// Load existing projects
	if err := rp.load(); err != nil {
		// If file doesn't exist, that's fine - we'll start with empty list
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading recent projects: %w", err)
		}
	}
	
	return rp, nil
}

// AddProject adds or updates a project in the recent list
func (rp *RecentProjects) AddProject(projectPath string) error {
	// Clean the path
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}
	
	// Check if project already exists
	for i, project := range rp.projects {
		if project.Path == absPath {
			// Update last used time and display name (in case it changed)
			displayName := filepath.Base(absPath) // Default to folder name
			if projectConfig, err := config.LoadConfigFromFile(absPath); err == nil && projectConfig.Name != "" {
				displayName = projectConfig.Name
			}
			
			rp.projects[i].LastUsed = time.Now()
			rp.projects[i].DisplayName = displayName
			
			// Move to front
			if i > 0 {
				project := rp.projects[i]
				rp.projects = append([]Project{project}, append(rp.projects[:i], rp.projects[i+1:]...)...)
			}
			return rp.save()
		}
	}
	
	// Add new project at front
	displayName := filepath.Base(absPath) // Default to folder name
	
	// Try to load the project's config to get a friendly name
	if projectConfig, err := config.LoadConfigFromFile(absPath); err == nil && projectConfig.Name != "" {
		displayName = projectConfig.Name
	}
	
	newProject := Project{
		Path:        absPath,
		LastUsed:    time.Now(),
		DisplayName: displayName,
	}
	
	rp.projects = append([]Project{newProject}, rp.projects...)
	
	// Keep only the most recent 10 projects
	if len(rp.projects) > 10 {
		rp.projects = rp.projects[:10]
	}
	
	return rp.save()
}

// RemoveProject removes a project from the recent list (does not delete project files)
func (rp *RecentProjects) RemoveProject(projectPath string) error {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}
	
	for i, project := range rp.projects {
		if project.Path == absPath {
			rp.projects = append(rp.projects[:i], rp.projects[i+1:]...)
			return rp.save()
		}
	}
	
	// Project not found, but that's not an error
	return nil
}

// GetProjects returns all recent projects, most recent first
func (rp *RecentProjects) GetProjects() []Project {
	return rp.projects
}

// GetProjectsExcluding returns all recent projects except the specified one
func (rp *RecentProjects) GetProjectsExcluding(currentPath string) []Project {
	absPath, err := filepath.Abs(currentPath)
	if err != nil {
		// If we can't get abs path, just return all projects
		return rp.projects
	}
	
	var filtered []Project
	for _, project := range rp.projects {
		if project.Path != absPath {
			filtered = append(filtered, project)
		}
	}
	return filtered
}

// ProjectExists checks if a project path exists on the filesystem
func (rp *RecentProjects) ProjectExists(projectPath string) bool {
	if projectPath == "" {
		return false
	}
	
	info, err := os.Stat(projectPath)
	return err == nil && info.IsDir()
}

// load reads the recent projects from disk
func (rp *RecentProjects) load() error {
	data, err := os.ReadFile(rp.configPath)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, &rp.projects)
}

// save writes the recent projects to disk
func (rp *RecentProjects) save() error {
	data, err := json.MarshalIndent(rp.projects, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling projects: %w", err)
	}
	
	return os.WriteFile(rp.configPath, data, 0644)
}