package engine

import (
	"math/rand"
	"testing"
)

func TestStartPosition(t *testing.T) {
	board := Start()

	// White uses its own numbering: point P lives at index P-1.
	// Black's point P is the mirror of White's, at index 24-P.
	cases := []struct {
		name string
		idx  int
		want int8
	}{
		{"White 24-point", 24 - 1, 2},
		{"White 13-point", 13 - 1, 5},
		{"White 8-point", 8 - 1, 3},
		{"White 6-point", 6 - 1, 5},
		{"Black 24-point", 24 - 24, -2},
		{"Black 13-point", 24 - 13, -5},
		{"Black 8-point", 24 - 8, -3},
		{"Black 6-point", 24 - 6, -5},
	}

	for _, c := range cases {
		if got := board.Points[c.idx]; got != c.want {
			t.Errorf("%s (index %d): got %d, want %d", c.name, c.idx, got, c.want)
		}
	}

	// Sanity: 15 checkers per side, nothing on bar/off, White to move.
	var white, black int
	for _, p := range board.Points {
		switch {
		case p > 0:
			white += int(p)
		case p < 0:
			black += int(-p)
		}
	}
	if white != 15 || black != 15 {
		t.Errorf("checker totals: white=%d black=%d, want 15/15", white, black)
	}
	if board.WhiteBar != 0 || board.BlackBar != 0 || board.WhiteOff != 0 || board.BlackOff != 0 {
		t.Errorf("bar/off should be zero at start: %+v", board)
	}
	if !board.WhiteToMove {
		t.Error("White should move first")
	}
}

func TestMirrorInvolution(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	for iter := 0; iter < 100; iter++ {
		b := Start()

		// Hand-perturb: nudge some points and bar/off counts randomly.
		for i := range b.Points {
			b.Points[i] += int8(rng.Intn(5) - 2)
		}
		b.WhiteBar = int8(rng.Intn(4))
		b.BlackBar = int8(rng.Intn(4))
		b.WhiteOff = int8(rng.Intn(16))
		b.BlackOff = int8(rng.Intn(16))
		b.WhiteToMove = rng.Intn(2) == 0

		if got := b.Mirror().Mirror(); got != b {
			t.Fatalf("iter %d: Mirror().Mirror() != original\n got:  %+v\n want: %+v", iter, got, b)
		}
	}
}

func TestCheckStart(t *testing.T) {
	board := Start()

	if err := board.Check(); err != nil {
		t.Errorf("start position failed check: %v", err)
	}
}

func TestCheckCatchesMissingCheckers(t *testing.T) {
	board := Start()
	board.Points[23] = 1
	if err := board.Check(); err == nil {
		t.Errorf("start position should fail check with missing checker")
	} else if err.Error() != "white total is 14, expected 15" {
		t.Errorf("unexpected error: %v", err)
	}
}
