#!/usr/bin/env tclsh
# Tcl/Tk Kanban Board Application
# Saves boards, swimlanes, lists, and cards to SQLite database

package require Tk
package require sqlite3

# Global variables
set ::currentBoard ""
set ::currentSwimlane ""

# Tooltip helper procedure
proc addTooltip {widget text} {
    if {$text eq ""} return
    bind $widget <Enter> [list showTooltip %W $text %X %Y]
    bind $widget <Leave> [list hideTooltip %W]
    bind $widget <Motion> [list updateTooltip %W %X %Y]
}

proc showTooltip {widget text x y} {
    set ::tooltip(afterid) [after 500 [list displayTooltip $widget $text $x $y]]
}

proc displayTooltip {widget text x y} {
    if {![winfo exists $widget]} return
    catch {destroy .tooltip}
    toplevel .tooltip -bg black
    wm overrideredirect .tooltip 1
    label .tooltip.l -text $text -bg lightyellow -fg black -relief solid -borderwidth 1 -padx 3 -pady 1
    pack .tooltip.l
    wm geometry .tooltip +[expr {$x + 10}]+[expr {$y + 10}]
}

proc updateTooltip {widget x y} {
    if {[winfo exists .tooltip]} {
        wm geometry .tooltip +[expr {$x + 10}]+[expr {$y + 10}]
    }
}

proc hideTooltip {widget} {
    if {[info exists ::tooltip(afterid)]} {
        after cancel $::tooltip(afterid)
        unset ::tooltip(afterid)
    }
    catch {destroy .tooltip}
}

# Initialize database
proc initDatabase {} {
    sqlite3 db wekan.db
    
    # Create tables if they don't exist
    db eval {
        CREATE TABLE IF NOT EXISTS boards (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            description TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        
        CREATE TABLE IF NOT EXISTS swimlanes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            board_id INTEGER NOT NULL,
            name TEXT NOT NULL,
            position INTEGER DEFAULT 0,
            FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE
        );
        
        CREATE TABLE IF NOT EXISTS lists (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            swimlane_id INTEGER NOT NULL,
            name TEXT NOT NULL,
            position INTEGER DEFAULT 0,
            FOREIGN KEY (swimlane_id) REFERENCES swimlanes(id) ON DELETE CASCADE
        );
        
        CREATE TABLE IF NOT EXISTS cards (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            list_id INTEGER NOT NULL,
            title TEXT NOT NULL,
            description TEXT,
            position INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (list_id) REFERENCES lists(id) ON DELETE CASCADE
        );
    }
}

# Board operations
proc createBoard {name description} {
    db eval {INSERT INTO boards (name, description) VALUES ($name, $description)}
    refreshBoards
}

proc getBoards {} {
    set boards {}
    db eval {SELECT id, name, description FROM boards ORDER BY name} {
        lappend boards [list $id $name $description]
    }
    return $boards
}

proc deleteBoard {boardId} {
    db eval {DELETE FROM boards WHERE id = $boardId}
    refreshBoards
}

# Swimlane operations
proc createSwimlane {boardId name} {
    set maxPos [db eval {SELECT COALESCE(MAX(position), -1) FROM swimlanes WHERE board_id = $boardId}]
    set position [expr {$maxPos + 1}]
    db eval {INSERT INTO swimlanes (board_id, name, position) VALUES ($boardId, $name, $position)}
    refreshSwimlanes $boardId
}

proc getSwimlanes {boardId} {
    set swimlanes {}
    db eval {SELECT id, name, position FROM swimlanes WHERE board_id = $boardId ORDER BY position} {
        lappend swimlanes [list $id $name $position]
    }
    return $swimlanes
}

proc deleteSwimlane {swimlaneId} {
    set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $swimlaneId}]
    db eval {DELETE FROM swimlanes WHERE id = $swimlaneId}
    refreshSwimlanes $boardId
}

# List operations
proc createList {swimlaneId name} {
    set maxPos [db eval {SELECT COALESCE(MAX(position), -1) FROM lists WHERE swimlane_id = $swimlaneId}]
    set position [expr {$maxPos + 1}]
    db eval {INSERT INTO lists (swimlane_id, name, position) VALUES ($swimlaneId, $name, $position)}
    set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $swimlaneId}]
    refreshSwimlanes $boardId
}

proc getLists {swimlaneId} {
    set lists {}
    db eval {SELECT id, name, position FROM lists WHERE swimlane_id = $swimlaneId ORDER BY position} {
        lappend lists [list $id $name $position]
    }
    return $lists
}

