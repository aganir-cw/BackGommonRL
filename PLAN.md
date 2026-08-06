# Backgammon self-play RL — task-level plan

Granularity: hour blocks, function signatures, test names. Assumes ~9 productive hours/day. Signatures are contracts — rename freely, but if you change a shape, know why.

Conventions fixed for the whole week (write these at the top of `board.go` on day 1 and never revisit):

- `Points[i] > 0` = white checkers, `< 0` = black.
- White moves from index 23 → 0, bears off past 0. Black 0 → 23, bears off past 23. White's home = indices 0–5, black's = 18–23.
- `V(s) = P(white wins)`, absolute perspective. White argmaxes, black argmins. No perspective flipping anywhere.
- Dice `d1 >= d2` normalized at generation time.

---



## Day 1 — engine core



### Block 1 (0:00–1:00) — types and constructors

```go
// engine/board.go
type Board struct {
    Points      [24]int8
    WhiteBar, BlackBar   int8
    WhiteOff, BlackOff   int8
    WhiteToMove bool
}

func Start() Board                 // standard opening position
func (b Board) String() string     // ASCII board, both bars, off counts, side to move
func (b Board) Mirror() Board      // reverse points, negate, swap bars/offs, flip turn
```

`String()` is not optional garnish — you will debug by eyeball all week. Format: two rows of 12 points, bar in the middle, `W`/`b` counts.

Test file `board_test.go`:

- `TestStartPosition` — exact checker counts at 24, 13, 8, 6-points both sides.
- `TestMirrorInvolution` — `b.Mirror().Mirror() == b` over 100 random-ish boards (hand-perturb Start()).



### Block 2 (1:00–1:45) — invariant

```go
func (b Board) Check() error
```

Checks: white total = black total = 15 across points+bar+off; all bar/off ≥ 0; `abs(Points[i]) <= 15`. Return a descriptive error, not a bool — you want to know *which* invariant died.

- `TestCheckStart` — Start() passes.
- `TestCheckCatchesMissing` — remove one checker, expect error containing "white total".



### Block 3 (1:45–3:30) — single-die application

```go
// engine/moves.go
// from: 0-23 for a point, or the constant FromBar.
// Returns the resulting board and false if illegal.
func applyDie(b Board, from int, die int) (Board, bool)
```

Logic order (short-circuit in this order, it prevents bugs):

1. If mover has bar checkers and `from != FromBar` → illegal.
2. Compute destination; bar entry: white enters at `24-die`, black at `die-1`.
3. If destination in [0,24): occupied by ≥2 opponents → illegal; exactly 1 → hit (decrement, increment opponent bar).
4. If destination out of range → bear-off path: all 15 in home or illegal; exact roll, or `from` is highest occupied and die overshoots → legal; else illegal.
5. Apply, flip nothing (turn flips at full-move level, not here).

Tests (`moves_test.go`), one per rule branch:

- `TestApplyDieSimpleMove`
- `TestApplyDieBlockedPoint`
- `TestApplyDieHit` — opponent lands on bar.
- `TestApplyDieBarEntryForced` — with a bar checker, `from=5` is illegal.
- `TestApplyDieBarEntryBlocked`
- `TestApplyDieBearOffExact`
- `TestApplyDieBearOffOvershootFromHighest` — legal.
- `TestApplyDieBearOffOvershootNotHighest` — illegal.
- `TestApplyDieBearOffNotAllHome` — illegal.

Boards for these: write a helper `mkBoard(white map[int]int8, black map[int]int8, wBar, bBar, wOff, bOff int8, whiteToMove bool) Board` that fills counts and panics if totals ≠ 15. Every hand test uses it; totals are then correct by construction.

### Block 4 (3:30–6:00) — full-turn generation

```go
func LegalAfterstates(b Board, d1, d2 int) []Board
```

Structure:

```go
func gen(b Board, dice []int, out map[Board]int) // recursive: try every from for dice[0], recurse on remainder; record depth used
```

1. Build dice slice: doubles → `[d,d,d,d]`, else `[max,min]` and `[min,max]` both explored.
2. Recurse; at each node record the board keyed with how many dice were consumed.
3. Post-pass A (max-dice rule): keep only boards at the maximum consumption depth. If max depth is 1 and both single-die options exist, keep only larger-die results.
4. Post-pass B: the map already deduped identical afterstates.
5. If no move at all is legal: return `[]Board{skip(b)}` where skip flips the turn only. Callers must handle len==1 forfeit uniformly with normal moves.
6. Flip `WhiteToMove` on every returned board.

Tests:

