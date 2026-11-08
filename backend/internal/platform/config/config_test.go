package config

import (
	"testing"
	"time"
)

func TestLoadDoesNotRequireSecretKey(t *testing.T) {
	t.Setenv("SECRET_KEY", "")
	if _, err := Load(); err != nil {
		t.Fatalf("Load without SECRET_KEY: %v (migrate must work without it)", err)
	}
}

func TestJudgeTraceSettings(t *testing.T) {
	t.Setenv("JUDGE_TRACE_WAIT", "0")
	t.Setenv("JUDGE_TRACE_POLL", "5s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JudgeTraceWait != 0 || cfg.JudgeTracePoll != 5*time.Second {
		t.Errorf("wait=%v poll=%v, want 0 and 5s", cfg.JudgeTraceWait, cfg.JudgeTracePoll)
	}
	t.Setenv("JUDGE_TRACE_WAIT", "")
	t.Setenv("JUDGE_TRACE_POLL", "")
	cfg, _ = Load()
	if cfg.JudgeTraceWait != 3*time.Minute || cfg.JudgeTracePoll != 20*time.Second {
		t.Errorf("defaults wait=%v poll=%v", cfg.JudgeTraceWait, cfg.JudgeTracePoll)
	}
	for _, bad := range []string{"0s", "500ms", "abc", "2h"} {
		t.Setenv("JUDGE_TRACE_POLL", bad)
		if _, err := Load(); err == nil {
			t.Errorf("JUDGE_TRACE_POLL=%q accepted", bad)
		}
	}
}

func TestPublicAPIURL(t *testing.T) {
	t.Setenv("PUBLIC_API_URL", "")
	if cfg, err := Load(); err != nil || cfg.PublicAPIURL != "http://localhost:8080" {
		t.Fatalf("default = %q, %v", cfg.PublicAPIURL, err)
	}
	t.Setenv("PUBLIC_API_URL", "https://testforge.example.com/")
	if cfg, err := Load(); err != nil || cfg.PublicAPIURL != "https://testforge.example.com" {
		t.Fatalf("trailing slash = %q, %v", cfg.PublicAPIURL, err)
	}
	for _, bad := range []string{"testforge.example.com", "ftp://x", "http://"} {
		t.Setenv("PUBLIC_API_URL", bad)
		if _, err := Load(); err == nil {
			t.Errorf("PUBLIC_API_URL=%q accepted", bad)
		}
	}
}
