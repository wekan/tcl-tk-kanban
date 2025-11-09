package main
import (
	"fmt"
	"fyne.io/fyne/v2/widget"
)
func main() {
	b := widget.NewButton("test", nil)
	fmt.Printf("Button: %+v\n", b)
}
