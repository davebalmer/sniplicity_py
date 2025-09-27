# Sniplicity Desktop App

A macOS desktop application for Sniplicity static site generator with GUI interface, file watching, and built-in web server.

## Features

- **Simple GUI**: Easy folder selection for input and output directories
- **File Watching**: Automatically rebuilds when source files change
- **Built-in Web Server**: Serves your site locally on http://127.0.0.1:3000
- **Activity Log**: Real-time feedback on build process and file changes
- **Persistent Settings**: Remembers your folder selections between sessions
- **One-Click Building**: Build your site instantly without command line

## Installation

1. Install Python dependencies:
   ```bash
   pip3 install -r requirements.txt
   ```

2. Run the application:
   ```bash
   python3 app.py
   ```

## Usage

### Initial Setup
1. Launch the app with `python3 app.py`
2. Click "Browse..." next to "Input Directory" and select your Sniplicity source folder
3. Click "Browse..." next to "Output Directory" and select where you want the built site

### Building Your Site
- **Build Once**: Click "Build Once" to generate your site immediately
- **Watch Mode**: Click "Start Watching" to automatically rebuild when files change
- **View Site**: Click "Start Server" then "View Site" to see your site in a browser

### Web Server
- The built-in server runs on `127.0.0.1:3000`
- Serves files from your output directory
- Perfect for testing your site locally

## App Structure

```
sniplicity_app/
├── app.py              # Main desktop application
├── core/
│   └── sniplicity.py   # Core Sniplicity functionality
├── requirements.txt    # Python dependencies
├── README.md          # This file
└── config.json        # App settings (created automatically)
```

## Features Detail

### GUI Components
- **Folder Selection**: Browse buttons for easy directory selection
- **Control Buttons**: Start/stop watching, build once, server control
- **Status Bar**: Shows current operation status and server state
- **Activity Log**: Scrollable log with timestamps showing all app activity
- **Auto-Save**: Settings are automatically saved when changed

### Threading
- File watching runs in background thread
- Web server runs in separate thread
- GUI remains responsive during operations
- Safe shutdown handles all threads properly

### Error Handling
- Validates directories before operations
- Shows user-friendly error messages
- Handles port conflicts for web server
- Graceful handling of file system errors

## Configuration

The app automatically creates a `config.json` file to remember:
- Input directory path
- Output directory path

This file is saved in the same directory as the app and loaded on startup.

## Creating a macOS App Bundle

You can build Sniplicity Desktop as a standalone macOS application that runs without the command line:

### Quick Build
```bash
./build_app.sh
```

### Manual Build Process

1. **Install build dependencies:**
   ```bash
   pip3 install -r requirements-build.txt
   ```

2. **Create the app bundle:**
   ```bash
   python3 setup.py py2app
   ```

3. **Find your app:**
   The app bundle will be created in `dist/Sniplicity Desktop.app`

### Installing the App

1. **Test the app:**
   ```bash
   open "dist/Sniplicity Desktop.app"
   ```

2. **Install to Applications:**
   ```bash
   mv "dist/Sniplicity Desktop.app" /Applications/
   ```

3. **Launch like any Mac app:**
   - Use Spotlight: Press Cmd+Space, type "Sniplicity"
   - Use Finder: Go to Applications folder and double-click
   - Use Launchpad: Find the Sniplicity Desktop icon

### App Bundle Features

- **Standalone**: No need for command line or terminal
- **Native macOS**: Appears in Applications folder, Launchpad, and Dock
- **Self-contained**: All dependencies bundled inside the app
- **Proper integration**: Supports macOS features like dark mode
- **File associations**: Can handle Markdown and HTML files (optional)

### Build Files

- `setup.py`: py2app configuration for creating the app bundle
- `build_app.sh`: Automated build script with error checking
- `create_icon.py`: Generates a simple app icon (requires Pillow)
- `requirements-build.txt`: Dependencies needed for building

### Troubleshooting App Bundle

**Build fails with module errors**: Make sure all dependencies are installed with `pip3 install -r requirements-build.txt`

**App won't launch**: Check Console.app for error messages. Common issues:
- Missing Python modules (rebuild with correct dependencies)
- Code signing issues (ignore for personal use)

**App is very large**: This is normal - py2app bundles Python and all dependencies. Typical size is 100-200 MB.

**"App is damaged" error**: This is a Gatekeeper security warning. Right-click the app and select "Open" to bypass.

## Requirements

- Python 3.7 or higher
- macOS (tested) or other Unix-like systems with tkinter support
- All dependencies listed in requirements.txt
- For app bundle: py2app and build dependencies

## Core Sniplicity Integration

The app includes a complete copy of sniplicity.py in the `core/` directory, allowing you to:
- Maintain sniplicity functionality separately
- Update the core without affecting the GUI
- Use all standard Sniplicity features (templates, variables, includes, etc.)

## Troubleshooting

**Port 3000 already in use**: Another application is using port 3000. Stop that application or change the port in the code.

**Directory doesn't exist**: Make sure both input and output directories exist and are accessible.

**Build fails**: Check the activity log for specific error messages. Ensure your Sniplicity source files are valid.

**GUI not responsive**: The app uses threading to prevent GUI freezing. If it becomes unresponsive, close and restart the application.