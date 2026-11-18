package main

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

type service struct {
	store *stateStore
	tmux  *tmuxManager
	ctx   context.Context
}

func constantEqual(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("one JSON object required")
	}
	return nil
}
func (s *service) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /api/v1/pair", s.pair)
	m.HandleFunc("POST /api/v1/browser/pair", s.pair)
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := requestToken(r)
			if !s.store.authorized(token) {
				failure(w, 401, "device authentication failed")
				return
			}
			next(w, r)
		}
	}
	m.HandleFunc("GET /api/v1/sessions", auth(s.sessions))
	m.HandleFunc("POST /api/v1/sessions", auth(s.create))
	m.HandleFunc("PATCH /api/v1/sessions/{id}", auth(s.rename))
	m.HandleFunc("DELETE /api/v1/sessions/{id}", auth(s.remove))
	m.HandleFunc("GET /api/v1/sessions/{id}/terminal", auth(s.terminal))
	m.HandleFunc("POST /api/v1/browser/logout", auth(s.browserLogout))
	m.Handle("/", browserFiles())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w, r)
		if !sameOrigin(r) {
			failure(w, 403, "cross-origin requests are not allowed")
			return
		}
		m.ServeHTTP(w, r)
	})
}
func validName(name string) bool {
	return strings.TrimSpace(name) != "" && len(name) <= 120 && strings.IndexFunc(name, unicode.IsControl) < 0
}
func (s *service) pair(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if decode(w, r, &in) != nil || !validName(in.Name) {
		failure(w, 400, "valid code and device name required")
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	var p pendingPair
	if s.store.read("pair.json", &p) != nil || time.Now().After(p.Expires) || !constantEqual(p.Hash, digest(in.Code)) {
		failure(w, 401, "pairing code invalid or expired")
		return
	}
	id := randomHex(12)
	token := id + "." + randomHex(32)
	// Remove the code before issuing credentials: a failed response requires fresh pairing.
	if err := os.Remove(filepath.Join(s.store.dir, "pair.json")); err != nil {
		failure(w, 500, "cannot consume pairing code")
		return
	}
	if err := s.store.write(filepath.Join("devices", id+".json"), device{Name: in.Name, TokenHash: digest(token)}); err != nil {
		failure(w, 500, "cannot save device")
		return
	}
	if r.URL.Path == "/api/v1/browser/pair" {
		// Browsers use host-only HttpOnly cookies; bearer credentials never enter JavaScript or URLs.
		setBrowserCookie(w, token, 30*24*60*60)
		respond(w, 201, map[string]string{"deviceId": id})
	} else {
		respond(w, 201, map[string]string{"token": token, "deviceId": id})
	}
}
func (s *service) sessions(w http.ResponseWriter, r *http.Request) {
	v, err := s.tmux.list()
	if err != nil {
		failure(w, 503, "cannot list tmux sessions")
		return
	}
	respond(w, 200, v)
}
func (s *service) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if decode(w, r, &in) != nil || !validName(in.Name) {
		failure(w, 400, "invalid session name")
		return
	}
	v, err := s.tmux.create(in.Name)
	if err != nil {
		failure(w, 503, "cannot create tmux session")
		return
	}
	respond(w, 201, v)
}
func (s *service) target(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !s.tmux.exists(id) {
		failure(w, 404, "session closed or unavailable")
		return "", false
	}
	return id, true
}
func (s *service) rename(w http.ResponseWriter, r *http.Request) {
	id, ok := s.target(w, r)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if decode(w, r, &in) != nil || !validName(in.Name) {
		failure(w, 400, "invalid session name")
		return
	}
	if _, err := s.tmux.run("set-option", "-t", id, "@tbridge_name", in.Name); err != nil {
		failure(w, 503, "rename failed")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *service) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := s.target(w, r)
	if !ok {
		return
	}
	if _, err := s.tmux.run("kill-session", "-t", id); err != nil {
		failure(w, 503, "session termination failed")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

type terminalMessage struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	ID      string `json:"id,omitempty"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
	Message string `json:"message,omitempty"`
}

func (s *service) terminal(w http.ResponseWriter, r *http.Request) {
	id, ok := s.target(w, r)
	if !ok {
		return
	}
	ws, err := upgrade(w, r)
	if err != nil {
		return
	}
	defer ws.conn.Close()
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	cmd := s.tmux.command(ctx, "attach-session", "-t", id)
	p, err := startPTY(cmd, 100, 30)
	if err != nil {
		_ = ws.send(terminalMessage{Type: "error", Message: "cannot attach terminal"})
		return
	}
	defer func() { p.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	var workers sync.WaitGroup
	// Closing both descriptors unblocks all workers before the handler returns.
	defer func() { cancel(); p.Close(); ws.conn.Close(); workers.Wait() }()
	if history, e := s.tmux.history(id); e == nil && history != "" {
		history = strings.ReplaceAll(history, "\n", "\r\n")
		for len(history) > 0 {
			n := len(history)
			if n > 16<<10 {
				n = 16 << 10
			}
			if ws.send(terminalMessage{Type: "output", Data: base64.StdEncoding.EncodeToString([]byte(history[:n]))}) != nil {
				return
			}
			history = history[n:]
		}
		_ = ws.send(terminalMessage{Type: "output", Data: base64.StdEncoding.EncodeToString([]byte("\r\n"))})
	}
	// Mark input ready before PTY output can trigger terminal identification replies.
	if ws.send(terminalMessage{Type: "ready"}) != nil {
		return
	}
	workers.Add(2)
	go func() {
		defer workers.Done()
		defer cancel()
		buf := make([]byte, 16<<10)
		for {
			n, e := p.Read(buf)
			if n > 0 {
				if ws.send(terminalMessage{Type: "output", Data: base64.StdEncoding.EncodeToString(buf[:n])}) != nil {
					return
				}
			}
			if e != nil {
				return
			}
		}
	}()
	token := requestToken(r)
	go func() {
		defer workers.Done()
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				ws.conn.Close()
				return
			case <-tick.C:
				if !s.store.authorized(token) {
					_ = ws.send(terminalMessage{Type: "error", Message: "device authorization revoked"})
					cancel()
					continue
				}
				if ws.writeFrame(9, nil) != nil {
					cancel()
				}
			}
		}
	}()
	seen := map[string]bool{}
	for {
		b, e := ws.read()
		if e != nil {
			return
		}
		var msg terminalMessage
		if json.Unmarshal(b, &msg) != nil {
			_ = ws.send(terminalMessage{Type: "error", Message: "invalid terminal message"})
			return
		}
		switch msg.Type {
		case "input":
			if msg.ID == "" || len(msg.ID) > 80 {
				_ = ws.send(terminalMessage{Type: "error", Message: "input ID required"})
				return
			}
			if seen[msg.ID] {
				continue
			}
			if len(seen) >= 65536 {
				_ = ws.send(terminalMessage{Type: "error", Message: "reconnect to continue; input was not sent"})
				return
			}
			data, e := base64.StdEncoding.DecodeString(msg.Data)
			if e != nil || len(data) > 64<<10 {
				_ = ws.send(terminalMessage{Type: "error", Message: "invalid input size or encoding"})
				return
			}
			seen[msg.ID] = true
			if writeAll(p, data) != nil {
				return
			}
			if ws.send(terminalMessage{Type: "ack", ID: msg.ID}) != nil {
				return
			}
		case "resize":
			if resizePTY(p, msg.Cols, msg.Rows) != nil {
				_ = ws.send(terminalMessage{Type: "error", Message: "invalid terminal dimensions"})
			}
		default:
			_ = ws.send(terminalMessage{Type: "error", Message: "unknown terminal message"})
			return
		}
	}
}
