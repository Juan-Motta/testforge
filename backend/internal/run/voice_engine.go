package run

import "context"

// VoiceEngine calls a voice agent on behalf of the simulated user.
type VoiceEngine struct {
	AllowPrivate bool
}

// Run is implemented in the voice pipeline slice.
func (e *VoiceEngine) Run(ctx context.Context, job Job, rec Recorder) Outcome {
	return Outcome{Status: StatusFailed, EndReason: EndError, Error: "Voice runs are not available yet."}
}
