package main

import (
	"fmt"
	"sniplicity/internal/parser"
)

func main() {
	testLines := []string{
		"<!-- template test -->",
		"<!-- template blog -->",
		"<!-- end -->",
		"<!-- endif -->",
		"<!-- copy snippet -->",
		"<!-- paste snippet -->",
	}
	
	for i, line := range testLines {
		directive := parser.ParseLine(line, i)
		if directive != nil {
			fmt.Printf("Line %d: '%s' -> Type: %v, Name: '%s'\n", i, line, directive.Type, directive.Name)
		} else {
			fmt.Printf("Line %d: '%s' -> No directive found\n", i, line)
		}
		
		// Test IsBlockEnd
		if parser.IsBlockEnd(line) {
			fmt.Printf("  -> This is a block end marker\n")
		}
	}
}