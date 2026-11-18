package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T) *stateStore {
	t.Helper()
	s, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func request(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestPairingAndRevocation(t *testing.T) {
	s := testStore(t)
	svc := &service{store: s, ctx: context.Background()}
	h := svc.routes()
	if e := s.write("pair.json", pendingPair{Hash: digest("secret"), Expires: time.Now().Add(time.Minute)}); e != nil {
		t.Fatal(e)
	}
	w := request(t, h, "POST", "/api/v1/pair", `{"code":"wrong","name":"Test client"}`, "")
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	w = request(t, h, "POST", "/api/v1/pair", `{"code":"secret","name":"Test client"}`, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var got map[string]string
	if e := json.Unmarshal(w.Body.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if !s.authorized(got["token"]) {
		t.Fatal("issued token not authorized")
	}
	if strings.Contains(string(mustRead(t, filepath.Join(s.dir, "devices", got["deviceId"]+".json"))), got["token"]) {
		t.Fatal("raw credential on disk")
	}
	if request(t, h, "POST", "/api/v1/pair", `{"code":"secret","name":"Test client"}`, "").Code != 401 {
		t.Fatal("code reused")
	}
	if err := os.Remove(filepath.Join(s.dir, "devices", got["deviceId"]+".json")); err != nil {
		t.Fatal(err)
	}
	if s.authorized(got["token"]) {
		t.Fatal("revocation failed")
	}
	if request(t, h, "GET", "/api/v1/sessions", "", got["token"]).Code != 401 {
		t.Fatal("API accepted revoked credential")
	}
}
func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestExpiredAndInvalidRequests(t *testing.T) {
	s := testStore(t)
	svc := &service{store: s}
	h := svc.routes()
	_ = s.write("pair.json", pendingPair{Hash: digest("old"), Expires: time.Now().Add(-time.Minute)})
	if request(t, h, "POST", "/api/v1/pair", `{"code":"old","name":"Test client"}`, "").Code != 401 {
		t.Fatal("expired code accepted")
	}
	for _, body := range []string{`{"code":"x","name":"A","extra":true}`, `{"code":"x","name":"A"} {}`, `{"code":"x","name":"\n"}`, strings.Repeat("x", 70000)} {
		if request(t, h, "POST", "/api/v1/pair", body, "").Code != 400 {
			t.Fatal("bad body accepted")
		}
	}
	for _, token := range []string{"", "../token", strings.Repeat("x", 89)} {
		if s.authorized(token) {
			t.Fatal("invalid token")
		}
	}
	for _, address := range []string{"http://host:8443", "https://user@host:8443", "https://host:8443/path", "https://host"} {
		if _, e := validateAddress(address); e == nil {
			t.Fatal("invalid address", address)
		}
	}
}
func TestCertificateAndPermissions(t *testing.T) {
	s := testStore(t)
	fp, e := s.fingerprint()
	if e != nil || len(fp) != 64 {
		t.Fatal(fp, e)
	}
	again, e := openStore(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	fp2, _ := again.fingerprint()
	if fp2 != fp {
		t.Fatal("certificate changed on restart")
	}
	for _, p := range []string{s.dir, filepath.Join(s.dir, "key.pem")} {
		info, e := os.Stat(p)
		if e != nil {
			t.Fatal(e)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatal("public credential permissions")
		}
	}
}

func maskedFrame(op byte, fin bool, data []byte) []byte {
	h := []byte{op}
	if fin {
		h[0] |= 128
	}
	n := len(data)
	if n < 126 {
		h = append(h, 128|byte(n))
	} else if n <= 65535 {
		h = append(h, 128|126, byte(n>>8), byte(n))
	} else {
		h = append(h, 128|127)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(n))
		h = append(h, b[:]...)
	}
	mask := []byte{1, 2, 3, 4}
	h = append(h, mask...)
	for i, b := range data {
		h = append(h, b^mask[i%4])
	}
	return h
}
func TestWebSocketFrames(t *testing.T) {
	for _, n := range []int{0, 10, 126, 70000} {
		t.Run(string(rune(n+65)), func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			ws := &webSocket{conn: a, r: bufio.NewReader(a)}
			want := bytes.Repeat([]byte("a"), n)
			go func() { _ = writeAll(b, maskedFrame(1, true, want)) }()
			got, e := ws.read()
			if e != nil || !bytes.Equal(got, want) {
				t.Fatal("read", e)
			}
		})
	}
	t.Run("fragmented", func(t *testing.T) {
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()
		ws := &webSocket{conn: a, r: bufio.NewReader(a)}
		go func() {
			_ = writeAll(b, append(maskedFrame(1, false, []byte("中")), maskedFrame(0, true, []byte("文"))...))
		}()
		got, e := ws.read()
		if e != nil || string(got) != "中文" {
			t.Fatal(string(got), e)
		}
	})
	for name, frame := range map[string][]byte{
		"unmasked": {0x81, 0}, "rsv": {0xc1, 0x80}, "oversize": {0x81, 0xff, 0, 0, 0, 0, 0, 4, 0, 0},
		"continuation": maskedFrame(0, true, nil), "binary": maskedFrame(2, true, nil), "utf8": maskedFrame(1, true, []byte{255}), "close": maskedFrame(8, true, []byte{1}),
	} {
		t.Run(name, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			ws := &webSocket{conn: a, r: bufio.NewReader(a)}
			go func() { _ = writeAll(b, frame) }()
			if _, e := ws.read(); e == nil {
				t.Fatal("invalid frame accepted")
			}
		})
	}
}
func TestWebSocketControlAndOutput(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	ws := &webSocket{conn: a, r: bufio.NewReader(a)}
	go func() {
		_ = writeAll(b, maskedFrame(9, true, []byte("ping")))
		reply := make([]byte, 6)
		_, _ = io.ReadFull(b, reply)
		_ = writeAll(b, maskedFrame(1, true, []byte(`{}`)))
	}()
	got, e := ws.read()
	if e != nil || string(got) != "{}" {
		t.Fatal(e)
	}
	go func() {
		_ = ws.send(terminalMessage{Type: "output", Data: base64.StdEncoding.EncodeToString([]byte("中文"))})
	}()
	h := make([]byte, 2)
	if _, e = io.ReadFull(b, h); e != nil {
		t.Fatal(e)
	}
	data := make([]byte, int(h[1]))
	if _, e = io.ReadFull(b, data); e != nil {
		t.Fatal(e)
	}
	if h[0] != 0x81 || !bytes.Contains(data, []byte("output")) {
		t.Fatal("bad server frame")
	}
}
func TestTmuxPersistenceAndIsolation(t *testing.T) {
	if _, e := exec.LookPath("tmux"); e != nil {
		t.Skip("tmux unavailable")
	}
	dir, e := os.MkdirTemp("/tmp", "tb-test-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	m, e := newTmux(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer m.run("kill-server")
	probe, _ := m.command(context.Background(), "list-sessions").CombinedOutput()
	if strings.Contains(string(probe), "Operation not permitted") {
		t.Skip("environment prohibits Unix sockets; rerun this test on the Linux/macOS host")
	}
	list, e := m.list()
	if e != nil || len(list) != 0 {
		t.Fatal("empty list", list, e)
	}
	one, e := m.create("中文 Agent")
	if e != nil {
		t.Fatal(e)
	}
	two, e := m.create("Build")
	if e != nil {
		t.Fatal(e)
	}
	if one.ID == two.ID || !sessionID.MatchString(one.ID) {
		t.Fatal("unstable identity")
	}
	if m.exists(one.ID + ";kill-server") {
		t.Fatal("invalid target accepted")
	}
	if _, e = m.run("send-keys", "-t", one.ID, "-l", "printf 'TBRIDGE_MARKER_中文\\n'"); e != nil {
		t.Fatal(e)
	}
	_, _ = m.run("send-keys", "-t", one.ID, "Enter")
	deadline := time.Now().Add(3 * time.Second)
	for {
		out, _ := m.run("capture-pane", "-p", "-t", one.ID)
		if strings.Contains(out, "TBRIDGE_MARKER_中文") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("command output missing")
		}
		time.Sleep(20 * time.Millisecond)
	}
	out, _ := m.run("capture-pane", "-p", "-t", two.ID)
	if strings.Contains(out, "TBRIDGE_MARKER") {
		t.Fatal("input leaked to another session")
	}
	m2, _ := newTmux(dir)
	if !m2.exists(one.ID) {
		t.Fatal("session did not survive manager restart")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := m.command(ctx, "attach-session", "-t", one.ID)
	p, e := startPTY(cmd, 90, 25)
	if e != nil {
		cancel()
		t.Fatal(e)
	}
	if e = resizePTY(p, 80, 24); e != nil {
		t.Fatal(e)
	}
	if resizePTY(p, 0, 24) == nil {
		t.Fatal("bad resize accepted")
	}
	p.Close()
	cancel()
	_ = cmd.Wait()
	if !m.exists(one.ID) {
		t.Fatal("detaching ended the task")
	}
	_, _ = m.run("kill-session", "-t", one.ID)
	if m.exists(one.ID) {
		t.Fatal("ended session still exists")
	}
}

func TestPTYInputAndResize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", `read -r line; printf 'PTY_OK_%s\n' "$line"`)
	p, err := startPTY(cmd, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { p.Close(); cancel(); _ = cmd.Wait() }()
	if err = resizePTY(p, 100, 30); err != nil {
		t.Fatal(err)
	}
	if resizePTY(p, 0, 30) == nil {
		t.Fatal("invalid resize accepted")
	}
	if err = writeAll(p, []byte("中文\n")); err != nil {
		t.Fatal(err)
	}
	var got []byte
	buf := make([]byte, 1024)
	for !bytes.Contains(got, []byte("PTY_OK_中文")) {
		n, err := p.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			t.Fatalf("PTY output missing: %q (%v)", got, err)
		}
	}
}
