// Package run executes scenarios against agents: batches of queued runs, a
// worker that claims them, conversation engines, live events and results.
package run

import (
	"time"

	"testforge/internal/agent"
	"testforge/internal/scenario"
)

// Statuses.
const (
	StatusQueued      = "queued"
	StatusRunning     = "running"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
	StatusInterrupted = "interrupted"
)

// End reasons.
const (
	EndUserHungUp  = "user_hung_up"
	EndAgentClosed = "agent_closed"
	EndMaxTurns    = "max_turns"
	EndMaxDuration = "max_duration"
	EndMaxSilence  = "max_silence"
	EndCancelled   = "cancelled"
	EndWorkerLost  = "worker_lost"
	EndError       = "error"
)

// Snapshot is what a run executes: copies taken at launch, without secret values.
type Snapshot struct {
	Agent    agent.Agent       `json:"agent"`
	Scenario scenario.Scenario `json:"scenario"`
	Persona  scenario.Persona  `json:"persona"`
}

// Turn is one transcript entry.
type Turn struct {
	Role        string `json:"role"`
	Text        string `json:"text"`
	StartMs     int    `json:"startMs"`
	EndMs       int    `json:"endMs"`
	LatencyMs   *int   `json:"latencyMs"`
	Interrupted bool   `json:"interrupted"`
}

// Metrics summarize a run.
type Metrics struct {
	DurationMs      int  `json:"durationMs"`
	UserTurns       int  `json:"userTurns"`
	AgentTurns      int  `json:"agentTurns"`
	LatencyP50Ms    *int `json:"latencyP50Ms"`
	LatencyP95Ms    *int `json:"latencyP95Ms"`
	AgentAudioMs    int  `json:"agentAudioMs"`
	LLMInputTokens  int  `json:"llmInputTokens"`
	LLMOutputTokens int  `json:"llmOutputTokens"`
}

// Creator identifies who launched a batch.
type Creator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Batch groups runs launched together.
type Batch struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Concurrency int            `json:"concurrency"`
	Total       int            `json:"total"`
	Counts      map[string]int `json:"counts"`
	CreatedAt   time.Time      `json:"createdAt"`
	CreatedBy   *Creator       `json:"createdBy"`
}

// Summary is a run in lists.
type Summary struct {
	ID           string     `json:"id"`
	BatchID      string     `json:"batchId"`
	ScenarioID   *string    `json:"scenarioId"`
	ScenarioName string     `json:"scenarioName"`
	AgentID      *string    `json:"agentId"`
	AgentName    string     `json:"agentName"`
	PersonaName  string     `json:"personaName"`
	Modality     string     `json:"modality"`
	Status       string     `json:"status"`
	EndReason    *string    `json:"endReason"`
	Error        *string    `json:"error"`
	Attempt      int        `json:"attempt"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	Metrics      *Metrics   `json:"metrics"`
}

// Run is a run with its results.
type Run struct {
	Summary
	ThreadID       *string  `json:"threadId"`
	TraceSessionID *string  `json:"traceSessionId"`
	Transcript     []Turn   `json:"transcript"`
	AudioTracks    []string `json:"audioTracks"`
}

// LaunchInput is the launch request.
type LaunchInput struct {
	ScenarioIDs []string `json:"scenarioIds"`
	Repetitions *int     `json:"repetitions"`
	Concurrency *int     `json:"concurrency"`
	Label       string   `json:"label"`
}

// Event is one live event.
type Event struct {
	Seq  int64  `json:"seq"`
	Kind string `json:"kind"`
	Data []byte `json:"data"`
}
