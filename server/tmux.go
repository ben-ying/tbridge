package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Random names are never reused after a tmux server restart, unlike numeric $0 IDs.
var sessionID = regexp.MustCompile(`^tb-[a-f0-9]{24}$`)

type terminalSession struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Attach string `json:"attach"`
}
type tmuxManager struct {
	socket string
	binary string
}

func newTmux(dir string) (*tmuxManager, error) {
	b, err := exec.LookPath("tmux")
	if err != nil {
		return nil, errors.New("tmux is required; install tmux before starting TBridge")
	}
	socket := filepath.Join(dir, "tmux.sock")
	if len(socket) > 95 {
		return nil, errors.New("state path too long for portable Unix socket; choose a shorter --state")
	}
	return &tmuxManager{socket: socket, binary: b}, nil
}
func (t *tmuxManager) command(ctx context.Context, args ...string) *exec.Cmd {
	// Force tmux's UTF-8 mode without replacing the user's shell locale.
	return exec.CommandContext(ctx, t.binary, append([]string{"-u", "-S", t.socket}, args...)...)
}
func (t *tmuxManager) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := t.command(ctx, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux operation failed: %w", err)
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}
func (t *tmuxManager) list() ([]terminalSession, error) {
	if _, err := os.Stat(t.socket); os.IsNotExist(err) {
		return []terminalSession{}, nil
	} else if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := t.command(ctx, "list-sessions", "-F", "#{session_name}\t#{@tbridge_name}").CombinedOutput()
	if err != nil {
		if strings.Contains(string(b), "no server running") || strings.Contains(string(b), "No such file or directory") {
			return []terminalSession{}, nil
		}
		return nil, err
	}
	out := strings.TrimSuffix(string(b), "\n")
	items := []terminalSession{}
	for _, line := range strings.Split(out, "\n") {
		p := strings.SplitN(line, "\t", 2)
		if len(p) != 2 || !sessionID.MatchString(p[0]) {
			continue
		}
		name := p[1]
		if name == "" {
			name = p[0]
		}
		items = append(items, terminalSession{p[0], name, fmt.Sprintf("tmux -S '%s' attach-session -t '%s'", strings.ReplaceAll(t.socket, "'", "'\\''"), p[0])})
	}
	return items, nil
}
func (t *tmuxManager) exists(id string) bool {
	if !sessionID.MatchString(id) {
		return false
	}
	_, err := t.run("has-session", "-t", id)
	return err == nil
}
func (t *tmuxManager) create(name string) (terminalSession, error) {
	id, err := t.run("-f", "/dev/null", "new-session", "-d", "-P", "-F", "#{session_name}", "-s", "tb-"+randomHex(12), "-x", "100", "-y", "30")
	if err != nil {
		return terminalSession{}, err
	}
	// Keep tmux's default prefix for desktop detach (Ctrl+B, D); Ctrl+B twice reaches the app.
	// Suppress tmux's outer alternate screen so restored history remains scrollable in xterm.
	for _, args := range [][]string{{"set-option", "-s", "terminal-overrides", "xterm-256color:smcup@:rmcup@"}, {"set-option", "-t", id, "history-limit", "1000"}, {"set-option", "-t", id, "status", "off"}, {"set-option", "-t", id, "@tbridge_name", name}} {
		if _, err = t.run(args...); err != nil {
			_, _ = t.run("kill-session", "-t", id)
			return terminalSession{}, err
		}
	}
	return terminalSession{ID: id, Name: name}, nil
}
func (t *tmuxManager) history(id string) (string, error) {
	count, err := t.run("display-message", "-p", "-t", id+":0.0", "#{history_size}")
	if err != nil {
		return "", err
	}
	n, err := strconv.Atoi(count)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	return t.run("capture-pane", "-p", "-e", "-t", id+":0.0", "-S", "-1000", "-E", "-1")
}
