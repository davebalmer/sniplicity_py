package types

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// preprocessMarkdownInHTML processes markdown content inside HTML tags with markdown attribute
func preprocessMarkdownInHTML(content string) string {
	// Regular expression to match HTML tags with markdown attribute
	// This matches: <tag ... markdown ...>content</tag>
	// Note: Go doesn't support backreferences in regexp, so we'll use a simpler approach
	re := regexp.MustCompile(`(?s)<([a-zA-Z][a-zA-Z0-9]*)\s+([^>]*\bmarkdown\b[^>]*?)>(.*?)</([a-zA-Z][a-zA-Z0-9]*)>`)
	
	// Create a basic markdown processor for the nested content
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.TaskList,
			extension.Strikethrough,
			extension.Linkify,
			extension.Typographer,
			extension.DefinitionList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	
	// Process all matches
	result := re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the parts
		submatch := re.FindStringSubmatch(match)
		if len(submatch) != 5 {
			return match // Return original if parsing fails
		}
		
		openTagName := submatch[1]
		attributes := submatch[2]
		innerContent := submatch[3]
		closeTagName := submatch[4]
		
		// Verify that opening and closing tags match
		if openTagName != closeTagName {
			return match // Return original if tags don't match
		}
		
		// Remove the markdown attribute from the attributes
		// Keep other attributes but remove markdown
		attrRe := regexp.MustCompile(`\s*\bmarkdown\b\s*`)
		cleanedAttributes := attrRe.ReplaceAllString(attributes, " ")
		cleanedAttributes = strings.TrimSpace(cleanedAttributes)
		
		// Process the inner content as markdown
		var buf bytes.Buffer
		if err := md.Convert([]byte(strings.TrimSpace(innerContent)), &buf); err != nil {
			// If markdown processing fails, return the content as-is
			if cleanedAttributes != "" {
				return "<" + openTagName + " " + cleanedAttributes + ">" + innerContent + "</" + openTagName + ">"
			} else {
				return "<" + openTagName + ">" + innerContent + "</" + openTagName + ">"
			}
		}
		
		// Get the processed HTML (remove any surrounding <p> tags if they exist)
		processedHTML := strings.TrimSpace(buf.String())
		
		// Build the final HTML tag
		if cleanedAttributes != "" {
			return "<" + openTagName + " " + cleanedAttributes + ">" + processedHTML + "</" + openTagName + ">"
		} else {
			return "<" + openTagName + ">" + processedHTML + "</" + openTagName + ">"
		}
	})
	
	return result
}