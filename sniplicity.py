#!/usr/bin/env python3

import os
import sys
import re
import time
import argparse
from typing import List, Dict, Optional, Tuple
import colorama
from colorama import Fore, Style
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

colorama.init()

import markdown
from markdown.extensions import fenced_code
import pymdownx.emoji
import yaml

# Global state
VAR_START = r"\-\-"
VAR_END = r"\-\-"
snippets: Dict[str, List[str]] = {}
defglob: Dict[str, str] = {}
templates: Dict[str, List[str]] = {}
verbose_cli = False

# Markdown configuration
MD_EXTENSIONS = [
    'markdown.extensions.fenced_code',    # ```code blocks```
    'markdown.extensions.tables',         # Tables
    'markdown.extensions.toc',            # [TOC] insertion
    # 'markdown.extensions.attr_list',      # {: #custom-id} attributes - REMOVED: conflicts with sniplicity variables
    'pymdownx.emoji',                     # :emoji: support
    'markdown.extensions.md_in_html'      # Markdown inside HTML blocks
]
MD_EXTENSION_CONFIGS = {
    'pymdownx.emoji': {
        'emoji_index': pymdownx.emoji.twemoji,
        'emoji_generator': pymdownx.emoji.to_svg,
    }
}

def print_banner() -> None:
    def cool(l: str, r: str) -> None:
        print(f"{Fore.GREEN}{l}{Fore.CYAN}{r}{Style.RESET_ALL}")

    cool("            _      ", " _  _       _             ")
    cool("           (_)     ", "| |(_)     (_)  _         ")
    cool("  ___ ____  _ ____ ", "| | _  ____ _ _| |_ _   _ ")
    cool(" /___)  _ \\| |  _ \\", "| || |/ ___) (_   _) | | |")
    cool("|___ | | | | | |_| ", "| || ( (___| | | |_| |_| |")
    cool("(___/|_| |_|_|  __/", " \\_)_|\\____)_|  \\__)\\__  |")
    cool("             |_|   ", "                   (____/ ")
    print(f"  {Style.DIM}{Fore.WHITE}http://github.com/davebalmer/sniplicity{Style.RESET_ALL}")

def verbose(msg: str) -> None:
    if verbose_cli:
        print(msg)

def warning(msg: str, filename: str = "", line: int = 0) -> None:
    pos = f" in {filename}:{line}" if line else f" in {filename}" if filename else ""
    print(f"{Fore.YELLOW}{Style.BRIGHT}Warning: {Style.RESET_ALL}{msg}{pos}")

def error(msg: str, filename: str = "", line: int = 0) -> None:
    pos = f" in {filename}:{line}" if line else f" in {filename}" if filename else ""
    print(f"\n{Fore.RED}{Style.BRIGHT}Error: {Style.RESET_ALL}{msg}{pos}\n")
    sys.exit(1)

def fix_dir(path: str) -> str:
    """Convert directory path to absolute path and ensure it ends with a separator"""
    if not path:
        return os.getcwd()
    return os.path.abspath(os.path.expanduser(path))

def parse_line(line: str) -> Optional[List[str]]:
    # Compile regex patterns once
    DIRECTIVE_PATTERN = re.compile(r'^\s*\<\!\-\-\s+(.*?)\s+\-\-\>')
    IDENTIFIER_PATTERN = re.compile(r'^[-\w.]+$')
    ID_COMMANDS = {"copy", "cut", "paste", "set", "global", "template"}
    
    # Match directive
    if not (match := DIRECTIVE_PATTERN.match(line.strip())):
        return None
        
    content = match.group(1)
    if not (parts := content.split(None, 1)):
        return None
        
    command = parts[0]
    if len(parts) == 1:
        return [command]
        
    # Handle identifier commands
    if command in ID_COMMANDS:
        rest = parts[1].strip()
        if not (id_parts := rest.split(None, 1)):
            return None
        
        identifier = id_parts[0]
        if not IDENTIFIER_PATTERN.match(identifier):
            warning(f"Invalid identifier '{identifier}'. Use only letters, numbers, hyphens, underscores, and periods.")
            return None
            
        return [command, identifier] + (id_parts[1].split() if len(id_parts) > 1 else [])
    
    # Handle other commands
    return [command] + parts[1].split()

def parse_value(parts: List[str]) -> str:
    if len(parts) > 2:
        return " ".join(parts[2:])
    return ""

