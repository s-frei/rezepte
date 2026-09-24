package user

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (OWASP recommendation: 64 MiB, 1 iteration, 4 lanes).
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16

	// Upper bounds on parameters accepted from an encoded hash: defense
	// against a tampered database forcing huge allocations via
	// VerifyPassword. This package never produces values above these.
	maxArgonMemory  = 1 << 20 // 1 GiB
	maxArgonTime    = 16
	maxArgonThreads = 32
)

var errMalformedHash = errors.New("malformed password hash")

// HashPassword returns an argon2id hash in PHC string format.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the PHC-encoded hash.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errMalformedHash
	}
	versionStr, ok := strings.CutPrefix(parts[2], "v=")
	if !ok {
		return false, errMalformedHash
	}
	version, err := strconv.Atoi(versionStr)
	if err != nil || version != argon2.Version {
		return false, errMalformedHash
	}
	memory, iterations, threads, err := parseArgonParams(parts[3])
	if err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != saltLen {
		return false, errMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != argonKeyLen {
		return false, errMalformedHash
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, argonKeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// parseArgonParams strictly parses the "m=...,t=...,p=..." PHC segment. It
// rejects anything but exactly three key=value items in that order (no
// trailing garbage, no extra fields) and bounds each value to a range this
// package could plausibly have produced.
func parseArgonParams(s string) (uint32, uint32, uint8, error) {
	items := strings.Split(s, ",")
	if len(items) != 3 {
		return 0, 0, 0, errMalformedHash
	}
	m, err := parseParam(items[0], "m=", 32)
	if err != nil {
		return 0, 0, 0, err
	}
	t, err := parseParam(items[1], "t=", 32)
	if err != nil {
		return 0, 0, 0, err
	}
	p, err := parseParam(items[2], "p=", 8)
	if err != nil {
		return 0, 0, 0, err
	}
	if m > maxArgonMemory || t > maxArgonTime || p > maxArgonThreads {
		return 0, 0, 0, errMalformedHash
	}
	return uint32(m), uint32(t), uint8(p), nil
}

// parseParam parses a single "<prefix><digits>" item, requiring an exact
// prefix match and a value that fits in bitSize bits with no trailing
// characters.
func parseParam(item, prefix string, bitSize int) (uint64, error) {
	digits, ok := strings.CutPrefix(item, prefix)
	if !ok {
		return 0, errMalformedHash
	}
	v, err := strconv.ParseUint(digits, 10, bitSize)
	if err != nil {
		return 0, errMalformedHash
	}
	return v, nil
}
