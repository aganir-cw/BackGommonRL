package engine

const (
	// EncodingDim is dimension of encoding space:
	// [0, 96) 0-23  white counts, n>=1, n>=2, n>=3, max(0,n-3)/2}
	// [96, 192) 0-23  black counts, n>=1, n>=2, n>=3, max(0,n-3)/2}
	// 192 whiteBar / 2.0
	// 193 blackBar / 2.0
	// 194 whiteOff / 15.0
	// 195 blackOff / 15.0
	// 196 1 if whiteToMove else 0
	// 197 1 if !whiteToMove else 0
	EncodingDim = 198
)

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func Encode(b Board) [EncodingDim]float64 {
	var enc [EncodingDim]float64
	for i := 0; i < 24; i++ {
		enc[4*i+0] = boolToFloat(b.Points[i] >= 1)
		enc[4*i+1] = boolToFloat(b.Points[i] >= 2)
		enc[4*i+2] = boolToFloat(b.Points[i] >= 3)
		enc[4*i+3] = max(0, float64(b.Points[i]-3)/2)

		enc[4*i+96] = boolToFloat(b.Points[i] <= -1)
		enc[4*i+97] = boolToFloat(b.Points[i] <= -2)
		enc[4*i+98] = boolToFloat(b.Points[i] <= -3)
		enc[4*i+99] = max(0, float64(-b.Points[i]-3)/2)
	}

	enc[192] = float64(b.WhiteBar) / 2.0
	enc[193] = float64(b.BlackBar) / 2.0
	enc[194] = float64(b.WhiteOff) / 15.0
	enc[195] = float64(b.BlackOff) / 15.0
	enc[196] = boolToFloat(b.WhiteToMove)
	enc[197] = boolToFloat(!b.WhiteToMove)

	return enc
}