proc deleteList {listId} {
    set swimlaneId [db eval {SELECT swimlane_id FROM lists WHERE id = $listId}]
    set boardId [db eval {SELECT s.board_id FROM swimlanes s JOIN lists l ON s.id = l.swimlane_id WHERE l.id = $listId}]
    db eval {DELETE FROM lists WHERE id = $listId}
    refreshSwimlanes $boardId
}

# Card operations
proc createCard {listId title description} {
    set maxPos [db eval {SELECT COALESCE(MAX(position), -1) FROM cards WHERE list_id = $listId}]
    set position [expr {$maxPos + 1}]
    db eval {INSERT INTO cards (list_id, title, description, position) VALUES ($listId, $title, $description, $position)}
    set boardId [db eval {
        SELECT s.board_id FROM swimlanes s 
        JOIN lists l ON s.id = l.swimlane_id 
        WHERE l.id = $listId
    }]
    refreshSwimlanes $boardId
}

proc getCards {listId} {
    set cards {}
    db eval {SELECT id, title, description, position FROM cards WHERE list_id = $listId ORDER BY position} {
        lappend cards [list $id $title $description $position]
    }
    return $cards
}

proc deleteCard {cardId} {
    set boardId [db eval {
        SELECT s.board_id FROM swimlanes s 
        JOIN lists l ON s.id = l.swimlane_id 
        JOIN cards c ON l.id = c.list_id 
        WHERE c.id = $cardId
    }]
    db eval {DELETE FROM cards WHERE id = $cardId}
    refreshSwimlanes $boardId
}

# Generic confirmation dialog before deletions.
# Usage: confirmDelete type id
# type: one of swimlane|list|card
proc confirmDelete {type id} {
    switch -- $type {
        board { set label "board" }
        swimlane { set label "swimlane" }
        list { set label "list" }
        card { set label "card" }
        default { set label "item" }
    }
    set answer [tk_messageBox -icon warning -type yesno -default no \
        -title "Confirm Delete" \
        -message "Are you sure you want to delete this $label? This cannot be undone." ]
    if {$answer eq "yes"} {
        switch -- $type {
            board { deleteBoard $id }
            swimlane { deleteSwimlane $id }
            list { deleteList $id }
            card { deleteCard $id }
        }
    }
}

proc updateCard {cardId title description} {
    db eval {UPDATE cards SET title = $title, description = $description WHERE id = $cardId}
    set boardId [db eval {
        SELECT s.board_id FROM swimlanes s 
        JOIN lists l ON s.id = l.swimlane_id 
        JOIN cards c ON l.id = c.list_id 
        WHERE c.id = $cardId
    }]
    refreshSwimlanes $boardId
}

# --- Drag/Drop (reorder) movement functions ---
# Card up/down within its list
proc moveCardUp {cardId} {
    set listId [db eval {SELECT list_id FROM cards WHERE id = $cardId}]
    set currentPos [db eval {SELECT position FROM cards WHERE id = $cardId}]
    if {$currentPos > 0} {
        set newPos [expr {$currentPos - 1}]
        set targetCard [db eval {SELECT id FROM cards WHERE list_id = $listId AND position = $newPos LIMIT 1}]
        if {$targetCard ne ""} {
            db eval {
                UPDATE cards SET position = $currentPos WHERE id = $targetCard;
                UPDATE cards SET position = $newPos WHERE id = $cardId;
            }
            set boardId [db eval {
                SELECT s.board_id FROM swimlanes s 
                JOIN lists l ON s.id = l.swimlane_id 
                WHERE l.id = $listId
            }]
            refreshSwimlanes $boardId
        }
    }
}

proc moveCardDown {cardId} {
    set listId [db eval {SELECT list_id FROM cards WHERE id = $cardId}]
    set currentPos [db eval {SELECT position FROM cards WHERE id = $cardId}]
    set maxPos [db eval {SELECT MAX(position) FROM cards WHERE list_id = $listId}]
    if {$currentPos < $maxPos} {
        set newPos [expr {$currentPos + 1}]
        set targetCard [db eval {SELECT id FROM cards WHERE list_id = $listId AND position = $newPos LIMIT 1}]
        if {$targetCard ne ""} {
            db eval {
                UPDATE cards SET position = $currentPos WHERE id = $targetCard;
                UPDATE cards SET position = $newPos WHERE id = $cardId;
            }
            set boardId [db eval {
                SELECT s.board_id FROM swimlanes s 
                JOIN lists l ON s.id = l.swimlane_id 
                WHERE l.id = $listId
            }]
            refreshSwimlanes $boardId
        }
    }
}

