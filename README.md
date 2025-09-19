# sniplicity_py
A Python version of sniplicity with enhanced features and capabilities

Simple comment-based static HTML build system that lets you reuse code snippets with simple variables and conditional inclusion. Great for building static websites with less hassle.

- Static page builder with Markdown support
- All commands are embedded in HTML comments
- Re-use snippets of HTML throughout your project
- Use variables to replace values and control your build
- Include other files
- **Template system with multiple template types**
- **Markdown frontmatter support (YAML-style)**
- **Index generation with automatic file listing and sorting**
- **Enhanced variable hierarchy (frontmatter + set commands)**

![Logo](sniplicity.png)

## Install

```bash
# Clone the repository
git clone https://github.com/davebalmer/sniplicity_py.git
cd sniplicity_py

# Install dependencies
pip install markdown colorama watchdog pymdownx
```

## Run

```bash
# Basic usage
python3 sniplicity.py -i source_dir -o output_dir

# With watch mode (rebuilds automatically on file changes)
python3 sniplicity.py -i source_dir -o output_dir -w

# With verbose output
python3 sniplicity.py -i source_dir -o output_dir -v
```

Currently **sniplicity** supports:

1. Read all `md`, `mdown`, `markdown`, `html`, `htm` and `txt` files found in the source directory
2. Process Markdown files (converted to HTML with frontmatter extraction)
3. Apply templates and process all sniplicity commands
4. Save compiled files to the output directory
5. Handle recursive sub-directories with proper path preservation

## Features

### Markdown Support
- Full Markdown processing with extensions (fenced code, tables, TOC, emoji)
- YAML frontmatter support for metadata
- Automatic conversion to HTML

### Template System
- Define reusable page templates with `<!-- template name -->`
- Multiple template types for different content (e.g., `blog`, `page`)
- Template inheritance with snippet processing
- Content injection with `{{content}}` placeholder

### Index Generation
- Automatic file listing with `<!-- index pattern template [sort_field] -->`
- Glob pattern matching (e.g., `blog/*.md`)
- Sorting by any metadata field (dates in descending order)
- Custom templates for index entries

## Options

| Flag | Short | Purpose |
|------|-------|---------|
| --in | -i | Input (source) directory |
| --out | -o | Output (destination) directory |
| --watch | -w | Watch the source directory and rebuild automatically when files change |
| --verbose | -v | Enable verbose output showing processing details |
| --help | -h | Show help message |
| --version | | Show version information |

# Commands

All **sniplicity** commands are embedded in HTML comments, so they will not interfere with your favorite editor. Here are some examples:

```html
<!-- set test -->
<!-- set title Hello World -->

<!-- if test -->
	<h1>--title--</h1>
<!-- endif -->

<!-- cut footer -->
<footer>
	Copyright &copy; 2016
</footer>
<!-- end -->

<!-- paste footer -->
<!-- include disclaimer.html -->

<!-- template blog -->
<!-- paste header -->
<main>{{content}}</main>
<!-- paste footer -->
<!-- end -->

<!-- index blog/*.md blog-item date -->
```

## Markdown Frontmatter

You can include YAML frontmatter at the beginning of Markdown files:

```markdown
---
title: My Blog Post
author: John Doe
date: 2025-09-18
description: This is a great blog post
tags: technology, programming, web
template: blog
---

# Content starts here

Your markdown content...
```

Variables from frontmatter are automatically available as `--title--`, `--author--`, etc.

## Templates

Define reusable page structures with the template command:

```html
<!-- template blog -->
<!DOCTYPE html>
<html>
<head>
    <title>--title--</title>
    <meta name="description" content="--description--">
</head>
<body>
    <!-- paste header -->
    <main>
        <h1>--title--</h1>
        <p>By --author-- on --date--</p>
        {{content}}
    </main>
    <!-- paste footer -->
</body>
</html>
<!-- end -->
```

Use templates in files with:

```markdown
---
title: My Post
template: blog
---

# My Content
This content will be injected where {{content}} appears in the template.
```

## Index Generation

Automatically generate file listings:

```html
<!-- index blog/*.md blog-item date -->
```

This finds all `.md` files in the `blog/` directory, uses the `blog-item` template for each entry, and sorts by the `date` field (most recent first).

Define the index item template:

```html
<!-- template blog-item -->
<article>
    <h3><a href="--filepath--">--title--</a></h3>
    <p>--description--</p>
    <small>Published: --date-- | Tags: --tags--</small>
</article>
<!-- end -->
```

## Marking snippets with `cut` and `copy`

