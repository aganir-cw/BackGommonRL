package engine

import (
	"encoding/binary"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// sumEnc returns the sum of an encoding's features; used as a deterministic
// stand-in "score" so tests can verify per-position transport and ordering.
func sumEnc(e [EncodingDim]float32) float32 {
	var s float32
	for _, f := range e {
		s += f
	}
	return s
}

// fakeScoreServer mimics serve.py's /score protocol in pure Go: it validates the
// request framing (uint32 count + count*198 float32 LE) and replies with one
// float32 per position (the sum of that position's features), no length prefix.
func fakeScoreServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/score" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		count := int(binary.LittleEndian.Uint32(body[:4]))
		if len(body) != 4+count*EncodingDim*4 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		out := make([]byte, count*4)
		off := 4
		for i := 0; i < count; i++ {
			var sum float32
			for k := 0; k < EncodingDim; k++ {
				sum += math.Float32frombits(binary.LittleEndian.Uint32(body[off : off+4]))
				off += 4
			}
			binary.LittleEndian.PutUint32(out[i*4:i*4+4], math.Float32bits(sum))
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(out)
	}
	return httptest.NewServer(http.HandlerFunc(h))
}

// collectEncodings gathers n distinct-ish encodings from a seeded random playout.
func collectEncodings(n int) [][EncodingDim]float32 {
	dice := NewDice(1)
	b := Start()
	encs := make([][EncodingDim]float32, 0, n)
	for len(encs) < n {
		encs = append(encs, Encode(b))
		if b.WhiteOff == 15 || b.BlackOff == 15 {
			b = Start()
			continue
		}
		d1, d2 := dice.Roll()
		states := LegalAfterstates(b, d1, d2)
		b = states[dice.Pick(len(states))]
	}
	return encs
}

// Score a single Start() position: proves the round-trip and shape.
func TestScorerStart(t *testing.T) {
	srv := fakeScoreServer(t)
	defer srv.Close()
	s := &Scorer{url: srv.URL}

	enc := Encode(Start())
	got, err := s.Score([][EncodingDim]float32{enc})
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0] != sumEnc(enc) {
		t.Fatalf("got[0] = %v, want %v", got[0], sumEnc(enc))
	}
	t.Logf("score(Start()) = %v", got[0])
}

// Score Start() and its mirror together: proves multi-position framing and that
// results come back in request order (each matches its own encoding's signature).
func TestScorerStartAndMirror(t *testing.T) {
	srv := fakeScoreServer(t)
	defer srv.Close()
	s := &Scorer{url: srv.URL}

	encs := [][EncodingDim]float32{Encode(Start()), Encode(Start().Mirror())}
	got, err := s.Score(encs)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for i, e := range encs {
		if got[i] != sumEnc(e) {
			t.Errorf("got[%d] = %v, want %v (order preserved?)", i, got[i], sumEnc(e))
		}
	}
}

// A 100-position batch: the plan's stride/shape check.
func TestScorerBatch100(t *testing.T) {
	srv := fakeScoreServer(t)
	defer srv.Close()
	s := &Scorer{url: srv.URL}

	encs := collectEncodings(100)
	got, err := s.Score(encs)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(got) != 100 {
		t.Fatalf("len(got) = %d, want 100", len(got))
	}
	for i, e := range encs {
		if got[i] != sumEnc(e) {
			t.Errorf("position %d mismatched: got %v want %v", i, got[i], sumEnc(e))
		}
	}
}

// A non-200 status must surface as an error, not silent garbage.
func TestScorerNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	s := &Scorer{url: srv.URL}

	if _, err := s.Score([][EncodingDim]float32{Encode(Start())}); err == nil {
		t.Fatal("expected an error on 500 status")
	}
}

// A response whose length doesn't match count must be rejected.
func TestScorerBadResponseLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0, 0, 0}) // 3 bytes: never a valid count*4
	}))
	defer srv.Close()
	s := &Scorer{url: srv.URL}

	if _, err := s.Score([][EncodingDim]float32{Encode(Start())}); err == nil {
		t.Fatal("expected an error on short response body")
	}
}

// Live Go->Python->Go smoke. Skipped unless SCORER_URL points at a running
// serve.py, e.g.:  SCORER_URL=http://127.0.0.1:8000 go test ./engine -run Integration
func TestScorerIntegration(t *testing.T) {
	url := os.Getenv("SCORER_URL")
	if url == "" {
		t.Skip("set SCORER_URL to run the live Go->Python->Go smoke")
	}
	s := &Scorer{url: url}

	encs := [][EncodingDim]float32{Encode(Start()), Encode(Start().Mirror())}
	got, err := s.Score(encs)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for i, v := range got {
		if v < 0 || v > 1 {
			t.Errorf("got[%d] = %v, not a probability in [0,1]", i, v)
		}
	}
	t.Logf("live scores: Start=%v Mirror=%v", got[0], got[1])

	big := collectEncodings(100)
	gotBig, err := s.Score(big)
	if err != nil {
		t.Fatalf("Score(100): %v", err)
	}
	if len(gotBig) != 100 {
		t.Fatalf("len(gotBig) = %d, want 100", len(gotBig))
	}
}
