<!-- Workspace-specific instructions for Sniplicity Go -->

This is a Go rewrite of the Python sniplicity static site generator. The project focuses on:

- Static site generation with markdown and HTML processing
- Snippet system with copy/cut/paste directives  
- Template system with variable substitution
- Index generation for file listings
- File watching and live rebuilds
- Command-line interface matching Python version exactly

Key requirements:
- Maintain exact feature parity with Python version
- Process files in same order as Python version
- Support all directive types: copy, cut, paste, set, global, template, include, index
- Handle nested snippets correctly
- Support YAML frontmatter parsing
- Command line: sniplicity -i input -o output [-w] [-v]

Architecture follows Go best practices with cmd/, internal/, and pkg/ structure.