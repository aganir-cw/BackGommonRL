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

	"golearner/engine"
)

const maxPlies = 1500

func main() {
	games := flag.Int("games", 10000, "number of games to play")
	random := flag.Bool("random", true, "use the random policy for both sides")
	seed := flag.Int("seed", 42, "base RNG seed (each game uses seed+index)")
	check := flag.Bool("check", false, "verify the board invariant after every ply")
	flag.Parse()

	if !*random {
		fmt.Println("note: only the random policy is implemented; running random-vs-random")
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

	for i := 0; i < *games; i++ {
		dice := engine.NewDice(uint64(*seed) + uint64(i))
		pick := func(states []engine.Board) int { return dice.Pick(len(states)) }

		res, invFailed := playSafely(pick, dice, *check)
		if invFailed {
			invFailures++
			continue
		}
		if res.WhiteWon {
			whiteWins++
		}
		if res.Plies >= maxPlies {
			capHits++
		}
		totalPlies += res.Plies
		totalForfeits += res.Forfeits
		totalHits += res.Hits
		lengths = append(lengths, res.Plies)
	}

	completed := len(lengths)
	fmt.Printf("games:            %d\n", *games)
	fmt.Printf("invariant fails:  %d\n", invFailures)
	if completed == 0 {
		fmt.Println("no games completed")
		return
	}

	sort.Ints(lengths)
	fmt.Printf("white win rate:   %.3f\n", float64(whiteWins)/float64(completed))
	fmt.Printf("plies min/median/mean/max: %d / %d / %.1f / %d\n",
		lengths[0], lengths[completed/2], float64(totalPlies)/float64(completed), lengths[completed-1])
	fmt.Printf("cap hits (>=%d): %d\n", maxPlies, capHits)
	fmt.Printf("forfeits total/per-game:   %d / %.2f\n", totalForfeits, float64(totalForfeits)/float64(completed))
	fmt.Printf("hits total/per-game:       %d / %.2f\n", totalHits, float64(totalHits)/float64(completed))
	printHistogram(lengths, 50)
}

// playSafely runs one game, converting an invariant panic (from PlayGame with
// checkInvariant=true) into a flag so the soak can keep going and tally it.
func playSafely(pick func([]engine.Board) int, dice *engine.Dice, check bool) (res engine.GameResult, invFailed bool) {
	defer func() {
		if r := recover(); r != nil {
			invFailed = true
			fmt.Printf("invariant failure: %v\n", r)
		}
	}()
	res = engine.PlayGame(pick, pick, dice, maxPlies, check)
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
