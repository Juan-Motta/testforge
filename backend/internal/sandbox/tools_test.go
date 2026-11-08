package sandbox

import (
	"context"
	"testing"
)

func TestRenderArgs(t *testing.T) {
	args := map[string]any{"id": "A1", "n": 2.5, "ok": true, "items": []any{"x"}, "addr": map[string]any{"city": "Lima"}, "none": nil}
	got := render(`{{args.id}}|{{ args.n }}|{{args.ok}}|{{args.items}}|{{args.addr.city}}|{{args.addr}}|{{args.none}}|{{args.missing}}|{{args.id.deep}}|{{other}}`, args)
	want := `A1|2.5|true|["x"]|Lima|{"city":"Lima"}||||{{other}}`
	if got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
}

func TestRunMockFaultInjection(t *testing.T) {
	tool := Tool{Response: "ok {{args.id}}", ErrorRate: 0.5}
	res, err := runMock(context.Background(), tool, map[string]any{"id": "7"}, func() float64 { return 0.49 })
	if err != nil || !res.error || res.text != `{"error":"The tool failed."}` {
		t.Fatalf("below rate = %+v, %v", res, err)
	}
	res, _ = runMock(context.Background(), tool, map[string]any{"id": "7"}, func() float64 { return 0.5 })
	if res.error || res.text != "ok 7" {
		t.Fatalf("at rate = %+v", res)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runMock(ctx, Tool{LatencyMs: 10000}, nil, defaultRand); err == nil {
		t.Fatal("latency ignored cancellation")
	}
}
