#!/usr/bin/env tclsh
# Demo script to populate the database with sample data

package require sqlite3

# Initialize database
sqlite3 db wekan.db

# Create tables if they don't exist
# Copied from kanban.tcl

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

# Create sample board
db eval {
    INSERT INTO boards (name, description) VALUES 
    ('Sample Project', 'A sample Kanban board to get you started');
}
set boardId [db last_insert_rowid]

# Create swimlanes
db eval {
    INSERT INTO swimlanes (board_id, name, position) VALUES 
    ($boardId, 'Development Team', 0),
    ($boardId, 'Design Team', 1);
}

set devSwimlaneId [lindex [db eval {SELECT id FROM swimlanes WHERE board_id = $boardId AND name = 'Development Team'}] 0]
set designSwimlaneId [lindex [db eval {SELECT id FROM swimlanes WHERE board_id = $boardId AND name = 'Design Team'}] 0]

# Create lists for Development Team
db eval {
    INSERT INTO lists (swimlane_id, name, position) VALUES 
    ($devSwimlaneId, 'Backlog', 0),
    ($devSwimlaneId, 'In Progress', 1),
    ($devSwimlaneId, 'Done', 2);
}

set backlogId [lindex [db eval {SELECT id FROM lists WHERE swimlane_id = $devSwimlaneId AND name = 'Backlog'}] 0]
set inProgressId [lindex [db eval {SELECT id FROM lists WHERE swimlane_id = $devSwimlaneId AND name = 'In Progress'}] 0]
set doneId [lindex [db eval {SELECT id FROM lists WHERE swimlane_id = $devSwimlaneId AND name = 'Done'}] 0]

# Create lists for Design Team
db eval {
    INSERT INTO lists (swimlane_id, name, position) VALUES 
    ($designSwimlaneId, 'Ideas', 0),
    ($designSwimlaneId, 'Working On', 1),
    ($designSwimlaneId, 'Review', 2);
}

set ideasId [lindex [db eval {SELECT id FROM lists WHERE swimlane_id = $designSwimlaneId AND name = 'Ideas'}] 0]
set workingOnId [lindex [db eval {SELECT id FROM lists WHERE swimlane_id = $designSwimlaneId AND name = 'Working On'}] 0]
set reviewId [lindex [db eval {SELECT id FROM lists WHERE swimlane_id = $designSwimlaneId AND name = 'Review'}] 0]

# Create sample cards for Development Team
db eval {
    INSERT INTO cards (list_id, title, description, position) VALUES 
    ($backlogId, 'Add user authentication', 'Implement login and registration system with OAuth support', 0),
    ($backlogId, 'Create API documentation', 'Document all REST API endpoints with examples', 1),
    ($inProgressId, 'Fix database connection bug', 'Users report intermittent connection timeouts', 0),
    ($inProgressId, 'Optimize query performance', 'Improve loading time for dashboard queries', 1),
    ($doneId, 'Setup CI/CD pipeline', 'Configure GitHub Actions for automated testing and deployment', 0),
    ($doneId, 'Update dependencies', 'Update all npm packages to latest stable versions', 1);
}

# Create sample cards for Design Team
db eval {
    INSERT INTO cards (list_id, title, description, position) VALUES 
    ($ideasId, 'Redesign landing page', 'Modern, minimalist design with better CTAs', 0),
    ($ideasId, 'Dark mode support', 'Add dark theme option for user preference', 1),
    ($workingOnId, 'Mobile responsive layout', 'Ensure all pages work perfectly on mobile devices', 0),
    ($reviewId, 'New logo design', 'Three logo concepts ready for stakeholder review', 0);
}

db close

puts "Sample data created successfully!"
puts "Run ./build.sh and select option 1 to see your sample board."
