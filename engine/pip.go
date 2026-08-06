package engine

import "math"

func PipCount(b Board, white bool) int {
	totalCount := 0
	for i := 0; i < 24; i++ {
		if white && b.Points[i] > 0 {
			total_pips_off := i + 1
			totalCount += total_pips_off * int(b.Points[i])
		} else if !white && b.Points[i] < 0 {
			total_pips_off := 24 - i
			totalCount -= total_pips_off * int(b.Points[i])

		}
	}
	if white {
		totalCount += int(b.WhiteBar) * 25
	} else {
		totalCount += int(b.BlackBar) * 25
	}
	return totalCount
}

func PipPick(boards []Board, whiteToMove bool) int {
	bestPipDiff := math.MaxInt
	bestPipDiffIdx := 0
	i := 0
	for _, board := range boards {
		pipDiff := PipCount(board, whiteToMove) - PipCount(board, !whiteToMove)
		if pipDiff < bestPipDiff {
			bestPipDiff = pipDiff
			bestPipDiffIdx = i
		}
		i++
	}
	return bestPipDiffIdx
}
