package netx

import "testing"

func TestFromListParsesAndDefaults(t *testing.T) {
	r := FromList("http://a.example:8080, b.example:3128;https://c.example:8443")
	if r.Len() != 3 {
		t.Fatalf("len = %d, want 3", r.Len())
	}
	if got := r.At(0).String(); got != "http://a.example:8080" {
		t.Fatalf("at0 = %s", got)
	}
	if got := r.At(1).String(); got != "http://b.example:3128" {
		t.Fatalf("at1 = %s", got)
	}
	if got := r.At(2).String(); got != "https://c.example:8443" {
		t.Fatalf("at2 = %s", got)
	}
	if got := r.At(3); got == nil || got.Host != "a.example:8080" {
		t.Fatalf("wrap = %v", got)
	}
}

func TestFromListRejectsUnusable(t *testing.T) {
	if r := FromList("   ,, ; "); r != nil {
		t.Fatalf("want nil, got %v", r)
	}
	if r := FromList("socks5://127.0.0.1:1080"); r != nil {
		t.Fatalf("socks5 must be ignored, got %v", r)
	}
}

func TestClientKeepsBaseAndUsesProxy(t *testing.T) {
	r := FromList("http://p.example:8080")
	if c := r.Client(nil, 0, 0); c.Transport == nil {
		t.Fatal("proxy client needs a transport")
	}
	var nilRot *Rotator
	if c := nilRot.Client(nil, 0, 0); c == nil {
		t.Fatal("nil rotator must still return a client")
	}
}
