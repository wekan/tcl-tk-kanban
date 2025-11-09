package main

import (
	"database/sql"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
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
	boardList := widget.NewList(
		func() int {
			return len(getBoards())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			boards := getBoards()
			obj.(*widget.Label).SetText(fmt.Sprintf("%d: %s", boards[id].ID, boards[id].Name))
		},
	)
	boardList.OnSelected = func(id widget.ListItemID) {
		boards := getBoards()
		currentBoardID = boards[id].ID
		loadBoard(currentBoardID)
	}

	addBoardBtn := widget.NewButton("Add Board", func() {
		fmt.Println("Add board not implemented in this version")
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
		boardList,
		addBoardBtn,
		exportBtn,
	)

	// Main area
	mainArea = container.NewVBox()

	content := container.NewHBox(sidebar, mainArea)
	w.SetContent(content)

	// Auto-select first board if available (after mainArea is initialized)
	boards := getBoards()
	if len(boards) > 0 {
		boardList.Select(0)
	}

	return w
}

func loadBoard(boardID int) {
	mainArea.RemoveAll()

	swimlanes := getSwimlanes(boardID)
	swimlaneContainers := make([]fyne.CanvasObject, len(swimlanes))
	for i, s := range swimlanes {
		// Swimlane header with move buttons
		swimlaneLabel := widget.NewLabel(s.Name)
		swimlaneUpBtn := widget.NewButton("▲", func() { moveSwimlaneUp(s.ID) })
		swimlaneDownBtn := widget.NewButton("▼", func() { moveSwimlaneDown(s.ID) })
		swimlaneHeader := container.NewHBox(swimlaneLabel, swimlaneUpBtn, swimlaneDownBtn)

		lists := getLists(s.ID)
		listContainers := make([]fyne.CanvasObject, len(lists))
		for j, l := range lists {
			// List header with move buttons
			listLabel := widget.NewLabel(l.Name)
			listUpBtn := widget.NewButton("▲", func() { moveListToAboveSwimlane(l.ID) })
			listDownBtn := widget.NewButton("▼", func() { moveListToBelowSwimlane(l.ID) })
			listLeftBtn := widget.NewButton("◀", func() { moveListLeft(l.ID) })
			listRightBtn := widget.NewButton("▶", func() { moveListRight(l.ID) })
			listHeader := container.NewHBox(listLabel, listUpBtn, listDownBtn, listLeftBtn, listRightBtn)

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
				
				// Add move buttons to the card
				cardUpBtn := widget.NewButton("▲", func() { moveCardUp(c.ID) })
				cardDownBtn := widget.NewButton("▼", func() { moveCardDown(c.ID) })
				cardLeftBtn := widget.NewButton("◀", func() { moveCardToLeftList(c.ID) })
				cardRightBtn := widget.NewButton("▶", func() { moveCardToRightList(c.ID) })
				
				// Create a container for the card with buttons
				cardContainer := container.NewVBox(
					container.NewHBox(cardUpBtn, cardDownBtn, cardLeftBtn, cardRightBtn),
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