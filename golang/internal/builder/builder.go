package builder

import (
	"context"
	"fmt"
	"io"
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
	"sniplicity/internal/web"

	"github.com/atotto/clipboard"
	"github.com/fatih/color"
	"github.com/skratchdot/open-golang/open"
)

// Builder handles the main build process
type Builder struct {
	config       config.Config
	files        []*types.FileInfo
	snippets     map[string][]string
	templates    map[string][]string
	globals      map[string]string
	processor    *processor.Processor
	clipboardOnly bool // When true, copy URL to clipboard instead of opening browser
}

// New creates a new Builder instance
func New(cfg config.Config) *Builder {
	return &Builder{
		config:        cfg,
		snippets:      make(map[string][]string),
		templates:     make(map[string][]string),
		globals:       make(map[string]string),
		processor:     processor.New(cfg.Verbose),
		clipboardOnly: false, // Default to opening browser
	}
}

// NewWithClipboardOnly creates a new Builder instance that only copies URLs to clipboard
func NewWithClipboardOnly(cfg config.Config) *Builder {
	return &Builder{
		config:        cfg,
		snippets:      make(map[string][]string),
		templates:     make(map[string][]string),
		globals:       make(map[string]string),
		processor:     processor.New(cfg.Verbose),
		clipboardOnly: true, // Copy to clipboard instead of opening browser
	}
}