def parse_markdown_meta(markdown_text: str) -> Dict[str, any]:
    """
    Parses the first markdown metadata block (YAML frontmatter style) into a dictionary.
    Supports # comments and comma-separated lists.
    """
    lines = markdown_text.splitlines()
    in_meta = False
    meta = {}
    
    for line in lines:
        line = line.strip()
        if line == '---':
            if not in_meta:
                in_meta = True
                continue
            else:
                break
        if not in_meta:
            continue
        if not line or line.startswith('#'):
            continue
        if ':' in line:
            key, value = line.split(':', 1)
            key = key.strip()
            value = value.strip()
            # Remove inline comment after value
            if '#' in value:
                value = value.split('#', 1)[0].strip()
            # Convert comma-separated values to list
            if ',' in value:
                value_list = [v.strip() for v in value.split(',') if v.strip()]
                meta[key] = value_list
            else:
                # Convert booleans and numbers
                if value.lower() == 'true':
                    meta[key] = True
                elif value.lower() == 'false':
                    meta[key] = False
                elif value.isdigit():
                    meta[key] = int(value)
                else:
                    meta[key] = value
    return meta

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=f"{Fore.WHITE}{Style.BRIGHT}Build simple static websites using:{Style.RESET_ALL}\n\n"
                    f"  - snippets with {Fore.GREEN}<!-- copy x -->{Style.RESET_ALL} and {Fore.GREEN}<!-- paste x -->{Style.RESET_ALL}\n"
                    f"  - variables using {Fore.GREEN}<!-- set y -->{Style.RESET_ALL} and {Fore.GREEN}<!-- global z -->{Style.RESET_ALL}\n"
                    f"  - include files with {Fore.GREEN}<!-- include filename.html -->{Style.RESET_ALL}\n\n"
                    f"  {Fore.YELLOW}{Style.BRIGHT}See README.md to get started.{Style.RESET_ALL}",
        usage="-i source_folder -o destination_folder -w"
    )
    parser.add_argument("-i", "--in", dest="input_dir", help="source directory")
    parser.add_argument("-o", "--out", dest="output_dir", required=True, help="output directory for compiled files")
    parser.add_argument("-w", "--watch", action="store_true", help="keep watching the input directory")
    parser.add_argument("-v", "--verbose", action="store_true", help="extra console messages")
    parser.add_argument("--version", action="version", version="%(prog)s 0.1.10")
    
    return parser.parse_args()

def main() -> None:
    global verbose_cli
    
    print_banner()
    args = parse_args()
    
    source_dir = fix_dir(args.input_dir)
    output_dir = fix_dir(args.output_dir)
    watch_mode = args.watch
    verbose_cli = args.verbose
    
    if not os.path.exists(source_dir):
        error(f"Source directory {Fore.CYAN}{source_dir}{Style.RESET_ALL} does not exist")
    
    os.makedirs(output_dir, exist_ok=True)
    
    build(source_dir, output_dir, watch_mode)

def get_file_list(source_dir: str) -> List[Tuple[str, str, bool]]:
    """
    Returns a list of tuples (relative_path, filename, is_markdown) for all processable files
    relative_path is the path relative to source_dir, including any subfolders
    """
    try:
        file_list = []
        for root, dirs, files in os.walk(source_dir):
            # Get path relative to source_dir
            rel_path = os.path.relpath(root, source_dir)
            rel_path_part = "" if rel_path == "." else rel_path
            
            for f in files:
                # Check if it's a markdown file
                if re.search(r'\.(md|mdown|markdown)$', f):
                    # For markdown files, we'll change the extension to .html in the output
                    file_list.append((rel_path_part, f, True))
                # Check if it's an HTML/HTM/TXT file
                elif re.search(r'(html|htm|txt)$', f):
                    file_list.append((rel_path_part, f, False))
        
        return file_list
    except OSError as e:
        error(f"Cannot open source directory {Fore.CYAN}{source_dir}{Style.RESET_ALL}")
        return []

def get_file_as_array(filepath: str, source_dir: str) -> Optional[List[str]]:
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            data = f.read()
    except OSError:
        try:
            full_path = os.path.join(source_dir, filepath)
            with open(full_path, 'r', encoding='utf-8') as f:
                data = f.read()
        except OSError:
            return None
    
    verbose(f"{Fore.GREEN}include {Fore.CYAN}{filepath}{Style.RESET_ALL}")
    return data.splitlines()

