package processor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"sniplicity/internal/parser"
)

// getSortKey gets the sort key for a file metadata matching Python's logic exactly
func (p *Processor) getSortKey(metadata map[string]interface{}, sortField string) float64 {
	value, exists := metadata[sortField]
	if !exists {
		return 0.0
	}
	
	// Handle date sorting with comprehensive date format support like Python
	if strings.ToLower(sortField) == "date" || 
	   strings.ToLower(sortField) == "created" || 
	   strings.ToLower(sortField) == "modified" || 
	   strings.ToLower(sortField) == "published" {
		return p.parseDateToTimestamp(fmt.Sprintf("%v", value))
	}
	
	// Handle numeric sorting
	if f, ok := value.(float64); ok {
		return f
	}
	if i, ok := value.(int); ok {
		return float64(i)
	}
	
	// Try to convert string to number
	if str := fmt.Sprintf("%v", value); str != "" {
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			return f
		}
	}
	
	// String sorting (case-insensitive) - convert to hash for numeric comparison
	str := strings.ToLower(fmt.Sprintf("%v", value))
	hash := 0.0
	for _, char := range str {
		hash = hash*31 + float64(char)
	}
	return hash
}

// parseDateToTimestamp parses date string using all Python-supported formats
func (p *Processor) parseDateToTimestamp(dateStr string) float64 {
	// Common date formats - try most specific first (matching Python exactly)
	dateFormats := []string{
		"2006-01-02",           // 2024-09-23
		"2006/01/02",           // 2024/09/23
		"01/02/2006",           // 09/23/2024
		"02/01/2006",           // 23/09/2024
		"Jan 02 2006",          // Sep 23 2024
		"January 02 2006",      // September 23 2024
		"Jan 02, 2006",         // Sep 23, 2024
		"January 02, 2006",     // September 23, 2024
		"02 Jan 2006",          // 23 Sep 2024
		"02 January 2006",      // 23 September 2006
		"2006-01-02 15:04:05",  // 2024-09-23 14:30:00
		"2006-01-02 15:04",     // 2024-09-23 14:30
	}
	
	for _, format := range dateFormats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return float64(t.Unix())
		}
	}
	
	// If no format matches, return epoch (sorts to bottom)
	return 0.0
}

// processIndexTemplate processes template for a single file in the index like Python's process_index_template
func (p *Processor) processIndexTemplate(templateContent []string, fileMetadata map[string]interface{}, snippets map[string][]string, globals map[string]string) string {
	// Work with a fresh copy of the template
	templateLines := make([]string, len(templateContent))
	copy(templateLines, templateContent)
	var processedLines []string
	
	// Process any snippets in the template first (like Python does)
	for _, line := range templateLines {
		directive := parser.ParseLine(line, 0)
		if directive != nil && directive.Type == parser.DirectivePaste {
			if snippetContent, exists := snippets[directive.Name]; exists {
				// Get snippet content and process with file metadata
				snippetText := strings.Join(snippetContent, "\n")
				// Convert metadata to string map for processing
				fileVars := make(map[string]string)
				for k, v := range fileMetadata {
					fileVars[k] = fmt.Sprintf("%v", v)
				}
				processedSnippet := ProcessContentWithDirectives(snippetText, fileVars, globals)
				processedLines = append(processedLines, strings.Split(processedSnippet, "\n")...)
			} else {
				if p.verbose {
					fmt.Printf("Warning: Index template references unknown snippet '%s'\n", directive.Name)
				}
				processedLines = append(processedLines, line)
			}
		} else {
			processedLines = append(processedLines, line)
		}
	}
	
	// Convert back to string and process variables
	templateStr := strings.Join(processedLines, "\n")
	
	// Convert metadata to string map for variable replacement
	fileVars := make(map[string]string)
	for k, v := range fileMetadata {
		fileVars[k] = fmt.Sprintf("%v", v)
	}
	
	// Process all variables and directives
	result := ProcessContentWithDirectives(templateStr, fileVars, globals)
	
	return result
}

// parseFrontmatter is moved here from types package to be accessible
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
		
		// Simple key: value parsing
		if colonIdx := strings.Index(line, ":"); colonIdx > 0 {
			key := strings.TrimSpace(line[:colonIdx])
			value := strings.TrimSpace(line[colonIdx+1:])
			
			// Remove quotes if present
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			   (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
			
			metadata[key] = value
		}
	}
	
	return metadata
}