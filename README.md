# Tcl/Tk Kanban Board

A full-featured Kanban board application built with Tcl/Tk that saves boards, swimlanes, lists, and cards to a SQLite database.

## Features

- 📋 **Multiple Boards**: Create and manage multiple Kanban boards
- 🏊 **Swimlanes**: Organize your workflow with swimlanes
- 📝 **Lists**: Create lists within each swimlane (Todo, In Progress, Done, etc.)
- 🎫 **Cards**: Add, edit, and manage cards with titles and descriptions
- 💾 **SQLite Storage**: All data persists to `wekan.db` SQLite database
- 🎨 **Clean GUI**: Intuitive Tcl/Tk interface with color-coded elements
- 📦 **Portable**: Can be built as a standalone .kit file

## Screenshot

The application provides:
- Sidebar with board navigation
- Swimlanes for organizing work horizontally
- Lists displayed in columns
- Cards with easy reordering via arrow controls (drag-and-drop style)
- Full CRUD operations for all entities

## Requirements

### Required
- **Tcl/Tk**: Version 8.5 or higher
- **SQLite3**: Tcl SQLite3 package

### Optional (for building .kit files)
- **sdx**: Starkit Developer eXtension

## Installation

### macOS
```bash
brew install tcl-tk
```

### Ubuntu/Debian
```bash
sudo apt-get install tcl tk libsqlite3-tcl
```

### Other Systems
Download from [https://www.tcl.tk/](https://www.tcl.tk/)

## Usage

### Using the build script (Recommended)

Make the build script executable:
```bash
chmod +x build.sh
```

Run the build script:
```bash
./build.sh
```

You'll see a menu with options:
1. **Run Kanban application** - Launch the app directly
2. **Build TclKit .kit file** - Create standalone executable
3. **Run .kit file** - Run the built .kit file
4. **Clean build artifacts** - Remove build files
5. **Check dependencies** - Verify all requirements are installed
6. **Exit** - Exit the script

### Running directly

```bash
tclsh kanban.tcl
# or
./kanban.tcl
```

### Create sample data (optional)

To populate the database with sample boards, swimlanes, lists, and cards:

```bash
./create_sample_data.tcl
```

This will create a "Sample Project" board with development and design teams, complete with sample cards.

## Database Schema

The application uses SQLite with the following schema:

### Tables

**boards**
- `id`: INTEGER PRIMARY KEY
- `name`: TEXT (board name)
- `description`: TEXT (board description)
- `created_at`: TIMESTAMP

**swimlanes**
- `id`: INTEGER PRIMARY KEY
- `board_id`: INTEGER (foreign key to boards)
- `name`: TEXT (swimlane name)
- `position`: INTEGER (display order)

**lists**
- `id`: INTEGER PRIMARY KEY
- `swimlane_id`: INTEGER (foreign key to swimlanes)
- `name`: TEXT (list name)
- `position`: INTEGER (display order)

**cards**
- `id`: INTEGER PRIMARY KEY
- `list_id`: INTEGER (foreign key to lists)
- `title`: TEXT (card title)
- `description`: TEXT (card details)
- `position`: INTEGER (display order)
- `created_at`: TIMESTAMP

## How to Use

1. **Create a Board**: Click "New Board" in the File menu or sidebar
2. **Add Swimlanes**: Click "+ Add Swimlane" button
3. **Create Lists**: Click "+ List" button in a swimlane header
4. **Add Cards**: Click "+ Add Card" button in a list
5. **Edit Cards**: Click "Edit" button on any card
6. **Delete Items**: Click the "×" button on boards, swimlanes, lists, or cards

### Reordering (Drag/Drop style)

You can quickly reorder items using arrow buttons:

- Swimlanes: use ▲ and ▼ in the swimlane header to move up/down
- Lists: use ◀ and ▶ in the list header to move left/right
- Cards: use ▲ and ▼ on each card to move up/down within a list

Note: Full mouse drag-and-drop is not implemented yet; these controls provide reliable, database-backed reordering.

## Building Standalone Executable

To create a standalone .kit file:

1. Install sdx (if not already installed)
2. Run the build script:
   ```bash
   ./build.sh
   ```
3. Select option 2 (Build TclKit .kit file)
4. The `kanban.kit` file will be created and can be distributed

## File Structure

```
TclTkKanban/
├── kanban.tcl          # Main application file
├── build.sh            # Build and run script
├── wekan.db            # SQLite database (created on first run)
├── README.md           # This file
└── kanban.vfs/         # VFS directory (created during build)
```

## Features in Detail

### Boards
- Create multiple boards for different projects
- Each board maintains its own swimlanes and cards
- Delete boards (cascades to all child elements)

### Swimlanes
- Horizontal organization within boards
- Useful for team members, priorities, or workflow stages
- Position-based ordering

### Lists
- Vertical columns within swimlanes
- Typically represent workflow stages (Todo, In Progress, Done)
- Scrollable card containers

### Cards
- Rich content with title and description
- Click to view full details
- Edit or delete capabilities
- Position tracking for ordering

## Data Persistence

All data is automatically saved to `wekan.db` using SQLite:
- No manual save required
- Data persists between sessions
- Database created automatically on first run
- Foreign key constraints ensure data integrity

## Keyboard Navigation

- Tab: Navigate between fields in dialogs
- Enter: Submit forms (when focused)
- Mouse: Click and interact with all elements

## Troubleshooting

### "can't find package sqlite3"
Install the SQLite3 Tcl package:
- macOS: `brew install tcl-tk`
- Linux: `apt-get install libsqlite3-tcl`

### "tclsh: command not found"
Install Tcl/Tk from your package manager or [tcl.tk](https://www.tcl.tk/)

### Database locked error
Close any other applications that might be accessing `wekan.db`

## Contributing

Feel free to contribute improvements:
1. Fork the repository
2. Make your changes
3. Test thoroughly
4. Submit a pull request

## License

This project is open source and available under the MIT License.

## Future Enhancements

Potential features for future versions:
- Mouse drag-and-drop for cards, lists, and swimlanes
- Card colors and labels
- Due dates and reminders
- Card attachments
- Search functionality
- Export/import capabilities
- Multi-user support
- Card history/activity log

## Credits

Built with:
- Tcl/Tk - GUI framework
- SQLite3 - Database engine

---

**Enjoy your Kanban workflow!** 🚀
