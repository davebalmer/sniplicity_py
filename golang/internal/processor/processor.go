package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sniplicity/internal/imgprocess"
	"sniplicity/internal/parser"
	"sniplicity/internal/types"

	"github.com/fatih/color"
)

// Processor handles file processing operations
type Processor struct {
	verbose bool
}

// New creates a new Processor instance
func New(verbose bool) *Processor {
	return &Processor{verbose: verbose}
}

// CollectSnippetsFromFile extracts snippets and templates from a file using stack-based processing like Python
func (p *Processor) CollectSnippetsFromFile(fileInfo *types.FileInfo, snippets, templates map[string][]string, verbose bool) error {
	// Stack to handle nested snippets/templates: (name, block, type, nesting_level, start_line)
	type stackItem struct {
		name         string
		block        []string  
		itemType     string    // "copy", "cut", or "template"
		nestingLevel int
		startLine    int
	}
	
	var contentStack []stackItem
	
	for i, line := range fileInfo.Content {
		directive := parser.ParseLine(line, i)
		
		if directive != nil {
			switch directive.Type {
			case parser.DirectiveCopy, parser.DirectiveCut, parser.DirectiveTemplate:
				// Add to stack with current nesting level (= current stack depth)
				nestingLevel := len(contentStack)
				itemType := "copy"
				if directive.Type == parser.DirectiveCut {
					itemType = "cut"
				} else if directive.Type == parser.DirectiveTemplate {
					itemType = "template"
				}
				
				contentStack = append(contentStack, stackItem{
					name:         directive.Name,
					block:        make([]string, 0),
					itemType:     itemType,
					nestingLevel: nestingLevel,
					startLine:    i,
				})
				
				if verbose {
					fmt.Printf("  Start %s '%s' at level %d in %s\n", itemType, directive.Name, nestingLevel, fileInfo.Filename)
				}
				
			default:
				// Check for end directive (Python uses "end" but our parser uses block end detection)
				if parser.IsBlockEnd(line) && len(contentStack) > 0 {
					// Pop the last started item
					item := contentStack[len(contentStack)-1]
					contentStack = contentStack[:len(contentStack)-1]
					
					if verbose {
						fmt.Printf("  End %s '%s' from level %d in %s\n", item.itemType, item.name, item.nestingLevel, fileInfo.Filename)
					}
					
					// Store the item based on type
					if item.itemType == "template" {
						templates[item.name] = make([]string, len(item.block))
						copy(templates[item.name], item.block)
						if verbose {
							fmt.Printf("  Stored template '%s' with %d lines\n", item.name, len(item.block))
						}
					} else {
						snippets[item.name] = make([]string, len(item.block))
						copy(snippets[item.name], item.block)
						if verbose {
							green := color.New(color.FgGreen)
							fmt.Printf("  Found %s: %s\n", green.Sprint("snippet"), item.name)
						}
					}
				} else {
					// Add the line to all active blocks
					for j := range contentStack {
						contentStack[j].block = append(contentStack[j].block, line)
					}
				}
			}
		} else {
			// Add non-directive line to all active blocks
			for j := range contentStack {
				contentStack[j].block = append(contentStack[j].block, line)
			}
		}
	}
	
	return nil
}

// CollectGlobalsFromFile extracts global variables from a file
func (p *Processor) CollectGlobalsFromFile(fileInfo *types.FileInfo, globals map[string]string, verbose bool) error {
	directives := parser.ParseDirectives(fileInfo.Content)
	
	for _, directive := range directives {
		if directive.Type == parser.DirectiveGlobal {
			globals[directive.Name] = directive.Args[0]
			if verbose {
				fmt.Printf("  Found global: %s = %s\n", directive.Name, directive.Args[0])
			}
		}
	}
	
	return nil
}

// ProcessIncludes processes include directives in a file
func (p *Processor) ProcessIncludes(fileInfo *types.FileInfo, inputDir string) error {
	var newContent []string
	directives := parser.ParseDirectives(fileInfo.Content)
	
	// Process content line by line
	for i, line := range fileInfo.Content {
		// Check if this line has an include directive
		hasInclude := false
		for _, directive := range directives {
			if directive.Type == parser.DirectiveInclude && directive.LineIndex == i {
				// Process include
				includePath := directive.Args[0]
				fullPath := filepath.Join(inputDir, includePath)
				
				// Read included file
				includeContent, err := os.ReadFile(fullPath)
				if err != nil {
					if p.verbose {
						fmt.Printf("Warning: Cannot read include file %s\n", fullPath)
					}
					newContent = append(newContent, line) // Keep original line
				} else {
					// Add included content
					includeLines := strings.Split(strings.TrimRight(string(includeContent), "\n"), "\n")
					newContent = append(newContent, includeLines...)
				}
				hasInclude = true
				break
			}
		}
		
		if !hasInclude {
			newContent = append(newContent, line)
		}
	}
	
	fileInfo.Content = newContent
	return nil
}

