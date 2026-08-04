// Command selfplay runs a random-vs-random self-play soak to shake out engine
// bugs: it reports invariant failures (must be 0), a game-length histogram, and
// cap hits. Very short games or cap hits mean bear-off or forfeit logic is off.
//
//	go run ./cmd/selfplay -games 10000 -check
package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"golearner/engine"
)

const maxPlies = 1500

type outcome struct {
	res       engine.GameResult
	invFailed bool
}

type Flags struct {
	Games       int
	Random      bool
	Seed        int
	Check       bool
	AgentName   string
	Concurrency int
	MaxBatch    int
	Timeout     int
}

type SelfPlayResult struct {
	WhiteWins     int
	InvFailures   int
	CapHits       int
	TotalPlies    int
	TotalForfeits int
	TotalHits     int
	Lengths       []int
}

func newFlags(games int, random bool, seed int, check bool, agentName string, concurrency int, maxBatch int, timeout int) *Flags {
	return &Flags{
		Games:       games,
		Random:      random,
		Seed:        seed,
		Check:       check,
		AgentName:   agentName,
		Concurrency: concurrency,
		MaxBatch:    maxBatch,
		Timeout:     timeout,
	}
}

func main() {
	games := flag.Int("games", 10000, "number of games to play")
	random := flag.Bool("random", true, "use the random policy for both sides")
	seed := flag.Int("seed", 42, "base RNG seed (each game uses seed+index)")
	check := flag.Bool("check", false, "verify the board invariant after every ply")
	agentName := flag.String("agent", "random", "picking strategies available: random, greedy")
	concurrency := flag.Int("concurrency", 1, "number of concurrent games")
	maxBatch := flag.Int("max-batch", 4096, "max positions per batch (greedy only)")
	timeout := flag.Int("timeout", 2, "timeout for each game in milliseconds")

	flag.Parse()
	if *concurrency < 1 {
		fmt.Println("concurrency must be at least 1")
		return
	}

	if *maxBatch < 1 {
		fmt.Println("max-batch must be at least 1")
		return
	}

	if !*random {
		fmt.Println("note: only the random policy is implemented; running random-vs-random")
	}
	flags := newFlags(*games, *random, *seed, *check, *agentName, *concurrency, *maxBatch, *timeout)

	var agent engine.Agent

	switch *agentName {
	case "random":
		agent = engine.RandomAgent()
	case "greedy":
		scorer := engine.NewScorer("http://localhost:8000")
		bt := engine.NewBatcher(*maxBatch, time.Duration(*timeout)*time.Millisecond)
		go bt.Run(scorer)
		agent = engine.GreedyAgent(bt)
	default:
		fmt.Printf("invalid agent name %q", *agentName)
	}

	var (
		whiteWins     int
		invFailures   int
		capHits       int
		totalPlies    int
		totalForfeits int
		totalHits     int
		lengths       []int
	)

	start := time.Now()
	var lastPrint time.Time

	jobs := make(chan int)
	results := make(chan outcome)

	// N Workers: each pulls game indices until job closes
	var wg sync.WaitGroup
	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				res, invFailed := runGame(agent, *seed, i, *check)
				results <- outcome{res, invFailed}
			}
		}()
	}

	// Producer: hands out game indices 0...games-1, then close so range ends
	go func() {
		for i := 0; i < *games; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collecctor:
	for o := range results {
		if o.invFailed {
			invFailures++
			continue
		}
		if o.res.WhiteWon {
			whiteWins++
		}
		if o.res.Plies >= maxPlies {
			capHits++
		}
		totalPlies += o.res.Plies
		totalForfeits += o.res.Forfeits
		totalHits += o.res.Hits
		lengths = append(lengths, o.res.Plies)

		done := len(lengths) + invFailures
		if time.Since(lastPrint) > 100*time.Millisecond || done == *games {
			lastPrint = time.Now()
			rate := float64(done) / time.Since(start).Seconds()
			fmt.Printf("\r\033[Kgames %d/%d (%.1f%%)  %.1f games/s",
				done, *games, 100*float64(done)/float64(*games), rate)
		}

	}
	sort.Ints(lengths)

	fmt.Printf("\r\033[Kgames:            %d\n", *games)
	fmt.Printf("Total time: %s\n", time.Since(start))
	fmt.Printf("invariant fails:  %d\n", invFailures)
	fmt.Printf("white win rate:   %.3f\n", float64(whiteWins)/float64(len(lengths)))
	fmt.Printf("plies min/median/mean/max: %d / %d / %.1f / %d\n",
		lengths[0], lengths[len(lengths)/2], float64(totalPlies)/float64(len(lengths)), lengths[len(lengths)-1])
	fmt.Printf("cap hits (>=%d): %d\n", maxPlies, capHits)
	fmt.Printf("forfeits total/per-game:   %d / %.2f\n", totalForfeits, float64(totalForfeits)/float64(len(lengths)))
	fmt.Printf("hits total/per-game:       %d / %.2f\n", totalHits, float64(totalHits)/float64(len(lengths)))
	printHistogram(lengths, 50)
}

func runGame(agent engine.Agent, seed, i int, check bool) (engine.GameResult, bool) {
	dice := engine.NewDice(uint64(seed) + uint64(i))
	white, black := agent(dice)
	res, invFailed := playSafely(white, black, dice, check)
	return res, invFailed
}

// playSafely runs one game, converting an invariant panic (from PlayGame with
// checkInvariant=true) into a flag so the soak can keep going and tally it.
func playSafely(white, black engine.Picker, dice *engine.Dice, check bool) (res engine.GameResult, invFailed bool) {
	defer func() {
		if r := recover(); r != nil {
			invFailed = true
			fmt.Printf("invariant failure: %v\n", r)
		}
	}()
	res = engine.PlayGame(white, black, dice, maxPlies, check)
	return
}

// printHistogram prints a text bar chart of game lengths bucketed by width plies.
func printHistogram(sortedLengths []int, width int) {
	if len(sortedLengths) == 0 {
		return
	}
	buckets := make([]int, sortedLengths[len(sortedLengths)-1]/width+1)
	for _, x := range sortedLengths {
		buckets[x/width]++
	}
	maxCount := 0
	for _, c := range buckets {
		if c > maxCount {
			maxCount = c
		}
	}
	fmt.Println("game-length histogram (plies):")
	for i, c := range buckets {
		if c == 0 {
			continue
		}
		bar := c * 40 / maxCount
		if bar == 0 {
			bar = 1
		}
		fmt.Printf("  %4d-%4d | %-40s %d\n", i*width, i*width+width-1, strings.Repeat("#", bar), c)
	}
}
