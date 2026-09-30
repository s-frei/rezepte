package demo

import "testing"

// dexSubject must reproduce the exact "sub" Dex issues, or a sign-in through
// the running test Dex ends "not connected" instead of landing on the
// pre-linked account. This value was checked against a live Dex: a
// freshly built instance, configured against it, seeded jonas's identity
// with it, and signing in as jonas@example.com through Dex landed on the
// jonas account.
func TestDexSubjectMatchesLiveDex(t *testing.T) {
	if got := dexSubject("jonas", dexConnector); got != "CgVqb25hcxIFbG9jYWw" {
		t.Fatalf("dexSubject(jonas, %s) = %q, want CgVqb25hcxIFbG9jYWw", dexConnector, got)
	}
}
