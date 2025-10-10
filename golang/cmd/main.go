package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
	
	// If serve is enabled, automatically enable watch mode
	if cfg.Serve {
		cfg.Watch = true
	}
	
	printBanner()
	
	if cfg.OutputDir == "" {
		flag.Usage()
		os.Exit(1)
	}
	
	// Convert to absolute paths
	if cfg.InputDir != "" {
		absInput, err := filepath.Abs(cfg.InputDir)
		if err != nil {
			log.Fatalf("Invalid input directory: %v", err)
		}
		cfg.InputDir = absInput
	} else {
		cfg.InputDir, _ = os.Getwd()
	}
	
	absOutput, err := filepath.Abs(cfg.OutputDir)
	if err != nil {
		log.Fatalf("Invalid output directory: %v", err)
	}
	cfg.OutputDir = absOutput
	
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