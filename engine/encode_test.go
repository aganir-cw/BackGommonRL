package engine

import "testing"

// TestEncodeStart asserts the exact feature vector for the opening position.
// Start() has White on 5(x5),7(x3),12(x5),23(x2) and Black on 0(x2),11(x5),
// 16(x3),18(x5); no bar/off; White to move. Each point contributes 4 features
// {n>=1, n>=2, n>=3, max(0,n-3)/2} in its color block, so the vector is sparse.
func TestEncodeStart(t *testing.T) {
	var want [EncodingDim]float32

	// White block [0:96): 4 features per point at 4*i.
	setPoint := func(base, i int, f0, f1, f2, f3 float32) {
		want[base+4*i+0] = f0
		want[base+4*i+1] = f1
		want[base+4*i+2] = f2
		want[base+4*i+3] = f3
	}
	// White (base 0)
	setPoint(0, 5, 1, 1, 1, 1.0) // 5 checkers -> (5-3)/2 = 1.0
	setPoint(0, 7, 1, 1, 1, 0.0) // 3 checkers
	setPoint(0, 12, 1, 1, 1, 1.0)
	setPoint(0, 23, 1, 1, 0, 0.0) // 2 checkers
	// Black (base 96)
	setPoint(96, 0, 1, 1, 0, 0.0)  // 2 checkers
	setPoint(96, 11, 1, 1, 1, 1.0) // 5 checkers
	setPoint(96, 16, 1, 1, 1, 0.0) // 3 checkers
	setPoint(96, 18, 1, 1, 1, 1.0) // 5 checkers

	// Scalars: no bar/off, White to move.
	want[196] = 1
	want[197] = 0

	got := Encode(Start())
	for i := 0; i < EncodingDim; i++ {
		if got[i] != want[i] {
			t.Errorf("Encode(Start())[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestEncodeMirrorSwapsColors checks the color-symmetry of the encoding:
// mirroring a board must swap the White and Black feature blocks (with point
// order reversed, since Mirror maps point i -> 23-i), swap the bar/off scalars,
// and swap the two turn units.
func TestEncodeMirrorSwapsColors(t *testing.T) {
	boards := []Board{
		Start(),
		mkBoard(
			map[int]int8{5: 5, 0: 8},
			map[int]int8{18: 5, 23: 8},
			1, 0, 1, 2, true,
		),
		mkBoard(
			map[int]int8{0: 15},
			map[int]int8{23: 15},
			0, 0, 0, 0, false,
		),
	}

	for _, b := range boards {
		e := Encode(b)
		got := Encode(b.Mirror())

		// Build what the mirrored encoding must be, purely by rearranging e.
		var want [EncodingDim]float32
		for pt := 0; pt < 24; pt++ {
			for k := 0; k < 4; k++ {
				// mirror's White block at pt == orig's Black block at 23-pt.
				want[4*pt+k] = e[4*(23-pt)+96+k]
				// mirror's Black block at pt == orig's White block at 23-pt.
				want[4*pt+96+k] = e[4*(23-pt)+k]
			}
		}
		want[192] = e[193] // WhiteBar <-> BlackBar
		want[193] = e[192]
		want[194] = e[195] // WhiteOff <-> BlackOff
		want[195] = e[194]
		want[196] = e[197] // turn units swap
		want[197] = e[196]

		for i := 0; i < EncodingDim; i++ {
			if got[i] != want[i] {
				t.Errorf("board %v: Encode(Mirror)[%d] = %v, want %v", b, i, got[i], want[i])
			}
		}
	}
}