// ProcessIndexCommands processes index directives in a file exactly like Python
func (p *Processor) ProcessIndexCommands(fileInfo *types.FileInfo, inputDir string, templates map[string][]string, snippets map[string][]string, globals map[string]string) error {
	var newContent []string
	
	for i, line := range fileInfo.Content {
		directive := parser.ParseLine(line, i)
		
		if directive != nil && directive.Type == parser.DirectiveIndex {
			// Parse index directive: <!-- index pattern template [sort_field] -->
			if len(directive.Args) < 2 {
				if p.verbose {
					fmt.Printf("Warning: Index command requires at least pattern and template: %s in %s:%d\n", line, fileInfo.Filename, i+1)
				}
				newContent = append(newContent, line)
				continue
			}
			
			pattern := directive.Args[0]    // e.g., "blog/*.md"
			templateName := directive.Args[1] // e.g., "blog-item"
			var sortField string
			if len(directive.Args) > 2 {
				sortField = directive.Args[2] // e.g., "date"
			}
			
			if p.verbose {
				fmt.Printf("  Processing index: pattern='%s' template='%s' sort='%s'\n", pattern, templateName, sortField)
			}
			
			// Check if template exists
			if _, exists := templates[templateName]; !exists {
				if p.verbose {
					fmt.Printf("Warning: Index template '%s' not found in %s:%d\n", templateName, fileInfo.Filename, i+1)
				}
				newContent = append(newContent, line)
				continue
			}
			
			// Find matching files using glob pattern
			matchingFiles, err := p.findMatchingFiles(pattern, inputDir)
			if err != nil {
				if p.verbose {
					fmt.Printf("Warning: Error finding files for pattern '%s': %v\n", pattern, err)
				}
				newContent = append(newContent, line)
				continue
			}
			
			if p.verbose {
				fmt.Printf("  Found %d matching files\n", len(matchingFiles))
			}
			
			// Load metadata from matching files
			var fileData []map[string]interface{}
			for _, filePath := range matchingFiles {
				metadata, err := p.loadFileMetadata(filePath, inputDir)
				if err != nil {
					if p.verbose {
						fmt.Printf("Warning: Cannot load metadata from %s: %v\n", filePath, err)
					}
					continue
				}
				if metadata != nil {
					fileData = append(fileData, metadata)
				}
			}
			
			// Sort files if sort field is specified
			if sortField != "" && len(fileData) > 0 {
				fileData = p.sortFileData(fileData, sortField)
			}
			
			// Generate HTML for each file using the template
			for _, fileMeta := range fileData {
				indexHTML := p.processIndexTemplate(templates[templateName], fileMeta, snippets, globals)
				newContent = append(newContent, strings.Split(indexHTML, "\n")...)
			}
		} else {
			newContent = append(newContent, line)
		}
	}
	
	fileInfo.Content = newContent
	return nil
}

// generateIndex creates an index of files in a directory
func (p *Processor) generateIndex(dirPath string, templates map[string][]string, snippets map[string][]string, globals map[string]string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	
	var files []string
	var dirs []string
	
	for _, entry := range entries {
		name := entry.Name()
		
		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		
		if entry.IsDir() {
			dirs = append(dirs, name)
		} else {
			// Only include certain file types
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".md" || ext == ".html" || ext == ".htm" || ext == ".txt" {
				files = append(files, name)
			}
		}
	}
	
	// Sort both lists
	sort.Strings(files)
	sort.Strings(dirs)
	
	var indexLines []string
	
	// Add directories first
	for _, dir := range dirs {
		indexLines = append(indexLines, fmt.Sprintf("* [%s/](%s/)", dir, dir))
	}
	
	// Add files
	for _, file := range files {
		displayName := file
		// Remove .md extension for display
		if strings.HasSuffix(displayName, ".md") {
			displayName = strings.TrimSuffix(displayName, ".md")
		}
		
		// Create link (convert .md to .html for links)
		linkName := file
		if strings.HasSuffix(linkName, ".md") {
			linkName = strings.TrimSuffix(linkName, ".md") + ".html"
		}
		
		indexLines = append(indexLines, fmt.Sprintf("* [%s](%s)", displayName, linkName))
	}
	
	return indexLines, nil
}

