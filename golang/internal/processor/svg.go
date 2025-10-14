package processor

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ProcessSVGFilters processes CSS filters in SVG content and bakes them into the SVG colors
func ProcessSVGFilters(content string) (string, error) {
	fmt.Printf("DEBUG: ProcessSVGFilters called with content length: %d\n", len(content))
	modifiedContent := content
	
	// Process inline filter attributes
	inlineProcessed, err := processInlineFilters(modifiedContent)
	if err != nil {
		fmt.Printf("DEBUG: Error in processInlineFilters: %v\n", err)
		return "", fmt.Errorf("processing inline filters: %w", err)
	}
	fmt.Printf("DEBUG: After inline processing, content length: %d\n", len(inlineProcessed))
	modifiedContent = inlineProcessed
	
	// Process CSS filters in style blocks
	fmt.Printf("DEBUG: About to call processStyleBlockFilters with content length: %d\n", len(modifiedContent))
	styleProcessed, err := processStyleBlockFilters(modifiedContent)
	if err != nil {
		return "", fmt.Errorf("processing style block filters: %w", err)
	}
	fmt.Printf("DEBUG: processStyleBlockFilters returned content length: %d\n", len(styleProcessed))
	fmt.Printf("DEBUG: After style block processing, content length: %d\n", len(styleProcessed))
	modifiedContent = styleProcessed
	
	fmt.Printf("DEBUG: Final ProcessSVGFilters result, content length: %d\n", len(modifiedContent))
	return modifiedContent, nil
}

// processInlineFilters processes filter attributes on SVG elements and bakes colors
func processInlineFilters(content string) (string, error) {
	fmt.Printf("DEBUG: processInlineFilters called with content: %.100s...\n", content)
	// Regex to find filter attributes on any element
	filterAttrRegex := regexp.MustCompile(`(<[^>]+?\s)filter="([^"]+)"([^>]*>)`)
	matches := filterAttrRegex.FindAllStringSubmatch(content, -1)
	
	fmt.Printf("DEBUG: Found %d filter matches\n", len(matches))
	if len(matches) == 0 {
		return content, nil
	}
	
	modifiedContent := content
	
	for i, match := range matches {
		fmt.Printf("DEBUG: Processing match %d: %s\n", i, match[0])
		filterValue := match[2]
		
		fmt.Printf("DEBUG: Filter value: %s\n", filterValue)
		
		// Parse the filter functions
		functions := parseFilterFunctions(filterValue)
		fmt.Printf("DEBUG: Parsed %d filter functions\n", len(functions))
		
		// Apply filters to all colors in the entire SVG
		var err error
		modifiedContent, err = applyFiltersToColors(modifiedContent, functions)
		if err != nil {
			return "", fmt.Errorf("applying filters to colors: %w", err)
		}
		fmt.Printf("DEBUG: After applying filters, content length: %d\n", len(modifiedContent))
		
		// Remove the filter attribute from the element
		// We need to find the updated element in the modified content and remove the filter attribute
		filterAttrPattern := regexp.MustCompile(`(\s)filter="[^"]*"`)
		modifiedContent = filterAttrPattern.ReplaceAllString(modifiedContent, "")
		fmt.Printf("DEBUG: After removing filter attribute, content length: %d\n", len(modifiedContent))
	}
	
	return modifiedContent, nil
}

