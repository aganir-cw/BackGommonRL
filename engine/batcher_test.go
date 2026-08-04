package engine

import (
	"encoding/binary"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// indexValueServer is a fake /score backend: for a batch of N positions it
// returns the float value i for position i (0,1,2,...). That makes the score of
// each concatenated position equal to its offset in the batch, so a correct
// scatter hands every request the slice of offsets it contributed.
func indexValueServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil || len(buf) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		count := int(binary.LittleEndian.Uint32(buf[:4]))
		out := make([]byte, count*4)
		for i := 0; i < count; i++ {
			binary.LittleEndian.PutUint32(out[i*4:i*4+4], math.Float32bits(float32(i)))
		}
		w.Write(out)
	}
	return httptest.NewServer(http.HandlerFunc(h))
}

func mkEncs(k int) [][EncodingDim]float32 {
	return make([][EncodingDim]float32, k)
}

func eqF32(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Three requests of 2/5/1 positions batched together: each reply must receive
// exactly its own contiguous slice of the concatenated scores.
func TestBatcherScatterOffsets(t *testing.T) {
	srv := indexValueServer(t)
	defer srv.Close()
	s := &Scorer{url: srv.URL}

	bt := &Batcher{In: make(chan EvalReq), MaxBatch: 4096, Timeout: 100 * time.Millisecond}
	go bt.Run(s)

	sizes := []int{2, 5, 1}
	reqs := make([]EvalReq, len(sizes))
	for i, k := range sizes {
		reqs[i] = EvalReq{Encs: mkEncs(k), Reply: make(chan []float32, 1)}
		bt.In <- reqs[i]
	}

	want := [][]float32{
		{0, 1},
		{2, 3, 4, 5, 6},
		{7},
	}
	for i := range reqs {
		select {
		case got := <-reqs[i].Reply:
			if !eqF32(got, want[i]) {
				t.Errorf("request %d (size %d): got %v, want %v", i, sizes[i], got, want[i])
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("request %d never received a reply", i)
		}
	}
}

// A single request with no further traffic can only be flushed by the timer;
// its reply must therefore arrive within roughly 2x the timeout.
func TestBatcherTimeoutFires(t *testing.T) {
	srv := indexValueServer(t)
	defer srv.Close()
	s := &Scorer{url: srv.URL}

	timeout := 50 * time.Millisecond
	bt := &Batcher{In: make(chan EvalReq), MaxBatch: 4096, Timeout: timeout}
	go bt.Run(s)

	req := EvalReq{Encs: mkEncs(1), Reply: make(chan []float32, 1)}
	start := time.Now()
	bt.In <- req

	select {
	case got := <-req.Reply:
		elapsed := time.Since(start)
		if !eqF32(got, []float32{0}) {
			t.Fatalf("got %v, want [0]", got)
		}
		if elapsed > 2*timeout {
			t.Errorf("reply took %v, want <= 2x timeout (%v)", elapsed, 2*timeout)
		}
		if elapsed < timeout/2 {
			t.Errorf("reply took %v, suspiciously fast (timer should gate on ~%v)", elapsed, timeout)
		}
		t.Logf("timeout-driven flush after %v", elapsed)
	case <-time.After(2 * time.Second):
		t.Fatal("no reply; timeout never fired")
	}
}
