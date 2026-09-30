package image

import "testing"

// TestIsID pins the id check that keeps path segments out of the image
// directory: the canonical lower-case 36-character spelling only.
func TestIsID(t *testing.T) {
	for s, want := range map[string]bool{
		"0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9f00":          true,
		"00000000-0000-0000-0000-000000000000":          true,
		"ffffffff-ffff-ffff-ffff-ffffffffffff":          true,
		"01234567-89ab-cdef-0123-456789abcdef":          true, // any version and variant
		"0192F0A4-7B1C-7D3E-8F4A-5B6C7D8E9F00":          false,
		"0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9F00":          false,
		"0192f0a47b1c7d3e8f4a5b6c7d8e9f00":              false,
		"{0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9f00}":        false,
		"urn:uuid:0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9f00": false,
		"0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9f0":           false,
		"0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9f000":         false,
		"0192f0a4x7b1c-7d3e-8f4a-5b6c7d8e9f00":          false,
		"0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9g00":          false,
		"0192f0a4-7b1c-7d3e-8f4a-5b6c7d8e9f/.":          false,
		"../../../../../../../../../../etc/pas":         false,
		"0192f0a47-b1c-7d3e-8f4a-5b6c7d8e9f00":          false,
		"":                                              false,
	} {
		if got := IsID(s); got != want {
			t.Errorf("IsID(%q) = %v, want %v", s, got, want)
		}
	}
}
