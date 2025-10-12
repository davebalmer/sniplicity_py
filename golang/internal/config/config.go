package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration options for sniplicity
type Config struct {
	ProjectDir string `yaml:"-"`          // Project root directory (where sniplicity.yaml lives)
	InputDir   string `yaml:"input_dir"`  // Input directory relative to project root (default: "snip")
	OutputDir  string `yaml:"output_dir"` // Output directory relative to project root (default: "www")
	Watch      bool   `yaml:"watch"`
	Verbose    bool   `yaml:"verbose"`
	Serve      bool   `yaml:"serve"`      // Enable web server and watch mode
	Port       int    `yaml:"port"`       // Port for web server (default 3000)
	ImgSize    bool   `yaml:"imgsize"`    // Automatically add width/height attributes to img tags (default: true)
}

// ConfigFile represents the structure saved to sniplicity.yaml
type ConfigFile struct {
	InputDir  string `yaml:"input_dir"`
	OutputDir string `yaml:"output_dir"`
	Watch     bool   `yaml:"watch"`
	Verbose   bool   `yaml:"verbose"`  
	Serve     bool   `yaml:"serve"`
	Port      int    `yaml:"port"`
	ImgSize   *bool  `yaml:"imgsize"` // Pointer to distinguish between unset and false
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
	cfg.ProjectDir = projectDir
	
	// If no config file exists, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}
	
	var configFile ConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		return cfg, fmt.Errorf("parsing config file: %w", err)
	}
	
	// Apply config file values, using defaults if not specified
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
		InputDir:  c.InputDir,
		OutputDir: c.OutputDir,
		Watch:     c.Watch,
		Verbose:   c.Verbose,
		Serve:     c.Serve,
		Port:      c.Port,
		ImgSize:   &c.ImgSize,
	}
	
	data, err := yaml.Marshal(configFile)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	
	// Add a header comment
	header := "# Sniplicity Configuration\n# Project structure:\n#   input_dir: source files (default: snip)\n#   output_dir: built files (default: www)\n# See https://github.com/davebalmer/sniplicity for documentation\n\n"
	data = append([]byte(header), data...)
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	
	return nil
}