package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/xuri/excelize/v2"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: xlsx_exporter <boardId> <outputFile>")
		os.Exit(1)
	}

	boardIdStr := os.Args[1]
	outputFile := os.Args[2]

	boardId, err := strconv.Atoi(boardIdStr)
	if err != nil {
		fmt.Printf("Invalid board ID: %v\n", err)
		os.Exit(1)
	}

	// Open database
	db, err := sql.Open("sqlite3", "wekan.db")
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create Excel file
	f := excelize.NewFile()
	defer f.Close()

	streamWriter, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		fmt.Printf("Failed to create stream writer: %v\n", err)
		os.Exit(1)
	}

	// Write header
	header := []interface{}{"Board", "Swimlane", "List", "Card Title", "Description", "Created At"}
	if err := streamWriter.SetRow("A1", header); err != nil {
		fmt.Printf("Failed to set header: %v\n", err)
		os.Exit(1)
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
	`, boardId)
	if err != nil {
		fmt.Printf("Failed to query data: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	rowNum := 2
	for rows.Next() {
		var boardName, swimlaneName, listName, title, description, createdAt string
		var attachment []byte
		err := rows.Scan(&boardName, &swimlaneName, &listName, &title, &description, &createdAt, &attachment)
		if err != nil {
			fmt.Printf("Failed to scan row: %v\n", err)
			continue
		}

		row := []interface{}{boardName, swimlaneName, listName, title, description, createdAt}
		cell, _ := excelize.CoordinatesToCellName(1, rowNum)
		if err := streamWriter.SetRow(cell, row); err != nil {
			fmt.Printf("Failed to set row: %v\n", err)
			continue
		}

		// If attachment exists, add as image (simplified, assuming PNG)
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
		fmt.Printf("Failed to flush stream: %v\n", err)
		os.Exit(1)
	}

	// Save file
	if err := f.SaveAs(outputFile); err != nil {
		fmt.Printf("Failed to save file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Exported board %d to %s\n", boardId, outputFile)
}