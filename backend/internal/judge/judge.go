// Package judge scores a run against its criteria with an LLM: anchored bands,
// worst case wins, cited evidence, three samples and the lower median.
package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"testforge/internal/llm"
)

// Criterion is one scenario criterion.
type Criterion struct {
	ID     string
	Text   string
	Kind   string
	Source string
	Weight int
}

// Fact is hidden scenario information.
type Fact struct{ Key, Value string }

// Turn is one transcript turn.
type Turn struct {
	Role        string
	Text        string
	StartMs     int
	EndMs       int
	LatencyMs   *int
	Interrupted bool
}

// Input is the evidence of one run.
type Input struct {
	Modality         string
	AgentDescription string
	Goal             string
	Instructions     string
	Context          string
	PersonaName      string
	Locale           string
	Temperament      string
	PersonaNotes     string
	Facts            []Fact
	Criteria         []Criterion
	Transcript       []Turn
	// TraceRows is the trace digest; nil when traces are unavailable.
	TraceRows   []string
	TraceStatus string // why traces are missing, for the prompt
}

// Failure tags; the first four cap a criterion at band 2.
var (
	FailureTags = []string{
		"fabricated_information", "wrong_information", "instruction_ignored", "unsafe_or_policy_violation",
		"missing_information_request", "incomplete_task", "off_topic", "unclear_or_unnatural_speech",
		"cut_off_or_incomplete_reply", "slow_response", "tool_or_trace_error",
	}
	cappingTags = map[string]bool{"fabricated_information": true, "wrong_information": true, "instruction_ignored": true, "unsafe_or_policy_violation": true}
	bandScore   = map[int]int{5: 100, 4: 90, 3: 70, 2: 50, 1: 25}
)

const capBand = 2

// Gate decides which criteria the judge can see evidence for. Gated criteria
// map to the reason they are not observable.
func Gate(in Input) ([]Criterion, map[string]string) {
	hasTiming := false
	for _, t := range in.Transcript {
		if t.Role == "agent" && t.LatencyMs != nil {
			hasTiming = true
		}
	}
	var judgeable []Criterion
	gated := map[string]string{}
	for _, c := range in.Criteria {
		switch {
		case c.Source == "trace" && in.TraceRows == nil:
			gated[c.ID] = "traces_unavailable"
		case c.Source == "timing" && !hasTiming:
			gated[c.ID] = "no_timing_data"
		default:
			judgeable = append(judgeable, c)
		}
	}
	return judgeable, gated
}

// Evidence is one cited piece of evidence.
type Evidence struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Quote  string `json:"quote"`
}

// Suggestion is a recommended change to the agent.
type Suggestion struct {
	Title        string   `json:"title"`
	Detail       string   `json:"detail"`
	Area         string   `json:"area"`
	Priority     string   `json:"priority"`
	EvidenceRefs []string `json:"evidenceRefs"`
}

// SampleCriterion is one criterion answer of one judge sample.
type SampleCriterion struct {
	CriterionID string     `json:"criterionId"`
	Evidence    []Evidence `json:"evidence"`
	Reasoning   string     `json:"reasoning"`
	Observable  bool       `json:"observable"`
	Band        int        `json:"band"`
	FailureTags []string   `json:"failureTags"`
}

// Sample is one judge answer.
type Sample struct {
	Summary     string            `json:"summary"`
	Criteria    []SampleCriterion `json:"criteria"`
	Suggestions []Suggestion      `json:"suggestions"`
	Strengths   []string          `json:"strengths"`
}

// ParseSample decodes and sanity-checks one judge answer.
func ParseSample(raw string) (Sample, error) {
	var s Sample
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &s); err != nil {
		return Sample{}, fmt.Errorf("the judge's answer is not valid JSON: %w", err)
	}
	for i, c := range s.Criteria {
		if c.Band < 1 || c.Band > 5 {
			s.Criteria[i].Band = max(1, min(5, c.Band))
		}
	}
	return s, nil
}

// CriterionResult is the aggregated judgement of one criterion.
type CriterionResult struct {
	CriterionID        string     `json:"criterionId"`
	Text               string     `json:"text"`
	Kind               string     `json:"kind"`
	Source             string     `json:"source"`
	Weight             int        `json:"weight"`
	Verdict            string     `json:"verdict"`
	Band               *int       `json:"band"`
	Score              *int       `json:"score"`
	FailureTags        []string   `json:"failureTags"`
	Capped             bool       `json:"capped"`
	Reasoning          string     `json:"reasoning"`
	Evidence           []Evidence `json:"evidence"`
	UnobservableReason *string    `json:"unobservableReason"`
	SampleBands        []*int     `json:"sampleBands"`
}

// Result is a run's judgement.
type Result struct {
	Summary      string            `json:"summary"`
	Score        *int              `json:"score"`
	Passed       *bool             `json:"passed"`
	MustFailed   bool              `json:"mustFailed"`
	Criteria     []CriterionResult `json:"criteria"`
	Suggestions  []Suggestion      `json:"suggestions"`
	Strengths    []string          `json:"strengths"`
	FailureTags  map[string]int    `json:"failureTags"`
	Samples      int               `json:"samples"`
	InputTokens  int               `json:"inputTokens"`
	OutputTokens int               `json:"outputTokens"`
}

func verdictFor(band int) string {
	switch {
	case band >= 4:
		return "pass"
	case band == 3:
		return "partial"
	}
	return "fail"
}