# List left/right within its swimlane
proc moveListLeft {listId} {
    set currentPos [db eval {SELECT position FROM lists WHERE id = $listId}]
    if {$currentPos > 0} {
        set newPos [expr {$currentPos - 1}]
        set swimlaneId [db eval {SELECT swimlane_id FROM lists WHERE id = $listId}]
        set targetList [db eval {SELECT id FROM lists WHERE swimlane_id = $swimlaneId AND position = $newPos LIMIT 1}]
        if {$targetList ne ""} {
            db eval {
                UPDATE lists SET position = $currentPos WHERE id = $targetList;
                UPDATE lists SET position = $newPos WHERE id = $listId;
            }
            set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $swimlaneId}]
            refreshSwimlanes $boardId
        }
    }
}

proc moveListRight {listId} {
    set swimlaneId [db eval {SELECT swimlane_id FROM lists WHERE id = $listId}]
    set currentPos [db eval {SELECT position FROM lists WHERE id = $listId}]
    set maxPos [db eval {SELECT MAX(position) FROM lists WHERE swimlane_id = $swimlaneId}]
    if {$currentPos < $maxPos} {
        set newPos [expr {$currentPos + 1}]
        set targetList [db eval {SELECT id FROM lists WHERE swimlane_id = $swimlaneId AND position = $newPos LIMIT 1}]
        if {$targetList ne ""} {
            db eval {
                UPDATE lists SET position = $currentPos WHERE id = $targetList;
                UPDATE lists SET position = $newPos WHERE id = $listId;
            }
            set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $swimlaneId}]
            refreshSwimlanes $boardId
        }
    }
}

# Move list to the swimlane below (next swimlane by position) and append at end
proc moveListToBelowSwimlane {listId} {
    set srcSwimlaneId [db eval {SELECT swimlane_id FROM lists WHERE id = $listId}]
    set srcPos [db eval {SELECT position FROM lists WHERE id = $listId}]
    set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $srcSwimlaneId}]
    set swPos [db eval {SELECT position FROM swimlanes WHERE id = $srcSwimlaneId}]
    set nextSwimlaneId [db eval {SELECT id FROM swimlanes WHERE board_id = $boardId AND position = $swPos + 1 LIMIT 1}]
    if {$nextSwimlaneId eq ""} { return }
    # Compact positions in source swimlane after removing this list
    db eval {
        UPDATE lists SET position = position - 1
        WHERE swimlane_id = $srcSwimlaneId AND position > $srcPos;
    }
    # Append to end of target swimlane
    set targetPos [db eval {SELECT COALESCE(MAX(position), -1) FROM lists WHERE swimlane_id = $nextSwimlaneId}]
    set newPos [expr {$targetPos + 1}]
    db eval {UPDATE lists SET swimlane_id = $nextSwimlaneId, position = $newPos WHERE id = $listId}
    refreshSwimlanes $boardId
}

# Move list to the swimlane above (previous swimlane by position) and append at end
proc moveListToAboveSwimlane {listId} {
    set srcSwimlaneId [db eval {SELECT swimlane_id FROM lists WHERE id = $listId}]
    set srcPos [db eval {SELECT position FROM lists WHERE id = $listId}]
    set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $srcSwimlaneId}]
    set swPos [db eval {SELECT position FROM swimlanes WHERE id = $srcSwimlaneId}]
    if {$swPos <= 0} { return }
    set prevSwimlaneId [db eval {SELECT id FROM swimlanes WHERE board_id = $boardId AND position = $swPos - 1 LIMIT 1}]
    if {$prevSwimlaneId eq ""} { return }
    db eval {
        UPDATE lists SET position = position - 1
        WHERE swimlane_id = $srcSwimlaneId AND position > $srcPos;
    }
    set targetPos [db eval {SELECT COALESCE(MAX(position), -1) FROM lists WHERE swimlane_id = $prevSwimlaneId}]
    set newPos [expr {$targetPos + 1}]
    db eval {UPDATE lists SET swimlane_id = $prevSwimlaneId, position = $newPos WHERE id = $listId}
    refreshSwimlanes $boardId
}

