# Tcl/Tk Kanban - Quick Start Guide

## Getting Started in 3 Steps

### 1. Install Dependencies

**macOS:**
```bash
brew install tcl-tk
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install tcl tk libsqlite3-tcl
```

### 2. Run the Application

```bash
./build.sh
```

Select option **1** to run the application.

### 3. Create Your First Board

When the app opens:
1. Click "New Board" or use the button in the sidebar
2. Enter a board name (e.g., "My Project")
3. Click "Create"

Then:
- Click "+ Add Swimlane" to create a swimlane
- Click "+ List" in the swimlane header to add lists
- Click "+ Add Card" in any list to create cards

## Quick Demo

Want to see it in action with sample data?

```bash
./create_sample_data.tcl
./build.sh
```

Then select option 1 to run and see a pre-populated sample board!

## Build Menu Options

When you run `./build.sh`, you'll see:

1. **Run Kanban application** ← Start here!
2. **Build TclKit .kit file** - Create standalone executable
3. **Run .kit file** - Run the built executable
4. **Clean build artifacts** - Remove build files
5. **Check dependencies** - Verify installation
6. **Exit**

## File Structure

```
TclTkKanban/
├── kanban.tcl                  # Main application
├── build.sh                    # Build & run script
├── create_sample_data.tcl      # Sample data generator
├── README.md                   # Full documentation
├── QUICKSTART.md               # This file
├── .gitignore                  # Git ignore rules
└── wekan.db                    # Database (created on first run)
```

## Features at a Glance

✅ Multiple boards  
✅ Swimlanes (horizontal organization)  
✅ Lists (vertical columns)  
✅ Cards with titles & descriptions  
✅ SQLite database (auto-saves everything)  
✅ Clean, intuitive GUI  
✅ Can build standalone .kit file  

## Common Commands

```bash
# Run the app
./build.sh                  # Interactive menu
./kanban.tcl                # Direct run

# Create sample data
./create_sample_data.tcl

# Check if everything is installed
./build.sh                  # Then select option 5
```

## Need Help?

- Full documentation: See `README.md`
- Check dependencies: Run `./build.sh` → Option 5
- Issues: Make sure Tcl/Tk and SQLite3 are installed

**Happy Kanbaning! 🚀**
