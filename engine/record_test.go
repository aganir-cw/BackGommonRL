package engine

// Tests for the trajectory recorder, written against the format contract in the
// plan (engine/record.go):
//
//	per record: uint32 nPlies (LE) | nPlies * [EncodingDim]float32 (LE) | uint8 whiteWon
//	shard files live under a dir and rotate to a new file at 64MB
//
// API under test:
//
//	NewRecorder(dir string) (*Recorder, error)
//	(*Recorder).Write(traj []Board, whiteWon bool) error
//	(*Recorder).Close() error
//	ReadRecords(rd io.Reader, fn func(encs [][EncodingDim]float32, whiteWon bool) error) error

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// testBoard builds a deterministic, varied Board from a seed. Validity (15
// checkers a side) is irrelevant here: the recorder only encodes whatever it is
// given, and distinct contents make encodings distinguishable for round-trip
// comparison.
func testBoard(seed int) Board {
	r := rand.New(rand.NewPCG(uint64(seed)+1, uint64(seed)*2654435761+1))
	var b Board
	for i := 0; i < 24; i++ {
		b.Points[i] = int8(r.IntN(11) - 5) // -5..5
	}
	b.WhiteBar = int8(r.IntN(3))
	b.BlackBar = int8(r.IntN(3))
	b.WhiteOff = int8(r.IntN(16))
	b.BlackOff = int8(r.IntN(16))
	b.WhiteToMove = r.IntN(2) == 0
	return b
}

// buildRecordBytes independently serializes one record in the wire format, so
// reader tests do not depend on the writer.
func buildRecordBytes(boards []Board, won bool) []byte {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(boards))); err != nil {
		panic(err)
	}
	for _, b := range boards {
		enc := Encode(b)
		if err := binary.Write(buf, binary.LittleEndian, enc); err != nil {
			panic(err)
		}
	}
	var w byte
	if won {
		w = 1
	}
	buf.WriteByte(w)
	return buf.Bytes()
}

type capturedRecord struct {
	encs [][EncodingDim]float32
	won  bool
}

// collectRecords drains a reader through ReadRecords, copying each record out of
// the callback (the reader may reuse its slice between calls).
func collectRecords(rd io.Reader) ([]capturedRecord, error) {
	var out []capturedRecord
	err := ReadRecords(rd, func(encs [][EncodingDim]float32, won bool) error {
		cp := make([][EncodingDim]float32, len(encs))
		copy(cp, encs)
		out = append(out, capturedRecord{encs: cp, won: won})
		return nil
	})
	return out, err
}

// readAllShards concatenates every shard file in dir (name order) and parses all
// records out of the combined stream. Rotation only ever happens between whole
// records, so concatenation reconstructs the original record stream.
func readAllShards(t *testing.T, dir string) []capturedRecord {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var readers []io.Reader
	for _, n := range names {
		f, err := os.Open(filepath.Join(dir, n))
		if err != nil {
			t.Fatalf("open shard %s: %v", n, err)
		}
		defer f.Close()
		readers = append(readers, f)
	}

	recs, err := collectRecords(io.MultiReader(readers...))
	if err != nil {
		t.Fatalf("ReadRecords over shards: %v", err)
	}
	return recs
}

// TestRecordRoundTrip writes several games of varied length, then reads them all
// back and asserts exact fidelity of every encoding vector and the outcome byte.
func TestRecordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rec, err := NewRecorder(dir)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	type game struct {
		boards []Board
		won    bool
	}
	plies := []int{1, 2, 5, 37, 250}
	var games []game
	seed := 0
	for gi, n := range plies {
		boards := make([]Board, n)
		for j := 0; j < n; j++ {
			boards[j] = testBoard(seed)
			seed++
		}
		won := gi%2 == 0
		games = append(games, game{boards: boards, won: won})
		if err := rec.Write(boards, won); err != nil {
			t.Fatalf("Write game %d: %v", gi, err)
		}
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := readAllShards(t, dir)
	if len(got) != len(games) {
		t.Fatalf("recovered %d records, want %d", len(got), len(games))
	}
	for i, g := range games {
		if got[i].won != g.won {
			t.Errorf("record %d whiteWon = %v, want %v", i, got[i].won, g.won)
		}
		if len(got[i].encs) != len(g.boards) {
			t.Errorf("record %d nPlies = %d, want %d", i, len(got[i].encs), len(g.boards))
			continue
		}
		for j, b := range g.boards {
			want := Encode(b)
			if got[i].encs[j] != want {
				t.Errorf("record %d ply %d encoding mismatch", i, j)
			}
		}
	}
}

// TestNewRecorderCreatesMissingDir checks NewRecorder makes the target directory
// (including parents) if it does not exist.
func TestNewRecorderCreatesMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "shards")
	rec, err := NewRecorder(dir)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	if err := rec.Write([]Board{testBoard(1)}, true); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("expected %s to be a directory (err=%v)", dir, err)
	}
	if got := readAllShards(t, dir); len(got) != 1 {
		t.Fatalf("recovered %d records, want 1", len(got))
	}
}

