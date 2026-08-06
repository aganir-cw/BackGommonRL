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

// PipPick: argmin on pip-diff
// Ties broken by largest hash
func PipPick(boards []Board, whiteToMove bool) int {
	bestPipDiff := math.MaxInt
	bestIdx := 0
	var bestHash uint64
	for i, board := range boards {
		pipDiff := PipCount(board, whiteToMove) - PipCount(board, !whiteToMove)
		switch {
		case pipDiff < bestPipDiff:
			bestPipDiff = pipDiff
			bestIdx = i
			bestHash = hashBoard(board)
		case pipDiff == bestPipDiff:
			if h := hashBoard(board); h > bestHash {
				bestIdx = i
				bestHash = h
			}
		}
	}
	return bestIdx
}

// hashBoard is a deterministic FNV-1a hash of a board's material fields, used as
// an order-independent pseudo-random tie-breaker.
func hashBoard(b Board) uint64 {
	const (
		offset = 1469598103934665603
		prime  = 1099511628211
	)
	h := uint64(offset)
	mix := func(x uint8) {
		h ^= uint64(x)
		h *= prime
	}
	for _, p := range b.Points {
		mix(uint8(p))
	}
	mix(uint8(b.WhiteBar))
	mix(uint8(b.BlackBar))
	mix(uint8(b.WhiteOff))
	mix(uint8(b.BlackOff))
	return h
}