class FileInfo:
    def __init__(self, file_path: str, filename: str, is_markdown: bool = False):
        self.file_path = file_path
        self.filename = filename
        self.is_markdown = is_markdown
        self.data: List[str] = []
        self.def_vars: Dict[str, str] = {}
        self.meta_vars: Dict[str, any] = {}  # Store markdown metadata
        self.output_rel_path = ""  # Will be set by build function

    def load(self) -> bool:
        verbose(f"Loading file: {self.filename} (is_markdown: {self.is_markdown})")
        try:
            with open(self.file_path, 'r', encoding='utf-8') as f:
                content = f.read()
                if self.is_markdown:
                    # Extract metadata from markdown before processing
                    self.meta_vars = parse_markdown_meta(content)
                    verbose(f"  Found metadata in {self.filename}: {list(self.meta_vars.keys())}")
                    
                    # Convert markdown to HTML immediately, preserving frontmatter
                    content_without_meta = self._strip_metadata_block(content)
                    html_content = markdown.markdown(
                        content_without_meta,
                        extensions=MD_EXTENSIONS,
                        extension_configs=MD_EXTENSION_CONFIGS
                    )
                    
                    # Replace the content with HTML (frontmatter was already extracted)
                    content = html_content
                    
                    # Change the output filename to .html
                    base = os.path.splitext(self.filename)[0]
                    self.filename = f"{base}.html"
                    
                    # Mark as no longer markdown since it's now HTML
                    self.is_markdown = False
                
                self.data = content.splitlines()
            return True
        except OSError:
            warning(f"Cannot read file {Fore.CYAN}{self.file_path}{Style.RESET_ALL}")
            return False

    def process_markdown_with_template(self) -> None:
        """Process markdown content after all set commands have been processed"""
        # Template processing now happens only during variable processing phase
        # This method is kept for compatibility but does nothing
        pass

    def needs_template_processing(self) -> bool:
        """Check if this markdown file needs template processing"""
        if not self.is_markdown:
            return False
        template_name = self.def_vars.get('template') or self.meta_vars.get('template')
        return template_name is not None and template_name in templates

    def _strip_metadata_block(self, content: str) -> str:
        """Remove the first metadata block (between --- markers) from content"""
        lines = content.splitlines()
        in_meta = False
        content_start = 0
        
        for i, line in enumerate(lines):
            line = line.strip()
            if line == '---':
                if not in_meta:
                    in_meta = True
                    continue
                else:
                    # Found the end of metadata block
                    content_start = i + 1
                    break
        
        # Return content without the metadata block
        return '\n'.join(lines[content_start:])

    def _replace_metadata_vars(self, content: str) -> str:
        """Replace metadata variables in content before markdown processing"""
        if not self.meta_vars:
            return content
        
        for key, value in self.meta_vars.items():
            if isinstance(value, list):
                replacement = ', '.join(str(v) for v in value)
            else:
                replacement = str(value)
            
            pattern = f"{VAR_START}{re.escape(key)}{VAR_END}"
            content = re.sub(pattern, replacement, content)
        
        return content

    def save(self, output_dir: str, content: str) -> bool:
        try:
            if self.output_rel_path:
                output_subdir = os.path.join(output_dir, self.output_rel_path)
                os.makedirs(output_subdir, exist_ok=True)
                output_path = os.path.join(output_subdir, self.filename)
            else:
                output_path = os.path.join(output_dir, self.filename)
            
            # Ensure output directory exists
            os.makedirs(os.path.dirname(output_path), exist_ok=True)
            
            verbose(f"  Writing {'markdown as HTML' if self.is_markdown else 'file'}: {output_path}")
            with open(output_path, 'w', encoding='utf-8') as f:
                f.write(content)
            return True
        except OSError as e:
            error(f"Cannot write file {Fore.CYAN}{output_path}{Style.RESET_ALL}: {str(e)}")
            return False

def get_file_list(source_dir: str) -> List[Tuple[str, str, bool]]:
    """
    Returns a list of tuples (relative_path, filename, is_markdown) for all processable files
    relative_path is the path relative to source_dir, including any subfolders
    """
    try:
        file_list = []
        for root, dirs, files in os.walk(source_dir):
            # Get path relative to source_dir
            rel_path = os.path.relpath(root, source_dir)
            rel_path_part = "" if rel_path == "." else rel_path
            
            for f in files:
                # Check if it's a markdown file
                if re.search(r'\.(md|mdown|markdown)$', f):
                    # For markdown files, we'll change the extension to .html in the output
                    file_list.append((rel_path_part, f, True))
                # Check if it's an HTML/HTM/TXT file
                elif re.search(r'(html|htm|txt)$', f):
                    file_list.append((rel_path_part, f, False))
        
        return file_list
    except OSError as e:
        error(f"Cannot open source directory {Fore.CYAN}{source_dir}{Style.RESET_ALL}")
        return []

