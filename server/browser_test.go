package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type hijackRecorder struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (h *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, bufio.NewReadWriter(bufio.NewReader(h.conn), bufio.NewWriter(h.conn)), nil
}
func TestWebSocketUpgrade(t *testing.T) {
	r := httptest.NewRequest("GET", "https://host:8443/api/v1/sessions/test/terminal", nil)
	r.Header.Set("Connection", "keep-alive, Upgrade")
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("Sec-WebSocket-Version", "13")
	r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	r.Header.Set("Origin", "https://host:8443")
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	_ = b.SetDeadline(time.Now().Add(2 * time.Second))
	result := make(chan error, 1)
	go func() {
		resp, err := http.ReadResponse(bufio.NewReader(b), r)
		if err == nil && (resp.StatusCode != 101 || resp.Header.Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=") {
			err = fmt.Errorf("invalid websocket upgrade response")
		}
		result <- err
	}()
	if _, err := upgrade(&hijackRecorder{httptest.NewRecorder(), a}, r); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"Origin": "https://evil.example", "Sec-WebSocket-Version": "12", "Sec-WebSocket-Key": "invalid"} {
		bad := r.Clone(context.Background())
		bad.Header = r.Header.Clone()
		bad.Header.Set(name, value)
		w := httptest.NewRecorder()
		if _, err := upgrade(w, bad); err == nil || w.Code != 400 {
			t.Fatal("invalid handshake accepted", name)
		}
	}
}

func TestBrowserPairCookieAndLogout(t *testing.T) {
	s := testStore(t)
	svc := &service{store: s, ctx: context.Background()}
	h := svc.routes()
	if err := s.write("pair.json", pendingPair{Hash: digest("browser-code"), Expires: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "https://host:8443/api/v1/browser/pair", strings.NewReader(`{"code":"browser-code","name":"Browser"}`))
	r.Header.Set("Origin", "https://host:8443")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["token"] != "" {
		t.Fatal("browser response exposed bearer credential")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" {
		t.Fatal("insecure browser cookie")
	}
	if !s.authorized(cookie.Value) {
		t.Fatal("cookie unauthorized")
	}
	r = httptest.NewRequest("POST", "https://host:8443/api/v1/browser/logout", nil)
	r.AddCookie(cookie)
	r.Header.Set("Origin", "https://host:8443")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || s.authorized(cookie.Value) {
		t.Fatal("logout did not revoke")
	}
	if w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not clear cookie")
	}
}
func TestOriginIsolationAndStaticAssets(t *testing.T) {
	s := testStore(t)
	h := (&service{store: s}).routes()
	for _, origin := range []string{"https://evil.example", "http://host:8443", "null", "https://host:8443.evil.example"} {
		r := httptest.NewRequest("POST", "https://host:8443/api/v1/browser/pair", strings.NewReader(`{}`))
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("cross-origin request allowed", origin, w.Code)
		}
	}
	for _, path := range []string{"/", "/app.js", "/terminal-frame.html"} {
		w := request(t, h, "GET", path, "", "")
		if w.Code != 200 {
			t.Fatal(path, w.Code)
		}
		if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("security headers absent")
		}
	}
	for _, path := range []string{"/missing", "/terminal/vendor/", "/../store.go"} {
		if w := request(t, h, "GET", path, "", ""); w.Code == 200 {
			t.Fatal("unexpected static path", path)
		}
	}
	if w := request(t, h, "POST", "/app.js", "", ""); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if !sameOrigin(httptest.NewRequest("GET", "https://host/", nil)) {
		t.Fatal("native clients must be allowed")
	}
}
