package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"testforge/internal/agent"
	"testforge/internal/agent/codec"
	"testforge/internal/audio"
	"testforge/internal/llm"
	"testforge/internal/tts"
)

// VoiceEngine calls a voice agent on behalf of the simulated user: the LLM
// decides what to say, TTS speaks it into a real-time microphone stream, and
// the agent's audio and transcript events drive turn-taking.
type VoiceEngine struct {
	AllowPrivate bool
	// Tunables; zero values use the defaults below.
	TurnGap         time.Duration // silence that ends an agent turn (900 ms)
	Settle          time.Duration // wait after an explicit turn-done event (1.2 s)
	PlaybackGrace   time.Duration // pause after the agent's audio finishes playing (400 ms)
	TrailingSilence time.Duration // silence appended to each user utterance (600 ms)
	ReadyWait       time.Duration // how long to wait for a ready event before speaking (2 s)
}

const frameDuration = 100 * time.Millisecond

type phase int

const (
	phaseStarting phase = iota
	phaseWaitingAgent
	phaseAgentSpeaking
	phaseAwaitPlayback
	phaseDeciding
	phaseUserSpeaking
)

type micEventKind int

const (
	micStart      micEventKind = iota // first frame of an utterance sent
	micSpeechDone                     // last speech frame sent (before trailing silence)
	micEnd                            // trailing silence sent too
)

type micEvent struct {
	kind       micEventKind
	at         time.Time
	speechEnds time.Time
}

// voiceCall is the state of one call; only the engine's main loop touches it.
type voiceCall struct {
	e       *VoiceEngine
	job     Job
	rec     Recorder
	conn    *agent.VoiceConn
	caller  *Caller
	speaker tts.Synthesizer
	caps    codec.Capabilities
	rate    int
	start   time.Time

	transcript []Turn
	phase      phase
	agentFirst bool
	readySeen  bool
	waitStart  time.Time

	// Current agent turn.
	agentTurn      int // transcript index, -1 when none
	agentTurns     int
	turnFirstPlay  time.Time
	lastActivity   time.Time
	turnDoneAt     time.Time
	playbackEnd    time.Time
	items          map[string]string
	itemOrder      []string
	currentItem    string
	currentResp    string
	itemPlayStart  time.Time
	itemAudioMs    int
	cutItems       map[string]bool
	interruptedNow bool

	// User turns.
	userTurn   int
	speechEnd  time.Time
	hangUpNext bool
	decisionID int

	redactor        interface{ Redact(string) string }
	agentAudioBytes int
	userTL, agentTL *audio.Timeline
	errors          int
}

func (e *VoiceEngine) defaults() {
	if e.TurnGap == 0 {
		e.TurnGap = 900 * time.Millisecond
	}
	if e.Settle == 0 {
		e.Settle = 1200 * time.Millisecond
	}
	if e.PlaybackGrace == 0 {
		e.PlaybackGrace = 400 * time.Millisecond
	}
	if e.TrailingSilence == 0 {
		e.TrailingSilence = 600 * time.Millisecond
	}
	if e.ReadyWait == 0 {
		e.ReadyWait = 2 * time.Second
	}
}

// Run plays the scenario on a voice agent.
func (e *VoiceEngine) Run(ctx context.Context, job Job, rec Recorder) Outcome {
	ve := *e
	ve.defaults()
	c := &voiceCall{e: &ve, job: job, rec: rec, start: time.Now(), agentTurn: -1, userTurn: -1,
		items: map[string]string{}, cutItems: map[string]bool{}}
	return c.run(ctx)
}

func (c *voiceCall) ms(t time.Time) int { return int(t.Sub(c.start).Milliseconds()) }

