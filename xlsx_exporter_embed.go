// xlsx_exporter_embed.go
// Go shared library for embedding XLSX export into TclKit .kit file
// Exports a C function for Tcl to call

package main

import "C"
import (
	"fmt"
	"github.com/xuri/excelize/v2"
)

//export ExportBoardToXLSX
func ExportBoardToXLSX(boardId C.int, outputFile *C.char) C.int {
	goOutput := C.GoString(outputFile)
	// TODO: Replace with actual SQLite reading and Excelize export logic
	f := excelize.NewFile()
	index, _ := f.NewSheet("Sheet1")
	f.SetCellValue("Sheet1", "A1", fmt.Sprintf("Board ID: %d", int(boardId)))
	f.SetActiveSheet(index)
	if err := f.SaveAs(goOutput); err != nil {
		return 1 // error
	}
	return 0 // success
}

func main() {}
