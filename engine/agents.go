package engine

type Picker func([]Board) int

type Agent func(dice *Dice) (white, black Picker)

func RandomAgent(dice *Dice) Agent {
	return func(dice *Dice) (white, black Picker) {
		p := func(boards []Board) int { return dice.Pick(len(boards)) }
		return p, p
	}
}

func GreedyAgent(bt *Batcher) Agent {
	return func(dice *Dice) (white, black Picker) {
		white = func(boards []Board) int {
			return GreedyPick(bt, boards, true)
		}
		black = func(boards []Board) int {
			return GreedyPick(bt, boards, false)
		}
		return white, black
	}
}

func GreedyPick(bt *Batcher, boards []Board, whiteToMove bool) int {
	// Encode all, one EvalReq, argmax if white / argmin if black
	encs := make([][EncodingDim]float32, len(boards))
	for i, b := range boards {
		encs[i] = Encode(b)
	}
	req := EvalReq{
		Encs:  encs,
		Reply: make(chan []float32),
	}
	bt.In <- req
	scores := <-req.Reply
	// argmax if white / argmin if black
	if whiteToMove {
		return ArgMax(scores)
	} else {
		return ArgMin(scores)
	}
}