// processStyleBlockFilters processes CSS filters in style blocks and bakes colors
func processStyleBlockFilters(content string) (string, error) {
	fmt.Printf("DEBUG: processStyleBlockFilters called\n")
	// Check if the SVG has any CSS filters
	styleRegex := regexp.MustCompile(`<style[^>]*>(.*?)</style>`)
	styleMatches := styleRegex.FindAllStringSubmatch(content, -1)
	
	fmt.Printf("DEBUG: Found %d style blocks\n", len(styleMatches))
	if len(styleMatches) == 0 {
		fmt.Printf("DEBUG: No style blocks found, returning unchanged\n")
		return content, nil // No style blocks found
	}
	
	modifiedContent := content
	
	for _, match := range styleMatches {
		originalStyle := match[0]
		styleContent := match[1]
		
		// Check for filter properties
		filterRegex := regexp.MustCompile(`filter:\s*([^;]+);`)
		filterMatches := filterRegex.FindAllStringSubmatch(styleContent, -1)
		
		if len(filterMatches) == 0 {
			continue // No filters in this style block
		}
		
		// Process all filter functions from all matches
		var allFunctions []filterFunction
		for _, filterMatch := range filterMatches {
			filterValue := strings.TrimSpace(filterMatch[1])
			functions := parseFilterFunctions(filterValue)
			allFunctions = append(allFunctions, functions...)
		}
		
		// Apply filters to colors in elements that use these CSS classes
		modifiedContent, err := applyFiltersToClassElements(modifiedContent, allFunctions, styleContent)
		if err != nil {
			return "", fmt.Errorf("applying filters to colors: %w", err)
		}
		
		// Remove CSS filters from style
		modifiedStyle := removeCSSFilters(styleContent, filterMatches)
		
		// Replace the original style block
		newStyle := fmt.Sprintf("<style>%s</style>", modifiedStyle)
		modifiedContent = strings.Replace(modifiedContent, originalStyle, newStyle, 1)
	}
	
	return modifiedContent, nil
}

// applyFiltersToColors applies filter functions to all color values in the SVG
func applyFiltersToColors(content string, functions []filterFunction) (string, error) {
	if len(functions) == 0 {
		return content, nil
	}
	
	// Find all color values in the SVG (fill, stroke, stop-color, etc.)
	colorRegex := regexp.MustCompile(`(fill|stroke|stop-color|color)="([^"]+)"`)
	
	return colorRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := colorRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		
		attribute := parts[1]
		colorValue := parts[2]
		
		// Skip non-color values
		if colorValue == "none" || colorValue == "transparent" || strings.HasPrefix(colorValue, "url(") {
			return match
		}
		
		// Parse the color
		r, g, b, err := parseColor(colorValue)
		if err != nil {
			return match // Return unchanged if we can't parse the color
		}
		
		// Apply each filter function in sequence
		for _, function := range functions {
			r, g, b = applyFilterFunction(r, g, b, function)
		}
		
		// Convert back to hex color
		newColor := fmt.Sprintf("#%02x%02x%02x", clamp(r), clamp(g), clamp(b))
		return fmt.Sprintf(`%s="%s"`, attribute, newColor)
	}), nil
}

// applyFiltersToClassElements applies filter functions only to elements that use specific CSS classes
func applyFiltersToClassElements(content string, functions []filterFunction, styleContent string) (string, error) {
	if len(functions) == 0 {
		return content, nil
	}
	
	// Extract class names from the style content
	classRegex := regexp.MustCompile(`\.([\w-]+)\s*\{[^}]*filter:`)
	classMatches := classRegex.FindAllStringSubmatch(styleContent, -1)
	
	if len(classMatches) == 0 {
		return content, nil
	}
	
	// Build list of class names that have filters
	var classNames []string
	for _, match := range classMatches {
		classNames = append(classNames, match[1])
	}
	
	// For each class name, find elements that use it and apply filters to their colors
	modifiedContent := content
	for _, className := range classNames {
		// Find elements with this class
		elementRegex := regexp.MustCompile(`(<[^>]+class="[^"]*` + regexp.QuoteMeta(className) + `[^"]*"[^>]*>)`)
		
		modifiedContent = elementRegex.ReplaceAllStringFunc(modifiedContent, func(element string) string {
			// Apply filters to colors within this element
			colorRegex := regexp.MustCompile(`(fill|stroke|stop-color|color)="([^"]+)"`)
			
			return colorRegex.ReplaceAllStringFunc(element, func(match string) string {
				parts := colorRegex.FindStringSubmatch(match)
				if len(parts) != 3 {
					return match
				}
				
				attribute := parts[1]
				colorValue := parts[2]
				
				// Skip non-color values
				if colorValue == "none" || colorValue == "transparent" || strings.HasPrefix(colorValue, "url(") {
					return match
				}
				
				// Parse the color
				r, g, b, err := parseColor(colorValue)
				if err != nil {
					return match // Return unchanged if we can't parse the color
				}
				
				// Apply each filter function in sequence
				for _, function := range functions {
					r, g, b = applyFilterFunction(r, g, b, function)
				}
				
				// Convert back to hex color
				newColor := fmt.Sprintf("#%02x%02x%02x", clamp(r), clamp(g), clamp(b))
				return fmt.Sprintf(`%s="%s"`, attribute, newColor)
			})
		})
	}
	
	return modifiedContent, nil
}

