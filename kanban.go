package main

import (
	"database/sql"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/xuri/excelize/v2"
	_ "github.com/mattn/go-sqlite3"
)

// Structs for data
type Board struct {
	ID          int
	Name        string
	Description string
}

type Swimlane struct {
	ID      int
	BoardID int
	Name    string
	Position int
}

type List struct {
	ID         int
	SwimlaneID int
	Name       string
	Position   int
}

type Card struct {
	ID          int
	ListID      int
	Title       string
	Description string
	Position    int
	CreatedAt   string
	Attachment  []byte
}

// Global variables
var db *sql.DB
var currentBoardID int
var currentSwimlaneID int
var mainArea *fyne.Container
var mainWindow fyne.Window
var draggedCard *DraggableCard
var draggedList *DraggableList
var boardContainer *fyne.Container
var currentTooltip *widget.PopUp
var tooltipTimer *time.Timer
var hideTooltipTimer *time.Timer

// Tooltip functions
func showTooltip(mousePos fyne.Position, text string) {
	// Cancel any pending hide
	if hideTooltipTimer != nil {
		hideTooltipTimer.Stop()
		hideTooltipTimer = nil
	}
	
	// If already showing a tooltip, update it immediately
	if currentTooltip != nil {
		currentTooltip.Hide()
		currentTooltip = nil
	}
	
	// Delay showing tooltip to prevent flashing
	if tooltipTimer != nil {
		tooltipTimer.Stop()
	}
	
	tooltipTimer = time.AfterFunc(500*time.Millisecond, func() {
		fyne.DoAndWait(func() {
			label := widget.NewLabel(text)
			label.Resize(fyne.NewSize(200, 30))
			
			currentTooltip = widget.NewPopUp(label, mainWindow.Canvas())
			
			// Position the tooltip near the mouse cursor
			tooltipPos := fyne.NewPos(mousePos.X+10, mousePos.Y+10)
			currentTooltip.Move(tooltipPos)
			currentTooltip.Show()
		})
	})
}

func hideTooltip() {
	// Cancel any pending show
	if tooltipTimer != nil {
		tooltipTimer.Stop()
		tooltipTimer = nil
	}
	
	// Delay hiding tooltip to prevent flashing when moving between button and tooltip
	if hideTooltipTimer != nil {
		hideTooltipTimer.Stop()
	}
	
	hideTooltipTimer = time.AfterFunc(200*time.Millisecond, func() {
		fyne.DoAndWait(func() {
			if currentTooltip != nil {
				currentTooltip.Hide()
				currentTooltip = nil
			}
		})
	})
}

// Custom button with tooltip support
type TooltipButton struct {
	widget.Button
	tooltipText string
}

func (t *TooltipButton) MouseIn(ev *desktop.MouseEvent) {
	showTooltip(ev.Position, t.tooltipText)
}

func (t *TooltipButton) MouseOut() {
	hideTooltip()
}

func NewTooltipButton(text string, tooltip string, tapped func()) *TooltipButton {
	btn := &TooltipButton{}
	btn.ExtendBaseWidget(btn)
	btn.SetText(text)
	btn.OnTapped = tapped
	btn.tooltipText = tooltip
	return btn
}

// Draggable card widget
type DraggableCard struct {
	*widget.Card
	CardID int
	ListID int
}

func (d *DraggableCard) Dragged(ev *fyne.DragEvent) {
	draggedCard = d
}

func (d *DraggableCard) DragEnd() {
	draggedCard = nil
}

func (d *DraggableCard) Dropped(ev *fyne.DragEvent) {}

// Draggable list widget
type DraggableList struct {
	*fyne.Container
	ListID    int
	SwimlaneID int
}

func (d *DraggableList) Dragged(ev *fyne.DragEvent) {
	draggedList = d
}

func (d *DraggableList) DragEnd() {
	draggedList = nil
}

func (d *DraggableList) Dropped(ev *fyne.DragEvent) {
	// Drag and drop
	if draggedCard != nil && draggedCard.ListID != d.ListID {
		// Move card to this list
		_, err := db.Exec("UPDATE cards SET list_id = ? WHERE id = ?", d.ListID, draggedCard.CardID)
		if err == nil {
			draggedCard.ListID = d.ListID
			// Update positions
			updateCardPositions(d.ListID)
			// Refresh the board
			loadBoard(currentBoardID)
		}
	}
}

// Update card positions in a list
func updateCardPositions(listID int) {
	cards := getCards(listID)
	for i, card := range cards {
		db.Exec("UPDATE cards SET position = ? WHERE id = ?", i, card.ID)
	}
}

// Update list positions in a swimlane
func updateListPositions(swimlaneID int) {
	lists := getLists(swimlaneID)
	for i, list := range lists {
		db.Exec("UPDATE lists SET position = ? WHERE id = ?", i, list.ID)
	}
}