// findMatchingFiles finds files matching the glob pattern like Python's find_matching_files
func (p *Processor) findMatchingFiles(pattern, sourceDir string) ([]string, error) {
	fullPattern := filepath.Join(sourceDir, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, err
	}
	
	// Filter to only include supported file types
	supportedExtensions := []string{".md", ".mdown", ".markdown", ".html", ".htm", ".txt"}
	var filteredMatches []string
	
	for _, match := range matches {
		ext := strings.ToLower(filepath.Ext(match))
		for _, supportedExt := range supportedExtensions {
			if ext == supportedExt {
				filteredMatches = append(filteredMatches, match)
				break
			}
		}
	}
	
	return filteredMatches, nil
}

// loadFileMetadata loads metadata from a file (frontmatter + computed fields) like Python's load_file_metadata
func (p *Processor) loadFileMetadata(filePath, sourceDir string) (map[string]interface{}, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	lines := strings.Split(string(content), "\n")
	
	// Parse frontmatter for metadata
	_, metadata := parseFrontmatter(lines)
	
	// Add computed fields like Python does
	relPath, err := filepath.Rel(sourceDir, filePath)
	if err != nil {
		relPath = filePath
	}
	
	// Convert to output path (change .md to .html)
	outputPath := relPath
	if strings.HasSuffix(strings.ToLower(outputPath), ".md") || 
	   strings.HasSuffix(strings.ToLower(outputPath), ".mdown") || 
	   strings.HasSuffix(strings.ToLower(outputPath), ".markdown") {
		ext := filepath.Ext(outputPath)
		outputPath = outputPath[:len(outputPath)-len(ext)] + ".html"
	}
	
	metadata["filepath"] = outputPath
	metadata["filename"] = filepath.Base(filePath)
	
	// Add title if not present
	if _, exists := metadata["title"]; !exists {
		// Try to extract title from first heading or use filename
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# ") {
				metadata["title"] = strings.TrimSpace(trimmed[2:])
				break
			}
		}
		if _, exists := metadata["title"]; !exists {
			// Use filename without extension as fallback
			name := filepath.Base(filePath)
			ext := filepath.Ext(name)
			if ext != "" {
				name = name[:len(name)-len(ext)]
			}
			metadata["title"] = name
		}
	}
	
	return metadata, nil
}

