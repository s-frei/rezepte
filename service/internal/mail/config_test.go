package mail

import (
	"errors"
	"testing"
)

func TestValidateFrom(t *testing.T) {
	for from, ok := range map[string]bool{" ": false, " a@b.c ": false, "a@b.c": true} {
		c := Config{Host: "h", Port: 25, Security: SecurityNone, From: from}
		err := c.Validate()
		if ok && err != nil {
			t.Errorf("From %q: %v", from, err)
		}
		if !ok && !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("From %q: err = %v, want ErrInvalidConfig", from, err)
		}
	}
}

func TestValidateCredentialsNeedTransportSecurity(t *testing.T) {
	for _, tc := range []struct {
		host string
		sec  Security
		user string
		ok   bool
	}{
		{"smtp.example.org", SecurityNone, "u", false},
		{"smtp.example.org", SecurityNone, "", true},
		{"smtp.example.org", SecuritySTARTTLS, "u", true},
		{"127.0.0.1", SecurityNone, "u", true},
		{"localhost", SecurityNone, "u", true},
		{"::1", SecurityNone, "u", true},
	} {
		c := Config{Host: tc.host, Port: 25, Security: tc.sec, Username: tc.user, From: "a@b.c"}
		if err := c.Validate(); (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrInvalidConfig)) {
			t.Errorf("%+v: err = %v, ok want %v", tc, err, tc.ok)
		}
	}
}