// TestNewRecorderError exercises the failure path: a shard dir cannot be created
// underneath an existing regular file.
func TestNewRecorderError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := NewRecorder(filepath.Join(blocker, "sub")); err == nil {
		t.Fatalf("expected error creating recorder under a file path, got nil")
	}
}

// TestWriteRotation writes past the 64MB shard cap and verifies the writer rolls
// over to a second file while losing no records.
func TestWriteRotation(t *testing.T) {
	dir := t.TempDir()
	rec, err := NewRecorder(dir)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Each record is 4 + pliesPer*EncodingDim*4 + 1 bytes. With EncodingDim=198,
	// pliesPer=1000 -> ~0.79MB/record; 90 records ~= 71MB > 64MB, forcing a roll.
	const nGames = 90
	const pliesPer = 1000
	wants := make([]bool, nGames)
	boards := make([]Board, pliesPer)
	seed := 0
	for g := 0; g < nGames; g++ {
		for j := range boards {
			boards[j] = testBoard(seed)
			seed++
		}
		won := g%3 == 0
		wants[g] = won
		if err := rec.Write(boards, won); err != nil {
			t.Fatalf("Write game %d: %v", g, err)
		}
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	files := 0
	for _, e := range entries {
		if !e.IsDir() {
			files++
		}
	}
	if files < 2 {
		t.Fatalf("expected >= 2 shard files after rotation, got %d", files)
	}

	got := readAllShards(t, dir)
	if len(got) != nGames {
		t.Fatalf("recovered %d records across shards, want %d", len(got), nGames)
	}
	for i := range wants {
		if got[i].won != wants[i] {
			t.Errorf("record %d whiteWon = %v, want %v", i, got[i].won, wants[i])
		}
		if len(got[i].encs) != pliesPer {
			t.Errorf("record %d nPlies = %d, want %d", i, len(got[i].encs), pliesPer)
		}
	}
}

// TestReadRecordsEmpty: an empty stream yields no records and no error.
func TestReadRecordsEmpty(t *testing.T) {
	recs, err := collectRecords(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("ReadRecords on empty stream: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("got %d records from empty stream, want 0", len(recs))
	}
}

// TestReadRecordsShortReads: a truncated trailing record (a game still being
// appended by a concurrent writer) must be ignored, and all preceding complete
// records returned, with no error.
func TestReadRecordsShortReads(t *testing.T) {
	full1 := buildRecordBytes([]Board{testBoard(1), testBoard(2)}, true)
	full2 := buildRecordBytes([]Board{testBoard(3)}, false)
	complete := append(append([]byte{}, full1...), full2...)

	headerClaiming5 := make([]byte, 4)
	binary.LittleEndian.PutUint32(headerClaiming5, 5)

	oneGame := buildRecordBytes([]Board{testBoard(9)}, true)

	cases := []struct {
		name string
		tail []byte
	}{
		{"clean_eof", nil},
		{"partial_header", []byte{0x01, 0x02}},          // fewer than 4 header bytes
		{"header_then_eof", headerClaiming5},            // full header, zero payload
		{"partial_payload", oneGame[:len(oneGame)-100]}, // header + incomplete payload
		{"missing_outcome_byte", oneGame[:len(oneGame)-1]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := append(append([]byte{}, complete...), tc.tail...)
			recs, err := collectRecords(bytes.NewReader(stream))
			if err != nil {
				t.Fatalf("ReadRecords returned error on partial tail: %v", err)
			}
			if len(recs) != 2 {
				t.Fatalf("got %d complete records, want 2", len(recs))
			}
			// Spot-check fidelity of the two complete records.
			if !recs[0].won || recs[1].won {
				t.Errorf("outcome mismatch: got %v, %v want true, false", recs[0].won, recs[1].won)
			}
			if len(recs[0].encs) != 2 || len(recs[1].encs) != 1 {
				t.Errorf("nPlies mismatch: got %d, %d want 2, 1", len(recs[0].encs), len(recs[1].encs))
			}
		})
	}
}

// TestReadRecordsCallbackError: an error from the callback halts iteration and
// propagates out.
func TestReadRecordsCallbackError(t *testing.T) {
	stream := append(
		buildRecordBytes([]Board{testBoard(1)}, true),
		buildRecordBytes([]Board{testBoard(2)}, false)...,
	)
	boom := errors.New("boom")
	calls := 0
	err := ReadRecords(bytes.NewReader(stream), func(encs [][EncodingDim]float32, won bool) error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("ReadRecords error = %v, want %v", err, boom)
	}
	if calls != 1 {
		t.Fatalf("callback called %d times, want 1 (stop on first error)", calls)
	}
}

type errReader struct{ err error }

func (e errReader) Read(p []byte) (int, error) { return 0, e.err }

// TestReadRecordsReadError: a non-EOF read failure propagates rather than being
// swallowed as a clean end-of-stream.
func TestReadRecordsReadError(t *testing.T) {
	boom := errors.New("read failure")
	err := ReadRecords(errReader{err: boom}, func(encs [][EncodingDim]float32, won bool) error {
		return nil
	})
	if !errors.Is(err, boom) {
		t.Fatalf("ReadRecords error = %v, want %v", err, boom)
	}
}
