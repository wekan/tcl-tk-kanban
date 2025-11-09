package main

import (
	"database/sql"
	"fmt"
	"os"
	"log"
	_ "github.com/mattn/go-sqlite3"
	"github.com/xuri/excelize/v2"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: xlsx_exporter <boardId> <output.xlsx>")
		os.Exit(1)
	}
	boardId := os.Args[1]
	output := os.Args[2]
	db, err := sql.Open("sqlite3", "wekan.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	f := excelize.NewFile()
	streamWriter, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		log.Fatal(err)
	}

	row := 1
	// Board info
	var boardName, boardDesc string
	db.QueryRow("SELECT name, description FROM boards WHERE id = ?", boardId).Scan(&boardName, &boardDesc)
	streamWriter.SetRow(fmt.Sprintf("A%d", row), []interface{}{"Board:", boardName})
	row++
	streamWriter.SetRow(fmt.Sprintf("A%d", row), []interface{}{"Description:", boardDesc})
	row++

	// Swimlanes
	swimRows, _ := db.Query("SELECT id, name FROM swimlanes WHERE board_id = ? ORDER BY position", boardId)
	defer swimRows.Close()
	for swimRows.Next() {
		var swimlaneId int
		var swimlaneName string
		swimRows.Scan(&swimlaneId, &swimlaneName)
		streamWriter.SetRow(fmt.Sprintf("A%d", row), []interface{}{"Swimlane:", swimlaneName})
		row++
		// Lists
		listRows, _ := db.Query("SELECT id, name FROM lists WHERE swimlane_id = ? ORDER BY position", swimlaneId)
		defer listRows.Close()
		for listRows.Next() {
			var listId int
			var listName string
			listRows.Scan(&listId, &listName)
			streamWriter.SetRow(fmt.Sprintf("B%d", row), []interface{}{"List:", listName})
			row++
			// Cards
			cardRows, _ := db.Query("SELECT title, description, attachment FROM cards WHERE list_id = ? ORDER BY position", listId)
			defer cardRows.Close()
			for cardRows.Next() {
				var cardTitle, cardDesc string
				var cardAttachment []byte
				cardRows.Scan(&cardTitle, &cardDesc, &cardAttachment)
				streamWriter.SetRow(fmt.Sprintf("C%d", row), []interface{}{"Card:", cardTitle})
				if cardDesc != "" {
					streamWriter.SetRow(fmt.Sprintf("D%d", row), []interface{}{"Description:", cardDesc})
				}
				if len(cardAttachment) > 0 {
					imgFile := fmt.Sprintf("card_img_%d.png", row)
					os.WriteFile(imgFile, cardAttachment, 0644)
					f.AddPicture("Sheet1", fmt.Sprintf("E%d", row), imgFile, &excelize.GraphicOptions{AutoFit: true})
					os.Remove(imgFile)
				}
				row++
			}
		}
	}
	streamWriter.Flush()
	if err := f.SaveAs(output); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Board exported to %s\n", output)
}
