package engine

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	limitChunkMin  = 16 << 10
	limitChunkMax  = 1 << 20
	limitChunkTime = 5 * time.Millisecond

	limitBurstMin  = 16 << 10
	limitBurstTime = 50 * time.Millisecond

	udpFree = 512

	bytesPerMbit = 1_000_000 / 8
)

type limitPair struct {
	in, out    *bucket
	mbitPerSec float64
	chunk      int
}

type bucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newBucket(bytesPerSec float64) *bucket {
	burst := math.Max(limitBurstMin, bytesPerSec*limitBurstTime.Seconds())
	return &bucket{rate: bytesPerSec, burst: burst, tokens: burst, last: time.Now()}
}

func (b *bucket) take(n int) {
	b.mu.Lock()
	now := time.Now()
	b.tokens = math.Min(b.burst, b.tokens+now.Sub(b.last).Seconds()*b.rate)
	b.last = now
	var wait time.Duration
	if need := float64(n); need > b.tokens {
		seconds := (need - b.tokens) / b.rate
		b.tokens = 0
		b.last = now.Add(time.Duration(seconds * float64(time.Second)))
		wait = time.Duration(seconds * float64(time.Second))
	} else {
		b.tokens -= need
	}
	b.mu.Unlock()
	if wait > 0 {
		time.Sleep(wait)
	}
}

const LimitMaxMbps = 1e6

func ParseLimit(s string) (float64, error) {
	t := strings.TrimSpace(s)
	switch strings.ToLower(t) {
	case "unlimited", "off", "none":
		return 0, nil
	}
	mbitPerSec, err := strconv.ParseFloat(t, 64)
	if err != nil || math.IsNaN(mbitPerSec) || math.IsInf(mbitPerSec, 0) {
		return 0, fmt.Errorf("enter a speed in Mbit/s, like 50 or 7.5 (0 for unlimited)")
	}
	if mbitPerSec < 0 || mbitPerSec > LimitMaxMbps {
		return 0, fmt.Errorf("the speed must be between 0 and %.0f Mbit/s", float64(LimitMaxMbps))
	}
	return mbitPerSec, nil
}

func limitChunkFor(bytesPerSec float64) int {
	n := bytesPerSec * limitChunkTime.Seconds()
	switch {
	case n < limitChunkMin:
		return limitChunkMin
	case n > limitChunkMax:
		return limitChunkMax
	default:
		return int(n)
	}
}

func (sl *Slot) setLimit(mbitPerSec float64) {
	if mbitPerSec <= 0 {
		sl.limit.Store(nil)
		return
	}
	rate := mbitPerSec * bytesPerMbit
	sl.limit.Store(&limitPair{
		in:         newBucket(rate),
		out:        newBucket(rate),
		mbitPerSec: mbitPerSec,
		chunk:      limitChunkFor(rate),
	})
}

func (sl *Slot) limitMbps() float64 {
	if p := sl.limit.Load(); p != nil {
		return p.mbitPerSec
	}
	return 0
}

func (sl *Slot) throttle(down bool, n int) {
	p := sl.limit.Load()
	if p == nil || n <= 0 {
		return
	}
	b := p.out
	if down {
		b = p.in
	}
	for n > 0 {
		k := min(n, p.chunk)
		b.take(k)
		n -= k
	}
}

func (sl *Slot) throttlePacket(down bool, n int) {
	if n <= udpFree {
		return
	}
	sl.throttle(down, n)
}

func (sl *Slot) writePaced(dst io.Writer, p []byte, down bool) (int, error) {
	pair := sl.limit.Load()
	if pair == nil {
		return dst.Write(p)
	}
	written := 0
	for len(p) > 0 {
		k := min(len(p), pair.chunk)
		sl.throttle(down, k)
		n, err := dst.Write(p[:k])
		written += n
		if err != nil {
			return written, err
		}
		if n < k {
			return written, io.ErrShortWrite
		}
		p = p[k:]
	}
	return written, nil
}
