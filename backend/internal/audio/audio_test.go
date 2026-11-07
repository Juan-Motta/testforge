package audio

import (
	"encoding/binary"
	"testing"
)

func samples(vals ...int16) []byte {
	b := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return b
}

func TestResample(t *testing.T) {
	in := samples(0, 100, 200, 300)
	if got := Resample(in, 24000, 24000); string(got) != string(in) {
		t.Fatal("same rate must return the input")
	}
	down := Resample(samples(0, 100, 200, 300, 400, 500), 24000, 16000)
	if len(down) != 8 {
		t.Fatalf("downsampled %d bytes, want 8", len(down))
	}
	up := Resample(samples(0, 300), 8000, 24000)
	if len(up) != 12 || int16(binary.LittleEndian.Uint16(up[2:])) != 100 {
		t.Fatalf("upsampled = %v", up)
	}
}

func TestWAVHeader(t *testing.T) {
	w := WAV(samples(1, 2, 3), 16000)
	if string(w[0:4]) != "RIFF" || string(w[8:12]) != "WAVE" || binary.LittleEndian.Uint32(w[24:28]) != 16000 ||
		binary.LittleEndian.Uint32(w[40:44]) != 6 || len(w) != 50 {
		t.Fatalf("wav = %v", w[:44])
	}
}

func TestTimelineMix(t *testing.T) {
	user := NewTimeline(1000) // 1 sample per ms
	agent := NewTimeline(1000)
	user.Place(0, samples(1000, 1000))
	agent.Place(1, samples(500, 32000))
	mixed := Mix(user, agent)
	got := []int16{int16(binary.LittleEndian.Uint16(mixed[0:])), int16(binary.LittleEndian.Uint16(mixed[2:])), int16(binary.LittleEndian.Uint16(mixed[4:]))}
	if got[0] != 1000 || got[1] != 1500 || got[2] != 32000 {
		t.Fatalf("mix = %v", got)
	}
	clip := Mix(agent, agent)
	if v := int16(binary.LittleEndian.Uint16(clip[4:])); v != 32767 {
		t.Fatalf("clipped = %d", v)
	}
}

func TestIsSilent(t *testing.T) {
	if !IsSilent(samples(0, 3, -2)) || IsSilent(samples(0, 4000)) {
		t.Fatal("silence detection")
	}
}
