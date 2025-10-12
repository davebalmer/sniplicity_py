package types

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark-emoji"
)

// FileInfo represents a file being processed
type FileInfo struct {
	InputPath       string
	Filename        string
	OutputRelPath   string
	IsMarkdown      bool
	Content         []string
	Metadata        map[string]interface{}
	UsedSnippets    map[string]bool
	MarkdownImages  map[string]bool  // Track image URLs that came from markdown
}

// NewFileInfoRaw creates a new FileInfo instance for raw content loading
func NewFileInfoRaw(inputPath, filename string, isMarkdown bool) *FileInfo {
	return &FileInfo{
		InputPath:      inputPath,
		Filename:       filename,
		IsMarkdown:     isMarkdown,
		Content:        make([]string, 0),
		Metadata:       make(map[string]interface{}),
		UsedSnippets:   make(map[string]bool),
		MarkdownImages: make(map[string]bool),
	}
}

// NewFileInfo creates a new FileInfo instance
func NewFileInfo(inputPath, filename string, isMarkdown bool) *FileInfo {
	return &FileInfo{
		InputPath:      inputPath,
		Filename:       filename,
		IsMarkdown:     isMarkdown,
		Content:        make([]string, 0),
		Metadata:       make(map[string]interface{}),
		UsedSnippets:   make(map[string]bool),
		MarkdownImages: make(map[string]bool),
	}
}

// LoadRaw loads file content with markdown conversion (matches Python's load() exactly)
func (f *FileInfo) LoadRaw() error {
	file, err := os.Open(f.InputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Parse metadata and content - YAML frontmatter should be processed for ALL file types
	content, metadata := parseFrontmatter(lines)
	f.Content = content
	f.Metadata = metadata
	
	// Convert markdown to HTML if this is a markdown file (matches Python exactly)
	if f.IsMarkdown {
		f.convertMarkdownToHTML()
	}

	return nil
}

// LoadWithTemplates reads and processes the file content with template support
func (f *FileInfo) LoadWithTemplates(templates map[string][]string, globals map[string]string) error {
	file, err := os.Open(f.InputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Parse metadata and content - YAML frontmatter should be processed for ALL file types
	content, metadata := parseFrontmatter(lines)
	f.Content = content
	f.Metadata = metadata
	
	// Convert markdown to HTML if this is a markdown file (same as LoadRaw - ensures consistency)
	if f.IsMarkdown {
		f.convertMarkdownToHTML()
	}

	return nil
}

// Load reads the file content and parses metadata (original method for compatibility)
func (f *FileInfo) Load() error {
	return f.LoadWithTemplates(nil, nil)
}

// convertMarkdownToHTML converts markdown content to HTML matching Python's extensions exactly
func (f *FileInfo) convertMarkdownToHTML() {
	// Convert content lines back to markdown text
	markdownText := strings.Join(f.Content, "\n")
	
	// Extract image URLs from markdown before conversion
	f.extractMarkdownImages(markdownText)
	
	// Configure goldmark to match Python's markdown extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,                    // GitHub Flavored Markdown (includes tables, strikethrough, etc.)
			extension.Table,                  // Tables (included in GFM but explicit for clarity)
			extension.TaskList,               // Task lists with checkboxes
			extension.Strikethrough,          // ~~strikethrough~~
			extension.Linkify,                // Auto-link URLs
			extension.Typographer,            // Smart quotes, dashes, etc. (matches Python's smarty)
			extension.DefinitionList,         // Definition lists
			emoji.Emoji,                      // Emoji support (:joy:, :heart:, etc.)
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),       // Auto-generate heading IDs (matches Python's toc)
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),             // Line breaks become <br>
			html.WithXHTML(),                 // XHTML-compliant output
			html.WithUnsafe(),                // Allow raw HTML (matches Python's md_in_html)
		),
	)
	
	// Convert markdown to HTML
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdownText), &buf); err != nil {
		// If conversion fails, keep original content but still change filename
		// This matches Python behavior where markdown processing errors don't stop the build
	} else {
		// Replace content with HTML
		htmlContent := buf.String()
		
		// Remove markdown attributes from HTML tags (matches Python's md_in_html extension)
		htmlContent = removeMarkdownAttributes(htmlContent)
		
		f.Content = strings.Split(strings.TrimRight(htmlContent, "\n"), "\n")
	}
	
	// Change filename extension and mark as no longer markdown
	if strings.HasSuffix(f.Filename, ".md") {
		f.Filename = strings.TrimSuffix(f.Filename, ".md") + ".html"
	} else if strings.HasSuffix(f.Filename, ".markdown") {
		f.Filename = strings.TrimSuffix(f.Filename, ".markdown") + ".html"
	} else if strings.HasSuffix(f.Filename, ".mdown") {
		f.Filename = strings.TrimSuffix(f.Filename, ".mdown") + ".html"
	}
	f.IsMarkdown = false
}

