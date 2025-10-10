package builder

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"sniplicity/internal/config"
	"sniplicity/internal/processor"
	"sniplicity/internal/types"
	"sniplicity/internal/watcher"

	"github.com/fatih/color"
)

// Builder handles the main build process
type Builder struct {
	config     config.Config
	files      []*types.FileInfo
	snippets   map[string][]string
	templates  map[string][]string
	globals    map[string]string
	processor  *processor.Processor
}

// New creates a new Builder instance
func New(cfg config.Config) *Builder {
	return &Builder{
		config:    cfg,
		snippets:  make(map[string][]string),
		templates: make(map[string][]string),
		globals:   make(map[string]string),
		processor: processor.New(cfg.Verbose),
	}
}

// Build performs the main build process
func (b *Builder) Build() error {
	if b.config.Serve {
		green := color.New(color.FgGreen, color.Bold)
		cyan := color.New(color.FgCyan)
		fmt.Printf("%s%s is watching files in %s and serving at http://127.0.0.1:%d\n\n", 
			green.Sprint("snip"), cyan.Sprint("licity"), cyan.Sprint(b.config.InputDir), b.config.Port)
	} else if b.config.Watch {
		green := color.New(color.FgGreen, color.Bold)
		cyan := color.New(color.FgCyan)
		fmt.Printf("%s%s is watching files in %s\n\n", 
			green.Sprint("snip"), cyan.Sprint("licity"), cyan.Sprint(b.config.InputDir))
	}

	if err := b.doBuild(); err != nil {
		return err
	}

	if b.config.Serve {
		return b.hostAndWatch()
	} else if b.config.Watch {
		return b.watchFiles()
	}

	return nil
}

func (b *Builder) doBuild() error {
	if b.config.Verbose {
		green := color.New(color.FgGreen)
		fmt.Printf("Loading %s files...\n", green.Sprint("sniplicity"))
	}

	// Reset state
	b.files = nil
	b.snippets = make(map[string][]string)
	b.templates = make(map[string][]string)
	b.globals = make(map[string]string)

	// Create output directory
	if err := os.MkdirAll(b.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}

	// Get file list - this matches Python version's get_file_list exactly
	fileList, err := b.getFileList(b.config.InputDir)
	if err != nil {
		return fmt.Errorf("cannot get file list: %w", err)
	}

	// PHASE 1: Pre-load files to collect templates/snippets/globals
	// This matches Python's "Pre-loading files to collect templates..." exactly
	if b.config.Verbose {
		fmt.Println("Pre-loading files to collect templates...")
	}
	
	tempFiles := make([]*types.FileInfo, 0)
	for _, item := range fileList {
		relPath, filename, isMarkdownStr := item[0], item[1], item[2]
		inputPath := filepath.Join(b.config.InputDir, relPath, filename)
		
		isMarkdown := isMarkdownStr == "true"
		// Create FileInfo but DON'T process markdown yet in pre-loading phase
		fileInfo := types.NewFileInfoRaw(inputPath, filename, isMarkdown)
		fileInfo.OutputRelPath = relPath
		
		if err := fileInfo.LoadRaw(); err != nil {
			if b.config.Verbose {
				log.Printf("Warning: Cannot read file %s", inputPath)
			}
			continue
		}
		tempFiles = append(tempFiles, fileInfo)
	}

	// Collect snippets, templates, and globals from raw content
	if err := b.collectSnippetsAndGlobals(tempFiles); err != nil {
		return fmt.Errorf("error collecting snippets: %w", err)
	}

	// PHASE 2: Reload files with template processing
	// This matches Python's "Reloading files with template processing..."
	if b.config.Verbose {
		fmt.Println("Reloading files with template processing...")
	}
	
	b.files = make([]*types.FileInfo, 0)
	for _, item := range fileList {
		relPath, filename, isMarkdownStr := item[0], item[1], item[2]
		inputPath := filepath.Join(b.config.InputDir, relPath, filename)
		
		isMarkdown := isMarkdownStr == "true"
		fileInfo := types.NewFileInfo(inputPath, filename, isMarkdown)
		fileInfo.OutputRelPath = relPath
		
		// Now load WITH template processing (templates are available)
		if err := fileInfo.LoadWithTemplates(b.templates, b.globals); err != nil {
			if b.config.Verbose {
				log.Printf("Warning: Cannot read file %s", inputPath)
			}
			continue
		}
		b.files = append(b.files, fileInfo)
	}

	// Process files in exact Python order:
	// 1. Process includes
	if err := b.processIncludes(); err != nil {
		return fmt.Errorf("error processing includes: %w", err)
	}

	// 2. Process index commands (before snippets and variables)
	if err := b.processIndexCommands(); err != nil {
		return fmt.Errorf("error processing index commands: %w", err)
	}

	// 3. Process snippets
	if err := b.processSnippets(); err != nil {
		return fmt.Errorf("error processing snippets: %w", err)
	}

	// 4. Process variables and write files
	if err := b.processVariables(); err != nil {
		return fmt.Errorf("error processing variables: %w", err)
	}

	// Success message
	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)
	fmt.Printf("%s from %s to %s\n", 
		green.Sprint("Compiled"), cyan.Sprint(b.config.InputDir), cyan.Sprint(b.config.OutputDir))
	
	if !b.config.Watch {
		fmt.Printf("%s\n", green.Sprint("Success!"))
	}

	return nil
}