// ProcessSnippets processes snippet directives using iterative processing like Python
func (p *Processor) ProcessSnippets(fileInfo *types.FileInfo, snippets map[string][]string) error {
	// First find any local snippets in this file and track cut regions
	localSnippets := make(map[string][]string)
	type StackItem struct {
		Name         string
		Block        []string
		IsCut        bool
		NestingLevel int
	}
	var snippetStack []StackItem
	var cutRanges [][2]int // Store start and end lines of cut regions
	nestingLevel := 0
	
	// First pass to find cut regions and local snippets
	for i, line := range fileInfo.Content {
		directive := parser.ParseLine(line, i)
		
		if directive != nil {
			switch directive.Type {
			case parser.DirectiveCut:
				nestingLevel++
				snippetStack = append(snippetStack, StackItem{
					Name:         directive.Name,
					Block:        make([]string, 0),
					IsCut:        true,
					NestingLevel: nestingLevel,
				})
			case parser.DirectiveCopy:
				nestingLevel++
				snippetStack = append(snippetStack, StackItem{
					Name:         directive.Name,
					Block:        make([]string, 0),
					IsCut:        false,
					NestingLevel: nestingLevel,
				})
			default:
				if parser.IsBlockEnd(line) {
					// Find the most recent snippet at this nesting level
					for len(snippetStack) > 0 && snippetStack[len(snippetStack)-1].NestingLevel > nestingLevel {
						nestingLevel--
					}
					
					if len(snippetStack) > 0 {
						stackItem := snippetStack[len(snippetStack)-1]
						snippetStack = snippetStack[:len(snippetStack)-1]
						localSnippets[stackItem.Name] = append([]string{}, stackItem.Block...)
						
						if stackItem.IsCut {
							// For cut snippets, find the start line
							cutStart := -1
							for j := 0; j < i; j++ {
								cutDirective := parser.ParseLine(fileInfo.Content[j], j)
								if cutDirective != nil && cutDirective.Type == parser.DirectiveCut && cutDirective.Name == stackItem.Name {
									cutStart = j
									break
								}
							}
							if cutStart >= 0 {
								cutRanges = append(cutRanges, [2]int{cutStart, i})
							}
						}
						
						// If this snippet is nested, add it to the parent's block
						if len(snippetStack) > 0 {
							snippetStack[len(snippetStack)-1].Block = append(snippetStack[len(snippetStack)-1].Block, line)
						}
					}
				} else {
					// Add line to all active snippet blocks
					for i := range snippetStack {
						snippetStack[i].Block = append(snippetStack[i].Block, line)
					}
				}
			}
		} else {
			// Add line to all active snippet blocks
			for i := range snippetStack {
				snippetStack[i].Block = append(snippetStack[i].Block, line)
			}
		}
	}
	
	// Now process the file, using local snippets where available and skipping cut regions
	var currentData []string
	for i, line := range fileInfo.Content {
		// Check if this line is in a cut region
		inCutRegion := false
		for _, cutRange := range cutRanges {
			if i >= cutRange[0] && i <= cutRange[1] {
				inCutRegion = true
				break
			}
		}
		if !inCutRegion {
			currentData = append(currentData, line)
		}
	}
	
	// Keep processing until no more paste directives are found
	maxIterations := 10 // Prevent infinite loops
	iteration := 0
	
	for iteration < maxIterations {
		var newFile []string
		foundPaste := false
		
		for _, line := range currentData {
			directive := parser.ParseLine(line, 0) // Line index doesn't matter here
			
			if directive != nil && directive.Type == parser.DirectivePaste {
				foundPaste = true
				// First try local snippets, then fall back to global
				if snippetContent, exists := localSnippets[directive.Name]; exists {
					newFile = append(newFile, snippetContent...)
					fileInfo.UsedSnippets[directive.Name] = true
				} else if snippetContent, exists := snippets[directive.Name]; exists {
					newFile = append(newFile, snippetContent...)
					fileInfo.UsedSnippets[directive.Name] = true
				} else {
					if p.verbose {
						fmt.Printf("Warning: Unable to insert %s because snippet doesn't exist in %s\n", directive.Name, fileInfo.Filename)
					}
					// Don't add the paste directive to output - remove it even if snippet doesn't exist
				}
			} else if directive != nil && (directive.Type == parser.DirectiveCopy || directive.Type == parser.DirectiveCut || directive.Type == parser.DirectiveTemplate || parser.IsBlockEnd(line)) {
				// Remove directive markers from output - they should not appear in final content
			} else {
				newFile = append(newFile, line)
			}
		}
		
		currentData = newFile
		iteration++
		
		// If no paste directives were found, we're done
		if !foundPaste {
			break
		}
	}
	
	if iteration >= maxIterations && p.verbose {
		fmt.Printf("Warning: Maximum snippet processing iterations reached in %s\n", fileInfo.Filename)
	}
	
	fileInfo.Content = currentData
	return nil
}

