package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDeviceListAuthenticationPresenceAndRevocation(t *testing.T) {
	store := testStore(t)
	svc := &service{store: store, ctx: context.Background()}
	token := strings.Repeat("a", 24) + "." + strings.Repeat("b", 64)
	other := strings.Repeat("c", 24) + "." + strings.Repeat("d", 64)
	for credential, name := range map[string]string{token: "Phone", other: "Laptop"} {
		if err := store.write(filepath.Join("devices", credential[:24]+".json"), device{Name: name, TokenHash: digest(credential)}); err != nil {
			t.Fatal(err)
		}
	}
	h := svc.routes()
	if w := request(t, h, "GET", "/api/v1/devices", "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	svc.noteDevice(other)
	w := request(t, h, "GET", "/api/v1/devices", "", token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var devices []deviceStatus
	if err := json.Unmarshal(w.Body.Bytes(), &devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 {
		t.Fatal(devices)
	}
	for _, d := range devices {
		if !d.Online || d.LastSeen == nil || d.Current != (d.ID == token[:24]) {
			t.Fatal(d)
		}
	}
	for _, secret := range []string{token, other, digest(token), digest(other), "tokenHash"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("credentials leaked")
		}
	}
	svc.presenceMu.Lock()
	svc.lastSeen[other[:24]] = time.Now().Add(-time.Minute)
	svc.presenceMu.Unlock()
	w = request(t, h, "GET", "/api/v1/devices", "", token)
	if err := json.Unmarshal(w.Body.Bytes(), &devices); err != nil {
		t.Fatal(err)
	}
	if !devices[0].Current || !devices[0].Online || devices[1].Online {
		t.Fatal(devices)
	}
	r := httptest.NewRequest("GET", "https://server.test/api/v1/devices", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Origin", "https://other.test")
	cross := httptest.NewRecorder()
	h.ServeHTTP(cross, r)
	if cross.Code != 403 {
		t.Fatal(cross.Code)
	}
	if err := os.Remove(filepath.Join(store.dir, "devices", other[:24]+".json")); err != nil {
		t.Fatal(err)
	}
	if w = request(t, h, "GET", "/api/v1/devices", "", other); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w = request(t, h, "GET", "/api/v1/devices", "", token)
	if err := json.Unmarshal(w.Body.Bytes(), &devices); err != nil || len(devices) != 1 {
		t.Fatal(err, devices)
	}
}

func TestDevicePresenceConcurrentUpdates(t *testing.T) {
	svc := &service{}
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); svc.noteDevice(strings.Repeat("a", 89)) }()
	}
	workers.Wait()
	if len(svc.lastSeen) != 1 {
		t.Fatal(svc.lastSeen)
	}
}
