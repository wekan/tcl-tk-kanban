#!/usr/bin/env tclsh
# Test script to verify button visibility

package require Tk

wm title . "Button Test"
wm geometry . 400x300

label .info -text "Testing button text visibility" -font {-size 14 -weight bold}
pack .info -pady 20

# Test the problematic "+ List" button style
frame .test1 -bg #2196F3 -height 40
pack .test1 -fill x -padx 20 -pady 10

button .test1.addlist -text "+ List" \
    -bg #1976D2 -fg white \
    -activebackground #1565C0 -activeforeground white \
    -relief raised -borderwidth 1 -highlightthickness 0
pack .test1.addlist -side right -padx 5 -pady 5

label .test1.label -text "Swimlane Header (Blue bg):" -bg #2196F3 -fg white -anchor w
pack .test1.label -side left -padx 10 -pady 5 -fill x -expand 1

# Test the "+ New Board" button style
button .newboard -text "+ New Board" \
    -bg #4CAF50 -fg white -activebackground #45a049 -activeforeground white \
    -relief raised -borderwidth 1 -highlightthickness 0
pack .newboard -fill x -padx 20 -pady 10

# Test the "+ Add Card" button style
button .addcard -text "+ Add Card" \
    -bg #e0e0e0 -fg black -activebackground #d0d0d0 -activeforeground black \
    -relief raised -borderwidth 1 -highlightthickness 0
pack .addcard -fill x -padx 20 -pady 10

label .instructions -text "All button text should be visible at all times.\nClick buttons to verify active state also shows text." \
    -justify left -wraplength 350
pack .instructions -pady 20