// parseColor parses a color string and returns RGB values (0-255)
func parseColor(color string) (int, int, int, error) {
	color = strings.TrimSpace(color)
	
	// Handle hex colors
	if strings.HasPrefix(color, "#") {
		hex := strings.TrimPrefix(color, "#")
		
		// Handle 3-digit hex
		if len(hex) == 3 {
			hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
		}
		
		if len(hex) != 6 {
			return 0, 0, 0, fmt.Errorf("invalid hex color: %s", color)
		}
		
		r, err := strconv.ParseInt(hex[0:2], 16, 64)
		if err != nil {
			return 0, 0, 0, err
		}
		g, err := strconv.ParseInt(hex[2:4], 16, 64)
		if err != nil {
			return 0, 0, 0, err
		}
		b, err := strconv.ParseInt(hex[4:6], 16, 64)
		if err != nil {
			return 0, 0, 0, err
		}
		
		return int(r), int(g), int(b), nil
	}
	
	// Handle named colors (basic set)
	namedColors := map[string][3]int{
		"red":     {255, 0, 0},
		"green":   {0, 128, 0},
		"blue":    {0, 0, 255},
		"white":   {255, 255, 255},
		"black":   {0, 0, 0},
		"yellow":  {255, 255, 0},
		"cyan":    {0, 255, 255},
		"magenta": {255, 0, 255},
		"gray":    {128, 128, 128},
		"grey":    {128, 128, 128},
		"orange":  {255, 165, 0},
		"purple":  {128, 0, 128},
	}
	
	if rgb, exists := namedColors[strings.ToLower(color)]; exists {
		return rgb[0], rgb[1], rgb[2], nil
	}
	
	return 0, 0, 0, fmt.Errorf("unsupported color format: %s", color)
}

// applyFilterFunction applies a single filter function to RGB values
func applyFilterFunction(r, g, b int, function filterFunction) (int, int, int) {
	switch function.name {
	case "invert":
		amount := 1.0 // default to 100%
		if function.value != "" {
			if strings.HasSuffix(function.value, "%") {
				if val, err := strconv.ParseFloat(strings.TrimSuffix(function.value, "%"), 64); err == nil {
					amount = val / 100.0
				}
			} else {
				if val, err := strconv.ParseFloat(function.value, 64); err == nil {
					amount = val
				}
			}
		}
		
		// Correct CSS invert formula: output = input * (1 - amount) + (255 - input) * amount
		newR := float64(r) * (1.0 - amount) + float64(255 - r) * amount
		func applyInvert(r, g, b int, amount float64) (int, int, int) {
	// W3C spec: feComponentTransfer with type="table" tableValues="[amount] (1 - [amount])"
	tableValues := []float64{amount, 1.0 - amount}
	
	rNorm := float64(r) / 255.0
	gNorm := float64(g) / 255.0
	bNorm := float64(b) / 255.0
	
	// Apply table-based transfer function per W3C spec
	newR := applyTableTransfer(rNorm, tableValues)
	newG := applyTableTransfer(gNorm, tableValues)
	newB := applyTableTransfer(bNorm, tableValues)
	
	return int(newR * 255), int(newG * 255), int(newB * 255)
}

func applyTableTransfer(input float64, tableValues []float64) float64 {
	if len(tableValues) == 0 {
		return input
	}
	if len(tableValues) == 1 {
		return tableValues[0]
	}
	
	// Clamp input to [0, 1]
	if input <= 0 {
		return tableValues[0]
	}
	if input >= 1 {
		return tableValues[len(tableValues)-1]
	}
	
	// Linear interpolation between table values
	scaledInput := input * float64(len(tableValues)-1)
	index := int(scaledInput)
	fraction := scaledInput - float64(index)
	
	if index >= len(tableValues)-1 {
		return tableValues[len(tableValues)-1]
	}
	
	return tableValues[index]*(1.0-fraction) + tableValues[index+1]*fraction
}  
		newB := float64(b) * (1.0 - amount) + float64(255 - b) * amount
		
		return clamp(int(newR)), clamp(int(newG)), clamp(int(newB))
		
	case "hue-rotate":
		angle := 0.0
		if function.value != "" {
			if strings.HasSuffix(function.value, "deg") {
				if val, err := strconv.ParseFloat(strings.TrimSuffix(function.value, "deg"), 64); err == nil {
					angle = val
				}
			} else {
				if val, err := strconv.ParseFloat(function.value, 64); err == nil {
					angle = val
				}
			}
		}
		
		// Convert to HSL, rotate hue, convert back to RGB
		h, s, l := rgbToHsl(r, g, b)
		h = math.Mod(h+angle/360.0, 1.0)
		if h < 0 {
			h += 1.0
		}
		resultR, resultG, resultB := hslToRgb(h, s, l)
		return resultR, resultG, resultB
	}
	
	return r, g, b
}