# Swimlane up/down on the board
proc moveSwimlaneUp {swimlaneId} {
    set currentPos [db eval {SELECT position FROM swimlanes WHERE id = $swimlaneId}]
    if {$currentPos > 0} {
        set newPos [expr {$currentPos - 1}]
        set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $swimlaneId}]
        set targetSwimlane [db eval {SELECT id FROM swimlanes WHERE board_id = $boardId AND position = $newPos LIMIT 1}]
        if {$targetSwimlane ne ""} {
            db eval {
                UPDATE swimlanes SET position = $currentPos WHERE id = $targetSwimlane;
                UPDATE swimlanes SET position = $newPos WHERE id = $swimlaneId;
            }
            refreshSwimlanes $boardId
        }
    }
}

proc moveSwimlaneDown {swimlaneId} {
    set boardId [db eval {SELECT board_id FROM swimlanes WHERE id = $swimlaneId}]
    set currentPos [db eval {SELECT position FROM swimlanes WHERE id = $swimlaneId}]
    set maxPos [db eval {SELECT MAX(position) FROM swimlanes WHERE board_id = $boardId}]
    if {$currentPos < $maxPos} {
        set newPos [expr {$currentPos + 1}]
        set targetSwimlane [db eval {SELECT id FROM swimlanes WHERE board_id = $boardId AND position = $newPos LIMIT 1}]
        if {$targetSwimlane ne ""} {
            db eval {
                UPDATE swimlanes SET position = $currentPos WHERE id = $targetSwimlane;
                UPDATE swimlanes SET position = $newPos WHERE id = $swimlaneId;
            }
            refreshSwimlanes $boardId
        }
    }
}

# GUI Functions
proc createMainWindow {} {
    wm title . "Tcl/Tk Kanban Board"
    wm geometry . 1200x800
    
    # Create menu bar
    menu .menubar
    . configure -menu .menubar
    
    menu .menubar.file
    .menubar add cascade -label "File" -menu .menubar.file
    .menubar.file add command -label "New Board" -command showNewBoardDialog
    .menubar.file add separator
    .menubar.file add command -label "Exit" -command exit
    
    # Create main frame with paned window
    ttk::panedwindow .paned -orient horizontal
    pack .paned -fill both -expand 1
    
    # Left sidebar for boards
    frame .sidebar -width 200 -bg #f0f0f0
    .paned add .sidebar -weight 0
    
    label .sidebar.title -text "Boards" -font {-size 14 -weight bold} -bg #f0f0f0
    pack .sidebar.title -pady 10
    
    frame .sidebar.boardsframe -bg #f0f0f0
    pack .sidebar.boardsframe -fill both -expand 1 -padx 5
    
    # Main content area
    frame .content -bg white
    .paned add .content -weight 1
    
    # Canvas with scrollbar for swimlanes
    canvas .content.canvas -bg white -yscrollcommand ".content.scroll set"
    scrollbar .content.scroll -command ".content.canvas yview"
    
    pack .content.scroll -side right -fill y
    pack .content.canvas -side left -fill both -expand 1
    
    frame .content.canvas.frame -bg white
    .content.canvas create window 0 0 -anchor nw -window .content.canvas.frame
    
    bind .content.canvas.frame <Configure> {
        .content.canvas configure -scrollregion [.content.canvas bbox all]
    }
    
    refreshBoards
}

proc refreshBoards {} {
    # Clear existing board buttons
    foreach child [winfo children .sidebar.boardsframe] {
        destroy $child
    }
    
    # Add "New Board" button at the top
    button .sidebar.boardsframe.new -text "+ New Board" -command showNewBoardDialog \
        -bg #e8f5e9 -fg #2e7d32 -activebackground #c8e6c9 -activeforeground #1b5e20 \
        -relief raised -borderwidth 1 -highlightthickness 0 -font {-weight bold}
    pack .sidebar.boardsframe.new -fill x -pady 10
    addTooltip .sidebar.boardsframe.new "Create a new board"
    
    # Add boards
    set boards [getBoards]
    foreach board $boards {
        lassign $board id name description
        frame .sidebar.boardsframe.b$id -bg white -relief raised -borderwidth 1
        pack .sidebar.boardsframe.b$id -fill x -pady 2
        
        button .sidebar.boardsframe.b$id.btn -text $name -command [list selectBoard $id $name] \
            -bg white -activebackground #e0e0e0 -relief flat -anchor w
        pack .sidebar.boardsframe.b$id.btn -side left -fill x -expand 1
        addTooltip .sidebar.boardsframe.b$id.btn "Select board: $name"
        
        button .sidebar.boardsframe.b$id.del -text "×" -command [list confirmDelete board $id] \
            -bg white -fg red -activebackground #ffcccc -relief flat -width 2
        pack .sidebar.boardsframe.b$id.del -side right
        addTooltip .sidebar.boardsframe.b$id.del "Delete board"
    }
}

