package main

import (
	"fmt"
	"golearner/engine"
	"sync"
	"time"
)

func RunLoop(agent engine.Agent, flags *Flags) (SelfPlayResult, error) {
	start := time.Now()
	var lastPrint time.Time

	jobs := make(chan int)
	results := make(chan outcome)

	var wg sync.WaitGroup
	for w := 0; w < flags.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				res, invFailed := runGame(agent, flags.Seed, i, flags.Check)
				results <- outcome{res, invFailed}
			}
		}()
	}

	// Producer: hands out game indices 0...games-1, then close so range ends
	go func() {
		for i := 0; i < flags.Games; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()
	result := &SelfPlayResult{
		WhiteWins:     0,
		InvFailures:   0,
		CapHits:       0,
		TotalPlies:    0,
		TotalForfeits: 0,
		TotalHits:     0,
		Lengths:       []int{},
	}

	for o := range results {
		if o.invFailed {
			result.InvFailures++
			continue
		}
		if o.res.WhiteWon {
			result.WhiteWins++
		}
		if o.res.Plies >= maxPlies {
			result.CapHits++
		}
		result.TotalPlies += o.res.Plies
		result.TotalForfeits += o.res.Forfeits
		result.TotalHits += o.res.Hits
		result.Lengths = append(result.Lengths, o.res.Plies)

		done := len(result.Lengths) + result.InvFailures
		if time.Since(lastPrint) > 100*time.Millisecond || done == flags.Games {
			lastPrint = time.Now()
			rate := float64(done) / time.Since(start).Seconds()
			fmt.Printf("\r\033[Kgames %d/%d (%.1f%%)  %.1f games/s",
				done, flags.Games, 100*float64(done)/float64(flags.Games), rate)
		}
	}

	return *result, nil
}
