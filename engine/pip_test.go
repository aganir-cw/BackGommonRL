package engine

import (
	"testing"
)

func TestPipCount(t *testing.T) {
	board := Start()
	if PipCount(board, true) != 167 {
		t.Errorf("PipCount(Start(), true) = %d, want 167", PipCount(board, true))
	}

	board.Points[23] -= 1
	board.WhiteBar += 1
	t.Log(board.Render())
	if PipCount(board, true) != 168 {
		t.Errorf("PipCount(Start(), true) = %d, want 168", PipCount(board, true))
	}
}