proc selectBoard {boardId boardName} {
    set ::currentBoard $boardId
    wm title . "Tcl/Tk Kanban Board - $boardName"
    refreshSwimlanes $boardId
}

proc refreshSwimlanes {boardId} {
    # Clear canvas
    foreach child [winfo children .content.canvas.frame] {
        destroy $child
    }
    
    set swimlanes [getSwimlanes $boardId]
    set row 0

    # Board-level actions (above swimlanes)
    frame .content.canvas.frame.boardactions -bg white
    grid .content.canvas.frame.boardactions -row $row -column 0 -sticky ew -padx 5 -pady 5
    button .content.canvas.frame.boardactions.delete -text "Delete Board" \
        -command [list confirmDelete board $boardId] -bg #ffebee -fg #c62828 \
        -activebackground #ff5252 -activeforeground white -relief raised \
        -borderwidth 1 -highlightthickness 0 -font {-weight bold}
    pack .content.canvas.frame.boardactions.delete -side left
    addTooltip .content.canvas.frame.boardactions.delete "Delete this board"
    incr row
    
    foreach swimlane $swimlanes {
        lassign $swimlane swimlaneId swimlaneName position
        
        # Swimlane frame
        frame .content.canvas.frame.sw$swimlaneId -bg #f5f5f5 -relief raised -borderwidth 2
        grid .content.canvas.frame.sw$swimlaneId -row $row -column 0 -sticky ew -padx 5 -pady 5
        
        # Swimlane header
        frame .content.canvas.frame.sw$swimlaneId.header -bg #2196F3
        pack .content.canvas.frame.sw$swimlaneId.header -fill x
        
        label .content.canvas.frame.sw$swimlaneId.header.title -text $swimlaneName \
            -bg #2196F3 -fg white -font {-size 12 -weight bold} -anchor w
        pack .content.canvas.frame.sw$swimlaneId.header.title -side left -padx 10 -pady 5

        # Drag handle and move buttons for swimlane
        button .content.canvas.frame.sw$swimlaneId.header.moveup -text "▲" \
            -command [list moveSwimlaneUp $swimlaneId] -bg #e3f2fd -fg #1976D2 \
            -relief raised -borderwidth 1 -width 2 -font {-size 8}
        pack .content.canvas.frame.sw$swimlaneId.header.moveup -side left -padx 1
        addTooltip .content.canvas.frame.sw$swimlaneId.header.moveup "Move swimlane up"

        button .content.canvas.frame.sw$swimlaneId.header.movedown -text "▼" \
            -command [list moveSwimlaneDown $swimlaneId] -bg #e3f2fd -fg #1976D2 \
            -relief raised -borderwidth 1 -width 2 -font {-size 8}
        pack .content.canvas.frame.sw$swimlaneId.header.movedown -side left -padx 1
        addTooltip .content.canvas.frame.sw$swimlaneId.header.movedown "Move swimlane down"

        button .content.canvas.frame.sw$swimlaneId.header.addlist -text "+ List" \
            -command [list showNewListDialog $swimlaneId] -bg #e8f5e9 -fg #2e7d32 \
            -activebackground #c8e6c9 -activeforeground #1b5e20 -relief raised \
            -borderwidth 1 -highlightthickness 0 -font {-weight bold}
        # Place "+ List" to the right of the swimlane move buttons (among left-packed controls)
        pack .content.canvas.frame.sw$swimlaneId.header.addlist -side left -padx 5
        addTooltip .content.canvas.frame.sw$swimlaneId.header.addlist "Add a new list to this swimlane"
        
        button .content.canvas.frame.sw$swimlaneId.header.del -text "Delete" \
            -command [list confirmDelete swimlane $swimlaneId] -bg #ffebee -fg #c62828 \
            -activebackground #ff5252 -activeforeground white -relief raised \
            -borderwidth 1 -font {-weight bold}
        pack .content.canvas.frame.sw$swimlaneId.header.del -side right
        addTooltip .content.canvas.frame.sw$swimlaneId.header.del "Delete this swimlane"
        
        # Lists container
        frame .content.canvas.frame.sw$swimlaneId.lists -bg #f5f5f5
        pack .content.canvas.frame.sw$swimlaneId.lists -fill both -expand 1 -padx 5 -pady 5
        
        set lists [getLists $swimlaneId]
        set col 0
        
        foreach list $lists {
            lassign $list listId listName listPosition
            
            # List frame
            frame .content.canvas.frame.sw$swimlaneId.lists.l$listId -bg white \
                -relief raised -borderwidth 1 -width 250
            grid .content.canvas.frame.sw$swimlaneId.lists.l$listId -row 0 -column $col \
                -sticky ns -padx 5 -pady 5
            grid columnconfigure .content.canvas.frame.sw$swimlaneId.lists $col -weight 0
            
            # List header
            frame .content.canvas.frame.sw$swimlaneId.lists.l$listId.header -bg #e0e0e0
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.header -fill x
            
            label .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.title \
                -text $listName -bg #e0e0e0 -font {-weight bold} -anchor w
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.title \
                -side left -padx 5 -pady 3

            # Drag handle and move buttons for list. To keep Delete at far right, pack it first with -side right.
            button .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.del -text "Delete" \
                -command [list confirmDelete list $listId] -bg #e0e0e0 -fg red \
                -activebackground #ffcccc -relief flat -width 2
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.del -side right -padx 1
            addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.del "Delete this list"

            # Move list to below/above swimlane (between Delete and other controls)
            button .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.movebelow -text "▼" \
                -command [list moveListToBelowSwimlane $listId] -bg #f5f5f5 -fg #424242 \
                -relief raised -borderwidth 1 -width 2 -font {-size 8}
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.movebelow -side right -padx 1
            addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.movebelow "Move list to below swimlane"

            button .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveabove -text "▲" \
                -command [list moveListToAboveSwimlane $listId] -bg #f5f5f5 -fg #424242 \
                -relief raised -borderwidth 1 -width 2 -font {-size 8}
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveabove -side right -padx 1
            addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveabove "Move list to above swimlane"

            button .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveright -text "▶" \
                -command [list moveListRight $listId] -bg #f5f5f5 -fg #424242 \
                -relief raised -borderwidth 1 -width 2 -font {-size 8}
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveright -side right -padx 1
            addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveright "Move list to the right"

            button .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveleft -text "◀" \
                -command [list moveListLeft $listId] -bg #f5f5f5 -fg #424242 \
                -relief raised -borderwidth 1 -width 2 -font {-size 8}
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveleft -side right -padx 1
            addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.header.moveleft "Move list to the left"
            
            # Cards container with scrollbar
            frame .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer \
                -fill both -expand 1
            
            canvas .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas \
                -bg white -height 400 -yscrollcommand \
                ".content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.scroll set"
            scrollbar .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.scroll \
                -command ".content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas yview"
            
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.scroll \
                -side right -fill y
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas \
                -side left -fill both -expand 1
            
            frame .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame \
                -bg white
            .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas create window \
                0 0 -anchor nw -window \
                .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame
            
            bind .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame \
                <Configure> [list updateScrollRegion $swimlaneId $listId]
            
            # Add cards
            set cards [getCards $listId]
            foreach card $cards {
                lassign $card cardId cardTitle cardDescription cardPosition
                
                frame .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId \
                    -bg #fafafa -relief raised -borderwidth 1
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId \
                    -fill x -padx 5 -pady 3
                
                # Card drag handle frame (top-right)
                frame .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe \
                    -bg #fafafa
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe \
                    -fill x -padx 2 -pady 2

                button .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe.up \
                    -text "▲" -command [list moveCardUp $cardId] -bg #fafafa -fg #666666 \
                    -relief flat -font {-size 8} -width 2
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe.up \
                    -side right -padx 1
                addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe.up "Move card up"

                button .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe.down \
                    -text "▼" -command [list moveCardDown $cardId] -bg #fafafa -fg #666666 \
                    -relief flat -font {-size 8} -width 2
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe.down \
                    -side right -padx 1
                addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.dragframe.down "Move card down"

                label .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.title \
                    -text $cardTitle -bg #fafafa -font {-weight bold} -anchor w -wraplength 220
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.title \
                    -fill x -padx 5 -pady 2
                
                if {$cardDescription ne ""} {
                    label .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.desc \
                        -text $cardDescription -bg #fafafa -anchor w -wraplength 220 -font {-size 9}
                    pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.desc \
                        -fill x -padx 5 -pady 2
                }
                
                frame .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons \
                    -bg #fafafa
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons \
                    -fill x
                
                button .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons.edit \
                    -text "Edit" -command [list showEditCardDialog $cardId $listId] \
                    -bg #fafafa -relief flat -font {-size 8}
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons.edit \
                    -side left -padx 2
                addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons.edit "Edit card"
                
                button .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons.del \
                    -text "Delete" -command [list confirmDelete card $cardId] -fg red \
                    -bg #fafafa -relief flat -font {-size 8}
                pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons.del \
                    -side left -padx 2
                addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId.buttons.del "Delete card"
                
                bind .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.c$cardId \
                    <Button-1> [list showCardDetails $cardId]
            }
            
            # Add card button
            button .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.addcard \
                -text "+ Add Card" -command [list showNewCardDialog $listId] \
                -bg #e8f5e9 -fg #2e7d32 -activebackground #c8e6c9 -activeforeground #1b5e20 \
                -relief raised -borderwidth 1 -highlightthickness 0 -font {-weight bold}
            pack .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.addcard \
                -fill x -padx 5 -pady 5
            addTooltip .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas.frame.addcard "Add a new card to this list"
            
            incr col
        }
        
        incr row
    }
    
    # Add swimlane button
    button .content.canvas.frame.addswimlane -text "+ Add Swimlane" \
        -command [list showNewSwimlaneDialog $boardId] \
        -bg #e8f5e9 -fg #2e7d32 -activebackground #c8e6c9 -activeforeground #1b5e20 \
        -relief raised -borderwidth 1 -highlightthickness 0 -font {-weight bold}
    grid .content.canvas.frame.addswimlane -row $row -column 0 -sticky ew -padx 5 -pady 10
    addTooltip .content.canvas.frame.addswimlane "Add a new swimlane to this board"
    
    grid columnconfigure .content.canvas.frame 0 -weight 1
}

