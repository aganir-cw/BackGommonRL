package engine

import "testing"

// These tests describe the intended contract of the full-turn generator:
//
//	func LegalAfterstates(b Board, d1, d2 int) []Board
//
// Conventions assumed (matching the rest of the engine):
//   - White moves from a high index toward 0 (target = from - die); Black is the
//     mirror. Home board for White is indices 0-5.
//   - LegalAfterstates returns every DISTINCT legal position after the mover has
//     played their whole turn, with WhiteToMove flipped on every returned board.
//   - Max-dice rule: you must play as many dice as legally possible. If you can
//     only ever play one die, you must play the larger one.
//   - If nothing is legal, exactly one afterstate is returned: the same position
//     with only the turn flipped (a forfeit).
//
// All boards are built with mkBoard (see moves_test.go) so side totals are 15 by
// construction.

// onlyBoard asserts exactly one afterstate was produced and returns it.
func onlyBoard(t *testing.T, bs []Board) Board {
	t.Helper()
	if len(bs) != 1 {
		t.Fatalf("expected exactly 1 afterstate, got %d", len(bs))
	}
	return bs[0]
}

// Every returned board must have the turn flipped to the opponent.
func TestLegalAfterstatesFlipsTurn(t *testing.T) {
	b := mkBoard(map[int]int8{12: 1, 5: 14}, map[int]int8{18: 2, 20: 13}, 0, 0, 0, 0, true)

	got := LegalAfterstates(b, 3, 1)
	if len(got) == 0 {
		t.Fatal("expected at least one afterstate")
	}
	for i, nb := range got {
		if nb.WhiteToMove {
			t.Fatalf("afterstate %d still has WhiteToMove=true; turn must flip", i)
		}
	}
}

// Doubles produce four moves: a lone checker advances 4x the die.
func TestDoublesFourMoves(t *testing.T) {
	// One white checker on 20, the other 14 already off; black all borne off.
	b := mkBoard(map[int]int8{20: 1}, map[int]int8{}, 0, 0, 14, 15, true)

	nb := onlyBoard(t, LegalAfterstates(b, 5, 5)) // 20 -> 15 -> 10 -> 5 -> 0
	if nb.Points[20] != 0 || nb.Points[0] != 1 {
		t.Fatalf("expected checker advanced 20->0 via four 5s, got Points[20]=%d Points[0]=%d", nb.Points[20], nb.Points[0])
	}
	if nb.WhiteToMove {
		t.Fatal("turn must flip")
	}
}

// Two die orderings that reach the same board collapse to a single afterstate.
func TestDedupeOrderings(t *testing.T) {
	// Lone checker on 10. 10->7->6 (3 then 1) and 10->9->6 (1 then 3) converge on 6.
	b := mkBoard(map[int]int8{10: 1}, map[int]int8{}, 0, 0, 14, 15, true)

	nb := onlyBoard(t, LegalAfterstates(b, 3, 1))
	if nb.Points[10] != 0 || nb.Points[6] != 1 {
		t.Fatalf("expected converged checker on 6, got Points[10]=%d Points[6]=%d", nb.Points[10], nb.Points[6])
	}
}

// When neither ordering plays both dice, only the larger single-die play survives.
func TestMaxDiceRuleOnlyLargerAlone(t *testing.T) {
	// Lone checker on 10; index 3 is blocked by 2 black.
	//   6 alone: 10->4 (then 1: 4->3 blocked)
	//   1 alone: 10->9 (then 6: 9->3 blocked)
	// Neither order plays both -> keep only the larger die (6): 10->4.
	b := mkBoard(map[int]int8{10: 1}, map[int]int8{3: 2, 0: 13}, 0, 0, 14, 0, true)

	nb := onlyBoard(t, LegalAfterstates(b, 6, 1))
	if nb.Points[4] != 1 || nb.Points[10] != 0 {
		t.Fatalf("expected larger-die play 10->4, got Points[10]=%d Points[4]=%d", nb.Points[10], nb.Points[4])
	}
	if nb.Points[9] != 0 {
		t.Fatal("smaller-die-only afterstate (10->9) must be dropped")
	}
}

// The classic trap: playing the small die first strands the checker, but the
// other order plays both. The generator must explore both orderings, and no
// one-die afterstate may survive.
func TestMaxDiceRuleBothPlayable(t *testing.T) {
	// Lone checker on 10; index 8 blocked by 2 black.
	//   [2,6]: 10->8 blocked immediately.
	//   [6,2]: 10->4 -> 2. Plays both.
	b := mkBoard(map[int]int8{10: 1}, map[int]int8{8: 2, 0: 13}, 0, 0, 14, 0, true)
	t.Logf("board: %v", b)
	t.Logf("LegalAfterstates(b, 6, 2): %v", LegalAfterstates(b, 6, 2))
	nb := onlyBoard(t, LegalAfterstates(b, 6, 2))
	t.Logf("afterstate: %v", nb)
	if nb.Points[2] != 1 || nb.Points[10] != 0 {
		t.Fatalf("expected both dice played 10->2, got Points[10]=%d Points[2]=%d", nb.Points[10], nb.Points[2])
	}
	if nb.Points[4] != 0 {
		t.Fatal("no one-die afterstate should survive when both dice are playable")
	}
}

// With a checker on the bar and every entry point blocked, the only afterstate
// is a forfeit: same position, turn flipped.
func TestForfeitTurn(t *testing.T) {
	// White has a checker on the bar; entries for dice 1 and 2 are 23 and 22,
	// both blocked by 2 black.
	b := mkBoard(map[int]int8{5: 14}, map[int]int8{22: 2, 23: 2, 0: 11}, 1, 0, 0, 0, true)

	nb := onlyBoard(t, LegalAfterstates(b, 1, 2))
	if nb.Points != b.Points || nb.WhiteBar != 1 {
		t.Fatalf("forfeit must leave the position unchanged, got %+v", nb)
	}
	if nb.WhiteToMove {
		t.Fatal("forfeit still flips the turn")
	}
}

// With a checker on the bar and open entry points, every afterstate must have
// entered from the bar (a bar checker is a forced play).
func TestBarEntryGenerated(t *testing.T) {
	// White has one checker on the bar; nothing blocks entry (black all off).
	b := mkBoard(map[int]int8{5: 14}, map[int]int8{}, 1, 0, 0, 15, true)

	got := LegalAfterstates(b, 6, 5)
	if len(got) == 0 {
		t.Fatal("expected bar entry afterstates, got none (is FromBar tried as a source?)")
	}
	for i, nb := range got {
		if nb.WhiteBar != 0 {
			t.Fatalf("afterstate %d did not clear the bar: WhiteBar=%d", i, nb.WhiteBar)
		}
		if nb.WhiteToMove {
			t.Fatalf("afterstate %d must flip the turn", i)
		}
	}
}
