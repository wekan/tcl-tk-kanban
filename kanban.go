package main

import (
	"database/sql"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/xuri/excelize/v2"
	_ "github.com/mattn/go-sqlite3"
	"image/color"
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
var mainArea *container.Scroll
var toolbar *fyne.Container
var mainWindow fyne.Window
var draggedCard *DraggableCard
var draggedList *DraggableList
var draggedSwimlaneID int
var draggingSwimlane bool

// Selection tracking
var selectedSwimlanes = make(map[int]bool)
var selectedLists = make(map[int]bool)
var selectedCards = make(map[int]bool)

// Reorder helpers move an item to a target index and re-pack positions 0..n-1
func reorderCards(listID int, cardID int, newIndex int) {
	cards := getCards(listID)
	// build ordered slice of IDs excluding moved card
	ids := make([]int, 0, len(cards))
	for _, c := range cards {
		if c.ID != cardID {
			ids = append(ids, c.ID)
		}
	}
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex > len(ids) {
		newIndex = len(ids)
	}
	// insert cardID at newIndex
	ids = append(ids[:newIndex], append([]int{cardID}, ids[newIndex:]...)...)
	// persist positions
	for i, id := range ids {
		db.Exec("UPDATE cards SET position = ? WHERE id = ?", i, id)
	}
}

func reorderLists(swimlaneID int, listID int, newIndex int) {
	lists := getLists(swimlaneID)
	ids := make([]int, 0, len(lists))
	for _, l := range lists {
		if l.ID != listID {
			ids = append(ids, l.ID)
		}
	}
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex > len(ids) {
		newIndex = len(ids)
	}
	ids = append(ids[:newIndex], append([]int{listID}, ids[newIndex:]...)...)
	for i, id := range ids {
		db.Exec("UPDATE lists SET position = ? WHERE id = ?", i, id)
	}
}

func reorderSwimlanes(boardID int, swimlaneID int, newIndex int) {
	swimlanes := getSwimlanes(boardID)
	ids := make([]int, 0, len(swimlanes))
	for _, s := range swimlanes {
		if s.ID != swimlaneID {
			ids = append(ids, s.ID)
		}
	}
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex > len(ids) {
		newIndex = len(ids)
	}
	ids = append(ids[:newIndex], append([]int{swimlaneID}, ids[newIndex:]...)...)
	for i, id := range ids {
		db.Exec("UPDATE swimlanes SET position = ? WHERE id = ?", i, id)
	}
}

// Selection operations
func moveSelectedUp() {
	for id := range selectedCards {
		moveCardUp(id)
	}
	for id := range selectedLists {
		moveListToAboveSwimlane(id)
	}
	for id := range selectedSwimlanes {
		moveSwimlaneUp(id)
	}
	loadBoard(currentBoardID)
}

func moveSelectedDown() {
	for id := range selectedCards {
		moveCardDown(id)
	}
	for id := range selectedLists {
		moveListToBelowSwimlane(id)
	}
	for id := range selectedSwimlanes {
		moveSwimlaneDown(id)
	}
	loadBoard(currentBoardID)
}

func moveSelectedLeft() {
	for id := range selectedCards {
		moveCardToLeftList(id)
	}
	for id := range selectedLists {
		moveListLeft(id)
	}
	loadBoard(currentBoardID)
}

func moveSelectedRight() {
	for id := range selectedCards {
		moveCardToRightList(id)
	}
	for id := range selectedLists {
		moveListRight(id)
	}
	loadBoard(currentBoardID)
}

func editSelected() {
	for id := range selectedCards {
		showEditCardDialog(id)
		return // Edit one at a time
	}
	for id := range selectedLists {
		showEditListDialog(id)
		return
	}
	for id := range selectedSwimlanes {
		showEditSwimlaneDialog(id)
		return
	}
}

func cloneSelected() {
	for id := range selectedCards {
		cloneCard(id)
	}
	for id := range selectedLists {
		cloneList(id)
	}
	for id := range selectedSwimlanes {
		cloneSwimlane(id)
	}
	loadBoard(currentBoardID)
}

func deleteSelected() {
	// Build confirmation message
	msg := "Are you sure you want to delete:\n"
	if len(selectedCards) > 0 {
		msg += fmt.Sprintf("- %d card(s)\n", len(selectedCards))
	}
	if len(selectedLists) > 0 {
		msg += fmt.Sprintf("- %d list(s)\n", len(selectedLists))
	}
	if len(selectedSwimlanes) > 0 {
		msg += fmt.Sprintf("- %d swimlane(s)\n", len(selectedSwimlanes))
	}
	msg += "\nThis action cannot be undone."
	
	showConfirmDialog("Delete Selected Items", msg, func() {
		for id := range selectedCards {
			deleteCard(id)
		}
		for id := range selectedLists {
			deleteList(id)
		}
		for id := range selectedSwimlanes {
			deleteSwimlane(id)
		}
		// Clear selections
		selectedCards = make(map[int]bool)
		selectedLists = make(map[int]bool)
		selectedSwimlanes = make(map[int]bool)
		loadBoard(currentBoardID)
	})
}

func clearSelections() {
	selectedCards = make(map[int]bool)
	selectedLists = make(map[int]bool)
	selectedSwimlanes = make(map[int]bool)
	loadBoard(currentBoardID)
}

var boardContainer *fyne.Container

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

// Draggable icon for triggering drag operations
type DraggableIcon struct {
	*widget.Button
	Card   *DraggableCard
	List   *DraggableList
	SwimlaneID int // For swimlane dragging (if implemented)
}

func (d *DraggableIcon) Dragged(ev *fyne.DragEvent) {
	if d.Card != nil {
		draggedCard = d.Card
	} else if d.List != nil {
		draggedList = d.List
	} else if d.SwimlaneID != 0 {
		draggingSwimlane = true
		draggedSwimlaneID = d.SwimlaneID
	}
	// Swimlane dragging not implemented yet
}

func (d *DraggableIcon) DragEnd() {
	draggedCard = nil
	draggedList = nil
	draggingSwimlane = false
}

// Generic drop slot that can accept cards, lists, or swimlanes and place them at a target index
type DropSlot struct {
	*fyne.Container
	Kind       string // "card"|"list"|"swimlane"
	ListID     int    // for card reordering context
	SwimlaneID int    // for list reordering context
	BoardID    int    // for swimlane reordering context
	Index      int    // target index where the dragged item should be inserted
}

func (d *DropSlot) Dragged(ev *fyne.DragEvent) {}

func (d *DropSlot) DragEnd() {}

func (d *DropSlot) Dropped(ev *fyne.DragEvent) {
	switch d.Kind {
	case "card":
		if draggedCard != nil && draggedCard.ListID == d.ListID {
			reorderCards(d.ListID, draggedCard.CardID, d.Index)
			var boardID int
			db.QueryRow(`SELECT s.board_id FROM swimlanes s JOIN lists l ON s.id = l.swimlane_id WHERE l.id = ?`, d.ListID).Scan(&boardID)
			loadBoard(boardID)
		}
	case "list":
		if draggedList != nil && draggedList.SwimlaneID == d.SwimlaneID {
			reorderLists(d.SwimlaneID, draggedList.ListID, d.Index)
			var boardID int
			db.QueryRow("SELECT board_id FROM swimlanes WHERE id = ?", d.SwimlaneID).Scan(&boardID)
			loadBoard(boardID)
		}
	case "swimlane":
		if draggingSwimlane && draggedSwimlaneID != 0 && d.BoardID != 0 {
			reorderSwimlanes(d.BoardID, draggedSwimlaneID, d.Index)
			loadBoard(d.BoardID)
		}
	}
}

func NewDropSlot(kind string, boardID, swimlaneID, listID, index int) *DropSlot {
	rect := canvas.NewRectangle(color.NRGBA{0, 120, 255, 80})
	rect.SetMinSize(fyne.NewSize(16, 16))
	c := container.NewMax(rect)
	slot := &DropSlot{
		Container: c,
		Kind: kind,
		BoardID: boardID,
		SwimlaneID: swimlaneID,
		ListID: listID,
		Index: index,
	}
	return slot
}

// Update list positions in a swimlane
func updateListPositions(swimlaneID int) {
	lists := getLists(swimlaneID)
	for i, list := range lists {
		db.Exec("UPDATE lists SET position = ? WHERE id = ?", i, list.ID)
	}
}

// Update card positions in a list
func updateCardPositions(listID int) {
	cards := getCards(listID)
	for i, card := range cards {
		db.Exec("UPDATE cards SET position = ? WHERE id = ?", i, card.ID)
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

	addBoardBtn := widget.NewButton("Add Board", func() {
		showNewBoardDialog()
	})

	cloneBoardBtn := widget.NewButton("Clone Board", func() {
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

	deleteBoardBtn := widget.NewButton("Delete Board", func() {
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

	editBoardBtn := widget.NewButton("Edit Board", func() {
		if currentBoardID > 0 {
			showEditBoardDialog(currentBoardID)
		}
	})

	exportBtn := widget.NewButton("Export Board", func() {
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
	mainArea = container.NewScroll(container.NewVBox())
	
	// Create toolbar (will be populated in loadBoard)
	toolbar = container.NewVBox()

	// Combine toolbar and main area with Border layout
	mainContent := container.NewBorder(toolbar, nil, nil, nil, mainArea)
	content := container.NewBorder(nil, nil, sidebar, nil, mainContent)
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
	
	// Update toolbar with action buttons
	upBtn := widget.NewButton("▲", moveSelectedUp)
	downBtn := widget.NewButton("▼", moveSelectedDown)
	leftBtn := widget.NewButton("◀", moveSelectedLeft)
	rightBtn := widget.NewButton("▶", moveSelectedRight)
	editBtn := widget.NewButton("Edit", editSelected)
	cloneBtn := widget.NewButton("Clone", cloneSelected)
	deleteBtn := widget.NewButton("Delete", deleteSelected)
	clearBtn := widget.NewButton("Clear Selection", clearSelections)
	
	selectionInfo := widget.NewLabel(fmt.Sprintf("Selected: %d swimlanes, %d lists, %d cards", 
		len(selectedSwimlanes), len(selectedLists), len(selectedCards)))
	
	toolbar.Objects = []fyne.CanvasObject{
		container.NewHBox(
			upBtn, downBtn, leftBtn, rightBtn, editBtn, cloneBtn, deleteBtn, clearBtn,
			layout.NewSpacer(),
			selectionInfo,
		),
		widget.NewSeparator(),
	}
	toolbar.Refresh()
	
	// Clear and set new content
	mainArea.Content = container.NewVBox()

	swimlanes := getSwimlanes(boardID)
	swimlaneContainers := make([]fyne.CanvasObject, len(swimlanes))
	for i, s := range swimlanes {
		// Swimlane header with checkbox and drag handle only
		swimlaneLabel := widget.NewLabel(s.Name)
		swimlaneCheck := widget.NewCheck("", func(checked bool) {
			if checked {
				selectedSwimlanes[s.ID] = true
			} else {
				delete(selectedSwimlanes, s.ID)
			}
			loadBoard(boardID)
		})
		swimlaneCheck.Checked = selectedSwimlanes[s.ID]
		
		swimlaneDragHandle := &DraggableIcon{Button: widget.NewButton("👋", func() {}), SwimlaneID: s.ID}
		swimlaneDragHandle.Resize(fyne.NewSize(30, 30))
		swimlaneHeader := container.NewHBox(swimlaneCheck, swimlaneLabel, layout.NewSpacer(), swimlaneDragHandle)

		lists := getLists(s.ID)
		listRow := make([]fyne.CanvasObject, 0, len(lists)*2+1)
		// leading list drop slot (index 0)
		listRow = append(listRow, NewDropSlot("list", 0, s.ID, 0, 0))
		for j, l := range lists {
			// List header with checkbox and drag handle only
			listLabel := widget.NewLabel(l.Name)
			listCheck := widget.NewCheck("", func(checked bool) {
				if checked {
					selectedLists[l.ID] = true
				} else {
					delete(selectedLists, l.ID)
				}
				loadBoard(boardID)
			})
			listCheck.Checked = selectedLists[l.ID]

			// Create draggable list container
			draggableList := &DraggableList{
				ListID:     l.ID,
				SwimlaneID: s.ID,
				Container:  container.NewVBox(),
			}
			// Drag handle for list
			listHandle := &DraggableIcon{Button: widget.NewButton("👋", func() {}), List: draggableList}
			listHandle.Resize(fyne.NewSize(30, 30))
			listHeader := container.NewHBox(listCheck, listLabel, layout.NewSpacer(), listHandle)
			draggableList.Container.Add(listHeader)

			// Cards with drop slots
			cards := getCards(l.ID)
			cardObjs := make([]fyne.CanvasObject, 0, len(cards)*2+1)
			// leading card slot index 0
			cardObjs = append(cardObjs, NewDropSlot("card", 0, 0, l.ID, 0))
			for idx, c := range cards {
				draggableCard := &DraggableCard{
					CardID: c.ID,
					ListID: l.ID,
					Card:   widget.NewCard("", c.Title, widget.NewLabel(c.Description)),
				}
				
				// Checkbox for card selection
				cardCheck := widget.NewCheck("", func(checked bool) {
					if checked {
						selectedCards[c.ID] = true
					} else {
						delete(selectedCards, c.ID)
					}
					loadBoard(boardID)
				})
				cardCheck.Checked = selectedCards[c.ID]
				
				// drag handle for card
				cardHandle := &DraggableIcon{Button: widget.NewButton("👋", func() {}), Card: draggableCard}
				cardHandle.Resize(fyne.NewSize(25, 25))
				cardTitleContainer := container.NewHBox(
					cardCheck,
					widget.NewLabelWithStyle(c.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					layout.NewSpacer(),
					cardHandle,
				)
				// Recreate card with custom header
				draggableCard.Card = widget.NewCard("", "", container.NewVBox(
					cardTitleContainer,
					widget.NewLabel(c.Description),
				))
				
				cardObjs = append(cardObjs, draggableCard.Card)
				cardObjs = append(cardObjs, NewDropSlot("card", 0, 0, l.ID, idx+1))
			}
			if len(cards) == 0 {
				addCardButton := widget.NewButton("Add Card", func() { showNewCardDialog(l.ID) })
				addCardButton.Importance = widget.HighImportance
				draggableList.Container.Add(container.NewVBox(
					widget.NewLabel("This list has no cards yet."),
					addCardButton,
				))
			} else {
				draggableList.Container.Add(container.NewVBox(cardObjs...))
			}

			listRow = append(listRow, draggableList.Container)
			listRow = append(listRow, NewDropSlot("list", 0, s.ID, 0, j+1))
		}

		// If no lists exist in this swimlane, show "Add List" message/button
		if len(lists) == 0 {
			addListButton := widget.NewButton("Add List", func() { showNewListDialog(s.ID) })
			addListButton.Importance = widget.HighImportance
			listRow = append(listRow, container.NewVBox(
				widget.NewLabel("This swimlane has no lists yet."),
				addListButton,
			))
		}

		// Create droppable swimlane container with swimlane-level drop slots at edges
		droppableSwimlane := &DroppableSwimlane{
			SwimlaneID: s.ID,
			Container:  container.NewVBox(),
		}
		droppableSwimlane.Container.Add(swimlaneHeader)
		// add swimlane drop slots before and after the lists row for reordering
		topSwimlaneSlot := NewDropSlot("swimlane", s.BoardID, 0, 0, i) // index i before current
		bottomSwimlaneSlot := NewDropSlot("swimlane", s.BoardID, 0, 0, i+1)
		droppableSwimlane.Container.Add(topSwimlaneSlot)
		droppableSwimlane.Container.Add(container.NewHBox(listRow...))
		droppableSwimlane.Container.Add(bottomSwimlaneSlot)
		
		swimlaneContainers[i] = droppableSwimlane.Container
	}

	// If no swimlanes exist, show "Add Swimlane" button
	if len(swimlanes) == 0 {
		addSwimlaneBtn := widget.NewButton("➕ Add Swimlane", func() {
			showNewSwimlaneDialog(boardID)
		})
		addSwimlaneBtn.Importance = widget.HighImportance
		swimlaneContainers = append(swimlaneContainers, container.NewVBox(
			widget.NewLabel("This board is empty. Add your first swimlane to get started."),
			addSwimlaneBtn,
		))
	}

	// Update main area content (swimlanes only, toolbar is separate)
	mainArea.Content = container.NewVBox(swimlaneContainers...)
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