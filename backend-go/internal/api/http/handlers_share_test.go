package httpapi

import "testing"

func TestIsShareTokenFormat(t *testing.T) {
	valid := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOP_"
	if len(valid) != shareTokenLength {
		t.Fatalf("test token length drifted: got %d", len(valid))
	}
	if !isShareTokenFormat(valid) {
		t.Fatalf("expected valid share token")
	}

	cases := []string{
		"",
		"short",
		valid + "x",
		"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO/",
		"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO.",
	}
	for _, tc := range cases {
		if isShareTokenFormat(tc) {
			t.Fatalf("expected invalid share token format for %q", tc)
		}
	}
}