proc updateScrollRegion {swimlaneId listId} {
    set canvas .content.canvas.frame.sw$swimlaneId.lists.l$listId.cardscontainer.canvas
    $canvas configure -scrollregion [$canvas bbox all]
}

# Dialog functions
proc showNewBoardDialog {} {
    toplevel .newboard
    wm title .newboard "Create New Board"
    wm geometry .newboard 400x200
    
    label .newboard.namelbl -text "Board Name:"
    entry .newboard.name -width 40
    pack .newboard.namelbl -pady 5
    pack .newboard.name -pady 5
    
    label .newboard.desclbl -text "Description:"
    entry .newboard.desc -width 40
    pack .newboard.desclbl -pady 5
    pack .newboard.desc -pady 5
    
    frame .newboard.buttons
    pack .newboard.buttons -pady 20
    
    button .newboard.buttons.create -text "Create" -command {
        set name [.newboard.name get]
        set desc [.newboard.desc get]
        if {$name ne ""} {
            createBoard $name $desc
            destroy .newboard
        }
    } -bg #4CAF50 -fg black -activebackground #45a049 -activeforeground black
    pack .newboard.buttons.create -side left -padx 5
    
    button .newboard.buttons.cancel -text "Cancel" -command {destroy .newboard}
    pack .newboard.buttons.cancel -side left -padx 5
    
    focus .newboard.name
}

