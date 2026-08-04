package engine

import "testing"

// TestHandPositions pins down LegalAfterstates on hand-built positions with an
// exact expected afterstate SET (order-independent, since the generator iterates
// a map). Each case also asserts the turn-level forfeit/hit facts, derived by
// comparing the input board to the afterstate the mover lands on. These cases
// are single-afterstate, so that comparison is unambiguous.
func TestHandPositions(t *testing.T) {
	cases := []struct {
		name        string
		b           Board
		d1, d2      int
		want        []Board
		wantForfeit bool
		wantHits    int
	}{
		{

			name: "bar_entry_blocked_forfeit",
			b: mkBoard(
				map[int]int8{0: 14},
				map[int]int8{18: 3, 19: 3, 20: 3, 21: 2, 22: 2, 23: 2},
				1, 0, 0, 0, true,
			),
			d1: 6, d2: 5,
			want: []Board{
				mkBoard(
					map[int]int8{0: 14},
					map[int]int8{18: 3, 19: 3, 20: 3, 21: 2, 22: 2, 23: 2},
					1, 0, 0, 0, false, // same board, turn flipped
				),
			},
			wantForfeit: true,
			wantHits:    0,
		},
		{
			name: "bar_entry_one_dice_only",
			b: mkBoard(
				map[int]int8{0: 14},
				map[int]int8{18: 3, 19: 3, 20: 3, 21: 3, 22: 3},
				1, 0, 0, 0, true),
			d1: 4, d2: 1,
			want: []Board{
				mkBoard(
					map[int]int8{0: 14, 23: 1},
					map[int]int8{18: 3, 19: 3, 20: 3, 21: 3, 22: 3},
					0, 0, 0, 0, false, // same board, turn flipped
				),
			},
			wantForfeit: false,
			wantHits:    0,
		},
		{
			name: "bar_entry_with_hit",
			b: mkBoard(
				map[int]int8{0: 14},
				map[int]int8{18: 2, 19: 2, 20: 2, 21: 4, 22: 1, 23: 4},
				1, 0, 0, 0, true),
			d1: 3, d2: 2,
			want: []Board{
				mkBoard(
					map[int]int8{0: 14, 22: 1},
					map[int]int8{18: 2, 19: 2, 20: 2, 21: 4, 23: 4},
					0, 1, 0, 0, false, // same board, turn flipped
				),
			},
			wantForfeit: false,
			wantHits:    1,
		},
		{
			// Lone movable checker on 10; index 2 (the common landing point
			// 10-6-2 = 10-2-6) is blocked by 2 black.
			//   6 first: 10->4, then 2: 4->2 blocked.
			//   2 first: 10->8, then 6: 8->2 blocked.
			// Neither order plays both dice, so the max-dice rule keeps only the
			// larger die played alone: 10->4.
			name: "must_play_larger_dice",
			b: mkBoard(
				map[int]int8{0: 14, 10: 1},
				map[int]int8{2: 2, 23: 13},
				0, 0, 0, 0, true,
			),
			d1: 6, d2: 2,
			want: []Board{
				mkBoard(
					map[int]int8{0: 14, 4: 1},
					map[int]int8{2: 2, 23: 13},
					0, 0, 0, 0, false, // larger die only: 10->4, turn flipped
				),
			},
			wantForfeit: false,
			wantHits:    0,
		},
		{
			name: "must_play_both_ordinary_trap",
			b: mkBoard(
				map[int]int8{0: 14, 11: 1},
				map[int]int8{5: 2, 23: 13},
				0, 0, 0, 0, true,
			),
			d1: 6, d2: 2,
			want: []Board{
				mkBoard(
					map[int]int8{0: 14, 3: 1},
					map[int]int8{5: 2, 23: 13},
					0, 0, 0, 0, false, // same board, turn flipped
				),
			},
			wantForfeit: false,
			wantHits:    0,
		},
		{
			name: "bear_off_exact",
			b: mkBoard(
				map[int]int8{5: 14, 4: 1},
				map[int]int8{23: 15},
				0, 0, 0, 0, true,
			),
			d1: 6, d2: 5,
			want: []Board{
				mkBoard(
					map[int]int8{5: 13},
					map[int]int8{23: 15},
					0, 0, 2, 0, false,
				),
				mkBoard(
					map[int]int8{5: 12, 4: 1, 0: 1},
					map[int]int8{23: 15},
					0, 0, 1, 0, false,
				),
			},
			wantForfeit: false,
			wantHits:    0,
		},
		{
			name: "bear_off_overshoot",
			b: mkBoard(
				map[int]int8{5: 2, 0: 13},
				map[int]int8{23: 15},
				0, 0, 0, 0, true,
			),
			d1: 6, d2: 5,
			want: []Board{
				mkBoard(
					map[int]int8{5: 0, 0: 14},
					map[int]int8{23: 15},
					0, 0, 1, 0, false,
				),
			},
			wantForfeit: false,
			wantHits:    0,
		},
		{
			// White is all home with checkers on 5 and 1. Neither die may bear
			// off the low checker on 1: 1 needs an exact 2, but a 4 (or 3)
			// overshoots and index 5 is higher, so overshoot bear-off is illegal.
			// Both dice are therefore forced to walk the high checker inward, and
			// every ordering converges on a single afterstate:
			//   [4,3]: 5->1 (now 1,1), then 1->off (overshoot from the highest).
			//   [3,4]: 5->2 (now 2,1), then 2->off (overshoot from the highest).
			// Result: one checker borne off, the other left on 1.
			name: "bearoff_forced_inside_move",
			b: mkBoard(
				map[int]int8{5: 1, 1: 1},
				map[int]int8{23: 15},
				0, 0, 13, 0, true,
			),
			d1: 4, d2: 3,
			want: []Board{
				mkBoard(
					map[int]int8{1: 1},
					map[int]int8{23: 15},
					0, 0, 14, 0, false,
				),
			},
			wantForfeit: false,
			wantHits:    0,
		},
		{
			// Doubles give four 2s, but only three are playable. The lone
			// traveler on 9 walks 9->7->5->3; the fourth step 3->1 is blocked by
			// 2 black, and the 14-stack on 0 can't move (bearing off is illegal
			// while the traveler is outside home, and later an overshoot from 0
			// is illegal with a higher checker present). Max consumption is 3, so
			// only the depth-3 afterstate survives.
			name: "doubles_three_of_four_playable",
			b: mkBoard(
				map[int]int8{9: 1, 0: 14},
				map[int]int8{1: 2, 23: 13},
				0, 0, 0, 0, true,
			),
			d1: 2, d2: 2,
			want: []Board{
				mkBoard(
					map[int]int8{3: 1, 0: 14},
					map[int]int8{1: 2, 23: 13},
					0, 0, 0, 0, false,
				),
			},
			wantForfeit: false,
			wantHits:    0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LegalAfterstates(tc.b, tc.d1, tc.d2)
			assertSameBoardSet(t, got, tc.want)

			// Forfeit/hit are properties of a chosen move, not of the generator.
			// With a single afterstate the mover has no choice, so compare directly.
			if len(got) == 1 {
				if f := IsForfeit(tc.b, got[0]); f != tc.wantForfeit {
					t.Errorf("forfeit = %v, want %v", f, tc.wantForfeit)
				}
				if h := Hits(tc.b, got[0]); h != tc.wantHits {
					t.Errorf("hits = %d, want %d", h, tc.wantHits)
				}
			}
		})
	}
}

