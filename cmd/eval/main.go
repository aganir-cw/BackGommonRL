package main

import (
	"flag"
	"fmt"
	"golearner/engine"
	"math"
	"time"
)

const maxPlies = 1500

func main() {
	// FLAGS
	a := flag.String("a", "greedy", "Mode for Agent A")
	b := flag.String("b", "greedy", "Mode for Agent B")
	gamePairs := flag.Int("pairs", 10000, "Number of game pairs to play")
	serverAddress := flag.String("server", "http://localhost:8000", "Server address for greedy mode")
	maxBatch := flag.Int("max-batch", 4096, "max positions per batch (greedy only)")
	timeoutMs := flag.Float64("timeout-ms", 2.0, "batch flush timeout in milliseconds")

	flag.Parse()

	timeout := time.Duration(*timeoutMs * float64(time.Millisecond))

	bundleA := engine.BuildAgent(*a, *serverAddress, *maxBatch, timeout)
	bundleB := engine.BuildAgent(*b, *serverAddress, *maxBatch, timeout)

	defer bundleA.Stop()
	defer bundleB.Stop()

	aWins := 0
	bWins := 0

	for i := 0; i < *gamePairs; i++ {
		aWon, bWon := crnLoop(bundleA.Agent, bundleB.Agent, i)
		if aWon {
			aWins++
		} else {
			bWins++
		}
		if bWon {
			bWins++
		} else {
			aWins++
		}
	}

	p := float64(aWins) / float64(2**gamePairs)
	fmt.Printf("p = %f\n", p)
	ci := 1.96 * math.Sqrt(p*(1-p)/float64(2**gamePairs))

	fmt.Printf("ci = %f\n", ci)
	fmt.Printf("A winrate: %f ± %f\n", p, ci)
}

func crnLoop(agentA, agentB engine.Agent, seed int) (aWon, bWon bool) {
	d1 := engine.NewDice(uint64(seed))
	aw, _ := agentA(d1)
	_, bb := agentB(d1)

	r1 := engine.PlayGame(aw, bb, d1, maxPlies, false, nil)

	d2 := engine.NewDice(uint64(seed))
	bw, _ := agentB(d2)
	_, ab := agentA(d2)

	r2 := engine.PlayGame(bw, ab, d2, maxPlies, false, nil)

	return r1.WhiteWon, r2.WhiteWon
}
