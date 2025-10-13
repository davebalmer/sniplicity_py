# Sniplicity Go

A Go rewrite of the sniplicity static site generator.

## Features

- Static site generation with markdown and HTML processing
- Snippet system with copy/cut/paste directives  
- Template system with variable substitution
- Index generation for file listings
- File watching and live rebuilds

## Usage

```bash
# Build the project (from main directory)
./build.sh

# Or build manually
cd golang && go build -o sniplicity ./cmd

# Project-based usage (recommended)
# Start project selector (opens browser automatically)
./sniplicity

# Start project selector in clipboard-only mode (for -s flag compatibility)
./sniplicity -s

# Legacy usage with explicit directories
./sniplicity -i input_dir -o output_dir

# With file watching
./sniplicity -i input_dir -o output_dir -w

# With web server and file watching
./sniplicity -i input_dir -o output_dir -s

# With web server on custom port
./sniplicity -i input_dir -o output_dir -s -p 8000

# With verbose output
./sniplicity -i input_dir -o output_dir -v

# All options combined
./sniplicity -i input_dir -o output_dir -s -p 8000 -v --imgsize on
```

## Command Line Options

| Flag | Long Form | Description |
|------|-----------|-------------|
| `-i` | `--in` | Input (source) directory (legacy mode) |
| `-o` | `--out` | Output (destination) directory (legacy mode) |
| `-w` | `--watch` | Watch source directory and rebuild on changes |
| `-s` | `--serve` | Start web server and enable watch mode |
| `-p` | `--port` | Port for web server (default: 3000) |
| `-v` | `--verbose` | Enable verbose output |
| | `--imgsize` | Auto-add width/height to img tags (on/off, default: on) |
| | `--version` | Show version information |

## Modern Workflow (Recommended)

Sniplicity now uses a project-based approach with a web interface:

```bash
# Start the project selector (no arguments needed)
./sniplicity
```

This will:
1. Open a web interface at http://127.0.0.1:3000/sniplicity
2. Allow you to select and manage projects
3. Automatically handle configuration via `sniplicity.yaml` files
4. Provide a clean development experience

### Project Configuration

Create a `sniplicity.yaml` file in your project directory:

```yaml
name: "My Website"
input_dir: "source"
output_dir: "build"
port: 3000
watch: true
serve: true
verbose: false
imgsize: true
```

## Legacy Mode

For backward compatibility, you can still use explicit directory flags:

```bash
./sniplicity -i my_site -o build -s
```

This will:
1. Build your site once
2. Start watching for file changes and rebuild automatically
3. Start a web server serving your built site
4. Allow graceful shutdown with Ctrl+C

## Directives

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
- `internal/builder/` - Main build orchestration and server management
- `internal/config/` - Configuration structures and YAML handling
- `internal/parser/` - Directive parsing logic
- `internal/processor/` - File processing logic
- `internal/projects/` - Project management and recent projects
- `internal/types/` - Core data types and file structures
- `internal/watcher/` - File watching functionality
- `internal/web/` - Web interface and API endpoints

## Web Interface Features

- **Project Selector**: Choose and switch between projects
- **Recent Projects**: Quick access to recently used projects
- **Settings Management**: Configure projects via web interface
- **Network Access**: Access from mobile devices on local network
- **Live Reloading**: Automatic rebuilds when files change

## Processing Order

1. Load all files and collect templates/snippets/globals
2. Process includes
3. Process index commands
4. Process snippets (paste directives)
5. Process variables and write output files
