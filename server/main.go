package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: tbridge serve|pair|devices|revoke [options]")
	}
	f := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	home, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := f.String("state", filepath.Join(home, "tbridge"), "private state directory")
	listen := f.String("listen", "127.0.0.1:8443", "HTTPS listen address; use VPN/LAN interface explicitly")
	address := f.String("address", "", "advertised https://host:port for pairing")
	id := f.String("id", "", "device ID to revoke")
	if err := f.Parse(os.Args[2:]); err != nil {
		return err
	}
	store, err := openStore(*dir)
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "pair":
		u, err := validateAddress(*address)
		if err != nil {
			return err
		}
		code := randomHex(24)
		if err := store.write("pair.json", pendingPair{Hash: digest(code), Expires: time.Now().Add(5 * time.Minute)}); err != nil {
			return err
		}
		fp, err := store.fingerprint()
		if err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]string{"address": u, "fingerprint": fp, "code": code})
		fmt.Printf("Certificate SHA-256: %s\n", fp)
		fmt.Println("Paste into this server's HTTPS web page within five minutes (one use):")
		fmt.Println("tbridge:" + base64.RawURLEncoding.EncodeToString(data))
	case "devices":
		entries, err := os.ReadDir(filepath.Join(store.dir, "devices"))
		if err != nil {
			return err
		}
		for _, e := range entries {
			var d device
			if store.read(filepath.Join("devices", e.Name()), &d) == nil {
				fmt.Printf("%s\t%s\n", strings.TrimSuffix(e.Name(), ".json"), d.Name)
			}
		}
	case "revoke":
		if !deviceID.MatchString(*id) {
			return errors.New("valid --id required; see devices")
		}
		if err := os.Remove(filepath.Join(store.dir, "devices", *id+".json")); err != nil {
			return err
		}
		fmt.Println("Device revoked; open connections close within 10 seconds.")
	case "serve":
		if _, err := fs.Stat(browserAssets, "ui/terminal/vendor/xterm.js"); err != nil {
			return errors.New("web terminal assets missing: run scripts/prepare-web.sh and rebuild the server")
		}
		mux, err := newTmux(store.dir)
		if err != nil {
			return err
		}
		svc := &service{store: store, tmux: mux}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		srv := &http.Server{Addr: *listen, Handler: svc.routes(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, TLSConfig: tlsSettings()}
		// Shutdown also cancels hijacked WebSockets; tmux children remain alive.
		svc.ctx = ctx
		go func() {
			<-ctx.Done()
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(c)
		}()
		log.Printf("TBridge listening on %s; run 'tbridge pair --address https://HOST:PORT' locally", *listen)
		err = srv.ListenAndServeTLS(filepath.Join(store.dir, "cert.pem"), filepath.Join(store.dir, "key.pem"))
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	default:
		return errors.New("unknown command")
	}
	return nil
}

func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func validateAddress(s string) (string, error) {
	if !strings.HasPrefix(s, "https://") {
		return "", errors.New("--address must be https://host:port")
	}
	host := strings.TrimPrefix(s, "https://")
	if strings.ContainsAny(host, "/@?#\\\r\n") {
		return "", errors.New("address must contain only scheme, host and port")
	}
	_, _, err := net.SplitHostPort(host)
	if err != nil {
		return "", err
	}
	return s, nil
}
