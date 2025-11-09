#!/bin/bash
# Build script for Tcl/Tk Kanban application

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# --- Utility Functions ---

# Display banner
show_banner() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════╗"
    echo "║   Tcl/Tk Kanban Build & Run Script     ║"
    echo "╚════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Display menu
show_menu() {
    echo -e "${GREEN}Please select an option:${NC}"
    echo "1) Check dependencies"
    echo "2) Build TclKit"
    echo "3) Build TclKit .kit file (Simple build)"
    echo "4) Build Go XLSX exporter (binary)"
    echo "5) Build Go XLSX exporter as .so and embed in .kit"
    echo "6) Build executable (macOS app bundle)"
    echo "7) Run Kanban application (Tcl interpreter)"
    echo "8) Run .kit file (if built)"
    echo "9) Clean build artifacts"
    echo "10) Exit"
    echo ""
    echo -n "Enter your choice [1-10]: "
}

# Check if tclsh is available
check_tclsh() {
    if ! command -v tclsh &> /dev/null; then
        echo -e "${RED}Error: tclsh is not installed or not in PATH${NC}"
        echo "Please install Tcl/Tk:"
        echo "  - macOS: brew install tcl-tk"
        echo "  - Linux: apt-get install tcl tk"
        echo "  - Or download from: https://www.tcl.tk/"
        return 1
    fi
    
    echo -e "${GREEN}✓ tclsh found: $(which tclsh)${NC}"
    tclsh --version 2>&1 || tclsh <<< 'puts [info patchlevel]'
    return 0
}

# Check if SQLite3 package is available
check_sqlite() {
    if tclsh << 'EOF' 2>&1 | grep -q "can't find package"; then
package require sqlite3
EOF
        echo -e "${RED}Error: SQLite3 Tcl package is not installed${NC}"
        echo "Please install it:"
        echo "  - macOS/Linux: Often included with tcl-tk or via libsqlite3-tcl package"
        return 1
    fi
    
    echo -e "${GREEN}✓ SQLite3 Tcl package found${NC}"
    return 0
}

# Check dependencies
check_dependencies() {
    echo -e "${BLUE}Checking dependencies...${NC}"
    echo ""
    
    local all_ok=true
    
    if ! check_tclsh; then
        all_ok=false
    fi
    
    echo ""
    
    if ! check_sqlite; then
        all_ok=false
    fi
    
    echo ""
    
    # Check for sdx (for building .kit files)
    if command -v sdx &> /dev/null; then
        echo -e "${GREEN}✓ sdx found: $(which sdx)${NC}"
    else
        echo -e "${YELLOW}⚠ sdx not found (optional, needed for building .kit files)${NC}"
        echo "To install sdx, get TclKit and sdx.kit from equi4.com or a Tclkit mirror."
    fi
    
    echo ""
    
    if [ "$all_ok" = true ]; then
        echo -e "${GREEN}All required dependencies are installed!${NC}"
    else
        echo -e "${RED}Some dependencies are missing. Please install them first.${NC}"
    fi
}

# Run the Tcl script directly
run_app() {
    echo -e "${BLUE}Starting Kanban application...${NC}"
    
    if [ ! -f "kanban.tcl" ]; then
        echo -e "${RED}Error: kanban.tcl not found!${NC}"
        return 1
    fi
    
    if ! check_tclsh > /dev/null 2>&1; then
        echo -e "${RED}Cannot run application - tclsh not available${NC}"
        return 1
    fi
    
    chmod +x kanban.tcl
    ./kanban.tcl
}

# Run the .kit file
run_kit() {
    echo -e "${BLUE}Starting Kanban application from kanban.kit...${NC}"

    if [ ! -f "kanban.kit" ]; then
        echo -e "${RED}Error: kanban.kit not found! Please build it first (Option 2 or 8).${NC}"
        return 1
    fi
    
    chmod +x kanban.kit
    ./kanban.kit
}