func normalizeTags(tags []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		known := false
		for _, k := range FailureTags {
			known = known || k == t
		}
		if known && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// effective applies the capping tags to one sample answer.
func effective(a SampleCriterion) (band int, tags []string, capped bool) {
	tags = normalizeTags(a.FailureTags)
	band = a.Band
	for _, t := range tags {
		if cappingTags[t] && band > capBand {
			band, capped = capBand, true
		}
	}
	if band == 5 {
		tags = []string{}
	}
	return band, tags, capped
}

// Aggregate combines samples: caps apply per sample, then the lower median band
// of the observable answers decides; the answer at that band supplies the
// reasoning, evidence and tags.
func Aggregate(in Input, judgeable []Criterion, gated map[string]string, samples []Sample) Result {
	r := Result{Criteria: []CriterionResult{}, Suggestions: []Suggestion{}, Strengths: []string{}, FailureTags: map[string]int{}, Samples: len(samples)}
	for _, s := range samples {
		if r.Summary == "" && s.Summary != "" {
			r.Summary, r.Suggestions, r.Strengths = s.Summary, s.Suggestions, s.Strengths
		}
	}
	if r.Suggestions == nil {
		r.Suggestions = []Suggestion{}
	}
	if r.Strengths == nil {
		r.Strengths = []string{}
	}
	weighted, weights := 0, 0
	for _, c := range in.Criteria {
		cr := CriterionResult{CriterionID: c.ID, Text: c.Text, Kind: c.Kind, Source: c.Source, Weight: c.Weight,
			FailureTags: []string{}, Evidence: []Evidence{}, SampleBands: []*int{}}
		if reason, ok := gated[c.ID]; ok {
			cr.Verdict, cr.UnobservableReason = "not_observable", &reason
			r.Criteria = append(r.Criteria, cr)
			continue
		}
		type pick struct {
			band   int
			tags   []string
			capped bool
			answer SampleCriterion
		}
		var picks []pick
		answered, observableCount := 0, 0
		for _, s := range samples {
			found := false
			for _, a := range s.Criteria {
				if a.CriterionID != c.ID || found {
					continue
				}
				found = true
				answered++
				if !a.Observable {
					cr.SampleBands = append(cr.SampleBands, nil)
					continue
				}
				observableCount++
				band, tags, capped := effective(a)
				b := band
				cr.SampleBands = append(cr.SampleBands, &b)
				picks = append(picks, pick{band, tags, capped, a})
			}
			if !found {
				cr.SampleBands = append(cr.SampleBands, nil)
			}
		}
		if answered == 0 || observableCount*2 <= answered {
			reason := "judge_marked_unobservable"
			if answered == 0 {
				reason = "judge_omitted"
			}
			cr.Verdict, cr.UnobservableReason = "not_observable", &reason
			r.Criteria = append(r.Criteria, cr)
			continue
		}
		sort.SliceStable(picks, func(i, j int) bool { return picks[i].band < picks[j].band })
		chosen := picks[(len(picks)-1)/2]
		band, score := chosen.band, bandScore[chosen.band]
		cr.Band, cr.Score, cr.Verdict = &band, &score, verdictFor(band)
		cr.FailureTags, cr.Capped = chosen.tags, chosen.capped
		cr.Reasoning = chosen.answer.Reasoning
		if chosen.answer.Evidence != nil {
			cr.Evidence = chosen.answer.Evidence
		}
		for _, t := range cr.FailureTags {
			r.FailureTags[t]++
		}
		if c.Kind == "must" && cr.Verdict == "fail" {
			r.MustFailed = true
		}
		weighted += score * c.Weight
		weights += c.Weight
		r.Criteria = append(r.Criteria, cr)
	}
	if weights > 0 {
		score := (weighted + weights/2) / weights
		passed := !r.MustFailed
		r.Score, r.Passed = &score, &passed
	}
	return r
}

// Run judges a run with up to samples model calls; failed samples are skipped.
func Run(ctx context.Context, client llm.Client, model string, in Input, samples int) (Result, error) {
	judgeable, gated := Gate(in)
	if len(judgeable) == 0 {
		return Aggregate(in, judgeable, gated, nil), nil
	}
	system, user := BuildPrompt(in, judgeable, gated)
	var got []Sample
	var lastErr error
	in0, out0 := 0, 0
	for range samples {
		res, err := client.Complete(ctx, llm.Request{Model: model, System: system, Prompt: user,
			SchemaName: "run_judgement", Schema: Schema, MaxTokens: 8000})
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				break
			}
			continue
		}
		in0 += res.InputTokens
		out0 += res.OutputTokens
		s, err := ParseSample(res.JSON)
		if err != nil {
			lastErr = err
			continue
		}
		got = append(got, s)
	}
	if len(got) == 0 {
		if lastErr == nil {
			lastErr = errors.New("no judge sample succeeded")
		}
		return Result{}, fmt.Errorf("judge error: %w", lastErr)
	}
	r := Aggregate(in, judgeable, gated, got)
	r.InputTokens, r.OutputTokens = in0, out0
	return r, nil
}

var roleLine = regexp.MustCompile(`(?im)^(\s*)(system|assistant|user|developer|human)\s*:`)

// escape keeps evidence inside its block: it cannot close a tag or pose as a chat role.
func escape(s string, limit int) string {
	s = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
	s = roleLine.ReplaceAllString(s, "$1> $2:")
	if r := []rune(s); len(r) > limit {
		s = string(r[:limit]) + "…"
	}
	return s
}
