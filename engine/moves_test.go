package engine

import (
	"fmt"
	"testing"
)

func mkBoard(white map[int]int8, black map[int]int8, wBar, bBar, wOff, bOff int8, whiteToMove bool) Board {
	b := Board{
		WhiteBar:    wBar,
		BlackBar:    bBar,
		WhiteOff:    wOff,
		BlackOff:    bOff,
		WhiteToMove: whiteToMove,
	}
	var wTotal, bTotal int8 = wBar + wOff, bBar + bOff
	for p, n := range white {
		b.Points[p] = n
		wTotal += n
	}
	for p, n := range black {
		b.Points[p] = -n
		bTotal += n
	}

	if wTotal != 15 || bTotal != 15 {
		panic(fmt.Sprintf("mkBoard totals: white=%d black=%d, want 15/15", wTotal, bTotal))
	}
	return b
}

// White moves from high index toward 0 (target = from - die) and never flips the
// turn inside applyDie. Home board is indices 0-5.

// Simple in-board move onto an empty point.
func TestApplyDieSimpleMove(t *testing.T) {
	b := mkBoard(map[int]int8{12: 1, 5: 14}, map[int]int8{6: 1, 18: 14}, 0, 0, 0, 0, true)
	nb, ok := applyDie(b, 12, 4) // 12 -> 8, empty
	if !ok {
		t.Fatal("expected legal move")
	}
	if nb.Points[12] != 0 || nb.Points[8] != 1 {
		t.Fatalf("expected checker moved 12->8, got Points[12]=%d Points[8]=%d", nb.Points[12], nb.Points[8])
	}
	if !nb.WhiteToMove {
		t.Fatal("applyDie must not flip the turn")
	}
}

// Target occupied by 2+ opponents is illegal and leaves the board unchanged.
func TestApplyDieBlockedPoint(t *testing.T) {
	b := mkBoard(map[int]int8{12: 1, 5: 14}, map[int]int8{8: 2, 18: 13}, 0, 0, 0, 0, true)
	nb, ok := applyDie(b, 12, 4) // 12 -> 8, blocked by 2 black
	if ok {
		t.Fatal("expected illegal move into blocked point")
	}
	if nb != b {
		t.Fatal("illegal move must return the board unchanged")
	}
}

// Landing on a lone opponent blot sends it to the bar.
func TestApplyDieHit(t *testing.T) {
	b := mkBoard(map[int]int8{12: 1, 5: 14}, map[int]int8{8: 1, 18: 14}, 0, 0, 0, 0, true)
	nb, ok := applyDie(b, 12, 4) // 12 -> 8, hits black blot
	if !ok {
		t.Fatal("expected legal hit")
	}
	if nb.Points[8] != 1 {
		t.Fatalf("expected white on 8 after hit, got %d", nb.Points[8])
	}
	if nb.BlackBar != 1 {
		t.Fatalf("expected BlackBar=1 after hit, got %d", nb.BlackBar)
	}
}

// With a checker on the bar, any non-bar source is illegal.
func TestApplyDieBarEntryForced(t *testing.T) {
	b := mkBoard(map[int]int8{5: 14}, map[int]int8{18: 13, 20: 2}, 1, 0, 0, 0, true)
	_, ok := applyDie(b, 5, 3)
	if ok {
		t.Fatal("must play the bar checker first; from=5 should be illegal")
	}
}

// Legal bar entry onto an empty point.
func TestApplyDieBarEntry(t *testing.T) {
	b := mkBoard(map[int]int8{5: 14}, map[int]int8{20: 8, 22: 7}, 1, 0, 0, 0, true)
	nb, ok := applyDie(b, FromBar, 6) // enters at 24-6 = 18, empty
	if !ok {
		t.Fatal("expected legal bar entry")
	}
	if nb.WhiteBar != 0 || nb.Points[18] != 1 {
		t.Fatalf("expected entry onto 18, got WhiteBar=%d Points[18]=%d", nb.WhiteBar, nb.Points[18])
	}
}