// assertSameBoardSet checks that got and want contain the same boards with the
// same multiplicities, ignoring order (Board is comparable, so usable as a key).
func assertSameBoardSet(t *testing.T, got, want []Board) {
	t.Helper()
	countBy := func(bs []Board) map[Board]int {
		m := make(map[Board]int, len(bs))
		for _, b := range bs {
			m[b]++
		}
		return m
	}
	gm, wm := countBy(got), countBy(want)

	if len(got) != len(want) {
		t.Errorf("afterstate count = %d, want %d", len(got), len(want))
	}
	for b, n := range wm {
		if gm[b] != n {
			t.Errorf("expected board (x%d) missing or miscounted (got x%d):\n%s", n, gm[b], b.Render())
		}
	}
	for b, n := range gm {
		if wm[b] != n {
			t.Errorf("unexpected board (x%d):\n%s", n, b.Render())
		}
	}
}

func TestLegalAfterstatesMirrorSymmetryRandom(t *testing.T) {
	const (
		seed        uint64 = 0x5eed
		sampleSize  int    = 2000
		sampleEvery int    = 10
	)

	dice := NewDice(seed)
	samples := make([]Board, 0, sampleSize)
	b := Start()
	plies := 0

	for len(samples) < sampleSize {
		d1, d2 := dice.Roll()
		afterstates := LegalAfterstates(b, d1, d2)
		b = afterstates[dice.Pick(len(afterstates))]
		plies++

		if plies%sampleEvery == 0 {
			samples = append(samples, b)
		}

		if b.WhiteOff == 15 || b.BlackOff == 15 {
			b = Start()
		}
	}

	asSet := func(boards []Board) map[Board]bool {
		set := make(map[Board]bool, len(boards))
		for _, board := range boards {
			set[board] = true
		}
		return set
	}

	for i, b := range samples {
		d1, d2 := dice.Roll()

		original := asSet(LegalAfterstates(b, d1, d2))

		mirroredAfterstates := LegalAfterstates(b.Mirror(), d1, d2)
		mirroredBack := make([]Board, len(mirroredAfterstates))
		for j, board := range mirroredAfterstates {
			mirroredBack[j] = board.Mirror()
		}
		got := asSet(mirroredBack)

		if len(original) != len(got) {
			t.Fatalf(
				"sample %d, dice (%d,%d): set size mismatch: original=%d mirrored=%d\nboard: %v",
				i, d1, d2, len(original), len(got), b,
			)
		}

		for board := range original {
			if !got[board] {
				t.Fatalf(
					"sample %d, dice (%d,%d): original afterstate missing from mirrored set\nboard: %v\nafterstate: %v",
					i, d1, d2, b, board,
				)
			}
		}
	}
}

