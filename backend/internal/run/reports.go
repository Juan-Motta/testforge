package run

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"testforge/internal/platform/apperr"
	"testforge/internal/workspace"
)

// TrendPoint is one day of a report.
type TrendPoint struct {
	Date     string   `json:"date"`
	Runs     int      `json:"runs"`
	AvgScore *float64 `json:"avgScore"`
	PassRate *float64 `json:"passRate"`
}

// WeakCriterion is a criterion that scores low.
type WeakCriterion struct {
	ScenarioID   *string `json:"scenarioId"`
	ScenarioName string  `json:"scenarioName"`
	CriterionID  string  `json:"criterionId"`
	Text         string  `json:"text"`
	AvgScore     float64 `json:"avgScore"`
	FailRate     float64 `json:"failRate"`
	Judged       int     `json:"judged"`
}

// ScenarioRow summarizes one scenario.
type ScenarioRow struct {
	ScenarioID   *string   `json:"scenarioId"`
	ScenarioName string    `json:"scenarioName"`
	AgentName    string    `json:"agentName"`
	Runs         int       `json:"runs"`
	AvgScore     *float64  `json:"avgScore"`
	PassRate     *float64  `json:"passRate"`
	LastRunAt    time.Time `json:"lastRunAt"`
}

// Overview is the reports page.
type Overview struct {
	Days            int             `json:"days"`
	Totals          map[string]int  `json:"totals"`
	Judged          int             `json:"judged"`
	AvgScore        *float64        `json:"avgScore"`
	PassRate        *float64        `json:"passRate"`
	LatencyP50Ms    *float64        `json:"latencyP50Ms"`
	LatencyP95Ms    *float64        `json:"latencyP95Ms"`
	LLMTokens       int             `json:"llmTokens"`
	Trend           []TrendPoint    `json:"trend"`
	WeakestCriteria []WeakCriterion `json:"weakestCriteria"`
	Scenarios       []ScenarioRow   `json:"scenarios"`
	FailureTags     map[string]int  `json:"failureTags"`
}

// OverviewFilter narrows a report.
type OverviewFilter struct {
	Days       int
	AgentID    string
	ScenarioID string
}

