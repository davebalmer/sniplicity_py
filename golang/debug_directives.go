package main

import (
	"fmt"
	"sniplicity/internal/parser"
	"sniplicity/internal/processor"
)

func main() {
	content := `<!-- set animate -->
<!-- if canonical -->
<link rel="canonical" href="test">
<!-- endif -->
<h1>Hello</h1>`

	// First test what ParseLine returns for each directive
	lines := []string{
		"<!-- set animate -->",
		"<!-- if canonical -->", 
		"<!-- endif -->",
	}
	
	fmt.Println("=== DIRECTIVE PARSING TEST ===")
	for i, line := range lines {
		directive := parser.ParseLine(line, i)
		if directive != nil {
			fmt.Printf("Line: '%s' -> Type: %v, Name: '%s'\n", line, directive.Type, directive.Name)
		} else {
			fmt.Printf("Line: '%s' -> No directive found\n", line)
		}
	}

	localVars := map[string]string{
		"animate": "yes",
	}
	metaVars := map[string]string{
		"canonical": "https://example.com",
	}

	result := processor.ProcessContentWithDirectives(content, localVars, metaVars)
	fmt.Println("\n=== RESULT ===")
	fmt.Print(result)
	fmt.Println("\n=== END ===")
}