# Core function to create VFS structure and wrap it into kanban.kit
wrap_kit() {
    echo -e "${BLUE}Creating VFS structure and wrapping into kanban.kit...${NC}"
    mkdir -p kanban.vfs/lib/app-kanban
    
    # Copy main script
    cp kanban.tcl kanban.vfs/lib/app-kanban/
    
    # Create main.tcl wrapper
    cat > kanban.vfs/main.tcl << 'EOF'
#!/usr/bin/env tclsh
package require starkit
starkit::startup
source [file join $starkit::topdir lib app-kanban kanban.tcl]
EOF
    
    # Create pkgIndex.tcl
    cat > kanban.vfs/lib/app-kanban/pkgIndex.tcl << 'EOF'
package ifneeded app-kanban 1.0 [list source [file join $dir kanban.tcl]]
EOF
    
    TCLKIT="./tclkit"
    SDXKIT="./sdx.kit"
    
    # Check for wrapping tool
    if [ -x "$TCLKIT" ] && [ -f "$SDXKIT" ]; then
        echo -e "${GREEN}Using local tclkit and sdx.kit for wrapping.${NC}"
        WRAP_CMD="$TCLKIT $SDXKIT wrap kanban.kit -vfs kanban.vfs"
    elif command -v sdx &> /dev/null; then
        echo -e "${YELLOW}Using system sdx for wrapping.${NC}"
        WRAP_CMD="sdx wrap kanban.kit -vfs kanban.vfs"
    else
        echo -e "${RED}Error: TclKit and SDX not found!${NC}"
        echo "Please ensure 'sdx' is in your PATH or place 'tclkit' (executable) and 'sdx.kit' in your project directory."
        return 1
    fi
    
    echo "Wrapping with SDX..."
    eval $WRAP_CMD
    
    if [ $? -eq 0 ]; then
        chmod +x kanban.kit
        echo ""
        echo -e "${GREEN}✓ Successfully built kanban.kit${NC}"
        echo "You can now run it with: ./kanban.kit"
        return 0
    else
        echo -e "${RED}Error during SDX wrapping.${NC}"
        return 1
    fi
}

# Build .kit file (Simple version without Go embed)
build_kit() {
    echo -e "${BLUE}Building TclKit .kit file (no Go embed)...${NC}"
    
    if ! command -v sdx &> /dev/null && (! [ -x "./tclkit" ] || ! [ -f "./sdx.kit" ]); then
        echo -e "${RED}Error: sdx or local tclkit/sdx.kit is not installed/present${NC}"
        return 1
    fi
    
    wrap_kit
}

# Clean build artifacts
clean_build() {
    echo -e "${BLUE}Cleaning build artifacts...${NC}"
    
    rm -rf kanban.kit kanban.vfs xlsx_exporter xlsx_exporter_embed.so
    
    echo -e "${GREEN}✓ Build artifacts cleaned${NC}"
}

# Build Go XLSX exporter binary
build_go_binary() {
    echo -e "${BLUE}Building Go XLSX exporter binary...${NC}"
    
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
        return 1
    fi
    
    if [ ! -f "xlsx.go" ]; then
        echo -e "${RED}Error: xlsx.go not found!${NC}"
        return 1
    fi
    
    go build -o xlsx_exporter xlsx.go
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Go binary built: xlsx_exporter${NC}"
        return 0
    else
        echo -e "${RED}Error building Go binary${NC}"
        return 1
    fi
}

# Build Go XLSX exporter as .so and embed in .kit
build_go_so_embed() {
    echo -e "${BLUE}Building Go XLSX exporter as .so and embedding in .kit...${NC}"
    
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
        return 1
    fi
    
    if [ ! -f "xlsx_exporter_embed.go" ]; then
        echo -e "${RED}Error: xlsx_exporter_embed.go not found!${NC}"
        return 1
    fi
    
    go build -buildmode=c-shared -o xlsx_exporter_embed.so xlsx_exporter_embed.go
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error building Go .so${NC}"
        return 1
    fi
    
    # Create VFS with .so embedded
    mkdir -p kanban.vfs/lib/app-kanban
    cp kanban.tcl kanban.vfs/lib/app-kanban/
    cp xlsx_exporter_embed.so kanban.vfs/lib/app-kanban/
    
    # Create main.tcl wrapper
    cat > kanban.vfs/main.tcl << 'EOF'
#!/usr/bin/env tclsh
package require starkit
starkit::startup
source [file join $starkit::topdir lib app-kanban kanban.tcl]
EOF
    
    # Create pkgIndex.tcl
    cat > kanban.vfs/lib/app-kanban/pkgIndex.tcl << 'EOF'
package ifneeded app-kanban 1.0 [list source [file join $dir kanban.tcl]]
EOF
    
    wrap_kit
}