func (c *voiceCall) run(ctx context.Context) Outcome {
	sc := c.job.Snapshot.Scenario
	if c.job.Voice == nil {
		return c.fail("The persona has no voice.")
	}
	client, err := llm.New(c.job.Simulator, c.e.AllowPrivate)
	if err != nil {
		return c.fail(err.Error())
	}
	c.caller = NewCaller(client, c.job.SimulatorModel, c.job.Snapshot)
	c.speaker, err = tts.New(c.job.Voice.Credentials, c.job.Voice.Voice, c.job.Voice.Model, c.job.Snapshot.Persona.Locale, c.e.AllowPrivate)
	if err != nil {
		return c.fail(err.Error())
	}

	connCtx, cancelConn := context.WithCancel(ctx)
	defer cancelConn()
	session, err := agent.OpenSession(connCtx, c.job.Snapshot.Agent, c.job.AgentSecrets, c.job.ThreadID, c.e.AllowPrivate,
		func(_, detail string) { c.rec.Log("info", detail) })
	if err != nil {
		return c.fail("Could not start the agent session: " + err.Error())
	}
	c.conn, err = session.DialVoice(connCtx)
	if err != nil {
		return c.withTrace(c.fail("Could not connect to the agent: "+err.Error()), session)
	}
	defer c.conn.Abort()
	c.redactor = session.Redactor
	c.caps = c.conn.Codec.Capabilities()
	c.rate = c.conn.Codec.SampleRate()
	c.userTL, c.agentTL = audio.NewTimeline(c.rate), audio.NewTimeline(c.rate)
	c.agentFirst = c.caps.AgentSpeaksFirst
	c.rec.Log("info", fmt.Sprintf("Connected to the agent (%d Hz).", c.rate))

	// The microphone streams 100 ms frames: queued speech, or silence like an open line.
	utterances := make(chan []byte, 4)
	micEvents := make(chan micEvent, 8)
	micErr := make(chan error, 1)
	go c.microphone(connCtx, utterances, micEvents, micErr)

	type decided struct {
		id  int
		d   Decision
		err error
	}
	decisions := make(chan decided, 1)
	speech := make(chan []byte, 1)
	speechErr := make(chan error, 1)
	decide := func() {
		c.phase = phaseDeciding
		c.decisionID++
		id := c.decisionID
		transcript := append([]Turn(nil), c.transcript...)
		status := fmt.Sprintf("Elapsed: %d s of %d s.", int(time.Since(c.start).Seconds()), sc.EndConditions.MaxDurationSeconds)
		go func() {
			d, err := c.caller.Decide(connCtx, transcript, status)
			select {
			case decisions <- decided{id, d, err}:
			case <-connCtx.Done(): // the call ended; never block forever
			}
		}()
	}

	limit := time.NewTimer(time.Duration(sc.EndConditions.MaxDurationSeconds) * time.Second)
	defer limit.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	maxSilence := time.Duration(sc.EndConditions.MaxSilenceSeconds) * time.Second
	c.waitStart = time.Now()

	end := func(status, reason, msg string) Outcome {
		return c.withTrace(c.finish(connCtx, status, reason, msg), session)
	}

	for {
		select {
		case <-ctx.Done():
			return end(StatusFailed, EndError, "stopped")
		case <-limit.C:
			return end(StatusCompleted, EndMaxDuration, "")
		case err := <-micErr:
			return end(StatusFailed, EndError, "Sending audio to the agent failed: "+err.Error())
		case err := <-c.conn.Closed:
			if c.phase == phaseUserSpeaking && c.hangUpNext {
				return end(StatusCompleted, EndUserHungUp, "")
			}
			c.rec.Log("info", "The agent closed the connection: "+c.redact(err.Error()))
			return end(StatusCompleted, EndAgentClosed, "")
		case f, ok := <-c.conn.Frames:
			if !ok {
				continue
			}
			if closed := c.onFrame(connCtx, f); closed {
				return end(StatusCompleted, EndAgentClosed, "")
			}
		case r := <-decisions:
			if r.id != c.decisionID || c.phase != phaseDeciding {
				continue // the agent spoke while the user was thinking
			}
			if r.err != nil {
				if errors.Is(r.err, context.Canceled) {
					continue
				}
				return end(StatusFailed, EndError, "Simulator error: "+r.err.Error())
			}
			if r.d.Say == nil {
				if r.d.HangUp {
					return end(StatusCompleted, EndUserHungUp, "")
				}
				c.phase, c.waitStart = phaseWaitingAgent, time.Now()
				continue
			}
			c.hangUpNext = r.d.HangUp
			c.phase = phaseUserSpeaking
			c.transcript = append(c.transcript, Turn{Role: "user", Text: *r.d.Say})
			c.userTurn = len(c.transcript) - 1
			text := *r.d.Say
			go func() {
				pcm, err := c.speaker.Synthesize(connCtx, text)
				if err != nil {
					speechErr <- err
					return
				}
				speech <- pcm
			}()
		case err := <-speechErr:
			return end(StatusFailed, EndError, "Speech synthesis failed: "+err.Error())
		case pcm := <-speech:
			pcm = audio.Resample(pcm, tts.SampleRate, c.rate)
			pcm = append(pcm, make([]byte, c.bytesFor(c.e.TrailingSilence))...)
			utterances <- pcm
		case ev := <-micEvents:
			if c.userTurn < 0 {
				continue
			}
			turn := &c.transcript[c.userTurn]
			switch ev.kind {
			case micStart:
				turn.StartMs = c.ms(ev.at)
			case micSpeechDone:
				// Latency counts from the end of speech, not of the trailing silence.
				turn.EndMs = c.ms(ev.speechEnds)
				c.speechEnd = ev.speechEnds
				c.rec.Transcript(c.userTurn, *turn, true)
				if c.hangUpNext {
					return end(StatusCompleted, EndUserHungUp, "")
				}
			case micEnd:
				c.userTurn = -1
				switch {
				case c.agentTurn >= 0:
					c.phase = phaseAgentSpeaking // the agent started talking over the user
				case c.phase == phaseUserSpeaking:
					c.phase, c.waitStart = phaseWaitingAgent, time.Now()
				}
			}
		case <-tick.C:
			now := time.Now()
			switch c.phase {
			case phaseStarting:
				if !c.agentFirst && (c.readySeen || now.Sub(c.start) >= c.e.ReadyWait) {
					decide()
				} else if c.agentFirst && now.Sub(c.waitStart) >= maxSilence {
					return end(StatusCompleted, EndMaxSilence, "")
				}
			case phaseWaitingAgent:
				if now.Sub(c.waitStart) >= maxSilence {
					return end(StatusCompleted, EndMaxSilence, "")
				}
			case phaseAgentSpeaking:
				if c.checkInterruption(connCtx, now) || c.agentTurnOver(now) {
					c.closeAgentTurn()
				}
			case phaseAwaitPlayback:
				if c.interruptedNow || !now.Before(c.playbackEnd.Add(c.e.PlaybackGrace)) {
					c.interruptedNow = false
					decide()
				}
			}
		}
	}
}

