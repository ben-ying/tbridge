package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var deviceID = regexp.MustCompile(`^[a-f0-9]{24}$`)

type device struct {
	Name      string `json:"name"`
	TokenHash string `json:"tokenHash"`
}
type pendingPair struct {
	Hash    string    `json:"hash"`
	Expires time.Time `json:"expires"`
}
type stateStore struct {
	dir string
	mu  sync.Mutex
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func tlsSettings() *tls.Config { return &tls.Config{MinVersion: tls.VersionTLS12} }
func openStore(dir string) (*stateStore, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	if err = os.Chmod(abs, 0700); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(abs, "devices"), 0700); err != nil {
		return nil, err
	}
	s := &stateStore{dir: abs}
	if _, err = os.Stat(filepath.Join(abs, "cert.pem")); errors.Is(err, os.ErrNotExist) {
		if err = s.makeCert(); err != nil {
			return nil, err
		}
	}
	if _, err = tls.LoadX509KeyPair(filepath.Join(abs, "cert.pem"), filepath.Join(abs, "key.pem")); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *stateStore) write(name string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(filepath.Join(s.dir, name)), ".tmp-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(s.dir, name))
}
func (s *stateStore) read(name string, v any) error {
	b, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
func (s *stateStore) makeCert() error {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	t := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "TBridge local server"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(5, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, t, t, &k.PublicKey, k)
	if err != nil {
		return err
	}
	priv, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(s.dir, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv}), 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
}
func (s *stateStore) fingerprint() (string, error) {
	b, err := os.ReadFile(filepath.Join(s.dir, "cert.pem"))
	if err != nil {
		return "", err
	}
	p, _ := pem.Decode(b)
	if p == nil {
		return "", errors.New("invalid certificate")
	}
	return digest(string(p.Bytes)), nil
}
func (s *stateStore) authorized(token string) bool {
	// Token IDs are validated before constructing paths; credentials never enter URLs.
	if len(token) != 24+1+64 || token[24] != '.' || !deviceID.MatchString(token[:24]) {
		return false
	}
	var d device
	if s.read(filepath.Join("devices", token[:24]+".json"), &d) != nil {
		return false
	}
	return constantEqual(d.TokenHash, digest(token))
}
