package main

import "testing"

func TestVersionGreater(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v1.2.0", "1.1.9", true},
		{"1.2.0", "v1.2.0", false},
		{"1.2.1", "1.2", true},
		{"1.2.0-rc1", "1.2.0", false},
		{"2", "1.9.9", true},
		{"bogus", "1.0", false},
	}
	for _, c := range cases {
		if got := versionGreater(c.a, c.b); got != c.want {
			t.Errorf("versionGreater(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
