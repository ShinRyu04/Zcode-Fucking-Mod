package update

import "testing"

func TestGreater(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.2.0", "0.1.9", true},
		{"0.1.0", "0.1.0", false},
		{"v0.1.10", "0.1.9", true},
		{"0.1.0-mod", "0.1.0", false},
		{"v1", "0.9.9", true},
		{"0.1", "0.1.0", false},
	}
	for _, c := range cases {
		if got := Greater(c.a, c.b); got != c.want {
			t.Fatalf("Greater(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