// Bar entry that hits a blot.
func TestApplyDieBarEntryHit(t *testing.T) {
	b := mkBoard(map[int]int8{5: 14}, map[int]int8{18: 1, 20: 14}, 1, 0, 0, 0, true)
	nb, ok := applyDie(b, FromBar, 6) // enters at 18, hits black blot
	if !ok {
		t.Fatal("expected legal bar entry with hit")
	}
	if nb.WhiteBar != 0 || nb.Points[18] != 1 || nb.BlackBar != 1 {
		t.Fatalf("expected entry+hit, got WhiteBar=%d Points[18]=%d BlackBar=%d", nb.WhiteBar, nb.Points[18], nb.BlackBar)
	}
}

// Bar entry onto a point held by 2+ opponents is illegal.
func TestApplyDieBarEntryBlocked(t *testing.T) {
	b := mkBoard(map[int]int8{5: 14}, map[int]int8{18: 2, 20: 13}, 1, 0, 0, 0, true)
	_, ok := applyDie(b, FromBar, 6) // entry 18 blocked by 2 black
	if ok {
		t.Fatal("expected blocked bar entry to be illegal")
	}
}

// Exact roll bears a checker off.
func TestApplyDieBearOffExact(t *testing.T) {
	b := mkBoard(map[int]int8{5: 2, 3: 13}, map[int]int8{18: 13, 20: 2}, 0, 0, 0, 0, true)
	nb, ok := applyDie(b, 5, 6) // distance-to-off of point index 5 is 6 -> exact
	if !ok {
		t.Fatal("expected legal exact bear-off")
	}
	if nb.Points[5] != 1 || nb.WhiteOff != 1 {
		t.Fatalf("expected one borne off from 5, got Points[5]=%d WhiteOff=%d", nb.Points[5], nb.WhiteOff)
	}
}

// A roll larger than needed may bear off from the highest occupied point.
func TestApplyDieBearOffOvershootFromHighest(t *testing.T) {
	b := mkBoard(map[int]int8{4: 2, 3: 13}, map[int]int8{18: 13, 20: 2}, 0, 0, 0, 0, true)
	nb, ok := applyDie(b, 4, 6) // no checker higher than index 4 -> overshoot legal
	if !ok {
		t.Fatal("expected legal overshoot bear-off from highest point")
	}
	if nb.Points[4] != 1 || nb.WhiteOff != 1 {
		t.Fatalf("expected one borne off from 4, got Points[4]=%d WhiteOff=%d", nb.Points[4], nb.WhiteOff)
	}
}

// Overshoot is illegal when a higher point is still occupied.
func TestApplyDieBearOffOvershootNotHighest(t *testing.T) {
	b := mkBoard(map[int]int8{5: 1, 2: 1, 0: 13}, map[int]int8{18: 13, 20: 2}, 0, 0, 0, 0, true)
	nb, ok := applyDie(b, 2, 6) // point index 5 still occupied -> must move it, not bear off from 2
	if ok {
		t.Fatal("expected illegal overshoot from a non-highest point")
	}
	if nb != b {
		t.Fatal("illegal bear-off must return the board unchanged")
	}
}

// Bearing off is illegal unless every checker is home.
func TestApplyDieBearOffNotAllHome(t *testing.T) {
	b := mkBoard(map[int]int8{5: 1, 10: 1, 0: 13}, map[int]int8{18: 13, 20: 2}, 0, 0, 0, 0, true)
	_, ok := applyDie(b, 5, 6) // a white checker sits on 10, outside home
	if ok {
		t.Fatal("expected illegal bear-off with a checker outside home")
	}
}

// Black is handled via mirror: exact bear-off for black, turn not flipped.
func TestApplyDieBlackBearOff(t *testing.T) {
	b := mkBoard(map[int]int8{5: 2, 3: 13}, map[int]int8{18: 2, 20: 13}, 0, 0, 0, 0, false)
	nb, ok := applyDie(b, 18, 6) // black home is 18-23; distance-to-off of 18 is 6 -> exact
	if !ok {
		t.Fatal("expected legal exact bear-off for black")
	}
	if nb.Points[18] != -1 || nb.BlackOff != 1 {
		t.Fatalf("expected one black borne off from 18, got Points[18]=%d BlackOff=%d", nb.Points[18], nb.BlackOff)
	}
	if nb.WhiteToMove {
		t.Fatal("applyDie must not flip the turn")
	}
}
