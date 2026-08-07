package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"golearner/engine"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
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
	gamesSeen := flag.Int("games", -1, "games_seen label for this eval row (x-axis for the learning curve)")
	out := flag.String("out", "", "CSV path to append one eval row to. empty == stdout only")
	concurrency := flag.Int("concurrency", 512, "number of pairs played concurrently (fills the batch, avoids per-decision timeout stalls)")

	flag.Parse()

	if *concurrency < 1 {
		*concurrency = 1
	}

	timeout := time.Duration(*timeoutMs * float64(time.Millisecond))

	bundleA := engine.BuildAgent(*a, *serverAddress, *maxBatch, timeout, 0)
	bundleB := engine.BuildAgent(*b, *serverAddress, *maxBatch, timeout, 0)

	defer bundleA.Stop()
	defer bundleB.Stop()

	var aWins, bWins atomic.Int64

	// Play pairs concurrently: sequential eval leaves the batcher with one tiny
	// request in flight per decision, so every ply eats the full flush timeout.
	// A worker pool keeps many decisions in flight, filling batches like selfplay.
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				aWon, bWon := crnLoop(bundleA.Agent, bundleB.Agent, i)
				if aWon {
					aWins.Add(1)
				} else {
					bWins.Add(1)
				}
				if bWon {
					bWins.Add(1)
				} else {
					aWins.Add(1)
				}
			}
		}()
	}
	for i := 0; i < *gamePairs; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	p := float64(aWins.Load()) / float64(2**gamePairs)
	ci := 1.96 * math.Sqrt(p*(1-p)/float64(2**gamePairs))

	fmt.Printf("A(%s) vs %s: winrate %f ± %f  (pairs=%d, games=%d)\n", *a, *b, p, ci, *gamePairs, *gamesSeen)

	if *out != "" {
		if err := writeEvalRow(*out, *gamesSeen, *b, *gamePairs, p, ci); err != nil {
			fmt.Printf("eval log write error: %v\n", err)
			os.Exit(1)
		}
	}
}

// writeEvalRow appends one result to the eval log CSV, writing a header first if
// the file is new. Columns match analysis/learning.py's expected schema.
func writeEvalRow(path string, games int, opponent string, pairs int, winrate, ci float64) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	_, statErr := os.Stat(path)
	newFile := os.IsNotExist(statErr)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if newFile {
		if err := w.Write([]string{"timestamp", "games_seen", "opponent", "pairs", "winrate", "ci"}); err != nil {
			return err
		}
	}
	row := []string{
		time.Now().UTC().Format(time.RFC3339),
		strconv.Itoa(games),
		opponent,
		strconv.Itoa(pairs),
		strconv.FormatFloat(winrate, 'f', 6, 64),
		strconv.FormatFloat(ci, 'f', 6, 64),
	}
	if err := w.Write(row); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
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
