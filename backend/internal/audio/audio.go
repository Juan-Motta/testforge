// Package audio holds PCM16 mono helpers: resampling, WAV encoding and
// placing chunks on a timeline to rebuild a conversation.
package audio

import (
	"encoding/binary"
	"math"
	"sync"
)

func sample(b []byte, i int) int16 { return int16(binary.LittleEndian.Uint16(b[i*2:])) }

// Resample converts PCM16 mono between sample rates with linear interpolation.
func Resample(pcm []byte, from, to int) []byte {
	if from == to || len(pcm) < 2 {
		return pcm
	}
	n := len(pcm) / 2
	outN := int(int64(n) * int64(to) / int64(from))
	out := make([]byte, outN*2)
	for i := range outN {
		pos := float64(i) * float64(from) / float64(to)
		j := int(pos)
		frac := pos - float64(j)
		a := float64(sample(pcm, min(j, n-1)))
		b := float64(sample(pcm, min(j+1, n-1)))
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(math.Round(a+(b-a)*frac))))
	}
	return out
}

// WAV wraps PCM16 mono in a WAV container.
func WAV(pcm []byte, rate int) []byte {
	out := make([]byte, 44+len(pcm))
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1) // PCM
	binary.LittleEndian.PutUint16(out[22:], 1) // mono
	binary.LittleEndian.PutUint32(out[24:], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(out[32:], 2)
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm)))
	copy(out[44:], pcm)
	return out
}

// IsSilent reports whether a chunk has no meaningful energy.
func IsSilent(pcm []byte) bool {
	for i := range len(pcm) / 2 {
		if v := sample(pcm, i); v > 300 || v < -300 {
			return false
		}
	}
	return true
}

// Timeline places PCM chunks at millisecond offsets.
type Timeline struct {
	mu   sync.Mutex
	rate int
	pcm  []byte
}

// NewTimeline creates an empty timeline at rate.
func NewTimeline(rate int) *Timeline { return &Timeline{rate: rate} }

// Place writes pcm starting at atMs, overwriting what was there.
func (t *Timeline) Place(atMs int, pcm []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	off := atMs * t.rate / 1000 * 2
	if need := off + len(pcm); need > len(t.pcm) {
		t.pcm = append(t.pcm, make([]byte, need-len(t.pcm))...)
	}
	copy(t.pcm[off:], pcm)
}

// Bytes returns the timeline's PCM.
func (t *Timeline) Bytes() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.pcm...)
}

// Empty reports whether nothing was placed.
func (t *Timeline) Empty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pcm) == 0
}

// Mix sums timelines of the same rate, clipping to int16.
func Mix(tracks ...*Timeline) []byte {
	var n int
	data := make([][]byte, len(tracks))
	for i, t := range tracks {
		data[i] = t.Bytes()
		n = max(n, len(data[i]))
	}
	out := make([]byte, n)
	for i := 0; i+1 < n; i += 2 {
		sum := 0
		for _, d := range data {
			if i+1 < len(d) {
				sum += int(int16(binary.LittleEndian.Uint16(d[i:])))
			}
		}
		sum = max(math.MinInt16, min(math.MaxInt16, sum))
		binary.LittleEndian.PutUint16(out[i:], uint16(int16(sum)))
	}
	return out
}