- `TestDoublesFourMoves` — open board, verify a checker can advance 4×die.
- `TestMaxDiceRuleBothPlayable` — construct the classic trap: playing the small die first blocks the large; correct generator finds the ordering that plays both. Assert no one-die afterstate survives.
- `TestMaxDiceRuleOnlyLargerAlone` — either die playable alone, never both → only larger-die afterstates remain.
- `TestForfeitTurn` — closed board vs bar checker → exactly one afterstate, same points, turn flipped.
- `TestDedupeOrderings` — a position where (d1 then d2) and (d2 then d1) reach the same board; assert set size, not sequence count.

The max-dice trap position for the test — construct something like: white checker on 23 only movable via 23→(23-d2)→bear-region path where playing d1 first strands it. Work it out on paper; ~20 minutes well spent.

### Block 5 (6:00–7:00) — dice, RNG, game loop

```go
// engine/game.go
type Dice struct{ rng *rand.Rand }        // seedable, one per game for CRN later
func (d *Dice) Roll() (int, int)

func PlayGame(pickWhite, pickBlack func([]Board) int, dice *Dice, maxPlies int, checkInvariant bool) (whiteWon bool, plies int)
```

Winner detection: `WhiteOff == 15` / `BlackOff == 15` checked after each ply. `maxPlies` safety cap 1500; log and count if ever hit (should be ~never with legal play).

### Block 6 (7:00–8:30) — random self-play soak

```go
// cmd/selfplay/main.go
flags: -games, -random, -seed, -check
```

Run `-games 10000 -random -check`. Collect: invariant failures (must be 0), game-length histogram, forfeit-turn count, hit count. Print summary.

Sanity numbers: random-vs-random games are long — several hundred plies median. Very short games or cap-hits mean bear-off or forfeit logic is wrong.

### Block 7 (8:30–9:00) — commit, write down day-2 hand positions

List the day-2 test positions you'll build (bear-off edge cases, bar scenarios) in a TODO while the rules are fresh in your head. Tomorrow-you starts from the list, not from recall.

**Gate: 10k games, 0 invariant violations, sane length histogram, all block-3/4 tests green.**

---



## Day 2 — verification, encoding, first bytes to Python



### Block 1 (0:00–2:00) — hand-built positions on paper, then as tests

The nine scenarios from day-1 block-7 list, as table-driven tests with *exact expected afterstate sets*:

```go
func TestHandPositions(t *testing.T) {
    cases := []struct{
        name string
        b    Board
        d1, d2 int
        want []Board   // built with mkBoard
    }{ ... }
}
```

Cases (names as documentation):

- `bar_entry_blocked_forfeit`
- `bar_entry_one_die_only`
- `bar_entry_with_hit`
- `must_play_larger`
- `must_play_both_ordering_trap`
- `bearoff_exact`
- `bearoff_overshoot_highest`
- `bearoff_forced_inside_move` — higher checkers exist, overshoot bear-off illegal, inside move forced.
- `doubles_three_of_four_playable` — exactly 3 dice consumable; assert depth-3 afterstates only.

Budget the full two hours. Paper first, Go second.

### Block 2 (2:00–3:00) — mirror property test

```go
func TestMirrorProperty(t *testing.T)
```

Sample ~2000 positions by playing random games and grabbing every 10th position. For each + random dice: mirror the afterstate set of the mirrored board, compare as sets against the original's. Use `map[Board]bool` equality.

### Block 3 (3:00–3:30) — fuzzer, then background

```go
func FuzzAfterstates(f *testing.F)
```

Fuzz inputs: uint64 seed → deterministic random position (replay a seeded random game k plies) + dice. Property: every afterstate passes `Check()`; set nonempty (forfeit counts as the one skip board).  in a spare terminal; check it at lunch.

### Block 4 (3:30–5:00) — encoding

```go
// engine/encode.go
const EncodingDim = 198
func Encode(b Board, out *[EncodingDim]float32)
```

Layout (document as a comment, this exact order):

```
[0:96)    white points 0-23, 4 units each: {n≥1, n≥2, n≥3
[96:192)  black points 0-23, same
[192]     WhiteBar / 2.0
[193]     BlackBar / 2.0
[194]     WhiteOff / 15.0
[195]     BlackOff / 15.0
[196]     1 if WhiteToMove else 0
[197]     1 if !WhiteToMove else 0
```

Tests:

