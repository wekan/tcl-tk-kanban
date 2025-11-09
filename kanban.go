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

	return w
}

func loadBoard(boardID int) {
	mainArea.RemoveAll()

	swimlanes := getSwimlanes(boardID)
	swimlaneContainers := make([]fyne.CanvasObject, len(swimlanes))
	for i, s := range swimlanes {
		swimlaneLabel := widget.NewLabel(s.Name)

		lists := getLists(s.ID)
		listContainers := make([]fyne.CanvasObject, len(lists))
		for j, l := range lists {
			listLabel := widget.NewLabel(l.Name)

			cards := getCards(l.ID)
			cardLabels := make([]fyne.CanvasObject, len(cards))
			for k, c := range cards {
				cardLabels[k] = widget.NewLabel(c.Title)
			}

			listContainer := container.NewVBox(
				append([]fyne.CanvasObject{listLabel}, cardLabels...)...,
			)
			listContainers[j] = listContainer
		}

		swimlaneContainer := container.NewVBox(
			swimlaneLabel,
			container.NewHBox(listContainers...),
		)
		swimlaneContainers[i] = swimlaneContainer
	}

	mainArea.Add(container.NewHBox(swimlaneContainers...))
	mainArea.Refresh()
}

func main() {
	initDatabase()
	defer db.Close()

	a := app.New()
	w := createMainWindow(a)
	w.ShowAndRun()
}