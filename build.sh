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

# Display banner
show_banner() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════╗"
    echo "║   Tcl/Tk Kanban Build & Run Script    ║"
    echo "╔════════════════════════════════════════╗"
    echo -e "${NC}"
}

# Display menu
show_menu() {
    echo -e "${GREEN}Please select an option:${NC}"
    echo "1) Run Kanban application"
    echo "2) Build TclKit .kit file"
    echo "3) Run .kit file (if built)"
    echo "4) Clean build artifacts"
    echo "5) Check dependencies"
    echo "7) Build Go XLSX exporter"
    echo "6) Exit"
    echo ""
    echo -n "Enter your choice [1-7]: "
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
        echo "  - macOS: brew install tcl-tk (includes sqlite3)"
        echo "  - Linux: apt-get install libsqlite3-tcl"
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
        echo "To install sdx:"
        echo "  1. Download from: https://github.com/aidanhs/starkit"
        echo "  2. Or install tclkit and sdx manually"
    fi
    
    echo ""
    
    if [ "$all_ok" = true ]; then
        echo -e "${GREEN}All required dependencies are installed!${NC}"
    else
        echo -e "${RED}Some dependencies are missing. Please install them first.${NC}"
    fi
}

# Run the application
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

# Build .kit file
build_kit() {
    echo -e "${BLUE}Building TclKit .kit file...${NC}"
    echo ""
    
    if ! command -v sdx &> /dev/null; then
        echo -e "${RED}Error: sdx is not installed${NC}"
        echo "sdx is required to build .kit files"
        echo ""
        echo "Alternative methods to create standalone executables:"
        echo "1. Use freewrap: https://freewrap.sourceforge.net/"
        echo "2. Use tclkit + sdx: https://wiki.tcl-lang.org/page/sdx"
        echo "3. Package as starpack/starkit"
        echo ""
        echo -e "${YELLOW}For now, you can run the application directly with: ./build.sh -> Option 1${NC}"
        return 1
    fi
    
    # Create VFS directory structure
    echo "Creating VFS structure..."
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
    
    # Wrap it
    echo "Wrapping with sdx..."
    sdx wrap kanban.kit -vfs kanban.vfs
    
    # Make it executable
    chmod +x kanban.kit
    
    echo ""
    echo -e "${GREEN}✓ Successfully built kanban.kit${NC}"
    echo "You can now run it with: ./kanban.kit"
}

# Run .kit file
run_kit() {
    echo -e "${BLUE}Running kanban.kit...${NC}"
    
    if [ ! -f "kanban.kit" ]; then
        echo -e "${RED}Error: kanban.kit not found!${NC}"
        echo "Please build it first using option 2"
        return 1
    fi
    
    ./kanban.kit
}

# Clean build artifacts
clean_build() {
    echo -e "${BLUE}Cleaning build artifacts...${NC}"
    
    if [ -d "kanban.vfs" ]; then
        rm -rf kanban.vfs
        echo "Removed kanban.vfs/"
    fi
    
    if [ -f "kanban.kit" ]; then
        rm -f kanban.kit
        echo "Removed kanban.kit"
    fi
    
    echo -e "${GREEN}✓ Clean complete${NC}"
}

# Build Go XLSX exporter binary
build_go_exporter() {
    echo -e "${BLUE}Building Go XLSX exporter...${NC}"
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
        echo "Please install Go from https://golang.org/dl/"
        return 1
    fi
    if [ ! -f "xlsx_exporter.go" ]; then
        echo -e "${RED}Error: xlsx_exporter.go not found!${NC}"
        return 1
    fi
    if [ ! -f "go.mod" ]; then
        echo -e "${YELLOW}Initializing Go module...${NC}"
        go mod init tcl-tk-kanban
    fi
    echo -e "${YELLOW}Ensuring Go dependencies...${NC}"
    go get github.com/mattn/go-sqlite3
    go get github.com/xuri/excelize/v2
    go build -o xlsx_exporter xlsx_exporter.go && echo -e "${GREEN}✓ Successfully built xlsx_exporter${NC}"
}

# Main menu loop
main() {
    show_banner
    
    while true; do
        echo ""
        show_menu
        read -r choice
        echo ""
        
        case $choice in
            1)
                run_app
                ;;
            2)
                build_kit
                ;;
            3)
                run_kit
                ;;
            4)
                clean_build
                ;;
            5)
                check_dependencies
                ;;
            7)
                build_go_exporter
                ;;
            6)
                echo -e "${GREEN}Goodbye!${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}Invalid option. Please choose 1-7.${NC}"
                ;;
        esac
    done
}

# Run main function
main
