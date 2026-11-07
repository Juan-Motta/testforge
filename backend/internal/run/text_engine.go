package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"testforge/internal/agent"
	"testforge/internal/llm"
)

// TextEngine chats with a text agent on behalf of the simulated user.
type TextEngine struct {
	AllowPrivate bool
	// ReplyTimeout bounds each agent reply (default 120 s).
	ReplyTimeout time.Duration
}

// Run plays the scenario until the user hangs up or a limit is reached.
func (e *TextEngine) Run(ctx context.Context, job Job, rec Recorder) Outcome {
	start := time.Now()
	elapsed := func() int { return int(time.Since(start).Milliseconds()) }
	sc := job.Snapshot.Scenario
	limit := time.Duration(sc.EndConditions.MaxDurationSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	var transcript []Turn
	out := func(status, reason, msg string, caller *Caller) Outcome {
		in, outTok := 0, 0
		if caller != nil {
			in, outTok = caller.Usage()
		}
		return Outcome{Status: status, EndReason: reason, Error: msg, Transcript: transcript,
			Metrics: summarize(transcript, elapsed(), 0, in, outTok)}
	}

	client, err := llm.New(job.Simulator, e.AllowPrivate)
	if err != nil {
		return out(StatusFailed, EndError, err.Error(), nil)
	}
	caller := NewCaller(client, job.SimulatorModel, job.Snapshot)
	session, err := agent.OpenSession(ctx, job.Snapshot.Agent, job.AgentSecrets, job.ThreadID, e.AllowPrivate,
		func(_, detail string) { rec.Log("info", detail) })
	if err != nil {
		return out(StatusFailed, EndError, "Could not start the agent session: "+err.Error(), caller)
	}
	trace := session.TraceSessionID
	finish := func(status, reason, msg string) Outcome {
		o := out(status, reason, msg, caller)
		o.TraceSessionID = trace
		return o
	}
	timeout := e.ReplyTimeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	userTurns, empty := 0, 0
	for {
		if userTurns >= sc.EndConditions.MaxTurns {
			return finish(StatusCompleted, EndMaxTurns, "")
		}
		status := fmt.Sprintf("Elapsed: %d s of %d s. Messages you sent: %d of %d.",
			elapsed()/1000, sc.EndConditions.MaxDurationSeconds, userTurns, sc.EndConditions.MaxTurns)
		d, err := caller.Decide(ctx, transcript, status)
		if err != nil {
			if reason, ok := limitReason(ctx); ok {
				return finish(StatusCompleted, reason, "")
			}
			return finish(StatusFailed, EndError, "Simulator error: "+err.Error())
		}
		if d.Say == nil {
			if d.HangUp {
				return finish(StatusCompleted, EndUserHungUp, "")
			}
			empty++
			if empty >= 3 {
				return finish(StatusFailed, EndError, "Simulator error: it returned nothing to say three times in a row.")
			}
			rec.Log("warn", "The simulator returned nothing to say; asking again.")
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
			}
			continue
		}
		empty = 0
		at := elapsed()
		transcript = append(transcript, Turn{Role: "user", Text: *d.Say, StartMs: at, EndMs: at})
		rec.Transcript(len(transcript)-1, transcript[len(transcript)-1], true)
		userTurns++
		if d.HangUp {
			return finish(StatusCompleted, EndUserHungUp, "")
		}

		sent := time.Now()
		reply, err := session.SendText(ctx, *d.Say, timeout, func(detail string) { rec.Log("info", "Agent event: "+detail) })
		if err != nil {
			if reason, ok := limitReason(ctx); ok {
				return finish(StatusCompleted, reason, "")
			}
			return finish(StatusFailed, EndError, "Agent error: "+err.Error())
		}
		latency := int(time.Since(sent).Milliseconds())
		end := elapsed()
		transcript = append(transcript, Turn{Role: "agent", Text: reply, StartMs: end, EndMs: end, LatencyMs: &latency})
		rec.Transcript(len(transcript)-1, transcript[len(transcript)-1], true)
		in, outTok := caller.Usage()
		rec.Metrics(summarize(transcript, end, 0, in, outTok))
	}
}

// limitReason explains a context error caused by the run's own limits.
func limitReason(ctx context.Context) (string, bool) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return EndMaxDuration, true
	}
	return "", false
}
