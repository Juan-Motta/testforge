package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coder/websocket"

	"testforge/internal/agent/codec"
	"testforge/internal/agent/tmpl"
	"testforge/internal/platform/egress"
)

// VoiceConn is a live WebSocket connection to a voice agent.
type VoiceConn struct {
	conn   *websocket.Conn
	Codec  codec.Voice
	Frames <-chan codec.Frame
	// Closed receives the read error once the connection ends.
	Closed <-chan error
}

// DialVoice connects to a voice agent, sends its initial messages and starts
// reading frames until ctx is cancelled or the agent closes.
func (s *Session) DialVoice(ctx context.Context) (*VoiceConn, error) {
	cvals := tmpl.Values{}
	for k, v := range s.vals {
		if !strings.HasPrefix(k, "secret.") {
			cvals[k] = v
		}
	}
	vc, err := codec.NewVoice(s.agent.Codec.Type, s.agent.Codec.Options, cvals)
	if err != nil {
		return nil, fmt.Errorf("the codec configuration is invalid: %w", err)
	}
	target, err := buildURL(s.agent.Transport.URL, s.agent.Transport.Query, s.vals)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	if err := setHeaders(header, s.agent.Transport.Headers, s.vals); err != nil {
		return nil, err
	}
	client := egress.NewClient(s.allowPrivate, connectTimeout)
	client.Timeout = 0
	dialCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, target, &websocket.DialOptions{HTTPClient: client, HTTPHeader: header})
	if err != nil {
		return nil, fmt.Errorf("%s", s.Redactor.Redact(describe(err)))
	}
	conn.SetReadLimit(1 << 20)
	for _, m := range s.agent.Transport.InitMessages {
		out, err := render(m, s.vals, tmpl.JSON)
		if err == nil {
			err = conn.Write(ctx, websocket.MessageText, []byte(out))
		}
		if err != nil {
			conn.CloseNow()
			return nil, fmt.Errorf("sending an initial message: %w", err)
		}
	}
	frames := make(chan codec.Frame, 64)
	closed := make(chan error, 1)
	go func() {
		defer close(frames)
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				closed <- err
				return
			}
			select {
			case frames <- codec.Frame{Binary: typ == websocket.MessageBinary, Data: data}:
			case <-ctx.Done():
				closed <- ctx.Err()
				return
			}
		}
	}()
	return &VoiceConn{conn: conn, Codec: vc, Frames: frames, Closed: closed}, nil
}

// Send writes frames to the agent.
func (v *VoiceConn) Send(ctx context.Context, frames []codec.Frame) error {
	for _, f := range frames {
		typ := websocket.MessageText
		if f.Binary {
			typ = websocket.MessageBinary
		}
		if err := v.conn.Write(ctx, typ, f.Data); err != nil {
			return err
		}
	}
	return nil
}

// Close ends the connection normally.
func (v *VoiceConn) Close() { _ = v.conn.Close(websocket.StatusNormalClosure, "conversation finished") }

// Abort drops the connection.
func (v *VoiceConn) Abort() { v.conn.CloseNow() }
