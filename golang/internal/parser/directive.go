package parser

import (
	"regexp"
	"strings"
)

// DirectiveType represents the type of directive
type DirectiveType int

const (
	DirectiveCopy DirectiveType = iota
	DirectiveCut
	DirectivePaste
	DirectiveSet
	DirectiveGlobal
	DirectiveTemplate
	DirectiveInclude
	DirectiveIndex
	DirectiveIf
	DirectiveEndif
	DirectiveUnknown
)

// Directive represents a sniplicity directive
type Directive struct {
	Type      DirectiveType
	Name      string
	Args      []string
	LineIndex int
	Content   []string
}

var (
	// Regex patterns matching Python version exactly - simple <!-- command --> format
	directiveRegex = regexp.MustCompile(`^\s*\<\!\-\-\s+(.*?)\s+\-\-\>`)
)

// ParseLine parses a line for sniplicity directives matching Python's exact logic
func ParseLine(line string, lineIndex int) *Directive {
	line = strings.TrimSpace(line)
	
	// Match directive pattern
	matches := directiveRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}
	
	content := matches[1]
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return nil
	}
	
	command := parts[0]
	idCommands := map[string]bool{
		"copy": true, "cut": true, "paste": true, 
		"set": true, "global": true, "template": true,
	}
	
	// Handle special end markers
	if command == "end" || command == "endif" {
		if command == "endif" {
			return &Directive{Type: DirectiveEndif, LineIndex: lineIndex}
		}
		return &Directive{Type: DirectiveUnknown, LineIndex: lineIndex} // Special marker for IsBlockEnd
	}
	
	// Handle if command
	if command == "if" {
		if len(parts) < 2 {
			return nil // if requires condition
		}
		condition := strings.Join(parts[1:], " ")
		return &Directive{
			Type:      DirectiveIf,
			Name:      condition,
			LineIndex: lineIndex,
		}
	}
	
	// Handle ID commands (require identifier)
	if idCommands[command] {
		if len(parts) < 2 {
			return nil // Invalid - missing identifier
		}
		
		identifier := parts[1]
		// Validate identifier pattern (alphanumeric, underscore, dash, dot)
		identifierRegex := regexp.MustCompile(`^[-\w.]+$`)
		if !identifierRegex.MatchString(identifier) {
			return nil
		}
		
		switch command {
		case "copy":
			return &Directive{
				Type:      DirectiveCopy,
				Name:      identifier,
				LineIndex: lineIndex,
				Content:   make([]string, 0),
			}
		case "cut":
			return &Directive{
				Type:      DirectiveCut,
				Name:      identifier,
				LineIndex: lineIndex,
				Content:   make([]string, 0),
			}
		case "paste":
			return &Directive{
				Type:      DirectivePaste,
				Name:      identifier,
				LineIndex: lineIndex,
			}
		case "template":
			return &Directive{
				Type:      DirectiveTemplate,
				Name:      identifier,
				LineIndex: lineIndex,
				Content:   make([]string, 0),
			}
		case "set":
			value := ""
			if len(parts) >= 3 {
				value = strings.Join(parts[2:], " ")
			} else {
				value = "true" // Default to true if no value provided
			}
			return &Directive{
				Type:      DirectiveSet,
				Name:      identifier,
				Args:      []string{value},
				LineIndex: lineIndex,
			}
		case "global":
			value := ""
			if len(parts) >= 3 {
				value = strings.Join(parts[2:], " ")
			} else {
				value = "true" // Default to true if no value provided
			}
			return &Directive{
				Type:      DirectiveGlobal,
				Name:      identifier,
				Args:      []string{value},
				LineIndex: lineIndex,
			}
		}
	}
	
	// Handle other commands
	switch command {
	case "if":
		if len(parts) < 2 {
			return nil // if requires a condition
		}
		condition := strings.Join(parts[1:], " ")
		return &Directive{
			Type:      DirectiveIf,
			Name:      condition, // Store condition in Name field
			LineIndex: lineIndex,
		}
	case "endif":
		return &Directive{
			Type:      DirectiveEndif,
			LineIndex: lineIndex,
		}
	case "include":
		if len(parts) < 2 {
			return nil
		}
		filename := strings.Join(parts[1:], " ")
		return &Directive{
			Type:      DirectiveInclude,
			Args:      []string{filename},
			LineIndex: lineIndex,
		}
	case "index":
		if len(parts) < 2 {
			return nil
		}
		// Keep arguments as separate elements for index commands
		return &Directive{
			Type:      DirectiveIndex,
			Args:      parts[1:], // Keep all arguments separate
			LineIndex: lineIndex,
		}
	}
	
	return nil
}

// IsBlockEnd checks if a line ends a copy/cut/template block
func IsBlockEnd(line string) bool {
	directive := ParseLine(line, 0)
	return directive != nil && directive.Type == DirectiveUnknown && (strings.Contains(line, "end") || strings.Contains(line, "endif"))
}

// ParseDirectives parses all directives from file content
func ParseDirectives(content []string) []*Directive {
	var directives []*Directive
	var currentBlock *Directive

	for i, line := range content {
		// Check for end of current block
		if currentBlock != nil && IsBlockEnd(line) {
			directives = append(directives, currentBlock)
			currentBlock = nil
			continue
		}

		// Check for new directive
		directive := ParseLine(line, i)
		if directive != nil {
			// If it's a block directive (copy, cut, template), start collecting content
			if directive.Type == DirectiveCopy || directive.Type == DirectiveCut || directive.Type == DirectiveTemplate {
				currentBlock = directive
			} else {
				// Single-line directive
				directives = append(directives, directive)
			}
			continue
		}

		// If we're in a block, collect content
		if currentBlock != nil {
			currentBlock.Content = append(currentBlock.Content, line)
		}
	}

	// If we have an unclosed block, still add it
	if currentBlock != nil {
		directives = append(directives, currentBlock)
	}

	return directives
}

// ExpandVariables replaces variables in a string using the provided variable map
func ExpandVariables(text string, variables map[string]string) string {
	result := text
	
	// Replace variables in the format {{variable_name}} (allows letters, numbers, hyphens, underscores, and dots)
	varRegex := regexp.MustCompile(`\{\{([-\w.]+)\}\}`)
	result = varRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract variable name (remove {{ and }})
		varName := match[2 : len(match)-2]
		if value, exists := variables[varName]; exists {
			return value
		}
		return match // Return original if variable not found
	})
	
	return result
}