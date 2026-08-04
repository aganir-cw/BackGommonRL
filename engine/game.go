package engine

import (
	"fmt"
	"math/rand/v2"
)

type Dice struct{ rng *rand.Rand }

func NewDice(seed uint64) *Dice {
	return &Dice{rng: rand.New(rand.NewPCG(seed, seed))}
}

func (d *Dice) Roll() (int, int) {
	d1 := d.rng.IntN(6) + 1
	d2 := d.rng.IntN(6) + 1
	return d1, d2
}

func (d *Dice) Pick(n int) int {
	return d.rng.IntN(n)
}

// GameResult summarizes a single completed (or capped) game.
type GameResult struct {
	WhiteWon bool // true if White borne off all 15 first
	Plies    int  // number of turns played (capped at maxPlies)
	Forfeits int  // turns where the mover had no legal play (dance)
	Hits     int  // opponent checkers sent to the bar over the game
}

func PlayGame(pickWhite, pickBlack func([]Board) int, dice *Dice, maxPlies int, checkInvariant bool) GameResult {
	b := Start()
	var res GameResult
	for plies := 1; plies <= maxPlies; plies++ {
		res.Plies = plies
		d1, d2 := dice.Roll()
		states := LegalAfterstates(b, d1, d2)
		pick := pickBlack
		if b.WhiteToMove {
			pick = pickWhite
		}
		prev := b
		b = states[pick(states)]

		// A hit shows up as the opponent's bar count rising during the mover's
		// turn (doubles can hit more than once, so use the delta).
		if prev.WhiteToMove {
			res.Hits += int(b.BlackBar - prev.BlackBar)
		} else {
			res.Hits += int(b.WhiteBar - prev.WhiteBar)
		}
		// A forfeit is the sole afterstate being identical to the prior
		// position except for the flipped turn: nothing moved.
		if b.Points == prev.Points &&
			b.WhiteBar == prev.WhiteBar && b.BlackBar == prev.BlackBar &&
			b.WhiteOff == prev.WhiteOff && b.BlackOff == prev.BlackOff {
			res.Forfeits++
		}

		if checkInvariant {
			if err := b.Check(); err != nil {
				panic(fmt.Sprintf("invariant violated at ply %d: %v\n%s", plies, err, b.Render()))
			}
		}
		if b.WhiteOff == 15 {
			res.WhiteWon = true
			return res
		}
		if b.BlackOff == 15 {
			return res
		}
	}
	return res
}
