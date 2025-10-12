package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"sniplicity/internal/builder"
	"sniplicity/internal/config"
)

const version = "0.1.10"

func printBanner() {
	fmt.Printf("\033[32m            _      \033[36m _  _       _             \033[0m\n")
	fmt.Printf("\033[32m           (_)     \033[36m| |(_)     (_)  _         \033[0m\n")
	fmt.Printf("\033[32m  ___ ____  _ ____ \033[36m| | _  ____ _ _| |_ _   _  \033[0m\n")
	fmt.Printf("\033[32m /___)  _ \\| |  _ \\\033[36m| || |/ ___) (_   _) | | |\033[0m\n")
	fmt.Printf("\033[32m|___ | | | | | |_| \033[36m| || ( (___| | | |_| |_| |\033[0m\n")
	fmt.Printf("\033[32m(___/|_| |_|_|  __/\033[36m \\_)_|\\____)_|  \\__)\\__  |\033[0m\n")
	fmt.Printf("\033[32m             |_|   \033[36m                   (____/ \033[0m\n")
	fmt.Printf("  \033[2;37mhttp://github.com/davebalmer/sniplicity\033[0m\n")
}

func main() {
	// Command line flags
	var cfg config.Config
	var imgSizeFlag string
	
	flag.StringVar(&cfg.InputDir, "i", "", "source directory")
	flag.StringVar(&cfg.InputDir, "in", "", "source directory")
	flag.StringVar(&cfg.OutputDir, "o", "", "output directory for compiled files")
	flag.StringVar(&cfg.OutputDir, "out", "", "output directory for compiled files")
	flag.BoolVar(&cfg.Watch, "w", false, "keep watching the input directory")
	flag.BoolVar(&cfg.Watch, "watch", false, "keep watching the input directory")
	flag.BoolVar(&cfg.Verbose, "v", false, "extra console messages")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "extra console messages")
	flag.BoolVar(&cfg.Serve, "s", false, "start web server and enable watch mode")
	flag.BoolVar(&cfg.Serve, "serve", false, "start web server and enable watch mode")
	flag.IntVar(&cfg.Port, "p", 3000, "port for web server (default 3000)")
	flag.IntVar(&cfg.Port, "port", 3000, "port for web server (default 3000)")
	flag.StringVar(&imgSizeFlag, "imgsize", "", "automatically add width/height to img tags (on/off, default: on)")
	
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "show version")
	
	// Track if -s flag was explicitly provided (for clipboard-only behavior)
	var explicitServeFlag bool
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\033[1;37mBuild simple static websites using:\033[0m\n\n")
		fmt.Fprintf(os.Stderr, "  - snippets with \033[32m<!-- copy x -->\033[0m and \033[32m<!-- paste x -->\033[0m\n")
		fmt.Fprintf(os.Stderr, "  - variables using \033[32m<!-- set y -->\033[0m and \033[32m<!-- global z -->\033[0m\n")
		fmt.Fprintf(os.Stderr, "  - include files with \033[32m<!-- include filename.html -->\033[0m\n\n")
		fmt.Fprintf(os.Stderr, "  \033[1;33mSee README.md to get started.\033[0m\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -i source_folder -o destination_folder [-w] [-v] [-s [-p port]]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	
	flag.Parse()
	
	// Check if -s flag was explicitly provided
	for _, arg := range os.Args[1:] {
		if arg == "-s" || arg == "-serve" {
			explicitServeFlag = true
			break
		}
	}
	
	if showVersion {
		fmt.Printf("sniplicity %s\n", version)
		return
	}
	
	// Determine project directory and handle backward compatibility
	var explicitInputDir, explicitOutputDir string
	var explicitImgSize *bool
	var isLegacyMode bool
	
	// Parse imgsize flag
	if imgSizeFlag != "" {
		switch strings.ToLower(imgSizeFlag) {
		case "on", "true", "1", "yes":
			explicitImgSize = &[]bool{true}[0]
		case "off", "false", "0", "no":
			explicitImgSize = &[]bool{false}[0]
		default:
			log.Fatalf("Invalid value for --imgsize: %s (use 'on' or 'off')", imgSizeFlag)
		}
	}
	
	// Project directory determination
	var projectDir string
	var err error
	
	// Check for any explicit command line flags that indicate legacy usage
	isLegacyMode = cfg.InputDir != "" || cfg.OutputDir != "" || cfg.Watch || cfg.Verbose || cfg.Port != 3000 || explicitImgSize != nil
	
	// Special case: if only -s (serve) flag is provided, treat as project selection mode, not legacy mode
	if cfg.Serve && cfg.InputDir == "" && cfg.OutputDir == "" && !cfg.Watch && !cfg.Verbose && cfg.Port == 3000 && explicitImgSize == nil {
		isLegacyMode = false
	}
	
	// If no flags at all were provided, start in project selection mode with serve enabled
	if !isLegacyMode && !cfg.Serve {
		cfg.Serve = true
	}
	
	if cfg.InputDir != "" || cfg.OutputDir != "" {
		// Legacy mode: explicit -i and/or -o flags provided
		explicitInputDir = cfg.InputDir
		explicitOutputDir = cfg.OutputDir
		
		// In legacy mode, determine project directory from input directory
		if cfg.InputDir != "" {
			// Use parent directory of input directory as project directory
			inputAbsPath, err := filepath.Abs(cfg.InputDir)
			if err != nil {
				log.Fatalf("Cannot get absolute path for input directory: %v", err)
			}
			projectDir = filepath.Dir(inputAbsPath)
		} else {
			// Fallback to current working directory
			projectDir, err = os.Getwd()
			if err != nil {
				log.Fatalf("Cannot get current working directory: %v", err)
			}
		}
		
		cfg.InputDir = ""  // Reset so we can override from config
		cfg.OutputDir = "" // Reset so we can override from config
	} else {
		// Project directory is the current working directory
		projectDir, err = os.Getwd()
		if err != nil {
			log.Fatalf("Cannot get current working directory: %v", err)
		}
	}
	
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		log.Fatalf("Cannot get absolute project directory: %v", err)
	}
	
	// Load configuration from file (if exists)
	fileCfg, err := config.LoadConfigFromFile(absProjectDir)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	
	// Command line flags override config file values
	if explicitInputDir != "" {
		// Legacy mode: -i flag overrides everything (absolute path)
		fileCfg.InputDir = explicitInputDir
		// Make it relative to project dir if possible, otherwise keep absolute
		if rel, err := filepath.Rel(absProjectDir, explicitInputDir); err == nil && !strings.HasPrefix(rel, "..") {
			fileCfg.InputDir = rel
		} else {
			fileCfg.InputDir = explicitInputDir // Keep absolute
		}
	}
	if explicitOutputDir != "" {
		// Legacy mode: -o flag overrides everything (absolute path)
		fileCfg.OutputDir = explicitOutputDir
		// Make it relative to project dir if possible, otherwise keep absolute
		if rel, err := filepath.Rel(absProjectDir, explicitOutputDir); err == nil && !strings.HasPrefix(rel, "..") {
			fileCfg.OutputDir = rel
		} else {
			fileCfg.OutputDir = explicitOutputDir // Keep absolute
		}
	}
	if cfg.Watch {
		fileCfg.Watch = cfg.Watch
	}
	if cfg.Verbose {
		fileCfg.Verbose = cfg.Verbose
	}
	if cfg.Serve {
		fileCfg.Serve = cfg.Serve
	}
	if cfg.Port != 3000 { // Only override if explicitly set
		fileCfg.Port = cfg.Port
	}
	if explicitImgSize != nil {
		fileCfg.ImgSize = *explicitImgSize
	}
	
	cfg = fileCfg
	
	// Set legacy mode flag
	cfg.LegacyMode = isLegacyMode
	
	// If serve is enabled, automatically enable watch mode
	if cfg.Serve {
		cfg.Watch = true
	}
	
	printBanner()
	
	// In project selection mode (non-legacy with serve), skip project validation and building
	if !isLegacyMode && cfg.Serve {
		// Try to load config from current working directory if it has a sniplicity.yaml file
		wd, err := os.Getwd()
		if err == nil {
			configPath := filepath.Join(wd, "sniplicity.yaml")
			if _, err := os.Stat(configPath); err == nil {
				// Config file exists in working directory, load it
				if workingDirCfg, err := config.LoadConfigFromFile(wd); err == nil {
					// If config was successfully loaded from working directory, use it
					// but preserve the serve flag and other command-line overrides
					workingDirCfg.Serve = cfg.Serve
					if cfg.Port != 3000 {
						workingDirCfg.Port = cfg.Port
					}
					if cfg.Verbose {
						workingDirCfg.Verbose = cfg.Verbose
					}
					cfg = workingDirCfg
				}
			}
		}
		
		// Start directly in web server mode for project selection
		var b *builder.Builder
		if explicitServeFlag {
			// Use clipboard-only mode when -s flag was explicitly provided
			b = builder.NewWithClipboardOnly(cfg)
		} else {
			// Use normal mode (with browser opening) when no args provided
			b = builder.New(cfg)
		}
		if err := b.StartProjectSelectionMode(); err != nil {
			log.Fatalf("Failed to start project selection mode: %v", err)
		}
		return
	}
	
	// Check if input directory exists
	absInputDir := cfg.GetAbsoluteInputDir()
	if _, err := os.Stat(absInputDir); os.IsNotExist(err) {
		log.Fatalf("Input directory %s does not exist", absInputDir)
	}
	
	// Create output directory if it doesn't exist
	absOutputDir := cfg.GetAbsoluteOutputDir()
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		log.Fatalf("Cannot create output directory: %v", err)
	}
	
	// Initialize and run the builder
	var b *builder.Builder
	if cfg.Serve && explicitServeFlag {
		// Use clipboard-only mode when -s flag was explicitly provided
		b = builder.NewWithClipboardOnly(cfg)
	} else {
		b = builder.New(cfg)
	}
	if err := b.Build(); err != nil {
		log.Fatalf("Build failed: %v", err)
	}
}