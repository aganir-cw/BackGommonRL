package main

import (
	"encoding/csv"
	"fmt"
	"golearner/engine"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type CellResult struct {
	Concurrency int
	TimeoutMs   float64
	WarmupS     float64
	MeasureS    float64
	Plies       int64   // plies processed during the measure window (live count)
	PliesPerSec float64 // true work rate; counts in-progress games, not just completions
	GamesPerSec float64 // derived: PliesPerSec / MeanGameLen
	MeanGameLen float64 // sweep-wide mean completed-game length used for the games/s conversion
	MeanBatch   float64
	CapHits     int64
}

func csvHeader() []string {
	return []string{
		"concurrency", "timeout_ms", "warmup_s", "measure_s",
		"plies", "plies_per_s", "games_per_s", "mean_game_len", "mean_batch", "cap_hits",
	}
}

func (c CellResult) CSVRow() []string {
	return []string{
		strconv.Itoa(c.Concurrency),
		strconv.FormatFloat(c.TimeoutMs, 'f', 3, 64),
		strconv.FormatFloat(c.WarmupS, 'f', 1, 64),
		strconv.FormatFloat(c.MeasureS, 'f', 1, 64),
		strconv.FormatInt(c.Plies, 10),
		strconv.FormatFloat(c.PliesPerSec, 'f', 2, 64),
		strconv.FormatFloat(c.GamesPerSec, 'f', 2, 64),
		strconv.FormatFloat(c.MeanGameLen, 'f', 2, 64),
		strconv.FormatFloat(c.MeanBatch, 'f', 2, 64),
		strconv.FormatInt(c.CapHits, 10),
	}
}

// countingAgent wraps an agent so each ply (one pick call in PlayGame) bumps a
// shared counter, giving a live plies/s that reflects work done by in-progress
// games — not just games that happen to finish inside a short measure window.
func countingAgent(a engine.Agent, plies *atomic.Int64) engine.Agent {
	return func(d *engine.Dice) (engine.Picker, engine.Picker) {
		white, black := a(d)
		return func(bs []engine.Board) int {
				plies.Add(1)
				return white(bs)
			}, func(bs []engine.Board) int {
				plies.Add(1)
				return black(bs)
			}
	}
}

// RunSweep runs the Block 4 throughput grid: a fresh batcher per cell (sharing
// one Scorer), 10s warmup + 60s measure by default, writing one flushed CSV row
// per cell so a crash mid-sweep keeps partial data.
func RunSweep(server string, maxBatch, baseSeed int, warmup, measure time.Duration, out string) error {
	concurrencies := []int{32, 128, 512, 2048, 8192}
	timeouts := []time.Duration{500 * time.Microsecond, 2 * time.Millisecond, 10 * time.Millisecond}

	scorer := engine.NewScorer(server)

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(csvHeader()); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	w.Flush()

	// Game length is concurrency-independent, so we pool completed games across
	// all cells into one running mean and use it to convert the (accurate)
	// plies/s into games/s — even for cells where nothing finishes in-window.
	var totalGames, totalPlies int64

	for _, timeout := range timeouts {
		for _, concurrency := range concurrencies {
			makeBundle := func() engine.AgentBundle {
				return engine.NewGreedyBundle(scorer, maxBatch, timeout, 0)
			}

			res, cGames, cPlies := RunCell(makeBundle, concurrency, timeout, warmup, measure, baseSeed)
			totalGames += cGames
			totalPlies += cPlies
			if totalGames > 0 {
				res.MeanGameLen = float64(totalPlies) / float64(totalGames)
				res.GamesPerSec = res.PliesPerSec / res.MeanGameLen
			}

			if err := w.Write(res.CSVRow()); err != nil {
				return fmt.Errorf("write row: %w", err)
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return fmt.Errorf("flush row: %w", err)
			}

			fmt.Printf("cell c=%-5d timeout=%6.3fms  %8.1f games/s  %9.1f plies/s  mean len %6.1f  mean batch %7.1f  cap hits %d\n",
				res.Concurrency, res.TimeoutMs, res.GamesPerSec, res.PliesPerSec, res.MeanGameLen, res.MeanBatch, res.CapHits)
		}
	}
	return nil
}

// RunCell measures one grid cell. It returns the cell result plus the total
// completed games and their plies over the whole run (warmup + measure) so the
// caller can maintain a sweep-wide mean game length.
func RunCell(makeBundle func() engine.AgentBundle, concurrency int, timeout time.Duration, warmup, measure time.Duration, baseSeed int) (res CellResult, completedGames, completedPlies int64) {
	b := makeBundle()
	defer b.Stop()

	var plies, cGames, cPlies, capHits atomic.Int64
	var gameIdx atomic.Int64 // distinct seeds
	stop := make(chan struct{})

	// Every ply increments plies live, so the measure-window delta is the true
	// work rate regardless of whether whole games finish inside the window.
	agent := countingAgent(b.Agent, &plies)

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
					r, _ := runGame(agent, baseSeed, i, false, nil)
					cGames.Add(1)
					cPlies.Add(int64(r.Plies))
					if r.Plies >= maxPlies {
						capHits.Add(1)
					}
				}
			}
		}()
	}

	time.Sleep(warmup)
	p0 := plies.Load()
	var pos0, flush0 int64
	if b.Batcher != nil {
		pos0, flush0 = b.Batcher.Snapshot()
	}
	t0 := time.Now()

	time.Sleep(measure)
	p1 := plies.Load()
	var pos1, flush1 int64
	if b.Batcher != nil {
		pos1, flush1 = b.Batcher.Snapshot()
	}
	elapsed := time.Since(t0).Seconds()

	close(stop)
	wg.Wait()

	meanBatch := 0.0
	if df := flush1 - flush0; df > 0 {
		meanBatch = float64(pos1-pos0) / float64(df)
	}

	res = CellResult{
		Concurrency: concurrency,
		TimeoutMs:   float64(timeout) / float64(time.Millisecond),
		WarmupS:     warmup.Seconds(),
		MeasureS:    measure.Seconds(),
		Plies:       p1 - p0,
		PliesPerSec: float64(p1-p0) / elapsed,
		MeanBatch:   meanBatch,
		CapHits:     capHits.Load(),
	}
	return res, cGames.Load(), cPlies.Load()
}

func RunLoop(agent engine.Agent, flags *Flags, rec *engine.Recorder) (SelfPlayResult, error) {
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
				res, invFailed := runGame(agent, flags.Seed, i, flags.Check, rec)
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

	return *result, nil
}
