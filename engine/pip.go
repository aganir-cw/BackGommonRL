package engine

func PipCount(b Board, white bool) int {
	totalCount := 0
	for i := 0; i < 24; i++ {
		if white && b.Points[i] > 0 {
			total_pips_off := i + 1
			totalCount += total_pips_off*int(b.Points[i]) + int(b.WhiteBar)*25
		} else if !white && b.Points[i] < 0 {
			total_pips_off := 24 - i
			totalCount -= total_pips_off*int(b.Points[i]) + int(b.BlackBar)*25

		}
	}
	return totalCount
}

func PipPick(boards []Board, whiteToMove bool) int {
	bestPip := 0
	for _, board := range boards {
		pipCount := PipCount(board, whiteToMove)
		if pipCount > bestPip {
			bestPip = pipCount
		}
	}
	return bestPip
}
