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

// Hits reports how many opponent checkers the mover sent to the bar in the turn
// that took the board from before to after. before.WhiteToMove identifies the
// mover; doubles can hit more than once, so this is a delta.
func Hits(before, after Board) int {
	if before.WhiteToMove {
		return int(after.BlackBar - before.BlackBar)
	}
	return int(after.WhiteBar - before.WhiteBar)
}

// IsForfeit reports whether after is before with only the turn flipped — i.e.
// the mover had no legal play and had to pass.
func IsForfeit(before, after Board) bool {
	after.WhiteToMove = before.WhiteToMove
	return after == before
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

		res.Hits += Hits(prev, b)
		if IsForfeit(prev, b) {
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
