# Sniplicity Go

A Go rewrite of the Python sniplicity static site generator with exact feature parity.

## Features

- Static site generation with markdown and HTML processing
- Snippet system with copy/cut/paste directives  
- Template system with variable substitution
- Index generation for file listings
- File watching and live rebuilds
- Command-line interface matching Python version exactly

## Usage

```bash
# Build the project
go build -o sniplicity ./cmd

# Basic usage
./sniplicity -i input_dir -o output_dir

# With file watching
./sniplicity -i input_dir -o output_dir -w

# With web server and file watching (serves at http://127.0.0.1:3000)
./sniplicity -i input_dir -o output_dir -s

# With web server on custom port
./sniplicity -i input_dir -o output_dir -s -p 8000

# With verbose output
./sniplicity -i input_dir -o output_dir -v

# All options combined
./sniplicity -i input_dir -o output_dir -s -p 8000 -v
```

## Command Line Options

| Flag | Long Form | Description |
|------|-----------|-------------|
| `-i` | `--in` | Input (source) directory |
| `-o` | `--out` | Output (destination) directory |
| `-w` | `--watch` | Watch source directory and rebuild on changes |
| `-s` | `--serve` | Start web server and enable watch mode |
| `-p` | `--port` | Port for web server (default: 3000) |
| `-v` | `--verbose` | Enable verbose output |
| `--version` | | Show version information |

## Development Workflow

The `-s/--serve` option is ideal for development:

```bash
./sniplicity -i my_site -o build -s
```

This will:
1. Build your site once
2. Start watching for file changes and rebuild automatically
3. Start a web server at http://127.0.0.1:3000 serving your built site
4. Allow graceful shutdown with Ctrl+C

Open http://127.0.0.1:3000 in your browser and any changes to source files will trigger an automatic rebuild.

## Directives

Same as Python version:

- `<!-- copy snippet_name -->...<!-- end -->` - Define a snippet
- `<!-- cut snippet_name -->...<!-- end -->` - Define and remove a snippet  
- `<!-- paste snippet_name -->` - Insert a snippet
- `<!-- set variable_name value -->` - Set a local variable
- `<!-- global variable_name value -->` - Set a global variable
- `<!-- template template_name -->...<!-- end -->` - Define a template
- `<!-- include path/to/file -->` - Include another file
- `<!-- index path/to/directory -->` - Generate directory index

## Architecture

- `cmd/` - Main application entry point
- `internal/builder/` - Main build orchestration
- `internal/config/` - Configuration structures
- `internal/parser/` - Directive parsing logic
- `internal/processor/` - File processing logic
- `internal/types/` - Core data types
- `internal/watcher/` - File watching functionality

## Processing Order

Matches Python version exactly:

1. Load all files and collect templates/snippets/globals
2. Process includes
3. Process index commands
4. Process snippets (paste directives)
5. Process variables and write output files