// Build performs the main build process
func (b *Builder) Build() error {
	if b.config.Serve {
		green := color.New(color.FgGreen, color.Bold)
		cyan := color.New(color.FgCyan)
		fmt.Printf("%s%s is watching files in %s and serving at http://127.0.0.1:%d\n\n", 
			green.Sprint("snip"), cyan.Sprint("licity"), cyan.Sprint(b.config.GetAbsoluteInputDir()), b.config.Port)
	} else if b.config.Watch {
		green := color.New(color.FgGreen, color.Bold)
		cyan := color.New(color.FgCyan)
		fmt.Printf("%s%s is watching files in %s\n\n", 
			green.Sprint("snip"), cyan.Sprint("licity"), cyan.Sprint(b.config.GetAbsoluteInputDir()))
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

// StartProjectSelectionMode starts the web server without building any project
// This is used when sniplicity is started without command line parameters
func (b *Builder) StartProjectSelectionMode() error {
	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)
	fmt.Printf("%s%s project selector starting at http://127.0.0.1:%d\n\n", 
		green.Sprint("snip"), cyan.Sprint("licity"), b.config.Port)

	return b.startWebServerOnly()
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
	if err := os.MkdirAll(b.config.GetAbsoluteOutputDir(), 0755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}

	// Get file list - this matches Python version's get_file_list exactly
	fileList, err := b.getFileList(b.config.GetAbsoluteInputDir())
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
		inputPath := filepath.Join(b.config.GetAbsoluteInputDir(), relPath, filename)
		
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
		inputPath := filepath.Join(b.config.GetAbsoluteInputDir(), relPath, filename)
		
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

	// 5. Copy assets (non-processed files) - AFTER all processing is complete
	if err := b.copyAssets(); err != nil {
		return fmt.Errorf("error copying assets: %w", err)
	}

	// Success message
	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)
	fmt.Printf("%s from %s to %s\n", 
		green.Sprint("Compiled"), cyan.Sprint(b.config.GetAbsoluteInputDir()), cyan.Sprint(b.config.GetAbsoluteOutputDir()))
	
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
		} else if ext == ".html" || ext == ".htm" {
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
		err := b.processor.ProcessIncludes(fileInfo, b.config.GetAbsoluteInputDir())
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
		err := b.processor.ProcessIndexCommands(fileInfo, b.config.GetAbsoluteInputDir(), b.templates, b.snippets, b.globals)
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
		err := b.processor.ProcessVariables(fileInfo, b.config.GetAbsoluteOutputDir(), b.templates, b.snippets, b.globals, b.config.ImgSize, b.config.Verbose)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) watchFiles() error {
	w, err := watcher.New(b.config.GetAbsoluteInputDir(), func() {
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
	w, err := watcher.New(b.config.GetAbsoluteInputDir(), func() {
		if err := b.doBuild(); err != nil {
			log.Printf("Build error: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("cannot create file watcher: %w", err)
	}
	defer w.Close()

	// Create custom handler that properly handles absolute paths for local navigation
	fileServer := http.FileServer(http.Dir(b.config.GetAbsoluteOutputDir()))
	
	// Create web interface handler
	webHandler, err := web.NewHandler(&b.config, func(newConfig *config.Config) error {
		// This callback is called when configuration is saved via web interface
		// Update the config and trigger a rebuild
		b.config = *newConfig
		
		// Rebuild with the new configuration
		if err := b.doBuild(); err != nil {
			return fmt.Errorf("rebuild failed: %w", err)
		}
		
		return nil
	}, func(newProjectPath string) error {
		// This callback is called when a project is switched via web interface
		// Reload configuration from the new project and rebuild
		newConfig, err := config.LoadConfigFromFile(newProjectPath)
		if err != nil {
			return fmt.Errorf("loading config from new project: %w", err)
		}
		
		b.config = newConfig
		
		// Rebuild with the new project
		if err := b.doBuild(); err != nil {
			return fmt.Errorf("rebuild failed: %w", err)
		}
		
		return nil
	})
	if err != nil {
		return fmt.Errorf("creating web handler: %w", err)
	}
	
	// Add current project to recent projects when starting server
	if err := webHandler.AddCurrentProjectToRecent(); err != nil {
		fmt.Printf("Warning: could not add current project to recent list: %v\n", err)
	}
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle sniplicity configuration interface
		if strings.HasPrefix(r.URL.Path, "/sniplicity") {
			webHandler.ServeHTTP(w, r)
			return
		}
		
		// Handle root path
		if r.URL.Path == "/" {
			// If not in legacy mode (no explicit command line params), redirect to project selector
			if !b.config.LegacyMode {
				http.Redirect(w, r, "/sniplicity", http.StatusTemporaryRedirect)
				return
			}
			
			// Legacy mode: serve index.html file directly without redirect
			indexPath := filepath.Join(b.config.GetAbsoluteOutputDir(), "index.html")
			http.ServeFile(w, r, indexPath)
			return
		}
		
		// Custom handling for file vs directory conflicts
		// Always check if the requested path corresponds to an actual file first
		requestedPath := strings.TrimPrefix(r.URL.Path, "/")
		filePath := filepath.Join(b.config.GetAbsoluteOutputDir(), requestedPath)
		
		// Security: clean the path to prevent directory traversal
		filePath = filepath.Clean(filePath)
		outputDir := b.config.GetAbsoluteOutputDir()
		if !strings.HasPrefix(filePath, outputDir) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		
		if b.config.Verbose {
			fmt.Printf("DEBUG: Requested path: %s -> File path: %s\n", r.URL.Path, filePath)
		}
		
		// Check if the exact file exists
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			// File exists and is not a directory, serve it directly
			if b.config.Verbose {
				fmt.Printf("DEBUG: Serving file directly: %s\n", filePath)
			}
			http.ServeFile(w, r, filePath)
			return
		}
		
		// If no file found, let the default file server handle it (for directories, etc.)
		if b.config.Verbose {
			fmt.Printf("DEBUG: Using default file server for: %s\n", r.URL.Path)
		}
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
		serverURL := fmt.Sprintf("http://127.0.0.1:%d", b.config.Port)
		
		fmt.Printf("Starting web server at %s\n", cyan.Sprint(serverURL))
		
		// Try to copy URL to clipboard
		if err := clipboard.WriteAll(serverURL); err == nil {
			fmt.Printf("✓ URL copied to clipboard - you can paste it anywhere!\n")
		} else {
			fmt.Printf("ℹ Copy this URL: %s\n", cyan.Sprint(serverURL))
		}
		
		// Try to open browser automatically (unless clipboard-only mode)
		if !b.clipboardOnly {
			if err := open.Run(serverURL); err == nil {
				fmt.Printf("✓ Opening in your default browser...\n")
			} else {
				fmt.Printf("ℹ Please open the URL above in your browser\n")
			}
		}
		
		fmt.Println()
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()	// Handle graceful shutdown
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

// startWebServerOnly starts just the web server for project selection, without file watching
func (b *Builder) startWebServerOnly() error {
	var w *watcher.Watcher
	
	// Create file watcher if we have a project
	if b.config.ProjectDir != "" && b.config.InputDir != "" {
		var err error
		w, err = watcher.New(b.config.GetAbsoluteInputDir(), func() {
			if err := b.doBuild(); err != nil {
				log.Printf("Build error: %v", err)
			}
		})
		if err != nil {
			log.Printf("Warning: Cannot create file watcher: %v", err)
		} else {
			defer w.Close()
		}
	}
	// Create web interface handler
	webHandler, err := web.NewHandler(&b.config, func(newConfig *config.Config) error {
		// This callback is called when configuration is saved via web interface
		// Update the config and trigger a rebuild
		b.config = *newConfig
		
		// Rebuild with the new configuration
		if err := b.doBuild(); err != nil {
			return fmt.Errorf("rebuild failed: %w", err)
		}
		
		return nil
	}, func(newProjectPath string) error {
		// This callback is called when a project is switched via web interface
		// Reload configuration from the new project and rebuild
		newConfig, err := config.LoadConfigFromFile(newProjectPath)
		if err != nil {
			return fmt.Errorf("loading config from new project: %w", err)
		}
		
		b.config = newConfig
		
		// Rebuild with the new project
		if err := b.doBuild(); err != nil {
			return fmt.Errorf("rebuild failed: %w", err)
		}
		
		return nil
	})
	if err != nil {
		return fmt.Errorf("creating web handler: %w", err)
	}
	
	// Add current project to recent projects when starting server
	if err := webHandler.AddCurrentProjectToRecent(); err != nil {
		fmt.Printf("Warning: could not add current project to recent list: %v\n", err)
	}
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle sniplicity configuration interface
		if strings.HasPrefix(r.URL.Path, "/sniplicity") {
			webHandler.ServeHTTP(w, r)
			return
		}
		
		// If we have a project with an output directory, serve files from it
		if b.config.ProjectDir != "" && b.config.OutputDir != "" {
			outputDir := b.config.GetAbsoluteOutputDir()
			
			// Handle root path by serving index.html directly
			if r.URL.Path == "/" {
				indexPath := filepath.Join(outputDir, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					http.ServeFile(w, r, indexPath)
					return
				}
			}
			
			// Serve other files from output directory
			requestedPath := strings.TrimPrefix(r.URL.Path, "/")
			filePath := filepath.Join(outputDir, requestedPath)
			
			// Security: clean the path to prevent directory traversal
			filePath = filepath.Clean(filePath)
			if !strings.HasPrefix(filePath, outputDir) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			
			// Check if the exact file exists
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				http.ServeFile(w, r, filePath)
				return
			}
			
			// If no file found, use default file server for directory listings, etc.
			fileServer := http.FileServer(http.Dir(outputDir))
			fileServer.ServeHTTP(w, r)
			return
		}
		
		// No project active - redirect to project selection
		if r.URL.Path != "/sniplicity" {
			http.Redirect(w, r, "/sniplicity", http.StatusTemporaryRedirect)
			return
		}
	})

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", b.config.Port),
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		cyan := color.New(color.FgCyan)
		serverURL := fmt.Sprintf("http://127.0.0.1:%d", b.config.Port)
		
		// In project selection mode, direct users to the /sniplicity endpoint
		projectSelectorURL := serverURL + "/sniplicity"
		
		fmt.Printf("Starting web server at %s\n", cyan.Sprint(serverURL))
		
		// Try to copy project selector URL to clipboard
		if err := clipboard.WriteAll(projectSelectorURL); err == nil {
			fmt.Printf("✓ Project selector URL copied to clipboard - you can paste it anywhere!\n")
		} else {
			fmt.Printf("ℹ Copy this URL: %s\n", cyan.Sprint(projectSelectorURL))
		}
		
		// Try to open browser automatically to project selector (unless clipboard-only mode)
		if !b.clipboardOnly {
			if err := open.Run(projectSelectorURL); err == nil {
				fmt.Printf("✓ Opening project selector in your default browser...\n")
			} else {
				fmt.Printf("ℹ Please open the URL above in your browser\n")
			}
		}
		
		fmt.Println()
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("%s\n", color.New(color.FgYellow).Sprint("Press Ctrl+C to stop server"))
	
	// Wait for signal
	<-c
	
	green := color.New(color.FgGreen)
	fmt.Printf("\n%s\n", green.Sprint("Stopping web server..."))
	
	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	
	fmt.Printf("%s\n", green.Sprint("Done!"))
	return nil
}

// copyAssets copies all non-processed files (CSS, JS, images, etc.) from input to output directory
func (b *Builder) copyAssets() error {
	inputDir := b.config.GetAbsoluteInputDir()
	outputDir := b.config.GetAbsoluteOutputDir()
	
	if b.config.Verbose {
		green := color.New(color.FgGreen)
		fmt.Printf("Copying %s...\n", green.Sprint("assets"))
	}
	
	return filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path from input directory
		relPath, err := filepath.Rel(inputDir, path)
		if err != nil {
			return err
		}

		// Check if this file should be processed (not copied)
		ext := strings.ToLower(filepath.Ext(path))
		isProcessedFile := ext == ".md" || ext == ".mdown" || ext == ".markdown" || 
		                   ext == ".html" || ext == ".htm"
		
		if isProcessedFile {
			// Skip files that are processed by sniplicity
			return nil
		}

		// Copy the asset file
		outputPath := filepath.Join(outputDir, relPath)
		
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", filepath.Dir(outputPath), err)
		}

		// Copy the file
		if err := b.copyFile(path, outputPath); err != nil {
			return fmt.Errorf("copying %s to %s: %w", path, outputPath, err)
		}

		if b.config.Verbose {
			cyan := color.New(color.FgCyan)
			fmt.Printf("  Copied %s\n", cyan.Sprint(relPath))
		}

		return nil
	})
}

// copyFile copies a single file from src to dst
func (b *Builder) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}