def get_file_as_array(filepath: str, source_dir: str) -> Optional[List[str]]:
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            data = f.read()
    except OSError:
        try:
            full_path = os.path.join(source_dir, filepath)
            with open(full_path, 'r', encoding='utf-8') as f:
                data = f.read()
        except OSError:
            return None
    
    verbose(f"{Fore.GREEN}include {Fore.CYAN}{filepath}{Style.RESET_ALL}")
    return data.splitlines()

def process_includes(file_list: List[FileInfo]) -> None:
    """Process all file includes before anything else"""
    verbose(f"Processing {Fore.CYAN}includes{Style.RESET_ALL}...")
    
    for file_info in file_list:
        i = 0
        while i < len(file_info.data):
            line = file_info.data[i]
            parts = parse_line(line)
            
            if parts and parts[0] == "include":
                included_lines = get_file_as_array(parts[1], os.path.dirname(file_info.file_path))
                if included_lines is None:
                    warning(f"Unable to {Fore.CYAN}include {Fore.CYAN}{parts[1]}{Style.RESET_ALL}", file_info.filename, i + 1)
                else:
                    file_info.data[i:i+1] = included_lines
                    i += len(included_lines) - 1
            i += 1

def collect_snippets_and_globals(file_list: List[FileInfo]) -> None:
    """First pass: collect all snippets, templates, and global variables from all files"""
    global snippets, defglob, templates
    
    verbose(f"Finding all {Fore.GREEN}snippets{Style.RESET_ALL}, templates, and globals...")
    verbose("Processing files in this order:")
    for file_info in file_list:
        verbose(f"  {file_info.filename}")
    
    # First collect all snippets and templates
    for file_info in file_list:
        # Stack to handle nested snippets/templates: (name, block, type, nesting_level, start_line)
        # type can be "copy", "cut", or "template"
        content_stack: List[Tuple[str, List[str], str, int, int]] = []
        
        for i, line in enumerate(file_info.data):
            parts = parse_line(line)
            
            if parts:
                if parts[0] in ["copy", "cut", "template"]:
                    # Add to stack with current nesting level (= current stack depth)
                    nesting_level = len(content_stack)
                    content_stack.append((parts[1], [], parts[0], nesting_level, i))
                    verbose(f"  Start {parts[0]} '{parts[1]}' at level {nesting_level} in {file_info.filename}")
                elif parts[0] == "end":
                    if content_stack:
                        # Get the last started item
                        name, block, item_type, level, start_line = content_stack.pop()
                        verbose(f"  End {item_type} '{name}' from level {level} in {file_info.filename}")
                        
                        # Store the item based on type
                        if item_type == "template":
                            templates[name] = block.copy()
                            verbose(f"  Stored template '{name}' with {len(block)} lines")
                        else:
                            snippets[name] = block.copy()
                        
                        # If there are still active items, add this entire block (including markers)
                        # to the parent's content
                        if content_stack:
                            parent_block = content_stack[-1][1]
                            # Add the start marker
                            start_line_content = file_info.data[start_line]
                            parent_block.append(start_line_content)
                            # Add all content
                            parent_block.extend(block)
                            # Add the end marker
                            parent_block.append(line)
                else:
                    # Add the line to all active blocks
                    for _, block, _, _, _ in content_stack:
                        block.append(line)
            else:
                # Add non-directive line to all active blocks
                for _, block, _, _, _ in content_stack:
                    block.append(line)

    # Then collect all globals
    for file_info in file_list:
        for i, line in enumerate(file_info.data):
            parts = parse_line(line)
            if parts and parts[0] == "global":
                defglob[parts[1]] = parse_value(parts) or True
                verbose(f"  Found global '{parts[1]}' in {file_info.filename}")

