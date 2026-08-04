package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
)

// client.go is a client for the server.serve.py server.
// Responsibilities:
// - Serialize a batch of [EncodingDim]float32 to a byte slice
//		- into the wire format: uint32 count (LE) + count*198 float32 (LE)
// Send that as a body of a request to <url>/score
// - Read the response bytes, deserialize them into a count little-endian f32 value
// return them as []float32, 1prob per input position(same order in as out)
// reuse the buffer across calls so a hot self-play loop doesn't reallocate
// Report http errors, non200s, response-len mismatches, etc.

type Scorer struct {
	url string
	buf []byte
}

func NewScorer(url string) *Scorer {
	return &Scorer{url: url}
}

func (s *Scorer) Score(encs [][EncodingDim]float32) ([]float32, error) {
	const dim = EncodingDim
	count := len(encs)

	need := 4 + count*dim*4 // Request buffer is 4 (uint header) + count positions * 198 floats per position * 4 bytes per float

	if cap(s.buf) < need {
		s.buf = make([]byte, need)
	}

	buf := s.buf[:need]

	binary.LittleEndian.PutUint32(buf[0:4], uint32(count))

	off := 4
	for _, enc := range encs {
		for _, f := range enc {
			binary.LittleEndian.PutUint32(buf[off:off+4], math.Float32bits(f))
			off += 4
		}
	}

	resp, err := http.Post(s.url+"/score", "application/octet-stream", bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("score: status %d", resp.StatusCode)
	}

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(out) != count*4 {
		return nil, fmt.Errorf("score: body len %d != %d", len(out), count*4)
	}

	probs := make([]float32, count)
	for i := range probs {
		bits := binary.LittleEndian.Uint32(out[i*4 : i*4+4])
		probs[i] = math.Float32frombits(bits)
	}
	return probs, nil
}