Snippets work like a text editor clipboard. Use `cut`, `copy` and `paste` to define and use snippets. You may cut or copy them anywhere in your project and they can be pasted anywhere in any file (even the one they were defined in).

```html
<!-- copy nav -->
<p>
	Nav: A, B, C<br>
	(will show up twice because we copied it)
</p>
<!-- end -->

<!-- cut footer -->
<p>
	Copyright &copy; 2016 A, B, C<br>
	(only shows up once because we cut it)
</p>
<!-- end -->
```

Both `cut` and `copy` make snippets. The
difference is `copy` will copy the snippet while, you guessed it, 
`cut` will cut it out of the current file. When in doubt, use `copy`.

## Pasting snippets with `paste`

To use any snippet, just `paste` it by name in any file in your project.

```html
<!-- paste footer -->
<!-- paste nav -->
```

## Assigning variables with `set`

Variables are available only within the file where you `set` them. The last declaration in a file takes precedence. Variables from Markdown frontmatter are also available and can be overridden by `set` commands.

```html
<!-- set test -->
<!-- set message Hello World! -->
```

**Variable Priority (highest to lowest):**
1. `<!-- set variable value -->` commands in the file
2. YAML frontmatter variables (in Markdown files)
3. `<!-- global variable value -->` default values

## Using variables with `--variable_name--` marks

With any variable (or default variable), you may include the value by using a simple mark anywhere in your file with the name of the variable surrounded by two dashes.

```html
<!-- set title Hello World! -->
<title>My title is --title--</title>
```

## Make global default variables with `global`

You may set global default values for variables with `global`. If you use `set` with the same variable name in a given file, or if the variable exists in frontmatter, those values will override the global default for that file.

```html
<!-- global development -->
<!-- global title My Website -->
<!-- global author Site Administrator -->
```

## Make conditional builds using `if` and `endif`

You can test a variable to decide whether to include or exclude a section. These work inside templates and snippets as well.

```html
<!-- if test -->
test is truthy!<br>
<!-- endif -->

<!-- if !test -->
test is falsy!<br>
<!-- endif -->
```

## Include other content files using `include`

Include content from other files at processing time:

```html
<!-- include header.html -->
<!-- include navigation.html -->
```

Included files are processed recursively, so they can contain sniplicity commands too.

## Example Project Structure

```
src/
├── blog-template.md          # Template definitions
├── blog.md                   # Blog index page
├── blog/
│   ├── post1.md             # Individual blog posts
│   └── post2.md
├── includes/
│   ├── header.html          # Reusable snippets
│   └── footer.html
└── css/
    └── styles.css

output/
├── blog.html                # Generated blog index
├── blog/
│   ├── post1.html           # Generated posts
│   └── post2.html
└── css/
    └── styles.css           # Copied as-is
```

# About

This build tool scratches an itch I had for a static HTML builder that could move easily from single-page design prototype to multi-page production site with incremental and minimal automation effort. This Python version adds significant new capabilities while maintaining the simple, comment-based approach.

## Key Improvements in Python Version

- **Markdown Support**: Full Markdown processing with YAML frontmatter
- **Template System**: Reusable page templates with content injection
- **Index Generation**: Automatic file listing and sorting
- **Enhanced Variables**: Support for frontmatter, with clear priority hierarchy
- **Better Watch Mode**: More reliable file watching with debouncing
- **Improved Error Handling**: Better error messages and validation

## Templates (not exactly)

**Sniplicity** is not a traditional template engine so much as a tool that lets you "short-cut" the more redundant efforts of producing hand-coded HTML by using code snippets, simple variable substitution, and conditional code and file inclusion. The new template system provides a middle ground - more structure than pure snippets, but simpler than complex template engines.

That said, it's designed to be completely compatible with other tools like preprocessors, bundlers, and deployment systems.

## Roadmap

Current features are stable and well-tested. Future improvements may include:

- Single file processing mode
- More markdown extensions
- Plugin system for custom processing
- Better error reporting and validation
- Additional template features
- CSS/JS bundling integration

## Dependencies

- Python 3.8+
- `markdown` - Markdown processing
- `colorama` - Colored console output  
- `watchdog` - File system monitoring
- `pymdownx` - Extended markdown features

## Contributing

Yes! Let me know of any errors or feature requests in the issue tracker. If you want to take a stab at making improvements, please do. Nothing special is required, just clone this repo and start coding. I'm open to all good pull requests.

## License

Copyright (C) 2016-2025 Dave Balmer  
Using the MIT License (MIT)
