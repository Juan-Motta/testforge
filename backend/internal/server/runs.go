package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"testforge/internal/authn"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/httpx"
	"testforge/internal/run"
	"testforge/internal/workspace"
)

type runPage struct {
	Items      []run.Summary `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}

func intParam(r *http.Request, name string) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, apperr.Invalid(map[string]string{name: "Use a whole number."})
	}
	return n, nil
}

func (s *server) launchRuns(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	if in, ok := decodeInto[run.LaunchInput](w, r); ok {
		b, err := s.runs.Launch(r.Context(), p, ws, in)
		respond(w, r, http.StatusCreated, b, err)
	}
}

func (s *server) listRuns(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	limit, err := intParam(r, "limit")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	q := r.URL.Query()
	items, next, err := s.runs.ListRuns(r.Context(), ws, run.ListFilter{
		BatchID: q.Get("batchId"), ScenarioID: q.Get("scenarioId"), AgentID: q.Get("agentId"),
		Status: q.Get("status"), Limit: limit, Before: q.Get("before"),
	})
	respond(w, r, http.StatusOK, runPage{Items: items, NextCursor: next}, err)
}

func (s *server) getRun(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	rn, err := s.runs.GetRun(r.Context(), ws, r.PathValue("id"))
	respond(w, r, http.StatusOK, rn, err)
}

func (s *server) cancelRun(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	sm, err := s.runs.Cancel(r.Context(), ws, r.PathValue("id"))
	respond(w, r, http.StatusOK, sm, err)
}

func (s *server) rerunRun(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	sm, err := s.runs.Rerun(r.Context(), ws, r.PathValue("id"))
	respond(w, r, http.StatusCreated, sm, err)
}

func (s *server) listRunBatches(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	limit, err := intParam(r, "limit")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items, err := s.runs.ListBatches(r.Context(), ws, limit)
	respond(w, r, http.StatusOK, listResponse[run.Batch]{Items: items}, err)
}

func (s *server) cancelRunBatch(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	b, err := s.runs.CancelBatch(r.Context(), ws, r.PathValue("id"))
	respond(w, r, http.StatusOK, b, err)
}

func (s *server) runAudio(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	track := r.PathValue("track")
	if track != "user" && track != "agent" && track != "conversation" {
		httpx.Error(w, r, apperr.NotFound("Unknown audio track."))
		return
	}
	data, err := s.runs.Audio(r.Context(), ws, r.PathValue("id"), track)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	// ServeContent answers Range requests, so the audio element can seek.
	http.ServeContent(w, r, track+".wav", time.Time{}, bytes.NewReader(data))
}

// runEvents streams a run's events as Server-Sent Events until the run ends.
func (s *server) runEvents(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	id := r.PathValue("id")
	var after int64
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		after, _ = strconv.ParseInt(v, 10, 64)
	}
	// Validate before switching to the event stream so errors keep the JSON shape.
	events, finished, err := s.runs.EventsAfter(r.Context(), ws, id, after)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the stream outlives the server's write timeout
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	// no-transform stops proxies (including the Next.js rewrite) from gzipping and buffering the stream.
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastWrite := time.Now()
	for {
		for _, e := range events {
			// jsonb comes back with spaces; one compact line per data field.
			var data bytes.Buffer
			if json.Compact(&data, e.Data) != nil {
				data.Reset()
				data.WriteString("{}")
			}
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Kind, data.Bytes())
			after = e.Seq
			lastWrite = time.Now()
		}
		if len(events) > 0 {
			_ = rc.Flush()
		}
		if finished && len(events) == 0 {
			return
		}
		if time.Since(lastWrite) > 15*time.Second {
			fmt.Fprint(w, ": keep-alive\n\n")
			_ = rc.Flush()
			lastWrite = time.Now()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
		events, finished, err = s.runs.EventsAfter(r.Context(), ws, id, after)
		if err != nil {
			return
		}
	}
}
