package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxShardBytes = 64 << 20 // multiplies by 2^20

type Recorder struct {
	mu          sync.Mutex
	dir, prefix string
	f           *os.File
	size        int64
	idx         int
}

func NewRecorder(dir string) (*Recorder, error) {
	os.MkdirAll(dir, 0750)
	// name shards <unixnano><idx>.bin
	prefix := time.Now().UnixNano()
	name := fmt.Sprintf("%d%05d.bin", prefix, 0)
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}
	return &Recorder{mu: sync.Mutex{}, dir: dir, prefix: fmt.Sprintf("%d", prefix), f: f, size: 0, idx: 0}, nil
}
func (r *Recorder) Write(trajectory []Board, whiteWon bool) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(trajectory))); err != nil {
		return err
	}
	for _, board := range trajectory {
		enc := Encode(board) // [EncodingDim]float32
		if err := binary.Write(&buf, binary.LittleEndian, enc); err != nil {
			return err
		}
	}

	var won byte
	if whiteWon {
		won = 1
	}
	buf.WriteByte(won)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size >= maxShardBytes {
		if err := r.rotate(); err != nil {
			return err
		}
	}
	n, err := r.f.Write(buf.Bytes())

	r.size += int64(n)
	return err
}

func (r *Recorder) rotate() error {
	if err := r.f.Close(); err != nil {
		return err
	}
	r.idx++
	name := fmt.Sprintf("%s%05d.bin", r.prefix, r.idx)
	f, err := os.OpenFile(filepath.Join(r.dir, name), os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	r.f = f
	r.size = 0
	return nil
}
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}

func ReadRecords(rd io.Reader, fn func(encs [][EncodingDim]float32, whiteWon bool) error) error {
	const stride = EncodingDim * 4 // bytes per board
	header := make([]byte, 4)
	won := make([]byte, 1)
	for {
		// Record boundary. Clean EOF = done, partial header = record
		// Still being appended. Also clean stop for tailer
		if _, err := io.ReadFull(rd, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}

		nPlies := binary.LittleEndian.Uint32(header)

		payload := make([]byte, int(nPlies)*stride)
		if _, err := io.ReadFull(rd, payload); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}
		if _, err := io.ReadFull(rd, won); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}

		encs := make([][EncodingDim]float32, nPlies)
		if err := binary.Read(bytes.NewReader(payload), binary.LittleEndian, encs); err != nil {
			return err
		}
		if err := fn(encs, won[0] == 1); err != nil {
			return err
		}
	}
}
