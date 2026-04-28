package httpapi

import (
	"reflect"
	"testing"
)

func TestWebsocketOriginPatterns_ConvertsFullOriginsToHosts(t *testing.T) {
	got := websocketOriginPatterns([]string{
		"http://fn.cky:14173",
		" https://example.com ",
		"*.ripple.test",
		"",
	})
	want := []string{"fn.cky:14173", "example.com", "*.ripple.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("patterns mismatch: got %#v want %#v", got, want)
	}
}
