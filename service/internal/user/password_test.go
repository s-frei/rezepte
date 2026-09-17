package user

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	encoded, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("unexpected format: %s", encoded)
	}
	ok, err := VerifyPassword(encoded, "correct horse")
	if err != nil || !ok {
		t.Fatalf("verify correct = %v, %v", ok, err)
	}
	ok, err = VerifyPassword(encoded, "wrong")
	if err != nil || ok {
		t.Fatalf("verify wrong = %v, %v", ok, err)
	}
}

func TestHashPasswordUsesFreshSalt(t *testing.T) {
	a, _ := HashPassword("x")
	b, _ := HashPassword("x")
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestVerifyPasswordRejectsGarbage(t *testing.T) {
	if _, err := VerifyPassword("not-a-hash", "x"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	encoded, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	parts := strings.Split(encoded, "$")
	salt, key := parts[4], parts[5]
	shortSalt := base64.RawStdEncoding.EncodeToString(make([]byte, 8))

	tests := []struct {
		name    string
		encoded string
	}{
		{
			name:    "empty key segment",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$", salt),
		},
		{
			name:    "version trailing garbage",
			encoded: fmt.Sprintf("$argon2id$v=19x$m=65536,t=1,p=4$%s$%s", salt, key),
		},
		{
			name:    "params trailing garbage",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4x$%s$%s", salt, key),
		},
		{
			name:    "extra param",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4,x=9$%s$%s", salt, key),
		},
		{
			name:    "wrong salt length",
			encoded: fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s", shortSalt, key),
		},
		{
			name:    "memory out of bounds",
			encoded: fmt.Sprintf("$argon2id$v=19$m=99999999,t=1,p=4$%s$%s", salt, key),
		},
		{
			name:    "wrong algorithm",
			encoded: fmt.Sprintf("$argon2i$v=19$m=65536,t=1,p=4$%s$%s", salt, key),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := VerifyPassword(tt.encoded, "correct horse")
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
			if ok {
				t.Fatalf("expected ok=false for %s", tt.name)
			}
		})
	}
}