// ProcessVariables processes variable substitution and writes the file
func (p *Processor) ProcessVariables(fileInfo *types.FileInfo, outputDir string, templates map[string][]string, snippets map[string][]string, globals map[string]string, imgSize bool, verbose bool) error {
	// Collect local variables from set directives
	localVars := make(map[string]string)
	directives := parser.ParseDirectives(fileInfo.Content)
	
	for _, directive := range directives {
		if directive.Type == parser.DirectiveSet {
			localVars[directive.Name] = directive.Args[0]
		}
	}
	
	// Merge globals and locals (locals override globals)
	allVars := make(map[string]string)
	for k, v := range globals {
		allVars[k] = v
	}
	for k, v := range localVars {
		allVars[k] = v
	}
	
	// Add metadata variables
	for k, v := range fileInfo.Metadata {
		if str, ok := v.(string); ok {
			allVars[k] = str
		}
	}
	
	// Remove directive lines and expand variables
	var finalContent []string
	for i, line := range fileInfo.Content {
		// Check if this line is a directive that should be removed
		isDirective := false
		for _, directive := range directives {
			if directive.LineIndex == i {
				// Remove set, global, and single-line directives
				if directive.Type == parser.DirectiveSet || 
				   directive.Type == parser.DirectiveGlobal ||
				   directive.Type == parser.DirectivePaste ||
				   directive.Type == parser.DirectiveInclude ||
				   directive.Type == parser.DirectiveIndex {
					isDirective = true
					break
				}
			}
		}
		
		if !isDirective {
			// Expand variables in the line
			expandedLine := parser.ExpandVariables(line, allVars)
			finalContent = append(finalContent, expandedLine)
		}
	}
	
	// Apply template if specified - check both local variables and metadata like Python
	var templateName string
	if localTemplate, exists := localVars["template"]; exists {
		templateName = localTemplate
	} else if metaTemplate, exists := allVars["template"]; exists {
		templateName = metaTemplate
	}
	
	if templateName != "" {
		if templateContent, templateExists := templates[templateName]; templateExists {
			if verbose {
				fmt.Printf("  Using template '%s' for %s\n", templateName, fileInfo.Filename)
			}
			
			// Get the template content and process snippets in it (like Python)
			var processedTemplate []string
			
			// Process snippets (paste commands) in the template like Python
			for _, line := range templateContent {
				directive := parser.ParseLine(line, 0)
				if directive != nil && directive.Type == parser.DirectivePaste {
					if snippetContent, exists := snippets[directive.Name]; exists {
						// Process the snippet content with directives
						snippetText := strings.Join(snippetContent, "\n")
						processedSnippet := ProcessContentWithDirectives(snippetText, localVars, allVars)
						processedTemplate = append(processedTemplate, strings.Split(processedSnippet, "\n")...)
					} else {
						if verbose {
							fmt.Printf("Warning: Template references unknown snippet '%s'\n", directive.Name)
						}
						processedTemplate = append(processedTemplate, line)
					}
				} else {
					processedTemplate = append(processedTemplate, line)
				}
			}
			
			// Convert template to string
			templateContentStr := strings.Join(processedTemplate, "\n")
			
			// Replace {{content}} in template with the file content (processed)
			fileContentStr := strings.Join(finalContent, "\n")
			processedFileContent := ProcessContentWithDirectives(fileContentStr, localVars, allVars)
			templateWithContent := strings.ReplaceAll(templateContentStr, "{{content}}", processedFileContent)
			
			// Process conditionals and variables in the complete template
			finalTemplateContent := ProcessContentWithDirectives(templateWithContent, localVars, allVars)
			finalContent = strings.Split(finalTemplateContent, "\n")
		} else if verbose {
			fmt.Printf("Warning: Template '%s' not found for file %s\n", templateName, fileInfo.Filename)
		}
	} else {
		if verbose {
			fmt.Printf("  Processing file without template: %s\n", fileInfo.Filename)
		}
		// Process all directives and variables in content without template
		contentText := strings.Join(finalContent, "\n")
		processedContent := ProcessContentWithDirectives(contentText, localVars, allVars)
		finalContent = strings.Split(processedContent, "\n")
	}	// Write output file
	outputPath := fileInfo.GetOutputPath(outputDir)
	
	// Create output directory if needed
	outputDirPath := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDirPath, 0755); err != nil {
		return fmt.Errorf("cannot create output directory %s: %w", outputDirPath, err)
	}
	
	// Write file
	finalContentStr := strings.Join(finalContent, "\n")
	
	// Process images if enabled and this file has markdown images to process
	if imgSize && len(fileInfo.MarkdownImages) > 0 && (strings.HasSuffix(strings.ToLower(outputPath), ".html") || strings.HasSuffix(strings.ToLower(outputPath), ".htm")) {
		if verbose {
			fmt.Printf("  Processing markdown images for %s\n", outputPath)
		}
		// Get the directory of the HTML file for resolving relative image paths
		htmlDir := filepath.Dir(outputPath)
		// Process only images that came from markdown
		processedContent, err := imgprocess.ProcessHTMLForMarkdownImages(finalContentStr, outputDir, htmlDir, fileInfo.MarkdownImages, verbose)
		if err != nil {
			if verbose {
				fmt.Printf("  Warning: Image processing failed for %s: %v\n", outputPath, err)
			}
			// Continue with unprocessed content if image processing fails
		} else {
			finalContentStr = processedContent
		}
	}
	
	if err := os.WriteFile(outputPath, []byte(finalContentStr), 0644); err != nil {
		return fmt.Errorf("cannot write file %s: %w", outputPath, err)
	}
	
	if verbose {
		fmt.Printf("  Wrote %s\n", outputPath)
	}
	
	return nil
}
// sortFileData sorts file data by the specified field like Python's sort_file_data
func (p *Processor) sortFileData(fileData []map[string]interface{}, sortField string) []map[string]interface{} {
// Sort by date in descending order (most recent first), others ascending
reverse := strings.ToLower(sortField) == "date" || 
   strings.ToLower(sortField) == "created" || 
   strings.ToLower(sortField) == "modified" || 
   strings.ToLower(sortField) == "published"

sort.Slice(fileData, func(i, j int) bool {
valueI := p.getSortKey(fileData[i], sortField)
valueJ := p.getSortKey(fileData[j], sortField)

if reverse {
return valueI > valueJ
}
return valueI < valueJ
})

return fileData
}
