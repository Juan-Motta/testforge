package identity

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash = %q", hash)
	}
	if !VerifyPassword(hash, "correct horse") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "wrong horse") {
		t.Fatal("wrong password accepted")
	}

	other, _ := HashPassword("correct horse")
	if other == hash {
		t.Fatal("hashes must be salted")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=19$m=x$c2FsdA$aGFzaA"} {
		if VerifyPassword(h, "anything") {
			t.Fatalf("malformed hash %q accepted", h)
		}
	}
}
