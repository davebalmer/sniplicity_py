package config

// Config holds all configuration options for sniplicity
type Config struct {
	InputDir  string
	OutputDir string
	Watch     bool
	Verbose   bool
	Serve     bool   // Enable web server and watch mode
	Port      int    // Port for web server (default 3000)
}