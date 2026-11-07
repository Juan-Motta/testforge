package config

import "testing"

func TestLoadDoesNotRequireSecretKey(t *testing.T) {
	t.Setenv("SECRET_KEY", "")
	if _, err := Load(); err != nil {
		t.Fatalf("Load without SECRET_KEY: %v (migrate must work without it)", err)
	}
}
