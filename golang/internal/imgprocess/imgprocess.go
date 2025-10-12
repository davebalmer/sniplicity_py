package imgprocess

import (
	"fmt"
	"image"
	_ "image/gif"  // Support for GIF
	_ "image/jpeg" // Support for JPEG
	_ "image/png"  // Support for PNG
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ImageDimensions holds width and height of an image
type ImageDimensions struct {
	Width  int
	Height int
}

// GetImageDimensions returns the width and height of an image file
func GetImageDimensions(imagePath string) (ImageDimensions, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return ImageDimensions{}, fmt.Errorf("opening image file: %w", err)
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return ImageDimensions{}, fmt.Errorf("decoding image config: %w", err)
	}

	return ImageDimensions{
		Width:  config.Width,
		Height: config.Height,
	}, nil
}

// ProcessHTMLForImages processes HTML content to add width and height attributes to img tags
func ProcessHTMLForImages(htmlContent string, outputDir string, verbose bool) (string, error) {
	lines := strings.Split(htmlContent, "\n")
	
	// Regex patterns for img tags and picture elements
	imgRegex := regexp.MustCompile(`<img\s+[^>]*>`)
	pictureRegex := regexp.MustCompile(`<picture\b[^>]*>`)
	pictureEndRegex := regexp.MustCompile(`</picture>`)
	
	var result []string
	insidePicture := false
	
	for _, line := range lines {
		// Check if we're entering or leaving a picture tag
		if pictureRegex.MatchString(line) {
			insidePicture = true
		}
		if pictureEndRegex.MatchString(line) {
			insidePicture = false
		}
		
		// Process img tags in this line
		processedLine := imgRegex.ReplaceAllStringFunc(line, func(match string) string {
			return processImgTag(match, outputDir, insidePicture, verbose)
		})
		
		result = append(result, processedLine)
	}
	
	return strings.Join(result, "\n"), nil
}

// ProcessHTMLForMarkdownImages processes HTML content to add width and height attributes to img tags that came from markdown
func ProcessHTMLForMarkdownImages(htmlContent string, outputDir string, htmlDir string, markdownImages map[string]bool, verbose bool) (string, error) {
	lines := strings.Split(htmlContent, "\n")
	
	// Regex patterns for img tags and picture elements
	imgRegex := regexp.MustCompile(`<img\s+[^>]*>`)
	pictureRegex := regexp.MustCompile(`<picture\b[^>]*>`)
	pictureEndRegex := regexp.MustCompile(`</picture>`)
	
	var result []string
	insidePicture := false
	
	for _, line := range lines {
		// Check if we're entering or leaving a picture tag
		if pictureRegex.MatchString(line) {
			insidePicture = true
		}
		if pictureEndRegex.MatchString(line) {
			insidePicture = false
		}
		
		// Process img tags in this line
		processedLine := imgRegex.ReplaceAllStringFunc(line, func(match string) string {
			return processMarkdownImgTag(match, outputDir, htmlDir, markdownImages, insidePicture, verbose)
		})
		
		result = append(result, processedLine)
	}
	
	return strings.Join(result, "\n"), nil
}

// processImgTag processes a single img tag
func processImgTag(imgTag string, outputDir string, insidePicture bool, verbose bool) string {
	// Check if width and height attributes already exist
	hasWidth := regexp.MustCompile(`(?i)\swidth\s*=`).MatchString(imgTag)
	hasHeight := regexp.MustCompile(`(?i)\sheight\s*=`).MatchString(imgTag)
	
	if hasWidth && hasHeight {
		// Both attributes already exist, no need to process
		return imgTag
	}
	
	// Extract src attribute
	srcRegex := regexp.MustCompile(`(?i)\ssrc\s*=\s*["']([^"']+)["']`)
	srcMatch := srcRegex.FindStringSubmatch(imgTag)
	if len(srcMatch) < 2 {
		// No src attribute found
		return imgTag
	}
	
	srcPath := srcMatch[1]
	
	// Skip external URLs (http/https)
	if strings.HasPrefix(strings.ToLower(srcPath), "http://") || strings.HasPrefix(strings.ToLower(srcPath), "https://") {
		return imgTag
	}
	
	// Skip data URLs
	if strings.HasPrefix(strings.ToLower(srcPath), "data:") {
		return imgTag
	}
	
	// Check if it's a supported image format
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
		return imgTag
	}
	
	// Construct full path to image file - look in output directory
	var imagePath string
	if strings.HasPrefix(srcPath, "/") {
		// Web-relative path starting with / - resolve relative to output directory
		imagePath = filepath.Join(outputDir, strings.TrimPrefix(srcPath, "/"))
	} else if filepath.IsAbs(srcPath) {
		// Absolute filesystem path
		imagePath = srcPath
	} else {
		// Relative path - resolve relative to output directory
		imagePath = filepath.Join(outputDir, srcPath)
	}
	
	// Get image dimensions
	dims, err := GetImageDimensions(imagePath)
	if err != nil {
		if verbose {
			fmt.Printf("Warning: Cannot get dimensions for image %s: %v\n", imagePath, err)
		}
		return imgTag
	}
	
	if verbose {
		fmt.Printf("  Adding dimensions to %s: %dx%d\n", srcPath, dims.Width, dims.Height)
	}
	
	// Add width and height attributes
	result := imgTag
	
	// For images inside picture tags, handle differently according to responsive image specs
	if insidePicture {
		// Inside picture tags, we typically don't want to set explicit width/height
		// as they can interfere with responsive behavior. However, we can add them
		// if the user specifically wants them for layout stability
		if !hasWidth {
			result = addAttribute(result, "width", fmt.Sprintf("%d", dims.Width))
		}
		if !hasHeight {
			result = addAttribute(result, "height", fmt.Sprintf("%d", dims.Height))
		}
	} else {
		// Regular img tags - add both attributes
		if !hasWidth {
			result = addAttribute(result, "width", fmt.Sprintf("%d", dims.Width))
		}
		if !hasHeight {
			result = addAttribute(result, "height", fmt.Sprintf("%d", dims.Height))
		}
	}
	
	return result
}