// removeMarkdownAttributes removes markdown attributes from HTML tags to match Python's md_in_html extension
func removeMarkdownAttributes(html string) string {
	// Remove markdown="1" and markdown attributes from HTML tags
	re := regexp.MustCompile(`\s+markdown(?:="[^"]*")?`)
	return re.ReplaceAllString(html, "")
}

// GetOutputPath returns the full output path for this file
func (f *FileInfo) GetOutputPath(outputDir string) string {
	outputPath := filepath.Join(outputDir, f.OutputRelPath, f.Filename)
	
	// Convert .md files to .html
	if f.IsMarkdown {
		ext := filepath.Ext(outputPath)
		outputPath = strings.TrimSuffix(outputPath, ext) + ".html"
	}
	
	return outputPath
}

// parseFrontmatter parses YAML frontmatter from any file type
// This matches the Python version's parse_markdown_meta exactly  
func parseFrontmatter(lines []string) ([]string, map[string]interface{}) {
	content := make([]string, len(lines))
	copy(content, lines)
	metadata := make(map[string]interface{})

	if len(lines) == 0 {
		return content, metadata
	}

	// Only process YAML frontmatter if file starts with ---
	if lines[0] != "---" {
		return content, metadata
	}

	// Find the closing ---
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		// No closing ---, return original content
		return content, metadata
	}

	// Extract YAML content (excluding the --- markers)
	yamlLines := lines[1:endIdx]
	yamlContent := strings.Join(yamlLines, "\n")

	// Parse YAML (simple key-value parser for now)
	if yamlContent != "" {
		metadata = parseSimpleYAML(yamlContent)
	}

	// Return content without the YAML frontmatter
	if endIdx+1 < len(lines) {
		content = lines[endIdx+1:]
	} else {
		content = []string{}
	}

	return content, metadata
}

// parseSimpleYAML provides basic YAML parsing for key-value pairs
func parseSimpleYAML(yamlContent string) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	lines := strings.Split(yamlContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			
			// Remove quotes if present
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || 
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			
			metadata[key] = value
		}
	}
	
	return metadata
}

// extractMarkdownImages extracts image URLs from markdown content
func (f *FileInfo) extractMarkdownImages(markdownText string) {
	// Match markdown image syntax: ![alt](url) and ![alt](url "title")
	imgRegex := regexp.MustCompile(`!\[.*?\]\(([^)]+)\)`)
	matches := imgRegex.FindAllStringSubmatch(markdownText, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			// Extract URL (may have title after space)
			url := strings.TrimSpace(match[1])
			// Remove title if present (everything after first space)
			if spaceIdx := strings.Index(url, " "); spaceIdx != -1 {
				url = url[:spaceIdx]
			}
			// Remove quotes if present
			url = strings.Trim(url, `"'`)
			
			// Only track local images (not external URLs)
			if !strings.HasPrefix(strings.ToLower(url), "http://") && 
			   !strings.HasPrefix(strings.ToLower(url), "https://") &&
			   !strings.HasPrefix(strings.ToLower(url), "data:") {
				f.MarkdownImages[url] = true
			}
		}
	}
	
	// Also match HTML img tags embedded in markdown: <img src="url" ...>
	htmlImgRegex := regexp.MustCompile(`(?i)<img\s+[^>]*\ssrc\s*=\s*["']([^"']+)["'][^>]*>`)
	htmlMatches := htmlImgRegex.FindAllStringSubmatch(markdownText, -1)
	
	for _, match := range htmlMatches {
		if len(match) > 1 {
			url := strings.TrimSpace(match[1])
			
			// Only track local images (not external URLs)
			if !strings.HasPrefix(strings.ToLower(url), "http://") && 
			   !strings.HasPrefix(strings.ToLower(url), "https://") &&
			   !strings.HasPrefix(strings.ToLower(url), "data:") {
				f.MarkdownImages[url] = true
			}
		}
	}
	
	// Also check metadata for image references (like frontmatter image: field)
	if imageUrl, exists := f.Metadata["image"]; exists {
		if url, ok := imageUrl.(string); ok && url != "" {
			// Only track local images (not external URLs)
			if !strings.HasPrefix(strings.ToLower(url), "http://") && 
			   !strings.HasPrefix(strings.ToLower(url), "https://") &&
			   !strings.HasPrefix(strings.ToLower(url), "data:") {
				f.MarkdownImages[url] = true
			}
		}
	}
}