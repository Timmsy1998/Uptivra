package cryptobox

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAuthenticatedEncryption(t *testing.T) {
	b, err := New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	sealed := b.Seal("authorization-secret")
	if sealed == b.Seal("authorization-secret") || strings.Contains(sealed, "authorization-secret") {
		t.Fatal("nonrandom or plaintext ciphertext")
	}
	if plain, err := b.Open(sealed); err != nil || plain != "authorization-secret" {
		t.Fatal("round trip", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(sealed)
	raw[len(raw)-1] ^= 1
	if _, err := b.Open(base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	for _, bad := range []string{"", "not-base64", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := b.Open(bad); err == nil {
			t.Fatal("invalid ciphertext accepted")
		}
	}
	if _, err := New("bad-key"); err == nil {
		t.Fatal("invalid key accepted")
	}
}