// addAttribute adds an attribute to an img tag
func addAttribute(imgTag, attrName, attrValue string) string {
	// Find the position to insert the attribute (before the closing > or />)
	closePos := strings.LastIndex(imgTag, ">")
	if closePos == -1 {
		return imgTag
	}
	
	// Check if it's a self-closing tag
	before := imgTag[:closePos]
	after := imgTag[closePos:]
	
	// If it's a self-closing tag, insert before the /
	if strings.HasSuffix(before, "/") {
		before = strings.TrimSuffix(before, "/")
		return fmt.Sprintf(`%s %s="%s"/%s`, before, attrName, attrValue, after)
	}
	
	// Regular tag
	return fmt.Sprintf(`%s %s="%s"%s`, before, attrName, attrValue, after)
}

// processMarkdownImgTag processes a single img tag, but only if it came from markdown
func processMarkdownImgTag(imgTag string, outputDir string, htmlDir string, markdownImages map[string]bool, insidePicture bool, verbose bool) string {
	// Extract src attribute first to check if this image came from markdown
	srcRegex := regexp.MustCompile(`(?i)\ssrc\s*=\s*["']([^"']+)["']`)
	srcMatch := srcRegex.FindStringSubmatch(imgTag)
	if len(srcMatch) < 2 {
		// No src attribute found
		return imgTag
	}
	
	srcPath := srcMatch[1]
	
	// Only process if this image URL was found in the original markdown
	if !markdownImages[srcPath] {
		return imgTag
	}
	
	// Use a modified version of processImgTag that uses htmlDir for relative paths
	return processImgTagWithContext(imgTag, outputDir, htmlDir, insidePicture, verbose)
}

// processImgTagWithContext processes a single img tag with HTML directory context for relative paths
func processImgTagWithContext(imgTag string, outputDir string, htmlDir string, insidePicture bool, verbose bool) string {
	// Check if width and height attributes already exist
	hasWidth := regexp.MustCompile(`(?i)\swidth\s*=`).MatchString(imgTag)
	hasHeight := regexp.MustCompile(`(?i)\sheight\s*=`).MatchString(imgTag)
	
	if hasWidth && hasHeight {
		// Both attributes already exist, no need to process
		return imgTag
	}
	
	// Extract src attribute
	srcRegex := regexp.MustCompile(`(?i)\ssrc\s*=\s*["']([^"']+)["']`)
	srcMatch := srcRegex.FindStringSubmatch(imgTag)
	if len(srcMatch) < 2 {
		// No src attribute found
		return imgTag
	}
	
	srcPath := srcMatch[1]
	
	// Skip external URLs (http/https)
	if strings.HasPrefix(strings.ToLower(srcPath), "http://") || strings.HasPrefix(strings.ToLower(srcPath), "https://") {
		return imgTag
	}
	
	// Skip data URLs
	if strings.HasPrefix(strings.ToLower(srcPath), "data:") {
		return imgTag
	}
	
	// Check if it's a supported image format
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
		return imgTag
	}
	
	// Construct full path to image file with proper context
	var imagePath string
	if strings.HasPrefix(srcPath, "/") {
		// Web-relative path starting with / - resolve relative to output directory
		imagePath = filepath.Join(outputDir, strings.TrimPrefix(srcPath, "/"))
	} else if filepath.IsAbs(srcPath) {
		// Absolute filesystem path
		imagePath = srcPath
	} else {
		// Relative path - resolve relative to HTML file's directory
		imagePath = filepath.Join(htmlDir, srcPath)
	}
	
	// Get image dimensions
	dims, err := GetImageDimensions(imagePath)
	if err != nil {
		if verbose {
			fmt.Printf("Warning: Cannot get dimensions for image %s: %v\n", imagePath, err)
		}
		return imgTag
	}
	
	result := imgTag
	if !hasWidth {
		result = addAttribute(result, "width", fmt.Sprintf("%d", dims.Width))
	}
	if !hasHeight {
		result = addAttribute(result, "height", fmt.Sprintf("%d", dims.Height))
	}
	
	if verbose {
		fmt.Printf("Adding dimensions to %s: %dx%d\n", srcPath, dims.Width, dims.Height)
	}
	
	return result
}