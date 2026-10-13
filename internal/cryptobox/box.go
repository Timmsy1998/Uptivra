package cryptobox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

type Box struct{ aead cipher.AEAD }

func New(key string) (*Box, error) {
	b, e := base64.StdEncoding.DecodeString(key)
	if e != nil || len(b) != 32 {
		return nil, errors.New("ENCRYPTION_KEY must be base64 encoding of 32 random bytes")
	}
	c, e := aes.NewCipher(b)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(c)
	return &Box{a}, e
}
func (b *Box) Seal(s string) string {
	n := make([]byte, b.aead.NonceSize())
	if _, e := io.ReadFull(rand.Reader, n); e != nil {
		panic(e)
	}
	return base64.StdEncoding.EncodeToString(b.aead.Seal(n, n, []byte(s), nil))
}
func (b *Box) Open(s string) (string, error) {
	v, e := base64.StdEncoding.DecodeString(s)
	if e != nil || len(v) < b.aead.NonceSize() {
		return "", errors.New("invalid encrypted value")
	}
	n := b.aead.NonceSize()
	p, e := b.aead.Open(nil, v[:n], v[n:], nil)
	return string(p), e
}
func Token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func Hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
