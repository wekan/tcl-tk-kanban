wm title . "Tcl/Tk kanban with drag drop"
wm geometry . 600x400

# Create background and columns
canvas .c -bg "#f8f9fa" -highlightthickness 0
pack .c -fill both -expand yes

# Draw column boundaries (To Do | Doing | Done)
.c create line 200 0 200 400 -fill "#ddd"
.c create line 400 0 400 400 -fill "#ddd"
.c create text 100 20 -text "TODO" -font {Helvetica 12 bold}
.c create text 300 20 -text "DOING" -font {Helvetica 12 bold}
.c create text 500 20 -text "DONE" -font {Helvetica 12 bold}

set card_number 0

proc create_card {x y text color} {
  global card_number
  incr card_number
  set tag "card_$card_number"

  # Create background and text with the same unique tag
  .c create rectangle $x $y [expr $x+120] [expr $y+60] \
    -fill $color -outline "#333" -width 2 -tags [list "card" $tag]

  .c create text [expr $x+60] [expr $y+30] \
    -text $text -width 110 -justify center -tags [list "card" $tag]

  # Bind events to the entire group (tag)
  .c bind $tag <ButtonPress-1> [list start_dragging %x %y $tag]
  .c bind $tag <B1-Motion> [list drag %x %y $tag]
  .c bind $tag <ButtonRelease-1> [list stop_dragging $tag]
}

proc start_dragging {x y tag} {
  global lastX lastY
  set lastX $x
  set lastY $y
  .c raise $tag ;# Raise the card on top of the others while dragging
}

proc drag {x y tag} {
  global lastX lastY
  set dx [expr $x - $lastX]
  set dy [expr $y - $lastY]

  .c move $tag $dx $dy

  set lastX $x
  set lastY $y
}

proc stop_dragging {tag} {
  # Snap-to-column logic
  set coords [.c coords $tag]
  set x [lindex $coords 0]

  if {$x < 200} { set new_x 40 } \
  elseif {$x < 400} { set new_x 240 } \
  else { set new_x 440 }

  set dx [expr $new_x - $x]
  .c move $tag $dx 0
}

# Adding test cards
create_card 40 50 "Buy milk" "#ffffff"
create_card 40 130 "Fix Tcl bug" "#e1f5fe"
create_card 240 50 "Code with Zig compiler" "#fff9c4"
