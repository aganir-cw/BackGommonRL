package engine

import "cmp"

func ArgMax[T cmp.Ordered](xs []T) int {
	best := 0
	for i, x := range xs {
		if x > xs[best] {
			best = i
		}
	}
	return best
}

func ArgMin[T cmp.Ordered](xs []T) int {
	best := 0
	for i, x := range xs {
		if x < xs[best] {
			best = i
		}
	}
	return best
}
