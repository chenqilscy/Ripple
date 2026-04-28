package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"nhooyr.io/websocket"
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

func TestWebsocketOriginPatterns_AllowsConfiguredCORSOrigin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: websocketOriginPatterns([]string{"http://fn.cky:14173"}),
		})
		if err != nil {
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "ok")
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://fn.cky:14173"}},
	})
	if err != nil {
		t.Fatalf("dial websocket with configured origin: %v", err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "done")
}
