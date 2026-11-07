package sealbox

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func testBox(t *testing.T) *Box {
	t.Helper()
	box, err := New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestSealOpenRoundTrip(t *testing.T) {
	box := testBox(t)
	sealed, err := box.Seal("sk-live-secret", "conn:1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealed, "v1.") || strings.Contains(sealed, "sk-live-secret") {
		t.Fatalf("sealed = %q", sealed)
	}
	again, _ := box.Seal("sk-live-secret", "conn:1")
	if again == sealed {
		t.Fatal("sealing must use a fresh nonce")
	}
	got, err := box.Open(sealed, "conn:1")
	if err != nil || got != "sk-live-secret" {
		t.Fatalf("Open = %q, %v", got, err)
	}
}

func TestOpenRejectsWrongContextAndTampering(t *testing.T) {
	box := testBox(t)
	sealed, _ := box.Seal("sk-live-secret", "conn:1")

	if _, err := box.Open(sealed, "conn:2"); err == nil {
		t.Fatal("ciphertext opened under another connection's context")
	}
	raw := []byte(sealed)
	raw[len(raw)-2] ^= 1
	if _, err := box.Open(string(raw), "conn:1"); err == nil {
		t.Fatal("tampered ciphertext opened")
	}
	for _, bad := range []string{"", "v1.", "v2.abc", "v1.!!!"} {
		if _, err := box.Open(bad, "conn:1"); err == nil {
			t.Fatalf("Open(%q) succeeded", bad)
		}
	}
}

func TestNewValidatesKey(t *testing.T) {
	for _, key := range []string{"", "not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := New(key); err == nil {
			t.Fatalf("New(%q) accepted an invalid key", key)
		}
	}
}
