// Package push sends Web Push messages: it keeps the devices that asked for
// them and the alarms they scheduled, encrypts each message for its device
// (RFC 8291) and signs the request with the instance's VAPID key (RFC 8292).
// It holds no timer state; an alarm is a message to send at a time.
package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// recordSize is the single aes128gcm record every message fits in; 4096 is
// what RFC 8291 has push services accept at least.
const recordSize = 4096

// Subscription is a device's push endpoint and the keys its messages are
// encrypted for, as PushManager.subscribe hands them out.
type Subscription struct {
	Endpoint string
	P256dh   []byte
	Auth     []byte
}

var b64 = base64.RawURLEncoding

// Encrypt builds an RFC 8291 aes128gcm body (record size 4096, one record)
// of plaintext for sub, with the given 16-byte salt and the sender's
// one-off key pair.
func Encrypt(sub Subscription, plaintext []byte, salt []byte, server *ecdh.PrivateKey) ([]byte, error) {
	if len(salt) != 16 || len(sub.Auth) != 16 {
		return nil, errors.New("push: salt and auth secret must be 16 bytes")
	}
	// One record holds the plaintext, the padding delimiter and the tag.
	if len(plaintext)+1+16 > recordSize {
		return nil, errors.New("push: message does not fit one record")
	}
	ua, err := ecdh.P256().NewPublicKey(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("push: receiver key: %w", err)
	}
	secret, err := server.ECDH(ua)
	if err != nil {
		return nil, fmt.Errorf("push: ecdh: %w", err)
	}
	asPublic := server.PublicKey().Bytes()
	keyInfo := "WebPush: info\x00" + string(sub.P256dh) + string(asPublic)
	ikm, err := hkdf.Key(sha256.New, secret, sub.Auth, keyInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("push: ikm: %w", err)
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, fmt.Errorf("push: prk: %w", err)
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, fmt.Errorf("push: cek: %w", err)
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, fmt.Errorf("push: nonce: %w", err)
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, fmt.Errorf("push: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("push: gcm: %w", err)
	}
	// Header: salt, record size, key id length, key id (the sender's key).
	out := make([]byte, 0, 16+4+1+len(asPublic)+len(plaintext)+1+16)
	out = append(out, salt...)
	out = binary.BigEndian.AppendUint32(out, recordSize)
	out = append(out, 65) // an uncompressed P-256 point
	out = append(out, asPublic...)
	// 0x02 marks the last (and only) record.
	return gcm.Seal(out, nonce, append(append([]byte{}, plaintext...), 2), nil), nil
}

// VAPID returns the Authorization header value "vapid t=<jwt>, k=<base64url
// public key>" for a request to endpoint: aud is the endpoint's
// scheme://host, exp now+12h, sub the given subject.
func VAPID(key *ecdsa.PrivateKey, endpoint, subject string, now time.Time) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("push: endpoint %q is not an absolute URL", endpoint)
	}
	claims, err := json.Marshal(struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}{u.Scheme + "://" + u.Host, now.Add(12 * time.Hour).Unix(), subject})
	if err != nil {
		return "", fmt.Errorf("push: claims: %w", err)
	}
	signed := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signed))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("push: sign: %w", err)
	}
	// JWS wants the raw r||s, each padded to 32 bytes, not ASN.1.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return "vapid t=" + signed + "." + b64.EncodeToString(sig) + ", k=" + PublicKey(key), nil
}

// NewVAPIDKey generates an instance's P-256 signing key.
func NewVAPIDKey() (*ecdsa.PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("push: generate vapid key: %w", err)
	}
	return k, nil
}

// MarshalVAPIDKey encodes key as its raw 32-byte scalar, the form stored in
// instance_settings.vapid_key.
func MarshalVAPIDKey(key *ecdsa.PrivateKey) ([]byte, error) {
	b, err := key.Bytes()
	if err != nil {
		return nil, fmt.Errorf("push: marshal vapid key: %w", err)
	}
	return b, nil
}

// ParseVAPIDKey decodes a key written by MarshalVAPIDKey.
func ParseVAPIDKey(b []byte) (*ecdsa.PrivateKey, error) {
	k, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), b)
	if err != nil {
		return nil, fmt.Errorf("push: parse vapid key: %w", err)
	}
	return k, nil
}

// PublicKey returns key's public half as a base64url uncompressed point, the
// applicationServerKey a browser subscribes with.
func PublicKey(key *ecdsa.PrivateKey) string {
	b, err := key.PublicKey.Bytes()
	if err != nil {
		// Only an invalid or non-NIST key fails, which NewVAPIDKey and
		// ParseVAPIDKey never return.
		panic(fmt.Sprintf("push: public key: %v", err))
	}
	return b64.EncodeToString(b)
}
