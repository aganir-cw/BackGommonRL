package engine

import (
	"fmt"
	"time"
)

type Picker func([]Board) int

type Agent func(dice *Dice) (white, black Picker)

func NewRandomAgent() Agent {
	return func(dice *Dice) (white, black Picker) {
		p := func(boards []Board) int { return dice.Pick(len(boards)) }
		return p, p
	}
}

func NewGreedyAgent(bt *Batcher) Agent {
	return func(_ *Dice) (Picker, Picker) {
		white := func(bs []Board) int { return GreedyPick(bt, bs, true) }
		black := func(bs []Board) int { return GreedyPick(bt, bs, false) }
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

type AgentBundle struct {
	Agent   Agent
	Batcher *Batcher // nil for random
	Stop    func()   // stops batcher goroutine
}

// NewGreedyBundle spins up a batcher goroutine feeding a greedy agent, returning
// the bundle whose Stop shuts the batcher down.
func NewGreedyBundle(scorer *Scorer, maxBatch int, timeout time.Duration) AgentBundle {
	bt := NewBatcher(maxBatch, timeout)
	go bt.Run(scorer)
	return AgentBundle{Agent: NewGreedyAgent(bt), Batcher: bt, Stop: bt.Stop}
}

func NewPipAgent() Agent {
	return func(_ *Dice) (Picker, Picker) {
		white := func(bs []Board) int { return PipPick(bs, true) }
		black := func(bs []Board) int { return PipPick(bs, false) }
		return white, black
	}
}

func BuildAgent(name, server string, maxBatch int, timeout time.Duration) AgentBundle {
	switch name {
	case "random":
		return AgentBundle{
			Agent: NewRandomAgent(),
			Stop:  func() {},
		}
	case "greedy":
		return NewGreedyBundle(NewScorer(server), maxBatch, timeout)
	case "pip":
		return AgentBundle{Agent: NewPipAgent(), Stop: func() {}}
	default:
		panic(fmt.Sprintf("invalid agent name %q", name))
	}
}