def process_snippets(file_list: List[FileInfo]) -> None:
    """Second pass: process each file, handling local snippet overrides and insertions"""
    verbose(f"Processing {Fore.GREEN}snippets{Style.RESET_ALL} in each file...")
    
    for file_info in file_list:
        # First find any local snippets in this file and track cut regions
        local_snippets: Dict[str, List[str]] = {}
        snippet_stack: List[Tuple[str, List[str], bool, int]] = []  # (name, block, is_cut, nesting_level)
        cut_ranges: List[Tuple[int, int]] = []  # Store start and end lines of cut regions
        nesting_level = 0
        
        # First pass to find cut regions and local snippets
        for i, line in enumerate(file_info.data):
            parts = parse_line(line)
            
            if parts:
                if parts[0] == "cut":
                    nesting_level += 1
                    snippet_stack.append((parts[1], [], True, nesting_level))
                elif parts[0] == "copy":
                    nesting_level += 1
                    snippet_stack.append((parts[1], [], False, nesting_level))
                elif parts[0] == "end":
                    # Find the most recent snippet at this nesting level
                    while snippet_stack and snippet_stack[-1][3] > nesting_level:
                        nesting_level -= 1
                        
                    if snippet_stack:
                        name, block, is_cut, _ = snippet_stack.pop()
                        local_snippets[name] = block.copy()
                        
                        if is_cut:
                            # For cut snippets, we need to track their range
                            cut_start = next((j for j, l in enumerate(file_info.data[0:i]) 
                                           if parse_line(l) and parse_line(l)[0] == "cut" 
                                           and parse_line(l)[1] == name), -1)
                            if cut_start >= 0:
                                cut_ranges.append((cut_start, i))
                        
                        # If this snippet is nested, add it to the parent's block
                        if snippet_stack:
                            snippet_stack[-1][1].append(line)
                else:
                    # Add line to all active snippet blocks
                    for _, block, _, _ in snippet_stack:
                        block.append(line)
            else:
                # Add line to all active snippet blocks
                for _, block, _, _ in snippet_stack:
                    block.append(line)
        
        # Now process the file, using local snippets where available and skipping cut regions
        new_file: List[str] = []
        for i, line in enumerate(file_info.data):
            # Check if this line is in a cut region
            is_in_cut = any(start <= i <= end for start, end in cut_ranges)
            if is_in_cut:
                continue
                
            parts = parse_line(line)
            
            if parts and parts[0] == "paste":
#                verbose(f"Processing paste of '{parts[1]}' in {file_info.filename}")
                # verbose(f"Available snippets: {', '.join(snippets.keys())}")
                # verbose(f"Available local snippets: {', '.join(local_snippets.keys())}")
                
                # First try local snippets, then fall back to global
                if parts[1] in local_snippets:
#                    verbose(f"Using local snippet '{parts[1]}'")
                    new_file.extend(local_snippets[parts[1]])
                elif parts[1] in snippets:
#                    verbose(f"Using global snippet '{parts[1]}'")
                    new_file.extend(snippets[parts[1]])
                else:
                    warning(f"Unable to {Fore.GREEN}insert {Fore.CYAN}{parts[1]}{Style.RESET_ALL} because snippet doesn't exist", file_info.filename, i + 1)
            else:
                new_file.append(line)
        
        file_info.data = new_file

def process_variables(file_list: List[FileInfo], output_dir: str) -> None:
    verbose("Writing files...")
    
    for file_info in file_list:
        write = True
        new_file: List[str] = []
        cutting = False
        
        for i, line in enumerate(file_info.data):
            parts = parse_line(line)
            
            if parts is not None:
                if parts[0] == "set":
                    file_info.def_vars[parts[1]] = parse_value(parts) or True
                elif parts[0] == "if":
                    if parts[1].startswith("!"):
                        var_name = parts[1][1:]
                        write = is_false(file_info.def_vars, var_name)
                    else:
                        write = is_true(file_info.def_vars, parts[1])
                elif parts[0] == "endif":
                    write = True
                elif parts[0] == "cut":
                    write = False
                    cutting = True
                elif cutting and parts[0] == "end":
                    write = True
                    cutting = False
            else:
                if write:
                    new_file.append(line)
        
        # Replace variables - all files are now HTML
        file_info.data = new_file
        
        # Check for template after all set commands have been processed
        template_name = file_info.def_vars.get('template') or file_info.meta_vars.get('template')
        
        if template_name and template_name in templates:
            # Use specified template
            verbose(f"  Using template '{template_name}' for {file_info.filename}")
            
            # Get the template content and process snippets in it
            template_lines = templates[template_name].copy()
            processed_template = []
            
            # Process snippets (paste commands) in the template
            for line in template_lines:
                parts = parse_line(line)
                if parts and parts[0] == "paste":
                    if parts[1] in snippets:
                        processed_template.extend(snippets[parts[1]])
                    else:
                        warning(f"Template references unknown snippet '{parts[1]}'")
                        processed_template.append(line)
                else:
                    processed_template.append(line)
            
            # Convert template to string
            template_content = '\n'.join(processed_template)
            
            # Replace {{content}} in template with the file content
            file_content = '\n'.join(file_info.data)
            template_with_content = template_content.replace('{{content}}', file_content)
            
            # Process conditionals and variables in the complete template
            content = process_conditionals_and_variables(template_with_content, file_info.def_vars, file_info.meta_vars)
        else:
            # Use default HTML wrapper or no wrapper
            verbose(f"  Processing file without template: {file_info.filename}")
            content_text = "\n".join(file_info.data)
            
            # Only wrap in HTML structure if no HTML tags present
            if not re.search(r'<html|<!DOCTYPE|<body', content_text, re.IGNORECASE):
                content_text = f"""<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
</head>
<body>
{content_text}
</body>
</html>"""
            
            content = do_replacements(content_text, file_info.def_vars, file_info.meta_vars)
        
        file_info.save(output_dir, content)

