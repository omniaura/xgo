package main

import "testing"

func TestFoldConstraints(t *testing.T) {
	const body = "package p\n"
	tests := []struct {
		src     string
		want    string
		keep    bool
		changed bool
	}{
		{"//go:build go1.22\n\n" + body, body, true, true},
		{"//go:build !go1.22\n\n" + body, "", false, true},
		{"//go:build go1.18 && !go1.20\n" + body, "", false, true},
		{"//go:build go1.25 && test_driver_is_xgo\n// +build go1.25,test_driver_is_xgo\n\n" + body, "//go:build test_driver_is_xgo\n\n" + body, true, true},
		{"//go:build !go1.28\n\n" + body, "//go:build !go1.28\n\n" + body, true, false},
		{"//go:build go1.20 || windows\n\n" + body, body, true, true},
		{"// comment\n\n" + body, "// comment\n\n" + body, true, false},
	}
	for _, tt := range tests {
		got, keep, changed, err := foldConstraints(tt.src, 27, 27)
		if err != nil {
			t.Fatal(err)
		}
		if keep != tt.keep || changed != tt.changed || (keep && got != tt.want) {
			t.Errorf("foldConstraints(%q) = %q, keep=%v changed=%v; want %q, keep=%v changed=%v", tt.src, got, keep, changed, tt.want, tt.keep, tt.changed)
		}
	}
}

func TestFoldConstraintsKeepsLanguageUpgrades(t *testing.T) {
	// in a go1.18 module, go1.22 still raises the file's language version
	src := "//go:build go1.22\n\npackage p\n"
	got, keep, changed, err := foldConstraints(src, 27, 18)
	if err != nil || !keep || changed || got != src {
		t.Errorf("got %q keep=%v changed=%v err=%v", got, keep, changed, err)
	}
	// but unsatisfiable files still go
	if _, keep, _, _ := foldConstraints("//go:build !go1.22\n\npackage p\n", 27, 18); keep {
		t.Error("expected !go1.22 file to be deleted")
	}
	// and implied tags are still dropped
	got, _, _, _ = foldConstraints("//go:build go1.18\n\npackage p\n", 27, 18)
	if got != "package p\n" {
		t.Errorf("go1.18 in a go1.18 module: got %q", got)
	}
}

func TestParseMinor(t *testing.T) {
	for _, v := range []string{"1.28", "go1.28"} {
		if n, err := parseMinor(v); err != nil || n != 28 {
			t.Errorf("parseMinor(%q) = %d, %v", v, n, err)
		}
	}
	if _, err := parseMinor("28"); err == nil {
		t.Error("expected error")
	}
}
