package engine

import (
	"fmt"
	"time"
)

// Single request to model eval worker
// Contains a batch of board encodings to evaluate (feature vector of size encodingDim)
// Contains a caller-channel where the worker sends one score per encoding
type EvalReq struct {
	Encs  [][EncodingDim]float32
	Reply chan []float32
}

type Batcher struct {
	In       chan EvalReq
	MaxBatch int           // positions, start 4096
	Timeout  time.Duration //start 2ms
}

func NewBatcher() *Batcher {
	return &Batcher{
		In:       make(chan EvalReq),
		MaxBatch: 4096,
		Timeout:  2 * time.Millisecond,
	}
}

// Main loop: read requests from input, batch, send to scorer
// Conmtains slice of requests, which each contain encodings. We count total encodings as n
// We have a flush that creates an encoding array of all encodings in the batch, sends it to scorer sorted
// It scores all, returns in original order with offsets
func (bt *Batcher) Run(s *Scorer) {
	pending := []EvalReq{}
	n := 0
	var timer <-chan time.Time

	flush := func() {
		if n == 0 {
			timer = nil
			return
		}

		encs := make([][EncodingDim]float32, 0, n)
		for _, req := range pending {
			encs = append(encs, req.Encs...)
		}

		scores, err := s.Score(encs)
		if err != nil {
			panic(fmt.Sprintf("batch scoring failed: %v", err))
		}
		if len(scores) != n {
			panic(fmt.Sprintf("scorer returned %d scores for %d encodings", len(scores), n))
		}

		offset := 0
		for _, req := range pending {
			end := offset + len(req.Encs)
			req.Reply <- scores[offset:end]
			offset = end
		}

		pending = pending[:0]
		n = 0
		timer = nil
	}

	for {
		select {
		case r := <-bt.In:
			pending = append(pending, r)
			n += len(r.Encs)
			if timer == nil {
				timer = time.After(bt.Timeout)
			}
			if n >= bt.MaxBatch {
				flush()
			}
		case <-timer:
			flush()
		}

	}
}
