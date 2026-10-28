package push

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func unb64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return b
}

// TestEncryptRFC8291Example encrypts the example of RFC 8291 Section 5,
// with the inputs listed in its Appendix A, and expects the RFC's body.
func TestEncryptRFC8291Example(t *testing.T) {
	sender, err := ecdh.P256().NewPrivateKey(unb64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	sub := Subscription{
		Endpoint: "https://push.example.net/push/JzLQ3raZJfFBR0aqvOMsLrt54w4rJUsV",
		P256dh:   unb64(t, "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"),
		Auth:     unb64(t, "BTBZMqHH6r4Tts7J_aSIgg"),
	}
	want := unb64(t, "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml"+
		"mlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPT"+
		"pK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN")
	got, err := Encrypt(sub, []byte("When I grow up, I want to be a watermelon"), unb64(t, "DGv6ra1nlYgDCS1FRnbzlw"), sender)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("body\n got %s\nwant %s", base64.RawURLEncoding.EncodeToString(got), base64.RawURLEncoding.EncodeToString(want))
	}
}

func TestEncryptRejectsOversizePlaintext(t *testing.T) {
	sender, _ := ecdh.P256().GenerateKey(rand.Reader)
	recv, _ := ecdh.P256().GenerateKey(rand.Reader)
	sub := Subscription{P256dh: recv.PublicKey().Bytes(), Auth: make([]byte, 16)}
	if _, err := Encrypt(sub, make([]byte, 4096), make([]byte, 16), sender); err == nil {
		t.Fatal("want an error for a plaintext beyond one record")
	}
}

func TestVAPIDVerifies(t *testing.T) {
	key, err := NewVAPIDKey()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	h, err := VAPID(key, "https://fcm.googleapis.com/fcm/send/abc:def", "https://rezepte.example", now)
	if err != nil {
		t.Fatal(err)
	}
	rest, ok := strings.CutPrefix(h, "vapid t=")
	if !ok {
		t.Fatalf("header %q", h)
	}
	jwt, k, ok := strings.Cut(rest, ", k=")
	if !ok || k != PublicKey(key) {
		t.Fatalf("header %q, want k=%s", h, PublicKey(key))
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt %q", jwt)
	}
	if got := string(unb64(t, parts[0])); got != `{"typ":"JWT","alg":"ES256"}` {
		t.Fatalf("header %s", got)
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(unb64(t, parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != "https://fcm.googleapis.com" || claims.Sub != "https://rezepte.example" {
		t.Fatalf("claims %+v", claims)
	}
	if d := time.Unix(claims.Exp, 0).Sub(now); d <= 0 || d > 24*time.Hour {
		t.Fatalf("exp %v after now", d)
	}
	sig := unb64(t, parts[2])
	if len(sig) != 64 {
		t.Fatalf("signature is %d bytes, want 64", len(sig))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&key.PublicKey, digest[:], r, s) {
		t.Fatal("signature does not verify")
	}
}

func TestVAPIDKeyRoundTrip(t *testing.T) {
	key, err := NewVAPIDKey()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalVAPIDKey(key)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseVAPIDKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Equal(key) {
		t.Fatal("parsed key differs")
	}
	if pub := unb64(t, PublicKey(key)); len(pub) != 65 || pub[0] != 4 {
		t.Fatalf("public key is not an uncompressed P-256 point: %x", pub)
	}
}