def process_index_commands(file_list: List[FileInfo], source_dir: str) -> None:
    """Process index commands to generate file listings"""
    verbose("Processing index commands...")
    
    for file_info in file_list:
        new_data = []
        
        for i, line in enumerate(file_info.data):
            parts = parse_line(line)
            
            if parts and parts[0] == "index":
                if len(parts) < 3:
                    warning(f"Index command requires at least pattern and template: <!-- index pattern template -->", file_info.filename, i + 1)
                    new_data.append(line)
                    continue
                
                pattern = parts[1]  # e.g., "blog/*.md"
                template_name = parts[2]  # e.g., "template_name"
                sort_field = parts[3] if len(parts) > 3 else None  # e.g., "date"
                
                verbose(f"  Processing index: pattern='{pattern}' template='{template_name}' sort='{sort_field}'")
                
                # Check if template exists
                if template_name not in templates:
                    warning(f"Index template '{template_name}' not found", file_info.filename, i + 1)
                    new_data.append(line)
                    continue
                
                # Find matching files
                matching_files = find_matching_files(pattern, source_dir)
                verbose(f"  Found {len(matching_files)} matching files")
                
                # Load metadata from matching files
                file_data = []
                for file_path in matching_files:
                    metadata = load_file_metadata(file_path, source_dir, source_dir)  # Using source_dir for both since we want relative paths
                    if metadata:
                        file_data.append(metadata)
                
                # Sort files if sort field is specified
                if sort_field and file_data:
                    file_data = sort_file_data(file_data, sort_field)
                
                # Generate HTML for each file using the template
                for file_meta in file_data:
                    index_html = process_index_template(templates[template_name], file_meta)
                    new_data.extend(index_html.splitlines())
                
            else:
                new_data.append(line)
        
        file_info.data = new_data

def find_matching_files(pattern: str, source_dir: str) -> List[str]:
    """Find files matching the glob pattern"""
    import glob
    
    # Convert pattern to absolute path
    full_pattern = os.path.join(source_dir, pattern)
    matches = glob.glob(full_pattern)
    
    # Filter to only include supported file types
    supported_extensions = ['.md', '.mdown', '.markdown', '.html', '.htm', '.txt']
    filtered_matches = []
    
    for match in matches:
        if any(match.lower().endswith(ext) for ext in supported_extensions):
            filtered_matches.append(match)
    
    return filtered_matches

def load_file_metadata(file_path: str, source_dir: str, output_dir: str) -> Optional[Dict[str, any]]:
    """Load metadata from a file (frontmatter for markdown, variables for others)"""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            content = f.read()
        
        # Calculate relative path from source to output
        rel_path = os.path.relpath(file_path, source_dir)
        
        # Convert to web path (change .md to .html, use forward slashes)
        web_path = rel_path.replace('\\', '/')  # Normalize path separators
        if any(web_path.lower().endswith(ext) for ext in ['.md', '.mdown', '.markdown']):
            # Change markdown extension to .html
            base = os.path.splitext(web_path)[0]
            web_path = f"{base}.html"
        
        metadata = {
            'filename': os.path.basename(file_path),
            'filepath': web_path,  # Web-relative path for links
            'sourcepath': file_path  # Keep original for reference
        }
        
        # Check if it's a markdown file
        if any(file_path.lower().endswith(ext) for ext in ['.md', '.mdown', '.markdown']):
            # Parse markdown frontmatter - ONLY add file-specific metadata
            frontmatter = parse_markdown_meta(content)
            metadata.update(frontmatter)
        
        # DO NOT add global variables here - let template processing handle fallback
        
        return metadata
        
    except OSError:
        warning(f"Cannot read file for indexing: {file_path}")
        return None