// rgbToHsl converts RGB values (0-255) to HSL values (0-1)
func rgbToHsl(r, g, b int) (float64, float64, float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	
	h := 0.0
	s := 0.0
	l := (max + min) / 2.0
	
	if max != min {
		d := max - min
		if l > 0.5 {
			s = d / (2.0 - max - min)
		} else {
			s = d / (max + min)
		}
		
		switch max {
		case rf:
			h = (gf-bf)/d + (map[bool]float64{true: 6.0, false: 0.0}[gf < bf])
		case gf:
			h = (bf-rf)/d + 2.0
		case bf:
			h = (rf-gf)/d + 4.0
		}
		h /= 6.0
	}
	
	return h, s, l
}

// hslToRgb converts HSL values (0-1) to RGB values (0-255)
func hslToRgb(h, s, l float64) (int, int, int) {
	var r, g, b float64
	
	if s == 0 {
		r = l
		g = l
		b = l
	} else {
		hue2rgb := func(p, q, t float64) float64 {
			if t < 0 {
				t += 1
			}
			if t > 1 {
				t -= 1
			}
			if t < 1.0/6.0 {
				return p + (q-p)*6*t
			}
			if t < 1.0/2.0 {
				return q
			}
			if t < 2.0/3.0 {
				return p + (q-p)*(2.0/3.0-t)*6
			}
			return p
		}
		
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		
		r = hue2rgb(p, q, h+1.0/3.0)
		g = hue2rgb(p, q, h)
		b = hue2rgb(p, q, h-1.0/3.0)
	}
	
	return int(r * 255), int(g * 255), int(b * 255)
}

// clamp ensures a value is between 0 and 255
func clamp(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}

// filterFunction represents a CSS filter function
type filterFunction struct {
	name  string
	value string
}

// parseFilterFunctions parses CSS filter functions from a filter value
func parseFilterFunctions(filterValue string) []filterFunction {
	var functions []filterFunction
	
	// Regex to match filter functions like invert(100%) or hue-rotate(180deg)
	// Updated to include hyphens in function names
	funcRegex := regexp.MustCompile(`([\w-]+)\(([^)]*)\)`)
	matches := funcRegex.FindAllStringSubmatch(filterValue, -1)
	
	for _, match := range matches {
		functions = append(functions, filterFunction{
			name:  strings.TrimSpace(match[1]),
			value: strings.TrimSpace(match[2]),
		})
	}
	
	return functions
}

// removeCSSFilters removes CSS filter properties from style content
func removeCSSFilters(styleContent string, filterMatches [][]string) string {
	modified := styleContent
	
	for _, match := range filterMatches {
		// Remove the entire filter property
		filterProperty := match[0]
		modified = strings.Replace(modified, filterProperty, "", 1)
	}
	
	return modified
}