func FuzzAfterstates(f *testing.F) {
	for _, seed := range []uint64{0, 1, 42, 0x5eed, ^uint64(0)} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, seed uint64) {
		dice := NewDice(seed)
		b := Start()

		// Deterministically replay a random sequence to obtain a valid position.
		k := 1 + int(seed%1000)
		for range k {
			d1, d2 := dice.Roll()
			afterstates := LegalAfterstates(b, d1, d2)
			if len(afterstates) == 0 {
				t.Fatalf("seed=%d: no afterstates during replay\nboard: %v", seed, b)
			}

			b = afterstates[dice.Pick(len(afterstates))]
			if b.WhiteOff == 15 || b.BlackOff == 15 {
				b = Start()
			}
		}

		d1, d2 := dice.Roll()
		afterstates := LegalAfterstates(b, d1, d2)

		if len(afterstates) == 0 {
			t.Fatalf("seed=%d dice=(%d,%d): no afterstates\nboard: %v",
				seed, d1, d2, b)
		}

		for i, afterstate := range afterstates {
			if err := afterstate.Check(); err != nil {
				t.Fatalf(
					"seed=%d dice=(%d,%d): afterstate %d violates invariants: %v\nbefore: %v\nafter:  %v",
					seed, d1, d2, i, err, b, afterstate,
				)
			}

			if afterstate.WhiteToMove == b.WhiteToMove {
				t.Fatalf(
					"seed=%d dice=(%d,%d): afterstate %d did not flip turn\nbefore: %v\nafter:  %v",
					seed, d1, d2, i, b, afterstate,
				)
			}
		}
	})
}
