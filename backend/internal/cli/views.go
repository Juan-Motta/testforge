package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"
)

// The views decode only what the tables print; --json prints the API's JSON.

type meView struct {
	User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
	Workspaces []struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"workspaces"`
	AuthMethod string `json:"authMethod"`
}

type agentView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Modality  string `json:"modality"`
	Transport struct {
		URL string `json:"url"`
	} `json:"transport"`
	Codec struct {
		Type string `json:"type"`
	} `json:"codec"`
}

type scenarioView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Modality string   `json:"modality"`
	Tags     []string `json:"tags"`
	Goal     string   `json:"goal"`
}

type metricsView struct {
	DurationMs   int  `json:"durationMs"`
	UserTurns    int  `json:"userTurns"`
	AgentTurns   int  `json:"agentTurns"`
	LatencyP50Ms *int `json:"latencyP50Ms"`
	LatencyP95Ms *int `json:"latencyP95Ms"`
}

type runView struct {
	ID              string       `json:"id"`
	BatchID         string       `json:"batchId"`
	ScenarioName    string       `json:"scenarioName"`
	AgentName       string       `json:"agentName"`
	PersonaName     string       `json:"personaName"`
	Modality        string       `json:"modality"`
	Status          string       `json:"status"`
	EndReason       *string      `json:"endReason"`
	Error           *string      `json:"error"`
	CreatedAt       time.Time    `json:"createdAt"`
	Metrics         *metricsView `json:"metrics"`
	JudgementStatus *string      `json:"judgementStatus"`
	Score           *int         `json:"score"`
	Passed          *bool        `json:"passed"`
	Transcript      []struct {
		Role        string `json:"role"`
		Text        string `json:"text"`
		StartMs     int    `json:"startMs"`
		LatencyMs   *int   `json:"latencyMs"`
		Interrupted bool   `json:"interrupted"`
	} `json:"transcript"`
}

type runPage struct {
	Items      []runView `json:"items"`
	NextCursor *string   `json:"nextCursor"`
}

type judgementView struct {
	Status string  `json:"status"`
	Model  string  `json:"model"`
	Error  *string `json:"error"`
	Result *struct {
		Summary    string `json:"summary"`
		Score      *int   `json:"score"`
		Passed     *bool  `json:"passed"`
		MustFailed bool   `json:"mustFailed"`
		Criteria   []struct {
			Text        string   `json:"text"`
			Kind        string   `json:"kind"`
			Verdict     string   `json:"verdict"`
			Band        *int     `json:"band"`
			Score       *int     `json:"score"`
			Reasoning   string   `json:"reasoning"`
			FailureTags []string `json:"failureTags"`
		} `json:"criteria"`
		Suggestions []struct {
			Title    string `json:"title"`
			Detail   string `json:"detail"`
			Area     string `json:"area"`
			Priority string `json:"priority"`
		} `json:"suggestions"`
		Strengths []string `json:"strengths"`
	} `json:"result"`
	Traces *struct {
		Status  string  `json:"status"`
		Message string  `json:"message"`
		URL     *string `json:"url"`
	} `json:"traces"`
}

type reportView struct {
	Days            int            `json:"days"`
	Totals          map[string]int `json:"totals"`
	Judged          int            `json:"judged"`
	AvgScore        *float64       `json:"avgScore"`
	PassRate        *float64       `json:"passRate"`
	LatencyP50Ms    *float64       `json:"latencyP50Ms"`
	LatencyP95Ms    *float64       `json:"latencyP95Ms"`
	LLMTokens       int            `json:"llmTokens"`
	WeakestCriteria []struct {
		ScenarioName string  `json:"scenarioName"`
		Text         string  `json:"text"`
		AvgScore     float64 `json:"avgScore"`
		FailRate     float64 `json:"failRate"`
		Judged       int     `json:"judged"`
	} `json:"weakestCriteria"`
	Scenarios []struct {
		ScenarioName string    `json:"scenarioName"`
		AgentName    string    `json:"agentName"`
		Runs         int       `json:"runs"`
		AvgScore     *float64  `json:"avgScore"`
		PassRate     *float64  `json:"passRate"`
		LastRunAt    time.Time `json:"lastRunAt"`
	} `json:"scenarios"`
	FailureTags map[string]int `json:"failureTags"`
}

func table(w io.Writer, header string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, header)
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	_ = tw.Flush()
}

// clip shortens text to n runes on one line.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func intOr(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p)
}

func boolOr(p *bool) string {
	switch {
	case p == nil:
		return "-"
	case *p:
		return "yes"
	default:
		return "no"
	}
}

func strOr(p *string) string {
	if p == nil {
		return "-"
	}
	return *p
}

func pct(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", *p*100)
}

func num(p *float64, unit string) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%s", *p, unit)
}

func ms(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p) + "ms"
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func runRows(runs []runView) [][]string {
	rows := make([][]string, 0, len(runs))
	for _, r := range runs {
		rows = append(rows, []string{r.ID, r.Status, strOr(r.JudgementStatus), intOr(r.Score), boolOr(r.Passed),
			clip(r.ScenarioName, 40), clip(r.PersonaName, 24), r.CreatedAt.Local().Format("2006-01-02 15:04")})
	}
	return rows
}

const runHeader = "ID\tSTATUS\tEVALUATION\tSCORE\tPASSED\tSCENARIO\tPERSONA\tCREATED"

func printRun(w io.Writer, r runView) {
	fmt.Fprintf(w, "Run        %s\n", r.ID)
	fmt.Fprintf(w, "Scenario   %s\n", r.ScenarioName)
	fmt.Fprintf(w, "Agent      %s (%s)\n", r.AgentName, r.Modality)
	fmt.Fprintf(w, "Persona    %s\n", r.PersonaName)
	fmt.Fprintf(w, "Batch      %s\n", r.BatchID)
	status := r.Status
	if r.EndReason != nil {
		status += " (" + *r.EndReason + ")"
	}
	fmt.Fprintf(w, "Status     %s\n", status)
	if r.Error != nil {
		fmt.Fprintf(w, "Error      %s\n", *r.Error)
	}
	if m := r.Metrics; m != nil {
		fmt.Fprintf(w, "Metrics    %.1fs, %d user / %d agent turns, latency p50 %s p95 %s\n",
			float64(m.DurationMs)/1000, m.UserTurns, m.AgentTurns, ms(m.LatencyP50Ms), ms(m.LatencyP95Ms))
	}
}

func printJudgement(w io.Writer, j judgementView) {
	fmt.Fprintf(w, "Evaluation %s", j.Status)
	if j.Model != "" {
		fmt.Fprintf(w, " (judge %s)", j.Model)
	}
	fmt.Fprintln(w)
	if j.Error != nil {
		fmt.Fprintf(w, "           %s\n", *j.Error)
	}
	if t := j.Traces; t != nil {
		line := t.Status
		if t.Message != "" {
			line += ": " + t.Message
		}
		if t.URL != nil {
			line += " " + *t.URL
		}
		fmt.Fprintf(w, "Traces     %s\n", line)
	}
	res := j.Result
	if res == nil {
		return
	}
	fmt.Fprintf(w, "Score      %s, passed %s", intOr(res.Score), boolOr(res.Passed))
	if res.MustFailed {
		fmt.Fprint(w, " (a must criterion failed)")
	}
	fmt.Fprintln(w)
	if res.Summary != "" {
		fmt.Fprintf(w, "Summary    %s\n", res.Summary)
	}
	if len(res.Criteria) > 0 {
		fmt.Fprintln(w, "\nCriteria")
		rows := make([][]string, 0, len(res.Criteria))
		for _, c := range res.Criteria {
			rows = append(rows, []string{c.Verdict, intOr(c.Band), intOr(c.Score), c.Kind, clip(c.Text, 70), strings.Join(c.FailureTags, ",")})
		}
		table(w, "VERDICT\tBAND\tSCORE\tKIND\tCRITERION\tTAGS", rows)
	}
	if len(res.Suggestions) > 0 {
		fmt.Fprintln(w, "\nSuggestions")
		for _, s := range res.Suggestions {
			fmt.Fprintf(w, "  [%s/%s] %s: %s\n", s.Priority, s.Area, s.Title, clip(s.Detail, 160))
		}
	}
	if len(res.Strengths) > 0 {
		fmt.Fprintln(w, "\nStrengths")
		for _, s := range res.Strengths {
			fmt.Fprintf(w, "  - %s\n", s)
		}
	}
}