proc showNewSwimlaneDialog {boardId} {
    toplevel .newswimlane
    wm title .newswimlane "Create New Swimlane"
    wm geometry .newswimlane 400x150
    
    label .newswimlane.namelbl -text "Swimlane Name:"
    entry .newswimlane.name -width 40
    pack .newswimlane.namelbl -pady 5
    pack .newswimlane.name -pady 5
    
    frame .newswimlane.buttons
    pack .newswimlane.buttons -pady 20
    
    button .newswimlane.buttons.create -text "Create" -command [list createSwimlaneFromDialog $boardId] \
        -bg #4CAF50 -fg black -activebackground #45a049 -activeforeground black
    pack .newswimlane.buttons.create -side left -padx 5
    
    button .newswimlane.buttons.cancel -text "Cancel" -command {destroy .newswimlane}
    pack .newswimlane.buttons.cancel -side left -padx 5
    
    focus .newswimlane.name
}

proc createSwimlaneFromDialog {boardId} {
    set name [.newswimlane.name get]
    if {$name ne ""} {
        createSwimlane $boardId $name
        destroy .newswimlane
    }
}

proc showNewListDialog {swimlaneId} {
    toplevel .newlist
    wm title .newlist "Create New List"
    wm geometry .newlist 400x150
    
    label .newlist.namelbl -text "List Name:"
    entry .newlist.name -width 40
    pack .newlist.namelbl -pady 5
    pack .newlist.name -pady 5
    
    frame .newlist.buttons
    pack .newlist.buttons -pady 20
    
    button .newlist.buttons.create -text "Create" -command [list createListFromDialog $swimlaneId] \
        -bg #4CAF50 -fg black -activebackground #45a049 -activeforeground black
    pack .newlist.buttons.create -side left -padx 5
    
    button .newlist.buttons.cancel -text "Cancel" -command {destroy .newlist}
    pack .newlist.buttons.cancel -side left -padx 5
    
    focus .newlist.name
}

