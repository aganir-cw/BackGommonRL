package engine

const FromBar = 24

// from: 0-23 for a point, or constant FromBar.
// Returns the resulting board and false if illegal.
func applyDie(b Board, from, die int) (Board, bool) {
	if b.WhiteToMove {
		return b.applyDieWhite(from, die)
	}
	mf := from
	if from != FromBar {
		mf = 23 - from // mirror the source coordinate
	}
	nb, ok := b.Mirror().applyDieWhite(mf, die)
	if !ok {
		return b, false
	}
	return nb.Mirror(), true
}

func (b Board) applyDieWhite(from int, die int) (Board, bool) {
	if b.WhiteBar > 0 && from != FromBar { // Must clear bar first
		return b, false
	}
	if from == FromBar {
		return b.enterFromBar(die)
	}
	if from < 0 || from > 23 || b.Points[from] < 1 {
		return b, false
	}

	target := from - die
	if target < 0 {
		return b.bearOffWhite(from, die)
	}
	if b.Points[target] <= -2 {
		return b, false
	}

	if b.Points[target] == -1 {
		b.Points[target] = 0
		b.BlackBar++
	}
	b.Points[from]--
	b.Points[target]++
	return b, true
}

func (b Board) bearOffWhite(from, die int) (Board, bool) {
	if !b.whitAllHome() {
		return b, false
	}
	if die != from+1 { // Not exact -> overshoot, only legal if highest occupied point
		for i := from + 1; i <= 5; i++ {
			if b.Points[i] > 0 {
				return b, false
			}
		}
	}
	b.Points[from]--
	b.WhiteOff++
	return b, true
}

func (b Board) whitAllHome() bool {
	if b.WhiteBar > 0 {
		return false
	}
	var sum int8
	for _, p := range b.Points[:6] {
		if p > 0 {
			sum += p
		}
	}
	return sum+b.WhiteOff == 15
}

func (b Board) enterFromBar(die int) (Board, bool) {
	entry := 24 - die
	if b.Points[entry] <= -2 {
		return b, false
	}
	if b.Points[entry] == -1 {
		b.Points[entry] = 0
		b.BlackBar++
	}
	b.WhiteBar--
	b.Points[entry]++
	return b, true
}
