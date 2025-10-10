package processor

import (
	"regexp"
	"strings"

	"sniplicity/internal/parser"
)

// ProcessContentWithDirectives handles conditionals, directive removal, and variable replacement like Python
func ProcessContentWithDirectives(content string, localVars, metaVars map[string]string) string {
	// First process inline conditionals (for mixed content lines)
	content = processInlineConditionals(content, localVars, metaVars)
	
	// Then process block-level conditionals and other directives
	lines := strings.Split(content, "\n")
	var processedLines []string
	write := true
	cutting := false
	
	for _, line := range lines {
		directive := parser.ParseLine(line, 0)
		
		if directive != nil {
			switch directive.Type {
			case parser.DirectiveIf:
				condition := directive.Name
				if strings.HasPrefix(condition, "!") {
					varName := strings.TrimSpace(condition[1:])
					write = isFalse(localVars, metaVars, varName)
				} else {
					write = isTrue(localVars, metaVars, condition)
				}
				continue // Skip adding the if directive to output
			case parser.DirectiveEndif:
				write = true
				continue // Skip adding the endif directive to output
			case parser.DirectiveCut:
				write = false
				cutting = true
				continue // Skip adding cut directive to output
			case parser.DirectiveUnknown:
				// This handles "end" directives
				if cutting && strings.Contains(strings.ToLower(line), "end") {
					write = true
					cutting = false
				}
				continue // Skip adding end directive to output
			case parser.DirectiveSet, parser.DirectiveCopy, parser.DirectivePaste, 
				 parser.DirectiveGlobal, parser.DirectiveTemplate, parser.DirectiveInclude, parser.DirectiveIndex:
				continue // Skip other directive commands that shouldn't appear in output
			}
		}
		
		if write {
			processedLines = append(processedLines, line)
		}
	}
	
	// Finally do variable replacements on the processed text
	processedText := strings.Join(processedLines, "\n")
	return doReplacements(processedText, localVars, metaVars)
}

// processInlineConditionals processes inline conditional directives within a line of text
func processInlineConditionals(text string, localVars, metaVars map[string]string) string {
	// Pattern to match inline conditionals like <!-- if var --> content <!-- endif -->
	inlineIfRegex := regexp.MustCompile(`<!--\s*if\s+([^>]+)\s*-->(.*?)<!--\s*endif\s*-->`)
	
	for {
		matches := inlineIfRegex.FindStringSubmatch(text)
		if matches == nil {
			break
		}
		
		condition := strings.TrimSpace(matches[1])
		content := matches[2]
		
		// Handle negation
		var showContent bool
		if strings.HasPrefix(condition, "!") {
			varName := strings.TrimSpace(condition[1:])
			showContent = isFalse(localVars, metaVars, varName)
		} else {
			showContent = isTrue(localVars, metaVars, condition)
		}
		
		replacement := ""
		if showContent {
			replacement = content
		}
		
		// Replace the first match
		text = strings.Replace(text, matches[0], replacement, 1)
	}
	
	return text
}

// isTrue checks if a variable is true (exists and not empty/false)
func isTrue(localVars, metaVars map[string]string, varName string) bool {
	// Check local vars first
	if val, exists := localVars[varName]; exists {
		return val != "" && val != "false" && val != "0"
	}
	// Check meta vars
	if val, exists := metaVars[varName]; exists {
		return val != "" && val != "false" && val != "0"
	}
	return false
}

// isFalse checks if a variable is false (doesn't exist or is empty/false)
func isFalse(localVars, metaVars map[string]string, varName string) bool {
	return !isTrue(localVars, metaVars, varName)
}

// doReplacements replaces all variables in text
func doReplacements(text string, localVars, metaVars map[string]string) string {
	// Create a dictionary with all variables (locals override meta)
	allVars := make(map[string]string)
	for k, v := range metaVars {
		allVars[k] = v
	}
	for k, v := range localVars {
		allVars[k] = v
	}
	
	// Replace all variables using the same pattern as Python: {{variable}} where variable can contain letters, numbers, hyphens, underscores, and dots
	result := text
	varRegex := regexp.MustCompile(`\{\{([-\w.]+)\}\}`)
	result = varRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract variable name (remove {{ and }})
		varName := match[2 : len(match)-2]
		if value, exists := allVars[varName]; exists {
			return value
		}
		return "" // Remove undefined variables (like Python)
	})
	
	return result
}