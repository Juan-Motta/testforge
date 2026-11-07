package run

import "testing"

func TestPercentile(t *testing.T) {
	if percentile(nil, 0.5) != nil {
		t.Fatal("empty percentile must be nil")
	}
	vals := []int{900, 100, 500, 300, 700}
	if got := *percentile(vals, 0.5); got != 500 {
		t.Fatalf("p50 = %d", got)
	}
	if got := *percentile(vals, 0.95); got != 900 {
		t.Fatalf("p95 = %d", got)
	}
	if got := *percentile([]int{42}, 0.95); got != 42 {
		t.Fatalf("single p95 = %d", got)
	}
}
