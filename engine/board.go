package engine

import (
	"fmt"
	"strings"
)

type Board struct {
	Points             [24]int8
	WhiteBar, BlackBar int8
	WhiteOff, BlackOff int8
	WhiteToMove        bool
}

// Standard opening position
// White is positive values, black is negative. Values are number of checkers on the point.
// Each side has 15 checkers total. Well have white start on 24 going backwards, black on 0 going forwards.
// Standard Setup:
//
//	6 | 6
//	6 | 6
func Start() Board {
	return Board{
		Points: [24]int8{
			-2, 0, 0, 0, 0, 5, 0, 3, 0, 0, 0, -5,
			5, 0, 0, 0, -3, 0, -5, 0, 0, 0, 0, 2,
		},
		WhiteBar:    0,
		BlackBar:    0,
		WhiteOff:    0,
		BlackOff:    0,
		WhiteToMove: true,
	}
}

// Compact one-line debug representation: occupied points as point:signedCount
// (+ = White, - = Black), plus bar/off counts and side to move.
func (b Board) String() string {
	var sb strings.Builder
	for i, n := range b.Points {
		if n != 0 {
			fmt.Fprintf(&sb, "%d:%+d ", i+1, n)
		}
	}
	turn := "B"
	if b.WhiteToMove {
		turn = "W"
	}
	fmt.Fprintf(&sb, "| bar W%d B%d | off W%d B%d | turn %s",
		b.WhiteBar, b.BlackBar, b.WhiteOff, b.BlackOff, turn)
	return sb.String()
}

// Render draws the full 2D ASCII board with both bars, off counts, and side to
// move. W = White (positive), B = Black (negative).
func (b Board) Render() string {
	abs := func(v int8) int {
		if v < 0 {
			return -int(v)
		}
		return int(v)
	}
	sym := func(v int8) string {
		if v > 0 {
			return "W"
		}
		return "B"
	}
	// token renders a 3-char cell for point index idx at stack depth.
	token := func(idx, depth int) string {
		v := b.Points[idx]
		n := abs(v)
		if n == 0 {
			return "   "
		}
		if n > 5 && depth == 4 {
			return fmt.Sprintf("%2d ", n)
		}
		if depth < n {
			return " " + sym(v) + " "
		}
		return "   "
	}

	topLeft := []int{13, 14, 15, 16, 17, 18}
	topRight := []int{19, 20, 21, 22, 23, 24}
	botLeft := []int{12, 11, 10, 9, 8, 7}
	botRight := []int{6, 5, 4, 3, 2, 1}

	row := func(pts []int, depth int) string {
		var s strings.Builder
		for _, p := range pts {
			s.WriteString(token(p-1, depth))
		}
		return s.String()
	}

	var sb strings.Builder
	sb.WriteString("+13-14-15-16-17-18-+-BAR-+19-20-21-22-23-24-+\n")
	for d := 0; d < 5; d++ {
		fmt.Fprintf(&sb, "|%s|     |%s|\n", row(topLeft, d), row(topRight, d))
	}
	sb.WriteString("|                  |     |                  |\n")
	for d := 4; d >= 0; d-- {
		fmt.Fprintf(&sb, "|%s|     |%s|\n", row(botLeft, d), row(botRight, d))
	}
	sb.WriteString("+12-11-10--9--8--7-+-BAR-+-6--5--4--3--2--1-+\n")

	fmt.Fprintf(&sb, "Bar: X=%d O=%d   Off: X=%d O=%d\n",
		b.WhiteBar, b.BlackBar, b.WhiteOff, b.BlackOff)
	turn := "O (Black)"
	if b.WhiteToMove {
		turn = "X (White)"
	}
	fmt.Fprintf(&sb, "Turn: %s\n", turn)
	return sb.String()
}

// Reverse points, negate, swap bars/off, flip turn
func (b Board) Mirror() Board {
	var m Board
	for i := 0; i < 24; i++ {
		m.Points[i] = -b.Points[23-i]
	}
	m.WhiteBar, m.BlackBar = b.BlackBar, b.WhiteBar
	m.WhiteOff, m.BlackOff = b.BlackOff, b.WhiteOff
	m.WhiteToMove = !b.WhiteToMove
	return m
}

func (b Board) Check() error {
	whiteTotal := b.WhiteOff + b.WhiteBar
	blackTotal := b.BlackOff + b.BlackBar
	for _, n := range b.Points {
		if n > 0 {
			whiteTotal += int8(n)
		} else {
			blackTotal += -n
		}
	}
	if whiteTotal != 15 {
		return fmt.Errorf("white total is %d, expected 15", whiteTotal)
	}
	if blackTotal != 15 {
		return fmt.Errorf("black total is %d, expected 15", blackTotal)
	}
	return nil
}

// Generate all legal board states after applying given 2 dice to the board
// This is the recursive helper function for gen()
// It takes the current board, the two dice, and a slice to store the legal states
// It returns a slice of legal boards
// Should also handle the case where the dice are the same, and the case where the dice are different
// D1=D2=0 means no dice were rolled (end of the line)
func LegalAfterstates(b Board, d1, d2 int) []Board {
	dice := []int{d1, d2}
	if d1 == d2 {
		dice = []int{d1, d1, d1, d1}
	}
	out := map[Board]int{}
	gen(b, dice, 0, out)
	if d1 != d2 {
		gen(b, []int{d2, d1}, 0, out)
	}
	states := make([]Board, 0, len(out))
	maxDepth := 0

	for _, d := range out {
		if d > maxDepth {
			maxDepth = d
		}
	}

	if maxDepth == 1 && d1 != d2 {
		big, small := d1, d2
		if small > big {
			big, small = small, big
		}
		out = map[Board]int{}
		gen(b, []int{big}, 0, out)
		if len(out) == 0 {
			gen(b, []int{small}, 0, out)
		}
	}

	for nb, d := range out {
		if d != maxDepth {
			continue
		}
		nb.WhiteToMove = !nb.WhiteToMove
		nb.Check()
		states = append(states, nb)
	}
	if len(out) == 0 {
		return []Board{skip(b)}
	}
	return states
}

func gen(b Board, dice []int, depth int, out map[Board]int) {
	if len(dice) == 0 {
		return
	}

	for from := 0; from < 25; from++ {
		nb, ok := applyDie(b, from, dice[0])
		if !ok {
			continue
		}
		if depth+1 > out[nb] {
			out[nb] = depth + 1
		}
		gen(nb, dice[1:], depth+1, out)
	}
}

func skip(b Board) Board {
	b.WhiteToMove = !b.WhiteToMove
	return b
}