- `TestEncodeStart` — compute the start vector by hand once (it's sparse; ~15 min), assert exact.
- `TestEncodeMirrorSwapsColors` — `Encode(b.Mirror())` equals `Encode(b)` with white/black blocks and scalars swapped, turn units swapped.



### Block 5 (5:00–6:30) — model + server skeleton

```python
# server/model.py
class ValueNet(nn.Module):
    def __init__(self, hidden=80):
        # 198 -> hidden (ReLU) -> 1 (sigmoid)
```

```python
# server/serve.py
POST /score   body: raw bytes = uint32 count | count*198 float32 LE
              resp: count float32 LE
POST /weights body: torch state_dict (torch.save bytes) -> loads into live model
GET  /healthz
```

Single-threaded is fine; the batcher serializes anyway. `torch.inference_mode()`, model on GPU, input pinned→device in one copy.

### Block 6 (6:30–7:30) — Go client, end-to-end smoke

```go
// engine/client.go
type Scorer struct{ url string; buf []byte }
func (s *Scorer) Score(encs [][EncodingDim]float32) ([]float32, error)
```

`binary.LittleEndian` throughout. Smoke test: score Start(), print. Then the real check: score Start() and its mirror — with random init the values differ, but shapes and transport are proven. Then a 100-position batch; assert response length.

Byte-order and stride bugs die here, today, in isolation.

### Block 7 (7:30–9:00) — buffer / spillover

This day runs over. Bear-off hand cases are the usual culprit. If somehow clean: start day-3 batcher types.

**Gate: hand positions green, mirror property green, fuzz 30min clean, batch of 100 scored Go→Python→Go with correct shapes.**

---



## Day 3 — batcher, greedy self-play, throughput



### Block 1 (0:00–2:30) — batcher

```go
// engine/batcher.go
type EvalReq struct {
    Encs  [][EncodingDim]float32
    Reply chan []float32
}

type Batcher struct {
    In       chan EvalReq
    MaxBatch int           // positions, start 4096
    Timeout  time.Duration // start 2ms
}
func (bt *Batcher) Run(s *Scorer)   // the loop
```

Loop shape:

```go
pending := []EvalReq{}; n := 0; var timer <-chan time.Time
for {
    select {
    case r := <-bt.In:
        pending = append(pending, r); n += len(r.Encs)
        if timer == nil { timer = time.After(bt.Timeout) }
        if n >= bt.MaxBatch { flush() }
    case <-timer:
        flush()
    }
}
```

`flush()`: concatenate, one `Score`, walk `pending` scattering slices by offset, reset. Record per-flush: batch size, queue latency (req carries enqueue time). Dump histograms on exit.

Unit test with a fake Scorer: `TestBatcherScatterOffsets` — 3 requests of 2/5/1 positions, fake scorer returns index-as-value, assert each reply gets its own slice. `TestBatcherTimeoutFires` — 1 request, no more traffic, reply arrives within ~2×timeout.

### Block 2 (2:30–3:30) — greedy agents

```go
func GreedyPick(bt *Batcher, boards []Board, whiteToMove bool) int
// encode all, one EvalReq, argmax if white / argmin if black
```

Wire into `PlayGame`. Flag `-agent greedy|random` in selfplay cmd.

### Block 3 (3:30–4:30) — concurrent driver + instrumentation

```go
// cmd/selfplay: -concurrency flag
// N goroutines each looping PlayGame; shared Batcher; atomic counters
```

Metrics: games/sec (1s ticker), plies/sec, batch-size histogram (from batcher), p50/p99 queue latency, ply cap hits. pprof HTTP endpoint on (`import _ "net/http/pprof"`).

### Block 4 (4:30–7:00) — the sweep

Grid: concurrency {32, 128, 512, 2048, 8192} × timeout {0.5ms, 2ms, 10ms}, 60s each after 10s warmup, random-init net, invariant OFF (flag), one CSV row per cell.

While it runs: capture one 30s pprof (`go tool pprof -top`) at the concurrency where games/sec flattens, and one `nvidia-smi dmon -s u` trace for the same window.

### Block 5 (7:00–8:30) — plots + the bottleneck paragraph

`analysis/throughput.py`: games/sec vs concurrency (line per timeout); mean achieved batch vs concurrency. Write, literally in the repo README, three sentences: what saturated (expect: Go move generation, GPU <10% at hidden=80), the pprof line that shows it, and what you predict happens at hidden=1024 (you'll test that day 5).

### Block 6 (8:30–9:00) — commit, tag `throughput-baseline`

**Gate: sweep CSV + plots exist; bottleneck named with profile evidence; batcher unit tests green.**

---



## Day 4 — training



### Block 1 (0:00–1:00) — trajectory writer

```go
// selfplay: -record dir flag
// Per finished game append one record to a shard file (rotate at 64MB):
// uint32 nPlies | nPlies × [EncodingDim]float32 | uint8 whiteWon
```

Chosen afterstates only (the position after each move actually played). Binary, dumb, append-only. `TestRecordRoundTrip` in Go; matching reader in Python.

### Block 2 (1:00–3:00) — trainer

```python
# server/train.py
# - shard tailer: watches dir, reads new complete records
# - ReplayBuffer: ring of 500k (encoding[198], outcome) pairs, numpy-backed
# - loop: sample 8192, MSE(V(s), outcome), Adam lr=1e-3
# - every 500 steps: POST /weights to server; bump sync counter
# - log every 100 steps: loss, buffer_fill, games_seen, syncs (CSV + stdout)
```

Trainer and server as separate processes is simplest to reason about; the weight POST makes it explicit. Same process with a shared model object is fine too — pick one, note it.

Unit check before wiring: overfit 100 records to near-zero loss. If a tiny MLP can't memorize 100 positions, the reader is broken, not the model.

### Block 3 (3:00–4:30) — opponents + eval harness

```go
func PipPick(boards []Board, whiteToMove bool) int   // min own pip count
func PipCount(b Board, white bool) int               // sum of distances to off

// cmd/eval/main.go
// -a, -b agent specs; -pairs N; CRN protocol:
// for seed in 0..N: play (A white, B black, dice seed) and (B white, A black, same seed)
// report: A winrate, ±1.96*sqrt(p(1-p)/(2N))
```

`TestPipCountStart` — both sides 167. `TestCRNDeterminism` — same seeds twice → identical outcomes.

### Block 4 (4:30–5:00) — launch the loop

Selfplay (greedy, recording, concurrency from day-3 sweet spot) + trainer + server. Watch the sync counter move and loss fall below the ~0.25 constant-prediction floor.

### Block 5 (5:00–8:00) — babysit, eval every ~20k games

Eval current vs Random (500 pairs) and vs Pip (1000 pairs) at each checkpoint. Expect vs-random >95% within tens of thousands of games; vs-pip crossing 50% and climbing after.

Debug ladder if flat, in this exact order:

1. Black minimizing? (Both-sides-argmax is *the* classic bug — net trains toward "everyone wins".)
2. Outcome label from white's perspective at write time?
3. Sync counter advancing? (Server on stale random weights learns fine, plays random.)
4. Encoding turn units correct after the flip-in-generator?
5. Loss falling but play bad → eval agent pointing at the right server/port.



### Block 6 (8:00–9:00) — learning-curve plot, commit

`analysis/learning.py`: winrate-vs-games for both opponents, error bars from the CRN formula.

**Gate: >95% vs random; >50% and rising vs pip; curves plotted; overfit-100 check passed earlier.**

---



## Day 5 — strength, ladder, ablations



### Block 1 (0:00–1:00) — checkpoint ladder

Trainer: save state_dict every 100k games consumed as `ckpt_{games}.pt`. Ladder script: each new ckpt vs previous 3 + vs `ckpt_100k` (fixed reference), 1000 CRN pairs each, append to `ladder.csv`. Plot winrate-vs-reference over training. Monotone = healthy. Rock-paper-scissors between checkpoints = cycling → switch selfplay to sample opponents uniformly from last 5 checkpoints (small change in the driver: two Scorers, two weight versions — the server needs a `?version=` param or a second port; second port is dumber and faster to build).

### Block 2 (1:00–1:30) — launch the long run

Best config, target ≥1M games (you know the games/sec; do the arithmetic and set expectations). This runs all day on GPU 0.

### Block 3–5 (1:30–7:00) — ablations, one per spare GPU, identical budget (~300k games each)

1. **hidden ∈ {40, 320, 1024}** (80 is the main run). For 1024: rerun three cells of the day-3 throughput sweep and note where the bottleneck moved — this closes the loop on your day-3 prediction.
2. **raw encoding**: one input per point per side = `n/15.0` signed, plus the 6 scalars (54 inputs). Same budget. Question answered: how much was Tesauro's feature engineering worth?
3. **TD(λ=0.7)** if blocks 1–2 went smoothly: per-parameter trace tensors, `trace = γλ·trace + grad(V(s_t))`, update `α·δ·trace` with `δ = V(s_{t+1}) - V(s_t)` (terminal: outcome − V). ~30 lines. Skip without guilt if behind — MC vs TD is the most cuttable ablation.

Each ablation evals vs the *fixed reference* checkpoint, not vs its own history — one common yardstick.

### Block 6 (7:00–8:30) — gnubg integration

Install gnubg (build with `--enable-simd`, headless `gnubg -t`). It has an external-player socket interface — the command names and handshake vary by version, so `info gnubg` / its manual for *your* build is authoritative; expect to spend most of this block on protocol friction (board format, whose turn encoding, resignation messages). Target: 10 completed cubeless money games, any strength, driven by a script. Strength settings and real matches are tomorrow.

### Block 7 (8:30–9:00) — ladder plot, ablation status, commit

**Gate: ladder monotone (or cycling identified and mitigation running); ≥2 ablations complete/running; 10 scripted gnubg games completed.**

---



## Day 6 — 2-ply search, the headline plot, gnubg



### Block 1 (0:00–2:30) — 2-ply expectimax

```go
var rolls21 = [...]struct{ d1, d2 int; w float64 }{ {1,1,1.0/36}, {2,1,2.0/36}, ... }

func TwoPlyPick(bt *Batcher, cands []Board, whiteToMove bool) int
```

Per candidate `a`: for each of 21 rolls, `LegalAfterstates(a, ...)` for the opponent, score all, opponent takes their best (min if you're white), accumulate `w * best`. Pick your argmax over candidates.

Batching structure is the whole game here: build the *entire* tree's encodings for one decision — all candidates × all rolls × all replies — as one `EvalReq` (or a few large ones), with an index array mapping scores back to (candidate, roll) cells. Do not send 21×K tiny requests.

`TestTwoPlyReducesToOnePlyAtTerminal` — position where every candidate immediately wins: 2-ply and 1-ply must agree. `TestRollWeightsSumToOne`.

### Block 2 (2:30–3:30) — optional pruning, measured

If a 2-ply decision is too slow for eval volumes: 1-ply pre-score, expand top-6 candidates only. Measure the pick-agreement rate between pruned and full 2-ply over 500 random decisions; report it (expect >95%). If full 2-ply is fast enough, skip.

### Block 3 (3:30–6:30) — the headline experiment

Fixed opponent: best 1-ply/hidden-80 checkpoint. Cells:


| agent       | evals/decision (measure it) |
| ----------- | --------------------------- |
| 1-ply h80   | ~20                         |
| 1-ply h1024 | ~20                         |
| 2-ply h80   | hundreds–thousands          |
| 2-ply h1024 | same, costlier each         |


1000 CRN pairs per cell. Record winrate, mean evals/decision, mean ms/move. Plot strength vs evals/decision (log-x), one point per cell, ms/move as annotation. This figure is the writeup's centerpiece: model scale vs search depth at matched measured cost.

### Block 4 (6:30–8:00) — gnubg for real

Best agent vs gnubg at 0-ply, then 1-ply: 200 CRN-ish games each (gnubg controls its own dice unless you configure otherwise — check whether your build allows external dice; if not, just report with wider error bars), cubeless money play. Record honestly. Beating 0-ply: strong week. Losing to 1-ply: expected, say so.

### Block 5 (8:00–9:00) — assemble all figures into `analysis/figures/`, commit

**Gate: headline plot with ≥3 cells; two gnubg numbers with sample sizes.**

---



## Day 7 — buffer, then one of two endings

Spillover absorbs first. Then:

**A — doubling cube (full day, only if everything is green).** Retrain with a 4-way head {win, gammon-win, loss, gammon-loss} (softmax, CE loss vs realized outcome — the recorder must now store gammon flags: loser borne off none = gammon). Equity `E = 2(p_w + p_gw) - 1 + (p_gw - p_gl)` roughly; implement double/take at the textbook thresholds (double when E in the doubling window, take when losing equity < 0.5 of a point — look up the exact cubeless criteria, don't derive from memory). Re-benchmark vs gnubg with cube on.

**B — writeup (half day).** Sections, one figure each:

1. Engine verification: the invariant, mirror property, the nine hand cases, fuzz hours. One paragraph on the must-play-both trap.
2. Throughput: sweep plot, bottleneck at h80 with pprof evidence, where it moved at h1024.
3. Learning: curves vs random/pip, the ladder.
4. Ablations: hidden size, encoding, (TD if run).
5. **Strength vs compute/decision** — the headline.
6. gnubg, honestly.

Close with the two systems observations that transfer: dynamic batching trades latency for occupancy exactly as in serving, and weight sync that's a one-line `load_state_dict` here is the hard problem at LLM scale — you now know precisely why.

---



## Standing rules

- A failing gate stops the day's remaining blocks; fix or invoke the fallback.
- Fallback (decide at day-2 gate only): port engine to Python (~300 lines, port the tests first), lose the throughput study, keep everything else at reduced scale.
- Every eval number gets: sample size, CRN yes/no, ± interval. No naked win rates anywhere, including Slack messages to Ace.