func (c *voiceCall) bytesFor(d time.Duration) int {
	n := int(int64(c.rate) * int64(d) / int64(time.Second))
	return n * 2
}

// microphone sends one frame every 100 ms: queued speech, else silence.
func (c *voiceCall) microphone(ctx context.Context, utterances <-chan []byte, events chan<- micEvent, errs chan<- error) {
	frame := c.bytesFor(frameDuration)
	silence := make([]byte, frame)
	trailing := c.bytesFor(c.e.TrailingSilence)
	t := time.NewTicker(frameDuration)
	defer t.Stop()
	var current []byte
	var speechBytes, sent int
	var started time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if len(current) == 0 {
			select {
			case u := <-utterances:
				current, sent = u, 0
				speechBytes = max(0, len(u)-trailing)
				started = time.Now()
				c.userTL.Place(c.ms(started), u[:speechBytes])
				if !emit(ctx, events, micEvent{kind: micStart, at: started}) {
					return
				}
			default:
			}
		}
		out := silence
		speechDoneNow := false
		if len(current) > 0 {
			n := min(frame, len(current))
			out = current[:n]
			if n < frame {
				out = append(append([]byte(nil), out...), silence[:frame-n]...)
			}
			current = current[n:]
			before := sent
			sent += n
			speechDoneNow = before < speechBytes && sent >= speechBytes
		}
		frames, err := c.conn.Codec.EncodeAudio(out)
		if err == nil {
			err = c.conn.Send(ctx, frames)
		}
		if err != nil {
			if ctx.Err() == nil {
				errs <- err
			}
			return
		}
		if speechDoneNow {
			speechEnds := started.Add(time.Duration(speechBytes/2) * time.Second / time.Duration(c.rate))
			if !emit(ctx, events, micEvent{kind: micSpeechDone, at: time.Now(), speechEnds: speechEnds}) {
				return
			}
		}
		if sent > 0 && len(current) == 0 {
			if !emit(ctx, events, micEvent{kind: micEnd, at: time.Now()}) {
				return
			}
			sent = 0
		}
	}
}