// getFileList matches Python's get_file_list exactly
func (b *Builder) getFileList(sourceDir string) ([][3]string, error) {
	var fileList [][3]string

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, filepath.Dir(path))
		if err != nil {
			return err
		}
		if relPath == "." {
			relPath = ""
		}

		filename := info.Name()
		ext := strings.ToLower(filepath.Ext(filename))

		// Check if it's a markdown file
		if ext == ".md" || ext == ".mdown" || ext == ".markdown" {
			fileList = append(fileList, [3]string{relPath, filename, "true"})
		} else if ext == ".html" || ext == ".htm" || ext == ".txt" {
			fileList = append(fileList, [3]string{relPath, filename, "false"})
		}

		return nil
	})

	return fileList, err
}

func (b *Builder) collectSnippetsAndGlobals(files []*types.FileInfo) error {
	if b.config.Verbose {
		green := color.New(color.FgGreen)
		fmt.Printf("Finding all %s, templates, and globals...\n", green.Sprint("snippets"))
		fmt.Println("Processing files in this order:")
		for _, file := range files {
			fmt.Printf("  %s\n", file.Filename)
		}
	}

	// First collect all snippets and templates - matches Python exactly
	for _, fileInfo := range files {
		err := b.processor.CollectSnippetsFromFile(fileInfo, b.snippets, b.templates, b.config.Verbose)
		if err != nil {
			return err
		}
	}

	// Then collect all globals - matches Python exactly  
	for _, fileInfo := range files {
		err := b.processor.CollectGlobalsFromFile(fileInfo, b.globals, b.config.Verbose)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) processIncludes() error {
	if b.config.Verbose {
		cyan := color.New(color.FgCyan)
		fmt.Printf("Processing %s...\n", cyan.Sprint("includes"))
	}

	for _, fileInfo := range b.files {
		err := b.processor.ProcessIncludes(fileInfo, b.config.InputDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) processIndexCommands() error {
	if b.config.Verbose {
		fmt.Println("Processing index commands...")
	}

	for _, fileInfo := range b.files {
		err := b.processor.ProcessIndexCommands(fileInfo, b.config.InputDir, b.templates, b.snippets, b.globals)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) processSnippets() error {
	if b.config.Verbose {
		green := color.New(color.FgGreen)
		fmt.Printf("Processing %s in each file...\n", green.Sprint("snippets"))
	}

	for _, fileInfo := range b.files {
		err := b.processor.ProcessSnippets(fileInfo, b.snippets)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) processVariables() error {
	if b.config.Verbose {
		fmt.Println("Writing files...")
	}

	for _, fileInfo := range b.files {
		err := b.processor.ProcessVariables(fileInfo, b.config.OutputDir, b.templates, b.snippets, b.globals, b.config.Verbose)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) watchFiles() error {
	w, err := watcher.New(b.config.InputDir, func() {
		if err := b.doBuild(); err != nil {
			log.Printf("Build error: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("cannot create file watcher: %w", err)
	}
	defer w.Close()

	// Block forever
	select {}
}

// hostAndWatch starts both file watching and web server with graceful shutdown
func (b *Builder) hostAndWatch() error {
	// Create file watcher
	w, err := watcher.New(b.config.InputDir, func() {
		if err := b.doBuild(); err != nil {
			log.Printf("Build error: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("cannot create file watcher: %w", err)
	}
	defer w.Close()

	// Create custom handler that properly handles absolute paths for local navigation
	fileServer := http.FileServer(http.Dir(b.config.OutputDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle root path by serving index.html directly
		if r.URL.Path == "/" {
			// Serve index.html file directly without redirect
			indexPath := filepath.Join(b.config.OutputDir, "index.html")
			http.ServeFile(w, r, indexPath)
			return
		}
		
		// For all other requests, use the file server normally
		fileServer.ServeHTTP(w, r)
	})

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", b.config.Port),
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		cyan := color.New(color.FgCyan)
		fmt.Printf("Starting web server at %s\n", cyan.Sprintf("http://127.0.0.1:%d", b.config.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("%s\n", color.New(color.FgYellow).Sprint("Press Ctrl+C to stop watching and server"))
	
	// Wait for signal
	<-c
	
	green := color.New(color.FgGreen)
	fmt.Printf("\n%s\n", green.Sprint("Stopping file watcher and web server..."))
	
	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	
	fmt.Printf("%s\n", green.Sprint("Done!"))
	return nil
}