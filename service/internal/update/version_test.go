package update

import "testing"

func TestNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want, wantErr   bool
	}{
		{"1.6.0", "1.5.0", true, false},
		{"1.5.0", "1.5.0", false, false},
		{"1.5.0", "1.6.0", false, false},
		{"1.10.0", "1.9.0", true, false},
		{"2.0.0", "1.99.99", true, false},
		{"1.6.0", "1.6.0-rc1", true, false},
		{"1.6.0", "1.6.0-rc1-3-gabc123", true, false},
		{"1.6.0-rc1", "1.6.0", false, false},
		{"1.6.0", "1.5.0-3-gabc123", true, false},
		{"1.5.0", "1.5.0-3-gabc123", false, false},
		{"1.6.0", "dev", false, true},
		{"1.6", "1.5.0", false, true},
		{"x.y.z", "1.5.0", false, true},
	}
	for _, tt := range tests {
		got, err := newer(tt.latest, tt.current)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("newer(%q, %q) = %v, %v; want %v, err %v", tt.latest, tt.current, got, err, tt.want, tt.wantErr)
		}
	}
}
