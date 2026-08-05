package main

import (
	"fmt"
	"golearner/engine"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type CellResult struct {
	Concurrency int
	TimeoutMs   float64
	Games       int64
	GamesPerSec float64
	PliesPerSec float64
}

func SweepGames(agent engine.Agent, flags []*Flags) ([]SelfPlayResult, error) {
	results := make([]SelfPlayResult, len(flags))
	for i, flag := range flags {
		var err error
		results[i], err = RunLoop(agent, flag)
		if err != nil {
			return nil, fmt.Errorf("error running game %d: %w", i, err)
		}
	}
	return results, nil
}

func RunCell(newAgent func() engine.Agent, concurrency int, timeout time.Duration, warmup, measure time.Duration) CellResult {
	agent := newAgent()
	var games, plies atomic.Int64
	var gameIdx atomic.Int64 // distinct seeds
	stop := make(chan struct{})

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					i := int(gameIdx.Add(1))
					res, _ := runGame(agent, i)
					games.Add(1)
					plies.Add(int64(res.Plies))
				}
			}
		}()
	}

	time.Sleep(warmup)
	g0, p0 := games.Load(), plies.Load()
	t0 := time.Now()

	time.Sleep(measure)
	g1, p1 := games.Load(), plies.Load()
	elapsed := time.Since(t0).Seconds()

	close(stop)
	wg.Wait()

	return CellResult{
		Concurrency: concurrency,
		TimeoutMs:   float64(timeout) / float64(time.Millisecond),
		Games:       g1 - g0,
		GamesPerSec: float64(g1-g0) / elapsed,
		PliesPerSec: float64(p1-p0) / elapsed,
	}
}

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
		StartTime:     start,
		EndTime:       time.Time{},
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

	sort.Ints(result.Lengths)

	result.EndTime = time.Now()

	return *result, nil
}
