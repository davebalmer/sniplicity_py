package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the configuration for a sniplicity project
type Config struct {
	Name       string   `yaml:"name"`       // Friendly name for the project
	ProjectDir string   `yaml:"-"`          // Full path to the project directory (not saved to YAML)
	InputDir   string   `yaml:"input_dir"`  // Relative path to input directory
	OutputDir  string   `yaml:"output_dir"` // Relative path to output directory
	Watch      bool     `yaml:"watch"`      // Whether to watch for file changes
	Verbose    bool     `yaml:"verbose"`    // Whether to enable verbose logging
	Serve      bool     `yaml:"serve"`      // Whether to serve files via HTTP
	Port       int      `yaml:"port"`       // Port for HTTP server
	ImgSize    bool     `yaml:"imgsize"`    // Whether to add width/height attributes to images
	SvgFilter  bool     `yaml:"svgfilter"`  // Whether to process SVG files with CSS filters
	LegacyMode bool     `yaml:"-"`          // Whether running in legacy mode (not saved to YAML)
}

// ConfigFile represents the structure of the configuration file on disk
type ConfigFile struct {
	Name      string   `yaml:"name"`
	InputDir  string   `yaml:"input_dir"`
	OutputDir string   `yaml:"output_dir"`
	Watch     bool     `yaml:"watch"`
	Verbose   bool     `yaml:"verbose"`
	Serve     bool     `yaml:"serve"`
	Port      int      `yaml:"port"`
	ImgSize   *bool    `yaml:"imgsize,omitempty"`   // Pointer to handle optional field
	SvgFilter *bool    `yaml:"svgfilter,omitempty"` // Pointer to handle optional field
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() Config {
	return Config{
		InputDir:  "snip",  // default input directory
		OutputDir: "www",   // default output directory
		Watch:     false,
		Verbose:   false,
		Serve:     false,
		Port:      3000,
		ImgSize:   true,    // default to enabled
		SvgFilter: true,    // default to enabled
	}
}

// GetAbsoluteInputDir returns the absolute path to the input directory
func (c *Config) GetAbsoluteInputDir() string {
	if filepath.IsAbs(c.InputDir) {
		return c.InputDir
	}
	return filepath.Join(c.ProjectDir, c.InputDir)
}

// GetAbsoluteOutputDir returns the absolute path to the output directory
func (c *Config) GetAbsoluteOutputDir() string {
	if filepath.IsAbs(c.OutputDir) {
		return c.OutputDir
	}
	return filepath.Join(c.ProjectDir, c.OutputDir)
}

// LoadConfigFromFile loads configuration from sniplicity.yaml in the given project directory
func LoadConfigFromFile(projectDir string) (Config, error) {
	configPath := filepath.Join(projectDir, "sniplicity.yaml")
	cfg := DefaultConfig()
	
	// If no config file exists, return defaults without setting ProjectDir
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}
	
	// Config file exists, so this is a valid project directory
	cfg.ProjectDir = projectDir
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}
	
	var configFile ConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		return cfg, fmt.Errorf("parsing config file: %w", err)
	}
	
	// Apply config file values, using defaults if not specified
	if configFile.Name != "" {
		cfg.Name = configFile.Name
	}
	if configFile.InputDir != "" {
		cfg.InputDir = configFile.InputDir
	}
	if configFile.OutputDir != "" {
		cfg.OutputDir = configFile.OutputDir
	}
	cfg.Watch = configFile.Watch
	cfg.Verbose = configFile.Verbose
	cfg.Serve = configFile.Serve
	if configFile.ImgSize != nil {
		cfg.ImgSize = *configFile.ImgSize
	}
	if configFile.SvgFilter != nil {
		cfg.SvgFilter = *configFile.SvgFilter
	}
	if configFile.Port != 0 {
		cfg.Port = configFile.Port
	}
	
	return cfg, nil
}

// SaveConfigToFile saves configuration to sniplicity.yaml in the project directory
func (c *Config) SaveConfigToFile() error {
	if c.ProjectDir == "" {
		return fmt.Errorf("project directory not set")
	}
	
	configPath := filepath.Join(c.ProjectDir, "sniplicity.yaml")
	
	configFile := ConfigFile{
		Name:      c.Name,
		InputDir:  c.InputDir,
		OutputDir: c.OutputDir,
		Watch:     c.Watch,
		Verbose:   c.Verbose,
		Serve:     c.Serve,
		Port:      c.Port,
		ImgSize:   &c.ImgSize,
		SvgFilter: &c.SvgFilter,
	}
	
	data, err := yaml.Marshal(configFile)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	
	// Add a header comment
	header := "# Sniplicity Configuration\n# Project structure:\n#   name: friendly project name (optional)\n#   input_dir: source files (default: snip)\n#   output_dir: built files (default: www)\n# See https://github.com/davebalmer/sniplicity for documentation\n\n"
	data = append([]byte(header), data...)
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	
	return nil
}