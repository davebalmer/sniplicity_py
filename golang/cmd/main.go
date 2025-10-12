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
	
	if showVersion {
		fmt.Printf("sniplicity %s\n", version)
		return
	}
	
	// Determine project directory and handle backward compatibility
	var explicitInputDir, explicitOutputDir string
	var explicitImgSize *bool
	
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
	
	// Project directory is always the current working directory
	projectDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Cannot get current working directory: %v", err)
	}
	
	if cfg.InputDir != "" || cfg.OutputDir != "" {
		// Legacy mode: explicit -i and/or -o flags provided
		explicitInputDir = cfg.InputDir
		explicitOutputDir = cfg.OutputDir
		cfg.InputDir = ""  // Reset so we can override from config
		cfg.OutputDir = "" // Reset so we can override from config
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
	
	// If serve is enabled, automatically enable watch mode
	if cfg.Serve {
		cfg.Watch = true
	}
	
	printBanner()
	
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
	
	// Check if input directory exists
	if _, err := os.Stat(cfg.InputDir); os.IsNotExist(err) {
		log.Fatalf("Source directory %s does not exist", cfg.InputDir)
	}
	
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		log.Fatalf("Cannot create output directory: %v", err)
	}
	
	// Initialize and run the builder
	b := builder.New(cfg)
	if err := b.Build(); err != nil {
		log.Fatalf("Build failed: %v", err)
	}
}