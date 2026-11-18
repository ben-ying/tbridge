package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed ui
var browserAssets embed.FS

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || strings.EqualFold(origin, "https://"+r.Host)
}
func requestToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		return ""
	}
	if cookie, err := r.Cookie("__Secure-tbridge"); err == nil {
		return cookie.Value
	}
	return ""
}
func setBrowserCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: "__Secure-tbridge", Value: token, Path: "/api/v1", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge})
}
func (s *service) browserLogout(w http.ResponseWriter, r *http.Request) {
	token := requestToken(r)
	if len(token) > 24 && deviceID.MatchString(token[:24]) {
		if err := os.Remove(filepath.Join(s.store.dir, "devices", token[:24]+".json")); err != nil && !os.IsNotExist(err) {
			failure(w, 500, "cannot revoke browser device")
			return
		}
	}
	setBrowserCookie(w, "", -1)
	respond(w, 200, map[string]bool{"ok": true})
}
func securityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	// Explicit wss origin covers browsers that do not map connect-src 'self' from HTTPS to WSS.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://"+r.Host+"; frame-src 'self'; frame-ancestors 'self'; font-src 'self'; img-src 'self'; base-uri 'none'; form-action 'self'")
}
func browserFiles() http.Handler {
	root, _ := fs.Sub(browserAssets, "ui")
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method not allowed", 405)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		entry, err := fs.Stat(root, name)
		if err != nil || entry.IsDir() {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