def sort_file_data(file_data: List[Dict[str, any]], sort_field: str) -> List[Dict[str, any]]:
    """Sort file data by the specified field"""
    def get_sort_key(file_meta):
        value = file_meta.get(sort_field, "")
        
        # Handle date sorting (most recent first)
        if sort_field.lower() in ['date', 'created', 'modified', 'published']:
            # Try to parse as date string, fallback to string comparison
            try:
                from datetime import datetime
                # Common date formats
                for fmt in ['%Y-%m-%d', '%Y/%m/%d', '%m/%d/%Y', '%d/%m/%Y']:
                    try:
                        return datetime.strptime(str(value), fmt)
                    except ValueError:
                        continue
                # If no format matches, use string comparison
                return str(value)
            except:
                return str(value)
        
        # Handle numeric sorting
        if isinstance(value, (int, float)):
            return value
        
        # Try to convert to number
        try:
            return float(str(value))
        except ValueError:
            # String sorting (case-insensitive)
            return str(value).lower()
    
    # Sort by date in descending order (most recent first), others ascending
    reverse = sort_field.lower() in ['date', 'created', 'modified', 'published']
    
    return sorted(file_data, key=get_sort_key, reverse=reverse)

def process_index_template(template_content: List[str], file_metadata: Dict[str, any]) -> str:
    """Process template for a single file in the index"""
    # Work with a fresh copy of the template
    template_lines = template_content.copy()
    processed_lines = []
    
    # Process any snippets in the template
    for line in template_lines:
        parts = parse_line(line)
        if parts and parts[0] == "paste":
            if parts[1] in snippets:
                processed_lines.extend(snippets[parts[1]])
            else:
                warning(f"Index template references unknown snippet '{parts[1]}'")
                processed_lines.append(line)
        else:
            processed_lines.append(line)
    
    # Join the processed template
    template_text = '\n'.join(processed_lines)
    
    # Replace variables ONLY with file metadata (not global/page variables)
    # Create a clean variable set with only this file's data and global fallbacks
    file_vars = {**defglob, **file_metadata}  # Global vars as fallback, file vars override
    
    for key, value in file_vars.items():
        if isinstance(value, list):
            replacement = ', '.join(str(v) for v in value)
        else:
            replacement = str(value)
        
        pattern = f"{VAR_START}{re.escape(key)}{VAR_END}"
        template_text = re.sub(pattern, replacement, template_text)
    
    # Clean up any remaining undefined variables
    template_text = re.sub(f"{VAR_START}[-\\w.]+{VAR_END}", "", template_text)
    
    return template_text

def is_true(local_vars: Dict[str, str], key: str) -> bool:
    if key not in local_vars:
        local_vars = defglob
    return key in local_vars and local_vars[key]

def is_false(local_vars: Dict[str, str], key: str) -> bool:
    if key not in local_vars:
        local_vars = defglob
    return key not in local_vars or not local_vars[key]

def process_conditionals_and_variables(text: str, local_vars: Dict[str, str], meta_vars: Dict[str, any] = None) -> str:
    """Process both conditionals and variable replacements in text"""
    lines = text.splitlines()
    processed_lines = []
    write = True
    cutting = False
    
    for line in lines:
        parts = parse_line(line)
        
        if parts is not None:
            if parts[0] == "if":
                if parts[1].startswith("!"):
                    var_name = parts[1][1:]
                    write = is_false(local_vars, var_name)
                else:
                    write = is_true(local_vars, parts[1])
            elif parts[0] == "endif":
                write = True
            elif parts[0] == "cut":
                write = False
                cutting = True
            elif cutting and parts[0] == "end":
                write = True
                cutting = False
            # Skip set commands in final output
            elif parts[0] == "set":
                continue
        else:
            if write:
                processed_lines.append(line)
    
    # Now do variable replacements on the processed text
    processed_text = '\n'.join(processed_lines)
    return do_replacements(processed_text, local_vars, meta_vars)

