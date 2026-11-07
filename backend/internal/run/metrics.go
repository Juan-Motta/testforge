package run

import "sort"

// percentile returns the nearest-rank percentile of values (nil when empty).
func percentile(values []int, p float64) *int {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	rank := int(p*float64(len(sorted))+0.999999) - 1
	rank = max(0, min(rank, len(sorted)-1))
	v := sorted[rank]
	return &v
}

// summarize computes metrics from a transcript.
func summarize(turns []Turn, durationMs, agentAudioMs, tokensIn, tokensOut int) Metrics {
	m := Metrics{DurationMs: durationMs, AgentAudioMs: agentAudioMs, LLMInputTokens: tokensIn, LLMOutputTokens: tokensOut}
	var latencies []int
	for _, t := range turns {
		if t.Role == "user" {
			m.UserTurns++
			continue
		}
		m.AgentTurns++
		if t.LatencyMs != nil {
			latencies = append(latencies, *t.LatencyMs)
		}
	}
	m.LatencyP50Ms = percentile(latencies, 0.5)
	m.LatencyP95Ms = percentile(latencies, 0.95)
	return m
}