// ReportOverview aggregates runs and judgements over a period.
func (s *Service) ReportOverview(ctx context.Context, ws workspace.Workspace, f OverviewFilter) (Overview, error) {
	if f.Days == 0 {
		f.Days = 7
	}
	fields := map[string]string{}
	if f.Days != 7 && f.Days != 30 && f.Days != 90 {
		fields["days"] = "Use 7, 30 or 90."
	}
	where := "r.workspace_id = $1 AND r.created_at >= date_trunc('day', now()) - make_interval(days => $2 - 1)"
	args := []any{ws.ID, f.Days}
	for _, c := range []struct{ name, val, col string }{{"agentId", f.AgentID, "agent_id"}, {"scenarioId", f.ScenarioID, "scenario_id"}} {
		if c.val == "" {
			continue
		}
		if !uuidPattern.MatchString(c.val) {
			fields[c.name] = "Use an id."
			continue
		}
		args = append(args, c.val)
		where += fmt.Sprintf(" AND r.%s = $%d", c.col, len(args))
	}
	if err := apperr.Invalid(fields); err != nil {
		return Overview{}, err
	}
	o := Overview{Days: f.Days, Totals: map[string]int{}, FailureTags: map[string]int{},
		Trend: []TrendPoint{}, WeakestCriteria: []WeakCriterion{}, Scenarios: []ScenarioRow{}}
	from := ` FROM runs r LEFT JOIN judgements j ON j.run_id = r.id AND j.status = 'completed' WHERE ` + where

	rows, err := s.pool.Query(ctx, `SELECT r.status, count(*)`+from+` GROUP BY r.status`, args...)
	if err != nil {
		return Overview{}, fmt.Errorf("report totals: %w", err)
	}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return Overview{}, err
		}
		o.Totals[st] = n
	}
	rows.Close()

	if err := s.pool.QueryRow(ctx, `SELECT count(j.score), avg(j.score)::float8,
		  avg(CASE WHEN j.passed THEN 1.0 WHEN j.passed IS NOT NULL THEN 0.0 END)::float8,
		  avg((r.metrics->>'latencyP50Ms')::float8), avg((r.metrics->>'latencyP95Ms')::float8),
		  coalesce(sum((r.metrics->>'llmInputTokens')::bigint + (r.metrics->>'llmOutputTokens')::bigint), 0)`+from, args...,
	).Scan(&o.Judged, &o.AvgScore, &o.PassRate, &o.LatencyP50Ms, &o.LatencyP95Ms, &o.LLMTokens); err != nil {
		return Overview{}, fmt.Errorf("report kpis: %w", err)
	}

	trend, err := s.pool.Query(ctx, `SELECT to_char(d.day, 'YYYY-MM-DD'), count(r.id), avg(j.score)::float8,
		  avg(CASE WHEN j.passed THEN 1.0 WHEN j.passed IS NOT NULL THEN 0.0 END)::float8
		FROM generate_series(date_trunc('day', now()) - make_interval(days => $2 - 1), date_trunc('day', now()), interval '1 day') AS d(day)
		LEFT JOIN runs r ON date_trunc('day', r.created_at) = d.day AND `+where+`
		LEFT JOIN judgements j ON j.run_id = r.id AND j.status = 'completed'
		GROUP BY d.day ORDER BY d.day`, args...)
	if err != nil {
		return Overview{}, fmt.Errorf("report trend: %w", err)
	}
	o.Trend, err = pgx.CollectRows(trend, func(r pgx.CollectableRow) (TrendPoint, error) {
		var p TrendPoint
		return p, r.Scan(&p.Date, &p.Runs, &p.AvgScore, &p.PassRate)
	})
	if err != nil {
		return Overview{}, err
	}

	weak, err := s.pool.Query(ctx, `SELECT (r.snapshot->'scenario'->>'id')::uuid, max(r.snapshot->'scenario'->>'name'), c->>'criterionId', max(c->>'text'),
		  avg((c->>'score')::float8), avg(CASE WHEN c->>'verdict' = 'fail' THEN 1.0 ELSE 0.0 END)::float8, count(*)
		FROM runs r JOIN judgements j ON j.run_id = r.id AND j.status = 'completed' AND jsonb_typeof(j.result->'criteria') = 'array'
		CROSS JOIN LATERAL jsonb_array_elements(j.result->'criteria') AS c
		WHERE `+where+` AND c->>'score' IS NOT NULL
		GROUP BY 1, 3 ORDER BY 5 ASC, 6 DESC LIMIT 5`, args...)
	if err == nil {
		o.WeakestCriteria, err = pgx.CollectRows(weak, func(r pgx.CollectableRow) (WeakCriterion, error) {
			var w WeakCriterion
			return w, r.Scan(&w.ScenarioID, &w.ScenarioName, &w.CriterionID, &w.Text, &w.AvgScore, &w.FailRate, &w.Judged)
		})
	}
	if err != nil {
		return Overview{}, fmt.Errorf("report criteria: %w", err)
	}

	scen, err := s.pool.Query(ctx, `SELECT (r.snapshot->'scenario'->>'id')::uuid, max(r.snapshot->'scenario'->>'name'), max(r.snapshot->'agent'->>'name'),
		  count(*), avg(j.score)::float8, avg(CASE WHEN j.passed THEN 1.0 WHEN j.passed IS NOT NULL THEN 0.0 END)::float8, max(r.created_at)`+from+
		` GROUP BY 1 ORDER BY 7 DESC LIMIT 100`, args...)
	if err == nil {
		o.Scenarios, err = pgx.CollectRows(scen, func(r pgx.CollectableRow) (ScenarioRow, error) {
			var sr ScenarioRow
			return sr, r.Scan(&sr.ScenarioID, &sr.ScenarioName, &sr.AgentName, &sr.Runs, &sr.AvgScore, &sr.PassRate, &sr.LastRunAt)
		})
	}
	if err != nil {
		return Overview{}, fmt.Errorf("report scenarios: %w", err)
	}

	tags, err := s.pool.Query(ctx, `SELECT t.key, sum(t.value::int)
		FROM runs r JOIN judgements j ON j.run_id = r.id AND j.status = 'completed' AND jsonb_typeof(j.result->'failureTags') = 'object'
		CROSS JOIN LATERAL jsonb_each_text(j.result->'failureTags') AS t
		WHERE `+where+` GROUP BY t.key`, args...)
	if err != nil {
		return Overview{}, fmt.Errorf("report tags: %w", err)
	}
	for tags.Next() {
		var k string
		var n int
		if err := tags.Scan(&k, &n); err != nil {
			return Overview{}, err
		}
		o.FailureTags[k] = n
	}
	tags.Close()
	return o, nil
}