def do_replacements(text: str, local_vars: Dict[str, str], meta_vars: Dict[str, any] = None) -> str:
    # Create a dictionary with global, local, and metadata variables
    all_vars = {**defglob, **local_vars}
    if meta_vars:
        # Convert metadata values to strings and add them
        for key, value in meta_vars.items():
            if isinstance(value, list):
                all_vars[key] = ', '.join(str(v) for v in value)
            else:
                all_vars[key] = str(value)
    
    # Replace all variables
    for key, value in all_vars.items():
        pattern = f"{VAR_START}{re.escape(key)}{VAR_END}"
        text = re.sub(pattern, str(value), text)
    
    # Clean up any undefined variables
    text = re.sub(f"{VAR_START}[-\\w.]+{VAR_END}", "", text)
    
    return text

def build(source_dir: str, output_dir: str, watch_mode: bool) -> None:
    if watch_mode:
        print(f"{Fore.GREEN}{Style.BRIGHT}snip{Fore.CYAN}licity{Style.RESET_ALL} is watching files in {Fore.CYAN}{source_dir}{Style.RESET_ALL}\n")

    def do_build():
        verbose(f"Loading {Fore.GREEN}sniplicity{Style.RESET_ALL} files...")
        
        # Make sure output directory exists
        os.makedirs(output_dir, exist_ok=True)
        
        # Initialize file list
        file_list = []
        for rel_path, filename, is_markdown in get_file_list(source_dir):
            # Construct input path and create FileInfo
            input_path = os.path.join(source_dir, rel_path, filename)
            file_info = FileInfo(input_path, filename, is_markdown)
            
            # Set the relative path on FileInfo so we know where to save it
            file_info.output_rel_path = rel_path
            
            # Load and add to list if successful
            if file_info.load():
                file_list.append(file_info)
        
        # Reset global state
        global snippets, defglob, templates
        snippets, defglob, templates = {}, {}, {}
        
        # First pass: collect templates and other content before loading files
        verbose("Pre-loading files to collect templates...")
        temp_file_list = []
        for rel_path, filename, is_markdown in get_file_list(source_dir):
            input_path = os.path.join(source_dir, rel_path, filename)
            file_info = FileInfo(input_path, filename, is_markdown)
            file_info.output_rel_path = rel_path
            
            # Load but don't process markdown yet
            if file_info.load():
                temp_file_list.append(file_info)
        
        # Collect templates from all files first
        collect_snippets_and_globals(temp_file_list)
        
        # Now reload files with template processing
        verbose("Reloading files with template processing...")
        file_list = []
        for rel_path, filename, is_markdown in get_file_list(source_dir):
            input_path = os.path.join(source_dir, rel_path, filename)
            file_info = FileInfo(input_path, filename, is_markdown)
            file_info.output_rel_path = rel_path
            
            # Load and process (templates are now available)
            if file_info.load():
                file_list.append(file_info)
        
        # Process files in stages
        process_includes(file_list)
        process_index_commands(file_list, source_dir)  # Process indexes before snippets and variables
        process_snippets(file_list)
        process_variables(file_list, output_dir)
        
        # Success message
        print(f"{Fore.GREEN}{Style.BRIGHT}Compiled{Style.RESET_ALL} from {Fore.CYAN}{source_dir}{Style.RESET_ALL} to {Fore.CYAN}{output_dir}{Style.RESET_ALL}")
        if not watch_mode:
            print(f"{Fore.GREEN}{Style.BRIGHT}Success!{Style.RESET_ALL}")

    do_build()

    if watch_mode:
        class ChangeHandler(FileSystemEventHandler):
            DEBOUNCE_DELAY = 0.2
            FILE_PATTERN = re.compile(r'(html|htm|txt|md|mdown|markdown)$')
            
            def __init__(self):
                self.last_triggered = 0
            
            def should_rebuild(self, event) -> bool:
                if event.is_directory or not self.FILE_PATTERN.search(event.src_path):
                    return False
                
                current_time = time.time()
                if current_time - self.last_triggered < self.DEBOUNCE_DELAY:
                    return False
                    
                self.last_triggered = current_time
                return True
            
            def on_any_event(self, event):
                if self.should_rebuild(event):
                    do_build()

        observer = Observer()
        observer.schedule(ChangeHandler(), source_dir, recursive=True)
        observer.start()
        
        try:
            while True:
                time.sleep(1)
        except KeyboardInterrupt:
            print(f"\n{Fore.GREEN}Stopping file watcher...{Style.RESET_ALL}")
        finally:
            observer.stop()
            observer.join()
            print(f"{Fore.GREEN}Done!{Style.RESET_ALL}")

if __name__ == "__main__":
    main()