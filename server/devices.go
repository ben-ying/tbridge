package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type deviceStatus struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Current  bool       `json:"current"`
	Online   bool       `json:"online"`
	LastSeen *time.Time `json:"lastSeen,omitempty"`
}

func (s *service) noteDevice(token string) {
	if len(token) < 24 {
		return
	}
	s.presenceMu.Lock()
	defer s.presenceMu.Unlock()
	if s.lastSeen == nil {
		s.lastSeen = make(map[string]time.Time)
	}
	s.lastSeen[token[:24]] = time.Now().UTC()
}

func (s *service) devices(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(filepath.Join(s.store.dir, "devices"))
	if err != nil {
		failure(w, 500, "cannot list devices")
		return
	}
	s.presenceMu.Lock()
	seen := make(map[string]time.Time, len(s.lastSeen))
	for id, timestamp := range s.lastSeen {
		seen[id] = timestamp
	}
	s.presenceMu.Unlock()
	current := requestToken(r)[:24] // Only reached after authentication validates the token.
	devices := make([]deviceStatus, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || !deviceID.MatchString(id) {
			continue
		}
		var record device
		if err := s.store.read(filepath.Join("devices", entry.Name()), &record); err != nil {
			// Revocation may race this read.
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			failure(w, 500, "cannot read device record")
			return
		}
		status := deviceStatus{ID: id, Name: record.Name, Current: id == current}
		if last, ok := seen[id]; ok {
			status.LastSeen = &last
			// Presence is recent authenticated traffic, not a guarantee of an open TCP connection.
			status.Online = time.Since(last) <= 30*time.Second
		}
		devices = append(devices, status)
	}
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Online != devices[j].Online {
			return devices[i].Online
		}
		if devices[i].Name != devices[j].Name {
			return devices[i].Name < devices[j].Name
		}
		return devices[i].ID < devices[j].ID
	})
	// Return an explicit public view: token hashes and credentials must never leave the server.
	respond(w, 200, devices)
}
