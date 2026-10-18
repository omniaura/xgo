package main

import "testing"

func TestNextVersion(t *testing.T) {
	tests := []struct {
		cur, bump, want string
		wantErr         bool
	}{
		{"1.3.0", "", "1.3.1", false},
		{"1.3.0", "patch", "1.3.1", false},
		{"1.3.9", "minor", "1.4.0", false},
		{"1.3.9", "major", "2.0.0", false},
		{"1.3.0", "1.10.2", "1.10.2", false},
		{"1.3.0", "v2.0.0", "2.0.0", false},
		{"1.2.9-omniaura.3", "patch", "", true},
		{"1.2.9-omniaura.3", "1.3.0", "1.3.0", false},
		{"1.3.0", "next", "", true},
	}
	for _, tt := range tests {
		got, err := nextVersion(tt.cur, tt.bump)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("nextVersion(%q, %q) = %q, %v; want %q, err=%v", tt.cur, tt.bump, got, err, tt.want, tt.wantErr)
		}
	}
}