func emit(ctx context.Context, events chan<- micEvent, ev micEvent) bool {
	select {
	case events <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

// onFrame processes one agent frame; it reports whether the agent ended the session.
func (c *voiceCall) onFrame(ctx context.Context, f codec.Frame) bool {
	now := time.Now()
	for _, e := range c.conn.Codec.Decode(f) {
		// Agent text may echo secrets: redact before it is stored, logged or streamed.
		e.Text = c.redact(e.Text)
		switch e.Kind {
		case codec.KindReady:
			if !c.readySeen {
				c.readySeen = true
				if frames := c.conn.Codec.OnReady(); len(frames) > 0 {
					_ = c.conn.Send(ctx, frames)
					c.agentFirst = true // the codec asked the agent to speak first
				}
			}
		case codec.KindAudio:
			if c.cutItems[e.ItemID] || (e.ResponseID != "" && c.cutItems["resp:"+e.ResponseID]) {
				continue
			}
			c.beginAgentTurn(now)
			play := now
			if c.playbackEnd.After(play) {
				play = c.playbackEnd
			}
			if e.ItemID != c.currentItem || c.itemPlayStart.IsZero() {
				c.currentItem, c.currentResp, c.itemPlayStart, c.itemAudioMs = e.ItemID, e.ResponseID, play, 0
			}
			dur := time.Duration(len(e.Audio)/2) * time.Second / time.Duration(c.rate)
			c.agentTL.Place(c.ms(play), e.Audio)
			c.playbackEnd = play.Add(dur)
			c.itemAudioMs += int(dur.Milliseconds())
			c.agentAudioBytes += len(e.Audio)
			if c.turnFirstPlay.IsZero() {
				c.turnFirstPlay = play
			}
			c.lastActivity, c.turnDoneAt = now, time.Time{}
		case codec.KindTranscript:
			if e.Role != codec.RoleAgent {
				continue
			}
			c.beginAgentTurn(now)
			id := e.ItemID
			if id == "" {
				id = "_"
			}
			if _, seen := c.items[id]; !seen {
				c.itemOrder = append(c.itemOrder, id)
			}
			if e.Cumulative {
				c.items[id] = e.Text
			} else {
				c.items[id] += e.Text
			}
			c.transcript[c.agentTurn].Text = c.agentText()
			c.rec.Transcript(c.agentTurn, c.transcript[c.agentTurn], false)
			c.lastActivity = now
		case codec.KindTurnDone:
			if c.phase == phaseAgentSpeaking {
				c.turnDoneAt = now
			}
		case codec.KindBusy:
			c.lastActivity = now
		case codec.KindError:
			c.errors++
			c.rec.Log("error", "Agent error: "+e.Text)
		case codec.KindClosed:
			return true
		}
	}
	return false
}

// beginAgentTurn opens an agent turn when the agent starts speaking.
func (c *voiceCall) beginAgentTurn(now time.Time) {
	if c.agentTurn >= 0 {
		return
	}
	if c.phase == phaseDeciding {
		c.decisionID++ // discard the pending decision: the agent spoke first
	}
	turn := Turn{Role: "agent", StartMs: c.ms(now)}
	if !c.speechEnd.IsZero() {
		latency := int(now.Sub(c.speechEnd).Milliseconds())
		turn.LatencyMs = &latency
		c.speechEnd = time.Time{} // only the first agent turn after the user answers them
	}
	c.transcript = append(c.transcript, turn)
	c.agentTurn = len(c.transcript) - 1
	c.agentTurns++
	c.turnFirstPlay, c.turnDoneAt = time.Time{}, time.Time{}
	c.items, c.itemOrder = map[string]string{}, nil
	c.lastActivity = now
	if c.phase != phaseUserSpeaking {
		c.phase = phaseAgentSpeaking
	}
}

func (c *voiceCall) agentText() string {
	text := ""
	for _, id := range c.itemOrder {
		if t := c.items[id]; t != "" {
			if text != "" {
				text += " "
			}
			text += t
		}
	}
	return text
}

// agentTurnOver applies the turn-end rule: explicit turn events plus settle
// time, else a silence gap without audio or transcript.
func (c *voiceCall) agentTurnOver(now time.Time) bool {
	if c.caps.ExplicitTurnEnd {
		if !c.turnDoneAt.IsZero() && now.Sub(c.turnDoneAt) >= c.e.Settle {
			return true
		}
		return now.Sub(c.lastActivity) >= max(c.e.TurnGap, 5*time.Second)
	}
	return now.Sub(c.lastActivity) >= c.e.TurnGap
}

// checkInterruption cuts the agent off when the scenario asks for it.
func (c *voiceCall) checkInterruption(ctx context.Context, now time.Time) bool {
	if c.turnFirstPlay.IsZero() || c.interruptedNow {
		return false
	}
	for _, it := range c.job.Snapshot.Scenario.Interruptions {
		if it.AgentTurn != c.agentTurns {
			continue
		}
		played := now.Sub(c.turnFirstPlay)
		if played < time.Duration(it.AfterMs)*time.Millisecond || now.After(c.playbackEnd) {
			return false
		}
		var endMs *int
		if !c.itemPlayStart.IsZero() && !now.Before(c.itemPlayStart) {
			v := min(int(now.Sub(c.itemPlayStart).Milliseconds()), c.itemAudioMs)
			endMs = &v
		}
		frames, err := c.conn.Codec.Interrupt(c.currentItem, c.currentResp, endMs)
		if err != nil {
			c.rec.Log("warn", "This codec cannot interrupt the agent: "+err.Error())
			return false
		}
		_ = c.conn.Send(ctx, frames)
		c.cutItems[c.currentItem] = true
		if c.currentResp != "" {
			c.cutItems["resp:"+c.currentResp] = true
		}
		c.transcript[c.agentTurn].Interrupted = true
		c.playbackEnd = now
		c.interruptedNow = true
		c.rec.Log("info", fmt.Sprintf("Interrupted agent turn %d after %d ms.", c.agentTurns, played.Milliseconds()))
		return true
	}
	return false
}

func (c *voiceCall) closeAgentTurn() {
	if c.agentTurn < 0 {
		return
	}
	turn := &c.transcript[c.agentTurn]
	turn.Text = c.agentText()
	end := c.playbackEnd
	if end.IsZero() || end.Before(c.lastActivity) {
		end = c.lastActivity
	}
	turn.EndMs = c.ms(end)
	c.rec.Transcript(c.agentTurn, *turn, true)
	c.agentTurn = -1
	c.phase = phaseAwaitPlayback
	c.rec.Metrics(c.metrics())
}

func (c *voiceCall) metrics() Metrics {
	in, out := 0, 0
	if c.caller != nil {
		in, out = c.caller.Usage()
	}
	audioMs := 0
	if c.rate > 0 {
		audioMs = c.agentAudioBytes * 1000 / (2 * c.rate)
	}
	return summarize(c.transcript, int(time.Since(c.start).Milliseconds()), audioMs, in, out)
}

// finish closes the call per codec and builds the outcome with audio tracks.
func (c *voiceCall) finish(ctx context.Context, status, reason, msg string) Outcome {
	if c.agentTurn >= 0 {
		c.closeAgentTurn()
	}
	if c.userTurn >= 0 && c.transcript[c.userTurn].StartMs == 0 && c.transcript[c.userTurn].EndMs == 0 {
		c.transcript = append(c.transcript[:c.userTurn], c.transcript[c.userTurn+1:]...) // never spoken
	}
	if status != StatusFailed || msg != "stopped" {
		frames, awaitAck := c.conn.Codec.Close()
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		_ = c.conn.Send(closeCtx, frames)
		if awaitAck {
		wait:
			for {
				select {
				case f, ok := <-c.conn.Frames:
					if !ok {
						break wait
					}
					if c.onFrame(closeCtx, f) {
						break wait
					}
				case <-closeCtx.Done():
					break wait
				}
			}
		}
		cancel()
		c.conn.Close()
	}
	o := Outcome{Status: status, EndReason: reason, Error: msg, Transcript: c.transcript, Metrics: c.metrics(), Audio: map[string][]byte{}}
	if !c.userTL.Empty() {
		o.Audio["user"] = audio.WAV(c.userTL.Bytes(), c.rate)
	}
	if !c.agentTL.Empty() {
		o.Audio["agent"] = audio.WAV(c.agentTL.Bytes(), c.rate)
	}
	if len(o.Audio) > 0 {
		o.Audio["conversation"] = audio.WAV(audio.Mix(c.userTL, c.agentTL), c.rate)
	}
	return o
}

func (c *voiceCall) redact(s string) string {
	if c.redactor == nil || s == "" {
		return s
	}
	return c.redactor.Redact(s)
}

func (c *voiceCall) fail(msg string) Outcome {
	return Outcome{Status: StatusFailed, EndReason: EndError, Error: msg, Transcript: c.transcript, Metrics: c.metrics()}
}

func (c *voiceCall) withTrace(o Outcome, s *agent.Session) Outcome {
	if s != nil {
		o.TraceSessionID = s.TraceSessionID
	}
	return o
}
