// Command selfplay runs a random-vs-random self-play soak to shake out engine
// bugs: it reports invariant failures (must be 0), a game-length histogram, and
// cap hits. Very short games or cap hits mean bear-off or forfeit logic is off.
//
//	go run ./cmd/selfplay -games 10000 -check
package main

import (
	"flag"
	"fmt"
	"strings"
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
	Seed        int
	Check       bool
	Concurrency int
}

type SelfPlayResult struct {
	StartTime     time.Time
	WhiteWins     int
	InvFailures   int
	CapHits       int
	TotalPlies    int
	TotalForfeits int
	TotalHits     int
	Lengths       []int
}

func main() {
	games := flag.Int("games", 10000, "number of games to play")
	seed := flag.Int("seed", 42, "base RNG seed (each game uses seed+index)")
	check := flag.Bool("check", false, "verify the board invariant after every ply")
	agentName := flag.String("agent", "random", "picking strategies available: random, greedy")
	concurrency := flag.Int("concurrency", 1, "number of concurrent games")
	maxBatch := flag.Int("max-batch", 4096, "max positions per batch (greedy only)")
	timeoutMs := flag.Float64("timeout-ms", 2.0, "batch flush timeout in milliseconds")
	mode := flag.String("mode", "soak", "soak or sweep")
	server := flag.String("server", "http://localhost:8000", "server URL for greedy mode")
	out := flag.String("out", "sweep.csv", "output CSV path for sweep mode")
	warmup := flag.Duration("warmup", 10*time.Second, "warmup window per sweep cell")
	measure := flag.Duration("measure", 60*time.Second, "measure window per sweep cell")
	record := flag.String("record", "", "dir to write trajectory shards to. empty == off")

	flag.Parse()
	if *concurrency < 1 {
		fmt.Println("concurrency must be at least 1")
		return
	}

	if *maxBatch < 1 {
		fmt.Println("max-batch must be at least 1")
		return
	}

	timeout := time.Duration(*timeoutMs * float64(time.Millisecond))

	switch *mode {
	case "soak":
		flags := &Flags{
			Games:       *games,
			Seed:        *seed,
			Check:       *check,
			Concurrency: *concurrency,
		}
		b := engine.BuildAgent(*agentName, *server, *maxBatch, timeout)
		defer b.Stop()
		var rec *engine.Recorder
		if *record != "" {
			r, err := engine.NewRecorder(*record)
			if err != nil {
				fmt.Printf("error opening recorder: %v\n", err)
				return
			}
			defer r.Close()
			rec = r
		}
		runSoak(b.Agent, flags, rec)
	case "sweep":
		fmt.Println("sweep mode: ignoring -agent (forcing greedy)")
		if err := RunSweep(*server, *maxBatch, *seed, *warmup, *measure, *out); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	default:
		fmt.Printf("invalid mode %q\n", *mode)
	}
}

func runSoak(agent engine.Agent, flags *Flags, rec *engine.Recorder) {
	result, err := RunLoop(agent, flags, rec)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	fmt.Printf("\r\033[Kgames:            %d\n", flags.Games)
	fmt.Printf("Total time: %s\n", time.Since(result.StartTime))
	fmt.Printf("invariant fails:  %d\n", result.InvFailures)
	fmt.Printf("white win rate:   %.3f\n", float64(result.WhiteWins)/float64(len(result.Lengths)))
	fmt.Printf("plies min/median/mean/max: %d / %d / %.1f / %d\n",
		result.Lengths[0], result.Lengths[len(result.Lengths)/2], float64(result.TotalPlies)/float64(len(result.Lengths)), result.Lengths[len(result.Lengths)-1])
	fmt.Printf("cap hits (>=%d): %d\n", maxPlies, result.CapHits)
	fmt.Printf("forfeits total/per-game:   %d / %.2f\n", result.TotalForfeits, float64(result.TotalForfeits)/float64(len(result.Lengths)))
	fmt.Printf("hits total/per-game:       %d / %.2f\n", result.TotalHits, float64(result.TotalHits)/float64(len(result.Lengths)))
	printHistogram(result.Lengths, 50)
}

func runGame(agent engine.Agent, seed, i int, check bool, rec *engine.Recorder) (engine.GameResult, bool) {
	dice := engine.NewDice(uint64(seed) + uint64(i))
	white, black := agent(dice)
	res, invFailed := playSafely(white, black, dice, check, rec)
	return res, invFailed
}

// playSafely runs one game, converting an invariant panic (from PlayGame with
// checkInvariant=true) into a flag so the soak can keep going and tally it. When
// rec is non-nil the game's afterstates are captured and, for a cleanly finished
// game (no invariant failure, not capped), written as one trajectory record.
func playSafely(white, black engine.Picker, dice *engine.Dice, check bool, rec *engine.Recorder) (res engine.GameResult, invFailed bool) {
	var traj []engine.Board
	var onAfterstate func(engine.Board)
	if rec != nil {
		onAfterstate = func(b engine.Board) { traj = append(traj, b) }
	}
	defer func() {
		if r := recover(); r != nil {
			invFailed = true
			fmt.Printf("invariant failure: %v\n", r)
		}
	}()
	res = engine.PlayGame(white, black, dice, maxPlies, check, onAfterstate)
	// Only record finished games: a capped or invariant-failed game has a
	// meaningless outcome label.
	if rec != nil && !invFailed && res.Plies < maxPlies {
		if err := rec.Write(traj, res.WhiteWon); err != nil {
			fmt.Printf("record write error: %v\n", err)
		}
	}
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