// Move functions for swimlanes, lists, and cards
func moveSwimlaneUp(swimlaneID int) {
	var currentPos int
	err := db.QueryRow("SELECT position FROM swimlanes WHERE id = ?", swimlaneID).Scan(&currentPos)
	if err != nil || currentPos <= 0 {
		return
	}
	
	var boardID int
	db.QueryRow("SELECT board_id FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID)
	
	newPos := currentPos - 1
	var targetSwimlaneID int
	err = db.QueryRow("SELECT id FROM swimlanes WHERE board_id = ? AND position = ?", boardID, newPos).Scan(&targetSwimlaneID)
	if err == nil {
		db.Exec("UPDATE swimlanes SET position = ? WHERE id = ?", currentPos, targetSwimlaneID)
		db.Exec("UPDATE swimlanes SET position = ? WHERE id = ?", newPos, swimlaneID)
		loadBoard(boardID)
	}
}

func moveSwimlaneDown(swimlaneID int) {
	var boardID, currentPos int
	db.QueryRow("SELECT board_id, position FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID, &currentPos)
	
	var maxPos int
	db.QueryRow("SELECT MAX(position) FROM swimlanes WHERE board_id = ?", boardID).Scan(&maxPos)
	
	if currentPos >= maxPos {
		return
	}
	
	newPos := currentPos + 1
	var targetSwimlaneID int
	err := db.QueryRow("SELECT id FROM swimlanes WHERE board_id = ? AND position = ?", boardID, newPos).Scan(&targetSwimlaneID)
	if err == nil {
		db.Exec("UPDATE swimlanes SET position = ? WHERE id = ?", currentPos, targetSwimlaneID)
		db.Exec("UPDATE swimlanes SET position = ? WHERE id = ?", newPos, swimlaneID)
		loadBoard(boardID)
	}
}

func moveListLeft(listID int) {
	var currentPos, swimlaneID int
	db.QueryRow("SELECT position, swimlane_id FROM lists WHERE id = ?", listID).Scan(&currentPos, &swimlaneID)
	
	if currentPos <= 0 {
		return
	}
	
	newPos := currentPos - 1
	var targetListID int
	err := db.QueryRow("SELECT id FROM lists WHERE swimlane_id = ? AND position = ?", swimlaneID, newPos).Scan(&targetListID)
	if err == nil {
		db.Exec("UPDATE lists SET position = ? WHERE id = ?", currentPos, targetListID)
		db.Exec("UPDATE lists SET position = ? WHERE id = ?", newPos, listID)
		var boardID int
		db.QueryRow("SELECT board_id FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID)
		loadBoard(boardID)
	}
}

func moveListRight(listID int) {
	var swimlaneID, currentPos int
	db.QueryRow("SELECT swimlane_id, position FROM lists WHERE id = ?", listID).Scan(&swimlaneID, &currentPos)
	
	var maxPos int
	db.QueryRow("SELECT MAX(position) FROM lists WHERE swimlane_id = ?", swimlaneID).Scan(&maxPos)
	
	if currentPos >= maxPos {
		return
	}
	
	newPos := currentPos + 1
	var targetListID int
	err := db.QueryRow("SELECT id FROM lists WHERE swimlane_id = ? AND position = ?", swimlaneID, newPos).Scan(&targetListID)
	if err == nil {
		db.Exec("UPDATE lists SET position = ? WHERE id = ?", currentPos, targetListID)
		db.Exec("UPDATE lists SET position = ? WHERE id = ?", newPos, listID)
		var boardID int
		db.QueryRow("SELECT board_id FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID)
		loadBoard(boardID)
	}
}

func moveListToAboveSwimlane(listID int) {
	var srcSwimlaneID, srcPos int
	db.QueryRow("SELECT swimlane_id, position FROM lists WHERE id = ?", listID).Scan(&srcSwimlaneID, &srcPos)
	
	var boardID, swPos int
	db.QueryRow("SELECT board_id, position FROM swimlanes WHERE id = ?", srcSwimlaneID).Scan(&boardID, &swPos)
	
	if swPos <= 0 {
		return
	}
	
	var prevSwimlaneID int
	err := db.QueryRow("SELECT id FROM swimlanes WHERE board_id = ? AND position = ?", boardID, swPos-1).Scan(&prevSwimlaneID)
	if err != nil {
		return
	}
	
	// Compact positions in source swimlane
	db.Exec("UPDATE lists SET position = position - 1 WHERE swimlane_id = ? AND position > ?", srcSwimlaneID, srcPos)
	
	// Append to end of target swimlane
	var targetPos int
	db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM lists WHERE swimlane_id = ?", prevSwimlaneID).Scan(&targetPos)
	newPos := targetPos + 1
	db.Exec("UPDATE lists SET swimlane_id = ?, position = ? WHERE id = ?", prevSwimlaneID, newPos, listID)
	loadBoard(boardID)
}

func moveListToBelowSwimlane(listID int) {
	var srcSwimlaneID, srcPos int
	db.QueryRow("SELECT swimlane_id, position FROM lists WHERE id = ?", listID).Scan(&srcSwimlaneID, &srcPos)
	
	var boardID, swPos int
	db.QueryRow("SELECT board_id, position FROM swimlanes WHERE id = ?", srcSwimlaneID).Scan(&boardID, &swPos)
	
	var nextSwimlaneID int
	err := db.QueryRow("SELECT id FROM swimlanes WHERE board_id = ? AND position = ?", boardID, swPos+1).Scan(&nextSwimlaneID)
	if err != nil {
		return
	}
	
	// Compact positions in source swimlane
	db.Exec("UPDATE lists SET position = position - 1 WHERE swimlane_id = ? AND position > ?", srcSwimlaneID, srcPos)
	
	// Append to end of target swimlane
	var targetPos int
	db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM lists WHERE swimlane_id = ?", nextSwimlaneID).Scan(&targetPos)
	newPos := targetPos + 1
	db.Exec("UPDATE lists SET swimlane_id = ?, position = ? WHERE id = ?", nextSwimlaneID, newPos, listID)
	loadBoard(boardID)
}

func moveCardUp(cardID int) {
	var listID, currentPos int
	db.QueryRow("SELECT list_id, position FROM cards WHERE id = ?", cardID).Scan(&listID, &currentPos)
	
	if currentPos <= 0 {
		return
	}
	
	newPos := currentPos - 1
	var targetCardID int
	err := db.QueryRow("SELECT id FROM cards WHERE list_id = ? AND position = ?", listID, newPos).Scan(&targetCardID)
	if err == nil {
		db.Exec("UPDATE cards SET position = ? WHERE id = ?", currentPos, targetCardID)
		db.Exec("UPDATE cards SET position = ? WHERE id = ?", newPos, cardID)
		var boardID int
		db.QueryRow(`SELECT s.board_id FROM swimlanes s 
			JOIN lists l ON s.id = l.swimlane_id 
			WHERE l.id = ?`, listID).Scan(&boardID)
		loadBoard(boardID)
	}
}

func moveCardDown(cardID int) {
	var listID, currentPos int
	db.QueryRow("SELECT list_id, position FROM cards WHERE id = ?", cardID).Scan(&listID, &currentPos)
	
	var maxPos int
	db.QueryRow("SELECT MAX(position) FROM cards WHERE list_id = ?", listID).Scan(&maxPos)
	
	if currentPos >= maxPos {
		return
	}
	
	newPos := currentPos + 1
	var targetCardID int
	err := db.QueryRow("SELECT id FROM cards WHERE list_id = ? AND position = ?", listID, newPos).Scan(&targetCardID)
	if err == nil {
		db.Exec("UPDATE cards SET position = ? WHERE id = ?", currentPos, targetCardID)
		db.Exec("UPDATE cards SET position = ? WHERE id = ?", newPos, cardID)
		var boardID int
		db.QueryRow(`SELECT s.board_id FROM swimlanes s 
			JOIN lists l ON s.id = l.swimlane_id 
			WHERE l.id = ?`, listID).Scan(&boardID)
		loadBoard(boardID)
	}
}

func moveCardToLeftList(cardID int) {
	var currentListID, currentPos int
	db.QueryRow("SELECT list_id, position FROM cards WHERE id = ?", cardID).Scan(&currentListID, &currentPos)
	
	var swimlaneID, listPos int
	db.QueryRow("SELECT swimlane_id, position FROM lists WHERE id = ?", currentListID).Scan(&swimlaneID, &listPos)
	
	if listPos <= 0 {
		return
	}
	
	var targetListID int
	err := db.QueryRow("SELECT id FROM lists WHERE swimlane_id = ? AND position = ?", swimlaneID, listPos-1).Scan(&targetListID)
	if err != nil {
		return
	}
	
	// Compact positions in source list
	db.Exec("UPDATE cards SET position = position - 1 WHERE list_id = ? AND position > ?", currentListID, currentPos)
	
	// Append to end of target list
	var targetPos int
	db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM cards WHERE list_id = ?", targetListID).Scan(&targetPos)
	newPos := targetPos + 1
	db.Exec("UPDATE cards SET list_id = ?, position = ? WHERE id = ?", targetListID, newPos, cardID)
	
	var boardID int
	db.QueryRow("SELECT board_id FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID)
	loadBoard(boardID)
}

func moveCardToRightList(cardID int) {
	var currentListID, currentPos int
	db.QueryRow("SELECT list_id, position FROM cards WHERE id = ?", cardID).Scan(&currentListID, &currentPos)
	
	var swimlaneID, listPos int
	db.QueryRow("SELECT swimlane_id, position FROM lists WHERE id = ?", currentListID).Scan(&swimlaneID, &listPos)
	
	var maxListPos int
	db.QueryRow("SELECT MAX(position) FROM lists WHERE swimlane_id = ?", swimlaneID).Scan(&maxListPos)
	
	if listPos >= maxListPos {
		return
	}
	
	var targetListID int
	err := db.QueryRow("SELECT id FROM lists WHERE swimlane_id = ? AND position = ?", swimlaneID, listPos+1).Scan(&targetListID)
	if err != nil {
		return
	}
	
	// Compact positions in source list
	db.Exec("UPDATE cards SET position = position - 1 WHERE list_id = ? AND position > ?", currentListID, currentPos)
	
	// Append to end of target list
	var targetPos int
	db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM cards WHERE list_id = ?", targetListID).Scan(&targetPos)
	newPos := targetPos + 1
	db.Exec("UPDATE cards SET list_id = ?, position = ? WHERE id = ?", targetListID, newPos, cardID)
	
	var boardID int
	db.QueryRow("SELECT board_id FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID)
	loadBoard(boardID)
}

// Drop zone for swimlanes
type DroppableSwimlane struct {
	*fyne.Container
	SwimlaneID int
}

func (d *DroppableSwimlane) Dropped(ev *fyne.DragEvent) {
	if draggedList != nil && draggedList.SwimlaneID != d.SwimlaneID {
		// Move list to this swimlane
		_, err := db.Exec("UPDATE lists SET swimlane_id = ? WHERE id = ?", d.SwimlaneID, draggedList.ListID)
		if err == nil {
			draggedList.SwimlaneID = d.SwimlaneID
			// Update positions
			updateListPositions(d.SwimlaneID)
			// Refresh the board
			loadBoard(currentBoardID)
		}
	}
}

func (d *DroppableSwimlane) Dragged(ev *fyne.DragEvent) {}

func (d *DroppableSwimlane) DragEnd() {}

// Database functions
func initDatabase() {
	var err error
	db, err = sql.Open("sqlite3", "wekan.db")
	if err != nil {
		panic(err)
	}

	// Create tables
	_, err = db.Exec(`
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
			attachment BLOB,
			FOREIGN KEY (list_id) REFERENCES lists(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		panic(err)
	}
}

func getBoardByID(boardID int) *Board {
	row := db.QueryRow("SELECT id, name, description FROM boards WHERE id = ?", boardID)
	var board Board
	err := row.Scan(&board.ID, &board.Name, &board.Description)
	if err != nil {
		return nil
	}
	return &board
}

func getBoards() []Board {
	rows, err := db.Query("SELECT id, name, description FROM boards ORDER BY name")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var b Board
		rows.Scan(&b.ID, &b.Name, &b.Description)
		boards = append(boards, b)
	}
	return boards
}

func createBoard(name, desc string) {
	_, err := db.Exec("INSERT INTO boards (name, description) VALUES (?, ?)", name, desc)
	if err != nil {
		fmt.Println("Error creating board:", err)
	}
	refreshBoardList()
}

func getSwimlanes(boardID int) []Swimlane {
	rows, err := db.Query("SELECT id, board_id, name, position FROM swimlanes WHERE board_id = ? ORDER BY position", boardID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var swimlanes []Swimlane
	for rows.Next() {
		var s Swimlane
		rows.Scan(&s.ID, &s.BoardID, &s.Name, &s.Position)
		swimlanes = append(swimlanes, s)
	}
	return swimlanes
}

func getLists(swimlaneID int) []List {
	rows, err := db.Query("SELECT id, swimlane_id, name, position FROM lists WHERE swimlane_id = ? ORDER BY position", swimlaneID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var lists []List
	for rows.Next() {
		var l List
		rows.Scan(&l.ID, &l.SwimlaneID, &l.Name, &l.Position)
		lists = append(lists, l)
	}
	return lists
}

func getCards(listID int) []Card {
	rows, err := db.Query("SELECT id, list_id, title, description, position, created_at, attachment FROM cards WHERE list_id = ? ORDER BY position", listID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var c Card
		rows.Scan(&c.ID, &c.ListID, &c.Title, &c.Description, &c.Position, &c.CreatedAt, &c.Attachment)
		cards = append(cards, c)
	}
	return cards
}

// Board management functions
func deleteBoard(boardID int) {
	_, err := db.Exec("DELETE FROM boards WHERE id = ?", boardID)
	if err != nil {
		fmt.Println("Error deleting board:", err)
	}
	refreshBoardList()
}

func cloneBoard(boardID int) {
	// Get original board info
	var origName, origDesc string
	err := db.QueryRow("SELECT name, description FROM boards WHERE id = ?", boardID).Scan(&origName, &origDesc)
	if err != nil {
		fmt.Println("Error getting board info:", err)
		return
	}

	// Create new board
	newName := origName + " (Copy)"
	_, err = db.Exec("INSERT INTO boards (name, description) VALUES (?, ?)", newName, origDesc)
	if err != nil {
		fmt.Println("Error creating board copy:", err)
		return
	}

	// Get the new board ID
	var newBoardID int
	err = db.QueryRow("SELECT last_insert_rowid()").Scan(&newBoardID)
	if err != nil {
		fmt.Println("Error getting new board ID:", err)
		return
	}

	// Clone all swimlanes
	swimlanes := getSwimlanes(boardID)
	for _, s := range swimlanes {
		cloneSwimlaneToBoard(s.ID, newBoardID)
	}
	refreshBoardList()
}

// Swimlane management functions
func createSwimlane(boardID int, name string) {
	// Get max position
	var maxPos int
	err := db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM swimlanes WHERE board_id = ?", boardID).Scan(&maxPos)
	if err != nil {
		fmt.Println("Error getting max position:", err)
		return
	}
	
	_, err = db.Exec("INSERT INTO swimlanes (board_id, name, position) VALUES (?, ?, ?)", boardID, name, maxPos+1)
	if err != nil {
		fmt.Println("Error creating swimlane:", err)
	}
}

func deleteSwimlane(swimlaneID int) {
	_, err := db.Exec("DELETE FROM swimlanes WHERE id = ?", swimlaneID)
	if err != nil {
		fmt.Println("Error deleting swimlane:", err)
	}
}

func cloneSwimlane(swimlaneID int) {
	// Get original swimlane info
	var boardID int
	var origName string
	var origPos int
	err := db.QueryRow("SELECT board_id, name, position FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID, &origName, &origPos)
	if err != nil {
		fmt.Println("Error getting swimlane info:", err)
		return
	}

	// Increment position of all swimlanes below the original
	_, err = db.Exec("UPDATE swimlanes SET position = position + 1 WHERE board_id = ? AND position > ?", boardID, origPos)
	if err != nil {
		fmt.Println("Error updating positions:", err)
		return
	}

	// Create new swimlane
	newName := origName + " (Copy)"
	newPos := origPos + 1
	_, err = db.Exec("INSERT INTO swimlanes (board_id, name, position) VALUES (?, ?, ?)", boardID, newName, newPos)
	if err != nil {
		fmt.Println("Error creating swimlane copy:", err)
		return
	}

	// Get the new swimlane ID
	var newSwimlaneID int
	err = db.QueryRow("SELECT last_insert_rowid()").Scan(&newSwimlaneID)
	if err != nil {
		fmt.Println("Error getting new swimlane ID:", err)
		return
	}

	// Clone all lists
	lists := getLists(swimlaneID)
	for _, l := range lists {
		cloneListToSwimlane(l.ID, newSwimlaneID)
	}
}

func cloneSwimlaneToBoard(swimlaneID, newBoardID int) {
	// Get original swimlane info
	var origName string
	var origPos int
	err := db.QueryRow("SELECT name, position FROM swimlanes WHERE id = ?", swimlaneID).Scan(&origName, &origPos)
	if err != nil {
		fmt.Println("Error getting swimlane info:", err)
		return
	}

	// Create new swimlane in the new board
	_, err = db.Exec("INSERT INTO swimlanes (board_id, name, position) VALUES (?, ?, ?)", newBoardID, origName, origPos)
	if err != nil {
		fmt.Println("Error creating swimlane in new board:", err)
		return
	}

	// Get the new swimlane ID
	var newSwimlaneID int
	err = db.QueryRow("SELECT last_insert_rowid()").Scan(&newSwimlaneID)
	if err != nil {
		fmt.Println("Error getting new swimlane ID:", err)
		return
	}

	// Clone all lists
	lists := getLists(swimlaneID)
	for _, l := range lists {
		cloneListToSwimlane(l.ID, newSwimlaneID)
	}
}

// List management functions
func createList(swimlaneID int, name string) {
	// Get max position
	var maxPos int
	err := db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM lists WHERE swimlane_id = ?", swimlaneID).Scan(&maxPos)
	if err != nil {
		fmt.Println("Error getting max position:", err)
		return
	}
	
	_, err = db.Exec("INSERT INTO lists (swimlane_id, name, position) VALUES (?, ?, ?)", swimlaneID, name, maxPos+1)
	if err != nil {
		fmt.Println("Error creating list:", err)
	}
}

func deleteList(listID int) {
	_, err := db.Exec("DELETE FROM lists WHERE id = ?", listID)
	if err != nil {
		fmt.Println("Error deleting list:", err)
	}
}

func cloneList(listID int) {
	// Get original list info
	var swimlaneID int
	var origName string
	err := db.QueryRow("SELECT swimlane_id, name FROM lists WHERE id = ?", listID).Scan(&swimlaneID, &origName)
	if err != nil {
		fmt.Println("Error getting list info:", err)
		return
	}

	// Find max position and add at end
	var maxPos int
	err = db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM lists WHERE swimlane_id = ?", swimlaneID).Scan(&maxPos)
	if err != nil {
		fmt.Println("Error getting max position:", err)
		return
	}

	newName := origName + " (Copy)"
	newPos := maxPos + 1
	
	_, err = db.Exec("INSERT INTO lists (swimlane_id, name, position) VALUES (?, ?, ?)", swimlaneID, newName, newPos)
	if err != nil {
		fmt.Println("Error creating list copy:", err)
		return
	}

	// Get the new list ID
	var newListID int
	err = db.QueryRow("SELECT last_insert_rowid()").Scan(&newListID)
	if err != nil {
		fmt.Println("Error getting new list ID:", err)
		return
	}

	// Clone all cards
	cards := getCards(listID)
	for _, c := range cards {
		cloneCardToList(c.ID, newListID)
	}
}

func cloneListToSwimlane(listID, newSwimlaneID int) {
	// Get original list info
	var origName string
	var origPos int
	err := db.QueryRow("SELECT name, position FROM lists WHERE id = ?", listID).Scan(&origName, &origPos)
	if err != nil {
		fmt.Println("Error getting list info:", err)
		return
	}

	// Create new list in the new swimlane
	_, err = db.Exec("INSERT INTO lists (swimlane_id, name, position) VALUES (?, ?, ?)", newSwimlaneID, origName, origPos)
	if err != nil {
		fmt.Println("Error creating list in new swimlane:", err)
		return
	}

	// Get the new list ID
	var newListID int
	err = db.QueryRow("SELECT last_insert_rowid()").Scan(&newListID)
	if err != nil {
		fmt.Println("Error getting new list ID:", err)
		return
	}

	// Clone all cards
	cards := getCards(listID)
	for _, c := range cards {
		cloneCardToList(c.ID, newListID)
	}
}

// Card management functions
func createCard(listID int, title, description string) {
	// Get max position
	var maxPos int
	err := db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM cards WHERE list_id = ?", listID).Scan(&maxPos)
	if err != nil {
		fmt.Println("Error getting max position:", err)
		return
	}
	
	_, err = db.Exec("INSERT INTO cards (list_id, title, description, position) VALUES (?, ?, ?, ?)", listID, title, description, maxPos+1)
	if err != nil {
		fmt.Println("Error creating card:", err)
	}
}

func deleteCard(cardID int) {
	_, err := db.Exec("DELETE FROM cards WHERE id = ?", cardID)
	if err != nil {
		fmt.Println("Error deleting card:", err)
	}
}

func cloneCard(cardID int) {
	// Get original card info
	var listID int
	var origTitle, origDesc string
	err := db.QueryRow("SELECT list_id, title, description FROM cards WHERE id = ?", cardID).Scan(&listID, &origTitle, &origDesc)
	if err != nil {
		fmt.Println("Error getting card info:", err)
		return
	}

	// Find max position and add at end
	var maxPos int
	err = db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM cards WHERE list_id = ?", listID).Scan(&maxPos)
	if err != nil {
		fmt.Println("Error getting max position:", err)
		return
	}

	newTitle := origTitle + " (Copy)"
	newPos := maxPos + 1
	
	_, err = db.Exec("INSERT INTO cards (list_id, title, description, position) VALUES (?, ?, ?, ?)", listID, newTitle, origDesc, newPos)
	if err != nil {
		fmt.Println("Error creating card copy:", err)
	}
}

func cloneCardToList(cardID, newListID int) {
	// Get original card info
	var origTitle, origDesc string
	var origPos int
	err := db.QueryRow("SELECT title, description, position FROM cards WHERE id = ?", cardID).Scan(&origTitle, &origDesc, &origPos)
	if err != nil {
		fmt.Println("Error getting card info:", err)
		return
	}

	// Create new card in the new list
	_, err = db.Exec("INSERT INTO cards (list_id, title, description, position) VALUES (?, ?, ?, ?)", newListID, origTitle, origDesc, origPos)
	if err != nil {
		fmt.Println("Error creating card in new list:", err)
	}
}

// UI Dialog functions
func showNewBoardDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Board name")
	
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Board description")
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	createBtn := widget.NewButton("Create", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Create New Board"),
		nameEntry,
		descEntry,
		container.NewHBox(cancelBtn, createBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	createBtn.OnTapped = func() {
		if nameEntry.Text != "" {
			createBoard(nameEntry.Text, descEntry.Text)
			// Refresh the board list - simplified
			boards := getBoards()
			if len(boards) > 0 {
				currentBoardID = boards[len(boards)-1].ID
				loadBoard(currentBoardID)
			}
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

func showNewSwimlaneDialog(boardID int) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Swimlane name")
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	createBtn := widget.NewButton("Create", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Create New Swimlane"),
		nameEntry,
		container.NewHBox(cancelBtn, createBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	createBtn.OnTapped = func() {
		if nameEntry.Text != "" {
			createSwimlane(boardID, nameEntry.Text)
			loadBoard(boardID)
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

func showNewListDialog(swimlaneID int) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("List name")
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	createBtn := widget.NewButton("Create", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Create New List"),
		nameEntry,
		container.NewHBox(cancelBtn, createBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	createBtn.OnTapped = func() {
		if nameEntry.Text != "" {
			createList(swimlaneID, nameEntry.Text)
			// Find board ID and refresh
			var boardID int
			db.QueryRow("SELECT board_id FROM swimlanes WHERE id = ?", swimlaneID).Scan(&boardID)
			loadBoard(boardID)
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

func showNewCardDialog(listID int) {
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Card title")
	
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Card description")
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	createBtn := widget.NewButton("Create", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Create New Card"),
		titleEntry,
		descEntry,
		container.NewHBox(cancelBtn, createBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	createBtn.OnTapped = func() {
		if titleEntry.Text != "" {
			createCard(listID, titleEntry.Text, descEntry.Text)
			// Find board ID and refresh
			var boardID int
			db.QueryRow(`SELECT s.board_id FROM swimlanes s 
				JOIN lists l ON s.id = l.swimlane_id 
				WHERE l.id = ?`, listID).Scan(&boardID)
			loadBoard(boardID)
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

// Update functions
func updateBoard(boardID int, name, description string) {
	_, err := db.Exec("UPDATE boards SET name = ?, description = ? WHERE id = ?", name, description, boardID)
	if err != nil {
		fmt.Println("Error updating board:", err)
	}
	refreshBoardContainer()
	// Update window title if current board was edited
	if boardID == currentBoardID {
		loadBoard(boardID)
	}
}

func updateSwimlane(swimlaneID int, name string) {
	_, err := db.Exec("UPDATE swimlanes SET name = ? WHERE id = ?", name, swimlaneID)
	if err != nil {
		fmt.Println("Error updating swimlane:", err)
	}
}

func updateList(listID int, name string) {
	_, err := db.Exec("UPDATE lists SET name = ? WHERE id = ?", name, listID)
	if err != nil {
		fmt.Println("Error updating list:", err)
	}
}

func updateCard(cardID int, title, description string) {
	_, err := db.Exec("UPDATE cards SET title = ?, description = ? WHERE id = ?", title, description, cardID)
	if err != nil {
		fmt.Println("Error updating card:", err)
	}
}

func showEditBoardDialog(boardID int) {
	var currentName, currentDesc string
	err := db.QueryRow("SELECT name, description FROM boards WHERE id = ?", boardID).Scan(&currentName, &currentDesc)
	if err != nil {
		fmt.Println("Error getting board info:", err)
		return
	}
	
	nameEntry := widget.NewEntry()
	nameEntry.SetText(currentName)
	
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetText(currentDesc)
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	saveBtn := widget.NewButton("Save", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Edit Board"),
		widget.NewLabel("Board Name:"),
		nameEntry,
		widget.NewLabel("Description:"),
		descEntry,
		container.NewHBox(cancelBtn, saveBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	saveBtn.OnTapped = func() {
		if nameEntry.Text != "" {
			updateBoard(boardID, nameEntry.Text, descEntry.Text)
			loadBoard(boardID)
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

func showEditSwimlaneDialog(swimlaneID int) {
	var currentName string
	var boardID int
	err := db.QueryRow("SELECT name, board_id FROM swimlanes WHERE id = ?", swimlaneID).Scan(&currentName, &boardID)
	if err != nil {
		fmt.Println("Error getting swimlane info:", err)
		return
	}
	
	nameEntry := widget.NewEntry()
	nameEntry.SetText(currentName)
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	saveBtn := widget.NewButton("Save", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Edit Swimlane"),
		widget.NewLabel("Swimlane Name:"),
		nameEntry,
		container.NewHBox(cancelBtn, saveBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	saveBtn.OnTapped = func() {
		if nameEntry.Text != "" {
			updateSwimlane(swimlaneID, nameEntry.Text)
			loadBoard(boardID)
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

func showEditListDialog(listID int) {
	var currentName string
	var boardID int
	err := db.QueryRow(`SELECT l.name, s.board_id FROM lists l 
		JOIN swimlanes s ON l.swimlane_id = s.id 
		WHERE l.id = ?`, listID).Scan(&currentName, &boardID)
	if err != nil {
		fmt.Println("Error getting list info:", err)
		return
	}
	
	nameEntry := widget.NewEntry()
	nameEntry.SetText(currentName)
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	saveBtn := widget.NewButton("Save", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Edit List"),
		widget.NewLabel("List Name:"),
		nameEntry,
		container.NewHBox(cancelBtn, saveBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	saveBtn.OnTapped = func() {
		if nameEntry.Text != "" {
			updateList(listID, nameEntry.Text)
			loadBoard(boardID)
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

func showEditCardDialog(cardID int) {
	var currentTitle, currentDesc string
	var boardID int
	err := db.QueryRow(`SELECT c.title, c.description, s.board_id FROM cards c
		JOIN lists l ON c.list_id = l.id
		JOIN swimlanes s ON l.swimlane_id = s.id
		WHERE c.id = ?`, cardID).Scan(&currentTitle, &currentDesc, &boardID)
	if err != nil {
		fmt.Println("Error getting card info:", err)
		return
	}
	
	titleEntry := widget.NewEntry()
	titleEntry.SetText(currentTitle)
	
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetText(currentDesc)
	
	cancelBtn := widget.NewButton("Cancel", func() {})
	saveBtn := widget.NewButton("Save", func() {})
	
	content := container.NewVBox(
		widget.NewLabel("Edit Card"),
		widget.NewLabel("Card Title:"),
		titleEntry,
		widget.NewLabel("Description:"),
		descEntry,
		container.NewHBox(cancelBtn, saveBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	saveBtn.OnTapped = func() {
		if titleEntry.Text != "" {
			updateCard(cardID, titleEntry.Text, descEntry.Text)
			loadBoard(boardID)
		}
		dialog.Hide()
	}
	
	dialog.Show()
}

// UI refresh functions
func refreshBoardContainer() {
	if boardContainer == nil {
		return
	}
	
	boardContainer.RemoveAll()
	
	boards := getBoards()
	for _, board := range boards {
		boardBtn := widget.NewButton(fmt.Sprintf("%d: %s", board.ID, board.Name), func(b Board) func() {
			return func() {
				currentBoardID = b.ID
				loadBoard(currentBoardID)
			}
		}(board))
		boardContainer.Add(boardBtn)
	}
	
	if mainWindow != nil {
		mainWindow.Content().Refresh()
	}
}

func refreshBoardList() {
	refreshBoardContainer()
}

// Confirmation dialog function
func showConfirmDialog(title, message string, onConfirm func()) {
	cancelBtn := widget.NewButton("Cancel", func() {})
	confirmBtn := widget.NewButton("Delete", func() {})
	
	content := container.NewVBox(
		widget.NewLabel(title),
		widget.NewLabel(message),
		container.NewHBox(cancelBtn, confirmBtn),
	)
	
	dialog := widget.NewModalPopUp(content, mainWindow.Canvas())
	cancelBtn.OnTapped = dialog.Hide
	confirmBtn.OnTapped = func() {
		onConfirm()
		dialog.Hide()
	}
	
	// Style the confirm button to look dangerous
	confirmBtn.Importance = widget.DangerImportance
	
	dialog.Show()
}

// XLSX Export function (from xlsx_exporter.go)
func exportBoardToXLSX(boardID int, outputFile string) error {
	f := excelize.NewFile()
	defer f.Close()

	streamWriter, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return err
	}

	// Write header
	header := []interface{}{"Board", "Swimlane", "List", "Card Title", "Description", "Created At"}
	if err := streamWriter.SetRow("A1", header); err != nil {
		return err
	}

	// Query data
	rows, err := db.Query(`
		SELECT b.name AS board_name, s.name AS swimlane_name, l.name AS list_name,
		       c.title, c.description, c.created_at, c.attachment
		FROM boards b
		JOIN swimlanes s ON b.id = s.board_id
		JOIN lists l ON s.id = l.swimlane_id
		JOIN cards c ON l.id = c.list_id
		WHERE b.id = ?
		ORDER BY s.position, l.position, c.position
	`, boardID)
	if err != nil {
		return err
	}
	defer rows.Close()

	rowNum := 2
	for rows.Next() {
		var boardName, swimlaneName, listName, title, description, createdAt string
		var attachment []byte
		err := rows.Scan(&boardName, &swimlaneName, &listName, &title, &description, &createdAt, &attachment)
		if err != nil {
			continue
		}

		row := []interface{}{boardName, swimlaneName, listName, title, description, createdAt}
		cell, _ := excelize.CoordinatesToCellName(1, rowNum)
		if err := streamWriter.SetRow(cell, row); err != nil {
			continue
		}

		// If attachment exists, add as image
		if len(attachment) > 0 {
			imageCell, _ := excelize.CoordinatesToCellName(7, rowNum)
			f.AddPictureFromBytes("Sheet1", imageCell, &excelize.Picture{
				File:      attachment,
				Extension: ".png",
			})
		}

		rowNum++
	}

	if err := streamWriter.Flush(); err != nil {
		return err
	}

	// Save file
	return f.SaveAs(outputFile)
}

// GUI functions
func createMainWindow(a fyne.App) fyne.Window {
	w := a.NewWindow("Go Kanban Board")
	mainWindow = w

	// Sidebar
	boardContainer = container.NewVBox()
	refreshBoardContainer()

	addBoardBtn := NewTooltipButton("Add Board", "Create a new board", func() {
		showNewBoardDialog()
	})

	cloneBoardBtn := NewTooltipButton("Clone Board", "Clone current board", func() {
		if currentBoardID > 0 {
			cloneBoard(currentBoardID)
			// Refresh board list - simplified
			boards := getBoards()
			if len(boards) > 0 {
				currentBoardID = boards[len(boards)-1].ID
				loadBoard(currentBoardID)
			}
		}
	})

	deleteBoardBtn := NewTooltipButton("Delete Board", "Delete current board", func() {
		if currentBoardID > 0 {
			board := getBoardByID(currentBoardID)
			if board != nil {
				showConfirmDialog(
					"Delete Board",
					fmt.Sprintf("Are you sure you want to delete board '%s'? This will also delete all swimlanes, lists, and cards in this board. This action cannot be undone.", board.Name),
					func() {
						deleteBoard(currentBoardID)
						// Select first available board
						boards := getBoards()
						if len(boards) > 0 {
							currentBoardID = boards[0].ID
							loadBoard(currentBoardID)
						} else {
							currentBoardID = 0
							loadBoard(0)
						}
					},
				)
			}
		}
	})

	editBoardBtn := NewTooltipButton("Edit Board", "Edit current board name and description", func() {
		if currentBoardID > 0 {
			showEditBoardDialog(currentBoardID)
		}
	})

	exportBtn := NewTooltipButton("Export Board", "Export board to Excel (A4 multipage)", func() {
		if currentBoardID > 0 {
			outputFile := fmt.Sprintf("board_%d_export.xlsx", currentBoardID)
			err := exportBoardToXLSX(currentBoardID, outputFile)
			if err != nil {
				fmt.Println("Export failed:", err)
			} else {
				fmt.Println("Exported to", outputFile)
			}
		}
	})

	sidebar := container.NewVBox(
		widget.NewLabel("Boards"),
		boardContainer,
		addBoardBtn,
		editBoardBtn,
		cloneBoardBtn,
		deleteBoardBtn,
		exportBtn,
	)

	// Main area
	mainArea = container.NewVBox()

	content := container.NewHBox(sidebar, mainArea)
	w.SetContent(content)

	// Auto-select first board if available (after mainArea is initialized)
	boards := getBoards()
	if len(boards) > 0 {
		currentBoardID = boards[0].ID
		loadBoard(currentBoardID)
	}

	return w
}

func loadBoard(boardID int) {
	// Update window title with current board
	if boardID > 0 {
		board := getBoardByID(boardID)
		if board != nil {
			mainWindow.SetTitle(fmt.Sprintf("Go Kanban Board - %d: %s", board.ID, board.Name))
		}
	} else {
		mainWindow.SetTitle("Go Kanban Board")
	}
	
	mainArea.RemoveAll()

	swimlanes := getSwimlanes(boardID)
	swimlaneContainers := make([]fyne.CanvasObject, len(swimlanes))
	for i, s := range swimlanes {
		// Swimlane header with move and management buttons
		swimlaneLabel := widget.NewLabel(s.Name)
		swimlaneUpBtn := NewTooltipButton("▲", "Move swimlane up", func() { moveSwimlaneUp(s.ID) })
		swimlaneDownBtn := NewTooltipButton("▼", "Move swimlane down", func() { moveSwimlaneDown(s.ID) })
		addSwimlaneBtn := NewTooltipButton("+", "Add swimlane", func() { showNewSwimlaneDialog(s.BoardID) })
		editSwimlaneBtn := NewTooltipButton("E", "Edit swimlane name", func() { showEditSwimlaneDialog(s.ID) })
		cloneSwimlaneBtn := NewTooltipButton("C", "Clone swimlane", func() { cloneSwimlane(s.ID); loadBoard(s.BoardID) })
		deleteSwimlaneBtn := NewTooltipButton("X", "Delete swimlane", func() { 
			showConfirmDialog(
				"Delete Swimlane", 
				fmt.Sprintf("Are you sure you want to delete swimlane '%s'? This will also delete all lists and cards in this swimlane. This action cannot be undone.", s.Name),
				func() { deleteSwimlane(s.ID); loadBoard(s.BoardID) },
			)
		})
		swimlaneHeader := container.NewHBox(swimlaneLabel, swimlaneUpBtn, swimlaneDownBtn, addSwimlaneBtn, editSwimlaneBtn, cloneSwimlaneBtn, deleteSwimlaneBtn)

		lists := getLists(s.ID)
		listContainers := make([]fyne.CanvasObject, len(lists))
		for j, l := range lists {
			// List header with move and management buttons
			listLabel := widget.NewLabel(l.Name)
			listUpBtn := NewTooltipButton("▲", "Move list to above swimlane", func() { moveListToAboveSwimlane(l.ID) })
			listDownBtn := NewTooltipButton("▼", "Move list to below swimlane", func() { moveListToBelowSwimlane(l.ID) })
			listLeftBtn := NewTooltipButton("◀", "Move list to the left", func() { moveListLeft(l.ID) })
			listRightBtn := NewTooltipButton("▶", "Move list to the right", func() { moveListRight(l.ID) })
			addListBtn := NewTooltipButton("+", "Add list", func() { showNewListDialog(l.SwimlaneID) })
			editListBtn := NewTooltipButton("E", "Edit list name", func() { showEditListDialog(l.ID) })
			cloneListBtn := NewTooltipButton("C", "Clone list", func() { cloneList(l.ID); loadBoard(s.BoardID) })
			deleteListBtn := NewTooltipButton("X", "Delete list", func() { 
				showConfirmDialog(
					"Delete List",
					fmt.Sprintf("Are you sure you want to delete list '%s'? This will also delete all cards in this list. This action cannot be undone.", l.Name),
					func() { deleteList(l.ID); loadBoard(s.BoardID) },
				)
			})
			listHeader := container.NewHBox(listLabel, listUpBtn, listDownBtn, listLeftBtn, listRightBtn, addListBtn, editListBtn, cloneListBtn, deleteListBtn)

			cards := getCards(l.ID)
			
			// Create draggable list container
			draggableList := &DraggableList{
				ListID:     l.ID,
				SwimlaneID: s.ID,
				Container:  container.NewVBox(),
			}
			draggableList.Container.Add(listHeader)
			
			// Add cards
			for _, c := range cards {
				// Create a draggable card widget with move buttons
				draggableCard := &DraggableCard{
					CardID: c.ID,
					ListID: l.ID,
					Card:   widget.NewCard("", c.Title, widget.NewLabel(c.Description)),
				}
				
				// Add move and management buttons to the card
				cardUpBtn := NewTooltipButton("▲", "Move card up", func() { moveCardUp(c.ID) })
				cardDownBtn := NewTooltipButton("▼", "Move card down", func() { moveCardDown(c.ID) })
				cardLeftBtn := NewTooltipButton("◀", "Move card to list at left", func() { moveCardToLeftList(c.ID) })
				cardRightBtn := NewTooltipButton("▶", "Move card to list at right", func() { moveCardToRightList(c.ID) })
				addCardBtn := NewTooltipButton("+", "Add card", func() { showNewCardDialog(c.ListID) })
				editCardBtn := NewTooltipButton("E", "Edit card title and description", func() { showEditCardDialog(c.ID) })
				cloneCardBtn := NewTooltipButton("C", "Clone card", func() { cloneCard(c.ID); loadBoard(s.BoardID) })
				deleteCardBtn := NewTooltipButton("X", "Delete card", func() { 
					showConfirmDialog(
						"Delete Card",
						fmt.Sprintf("Are you sure you want to delete card '%s'? This action cannot be undone.", c.Title),
						func() { deleteCard(c.ID); loadBoard(s.BoardID) },
					)
				})
				
				// Create a container for the card with buttons
				cardContainer := container.NewVBox(
					container.NewHBox(cardUpBtn, cardDownBtn, cardLeftBtn, cardRightBtn, addCardBtn, editCardBtn, cloneCardBtn, deleteCardBtn),
					draggableCard.Card,
				)
				
				draggableList.Container.Add(cardContainer)
			}

			listContainers[j] = draggableList.Container
		}

		// Create droppable swimlane container
		droppableSwimlane := &DroppableSwimlane{
			SwimlaneID: s.ID,
			Container:  container.NewVBox(),
		}
		droppableSwimlane.Container.Add(swimlaneHeader)
		droppableSwimlane.Container.Add(container.NewHBox(listContainers...))
		
		swimlaneContainers[i] = droppableSwimlane.Container
	}

	mainArea.Add(container.NewVBox(swimlaneContainers...))
	mainArea.Refresh()
	
	// Also refresh the window to ensure the layout updates
	if mainWindow != nil {
		mainWindow.Canvas().Refresh(mainArea)
	}
}

func main() {
	initDatabase()
	defer db.Close()

	a := app.New()
	w := createMainWindow(a)
	w.ShowAndRun()
}