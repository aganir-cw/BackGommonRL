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
	Games       int64
	GamesPerSec float64
	PliesPerSec float64
	MeanBatch   float64
	CapHits     int64
}

func csvHeader() []string {
	return []string{
		"concurrency", "timeout_ms", "warmup_s", "measure_s",
		"games", "games_per_s", "plies_per_s", "mean_batch", "cap_hits",
	}
}

func (c CellResult) CSVRow() []string {
	return []string{
		strconv.Itoa(c.Concurrency),
		strconv.FormatFloat(c.TimeoutMs, 'f', 3, 64),
		strconv.FormatFloat(c.WarmupS, 'f', 1, 64),
		strconv.FormatFloat(c.MeasureS, 'f', 1, 64),
		strconv.FormatInt(c.Games, 10),
		strconv.FormatFloat(c.GamesPerSec, 'f', 2, 64),
		strconv.FormatFloat(c.PliesPerSec, 'f', 2, 64),
		strconv.FormatFloat(c.MeanBatch, 'f', 2, 64),
		strconv.FormatInt(c.CapHits, 10),
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

	for _, timeout := range timeouts {
		for _, concurrency := range concurrencies {
			makeBundle := func() engine.AgentBundle {
				bt := engine.NewBatcher(maxBatch, timeout)
				go bt.Run(scorer)
				return engine.AgentBundle{
					Agent:   engine.NewGreedyAgent(bt),
					Batcher: bt,
					Stop:    bt.Stop,
				}
			}

			res := RunCell(makeBundle, concurrency, timeout, warmup, measure, baseSeed)
			if err := w.Write(res.CSVRow()); err != nil {
				return fmt.Errorf("write row: %w", err)
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return fmt.Errorf("flush row: %w", err)
			}

			fmt.Printf("cell c=%-5d timeout=%6.3fms  %8.1f games/s  %9.1f plies/s  mean batch %7.1f  cap hits %d\n",
				res.Concurrency, res.TimeoutMs, res.GamesPerSec, res.PliesPerSec, res.MeanBatch, res.CapHits)
		}
	}
	return nil
}

func RunCell(makeBundle func() engine.AgentBundle, concurrency int, timeout time.Duration, warmup, measure time.Duration, baseSeed int) CellResult {
	b := makeBundle()
	defer b.Stop()

	var games, plies, capHits atomic.Int64
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
					res, _ := runGame(b.Agent, baseSeed, i, false)
					games.Add(1)
					plies.Add(int64(res.Plies))
					if res.Plies >= maxPlies {
						capHits.Add(1)
					}
				}
			}
		}()
	}

	time.Sleep(warmup)
	g0, p0 := games.Load(), plies.Load()
	var pos0, flush0 int64
	if b.Batcher != nil {
		pos0, flush0 = b.Batcher.Snapshot()
	}
	t0 := time.Now()

	time.Sleep(measure)
	g1, p1 := games.Load(), plies.Load()
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

	return CellResult{
		Concurrency: concurrency,
		TimeoutMs:   float64(timeout) / float64(time.Millisecond),
		WarmupS:     warmup.Seconds(),
		MeasureS:    measure.Seconds(),
		Games:       g1 - g0,
		GamesPerSec: float64(g1-g0) / elapsed,
		PliesPerSec: float64(p1-p0) / elapsed,
		MeanBatch:   meanBatch,
		CapHits:     capHits.Load(),
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
