# Sniplicity Desktop - Quick Reference

## 🚀 Building the macOS App Bundle

### One-Command Build
```bash
./build_app.sh
```

### Manual Steps
```bash
# 1. Install build tools
pip3 install -r requirements-build.txt

# 2. Build app bundle
python3 setup.py py2app

# 3. Find your app in dist/
open "dist/Sniplicity Desktop.app"
```

### Install to Applications
```bash
mv "dist/Sniplicity Desktop.app" /Applications/
```

## 📱 Using the App

### Launch Methods
- **Spotlight**: Cmd+Space → "Sniplicity"
- **Finder**: Applications folder
- **Launchpad**: App icon

### First Time Setup
1. Select input directory (your Sniplicity source)
2. Select output directory (where built site goes)
3. Click "Build Once" or "Start Watching"

### Features
- ✅ GUI folder selection
- ✅ Auto-rebuild on file changes  
- ✅ Built-in web server (localhost:3000)
- ✅ Real-time activity log
- ✅ Settings persistence

## 🔧 File Structure

```
sniplicity_app/
├── app.py                    # Main GUI application
├── setup.py                  # App bundle configuration
├── build_app.sh             # Automated build script
├── create_icon.py           # Icon generator
├── requirements.txt         # Runtime dependencies
├── requirements-build.txt   # Build dependencies
├── core/
│   └── sniplicity.py       # Complete Sniplicity engine
└── dist/                   # Built app (after building)
    └── Sniplicity Desktop.app
```

## 🎯 Common Tasks

**Test development version**: `python3 app.py`  
**Build app bundle**: `./build_app.sh`  
**Install app**: Move from `dist/` to `/Applications/`  
**Update core**: Replace `core/sniplicity.py` and rebuild  

## 🐛 Troubleshooting

| Problem | Solution |
|---------|----------|
| Build fails | Install: `pip3 install -r requirements-build.txt` |
| "App damaged" | Right-click app → Open (bypass Gatekeeper) |
| App won't start | Check Console.app for Python errors |
| Port 3000 busy | Stop other web servers or change port in code |

## 📊 App Bundle Details

- **Size**: ~100-200 MB (includes Python + dependencies)
- **Platform**: macOS 10.15+ (Catalina)
- **Architecture**: Universal (depends on Python build)
- **Signing**: Unsigned (for personal use)
- **Distribution**: Manual (.app file or DMG)