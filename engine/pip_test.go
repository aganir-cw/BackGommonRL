package engine

import "testing"

func TestPipCount(t *testing.T) {
	board := Start()
	if PipCount(board, true) != 167 {
		t.Errorf("PipCount(Start(), true) = %d, want 167", PipCount(board, true))
	}
}