proc createListFromDialog {swimlaneId} {
    set name [.newlist.name get]
    if {$name ne ""} {
        createList $swimlaneId $name
        destroy .newlist
    }
}

proc showNewCardDialog {listId} {
    toplevel .newcard
    wm title .newcard "Create New Card"
    wm geometry .newcard 400x250
    
    label .newcard.titlelbl -text "Card Title:"
    entry .newcard.title -width 40
    pack .newcard.titlelbl -pady 5
    pack .newcard.title -pady 5
    
    label .newcard.desclbl -text "Description:"
    text .newcard.desc -width 40 -height 5
    pack .newcard.desclbl -pady 5
    pack .newcard.desc -pady 5
    
    frame .newcard.buttons
    pack .newcard.buttons -pady 20
    
    button .newcard.buttons.create -text "Create" -command [list createCardFromDialog $listId] \
        -bg #4CAF50 -fg black -activebackground #45a049 -activeforeground black
    pack .newcard.buttons.create -side left -padx 5
    
    button .newcard.buttons.cancel -text "Cancel" -command {destroy .newcard}
    pack .newcard.buttons.cancel -side left -padx 5
    
    focus .newcard.title
}

proc createCardFromDialog {listId} {
    set title [.newcard.title get]
    set description [.newcard.desc get 1.0 end-1c]
    if {$title ne ""} {
        createCard $listId $title $description
        destroy .newcard
    }
}

proc showEditCardDialog {cardId listId} {
    # Get card details
    db eval {SELECT title, description FROM cards WHERE id = $cardId} {
        toplevel .editcard
        wm title .editcard "Edit Card"
        wm geometry .editcard 400x250
        
        label .editcard.titlelbl -text "Card Title:"
        entry .editcard.title -width 40
        .editcard.title insert 0 $title
        pack .editcard.titlelbl -pady 5
        pack .editcard.title -pady 5
        
        label .editcard.desclbl -text "Description:"
        text .editcard.desc -width 40 -height 5
        .editcard.desc insert 1.0 $description
        pack .editcard.desclbl -pady 5
        pack .editcard.desc -pady 5
        
        frame .editcard.buttons
        pack .editcard.buttons -pady 20
        
        button .editcard.buttons.save -text "Save" -command [list saveCardFromDialog $cardId] \
            -bg #4CAF50 -fg black -activebackground #45a049 -activeforeground black
        pack .editcard.buttons.save -side left -padx 5
        
        button .editcard.buttons.cancel -text "Cancel" -command {destroy .editcard}
        pack .editcard.buttons.cancel -side left -padx 5
        
        focus .editcard.title
    }
}

proc saveCardFromDialog {cardId} {
    set title [.editcard.title get]
    set description [.editcard.desc get 1.0 end-1c]
    if {$title ne ""} {
        updateCard $cardId $title $description
        destroy .editcard
    }
}

proc showCardDetails {cardId} {
    db eval {SELECT title, description, created_at FROM cards WHERE id = $cardId} {
        tk_messageBox -title "Card Details" -message "Title: $title\n\nDescription: $description\n\nCreated: $created_at" -type ok
    }
}

# Main execution
initDatabase
createMainWindow

# If no boards exist, show welcome dialog
if {[llength [getBoards]] == 0} {
    after 500 {
        set answer [tk_messageBox -title "Welcome" -message "Welcome to Tcl/Tk Kanban!\n\nNo boards found. Would you like to create your first board?" -type yesno -icon question]
        if {$answer eq "yes"} {
            showNewBoardDialog
        }
    }
}