# Build single executable (tclkit + kanban.kit)
build_single_executable() {
    echo -e "${BLUE}Building single executable (tclkit + kanban.kit)...${NC}"
    
    if [ ! -x "./tclkit" ]; then
        echo -e "${RED}Error: tclkit not found or not executable${NC}"
        return 1
    fi
    
    if [ ! -f "kanban.kit" ]; then
        echo -e "${RED}Error: kanban.kit not found! Build it first (Option 2 or 8).${NC}"
        return 1
    fi
    
    if [ ! -f "./sdx.kit" ]; then
        echo -e "${RED}Error: sdx.kit not found!${NC}"
        return 1
    fi
    
    # Create macOS app bundle structure
    mkdir -p kanban.app/Contents/MacOS
    mkdir -p kanban.app/Contents/Resources
    
    # Wrap the executable into the app bundle
    ./tclkit ./sdx.kit wrap kanban.app/Contents/MacOS/kanban kanban.kit -runtime ./tclkit
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error wrapping executable${NC}"
        return 1
    fi
    
    # Create Info.plist for macOS app bundle
    cat > kanban.app/Contents/Info.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>kanban</string>
    <key>CFBundleIdentifier</key>
    <string>com.example.kanban</string>
    <key>CFBundleName</key>
    <string>Kanban</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.10</string>
</dict>
</plist>
EOF
    
    chmod +x kanban.app/Contents/MacOS/kanban
    
    echo -e "${GREEN}✓ macOS app bundle built: kanban.app${NC}"
    echo "You can run it with: open kanban.app"
    return 0
}

# Build TclKit for macOS
build_tclkit() {
    echo -e "${BLUE}Building TclKit for macOS...${NC}"
    
    mkdir -p build
    cd build
    
    # Download kitgen if not present
    if [ ! -f kitgen ]; then
        echo "Downloading kitgen..."
        curl -O http://kitgen.sourceforge.net/kitgen
        chmod +x kitgen
    fi
    
    # Download Tcl source
    if [ ! -f tcl8.6.13-src.tar.gz ]; then
        echo "Downloading Tcl 8.6.13 source..."
        curl -O https://prdownloads.sourceforge.net/tcl/tcl8.6.13-src.tar.gz
    fi
    
    # Download Tk source
    if [ ! -f tk8.6.13-src.tar.gz ]; then
        echo "Downloading Tk 8.6.13 source..."
        curl -O https://prdownloads.sourceforge.net/tcl/tk8.6.13-src.tar.gz
    fi
    
    # Build TclKit using kitgen
    echo "Building TclKit with kitgen..."
    ./kitgen tcl tcl8.6.13-src.tar.gz tk tk8.6.13-src.tar.gz
    
    if [ -f tclkit ]; then
        cp tclkit ../
        echo -e "${GREEN}✓ TclKit built and copied to project root${NC}"
        cd ..
        return 0
    else
        echo -e "${RED}Error: TclKit build failed${NC}"
        cd ..
        return 1
    fi
}

# --- Main Script ---

show_banner

while true; do
    show_menu
    read choice
    case $choice in
        1)
            check_dependencies
            ;;
        2)
            build_tclkit
            ;;
        3)
            build_kit
            ;;
        4)
            build_go_binary
            ;;
        5)
            build_go_so_embed
            ;;
        6)
            build_single_executable
            ;;
        7)
            run_app
            ;;
        8)
            run_kit
            ;;
        9)
            clean_build
            ;;
        10)
            echo -e "${GREEN}Exiting...${NC}"
            exit 0
            ;;
        *)
            echo -e "${RED}Invalid option. Please choose 1-10.${NC}"
            ;;
    esac
    echo ""
done