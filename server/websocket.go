package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const maxMessage = 128 << 10

type webSocket struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex
}

func hasHeader(v, want string) bool {
	for _, s := range strings.Split(v, ",") {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}
func upgrade(w http.ResponseWriter, r *http.Request) (*webSocket, error) {
	key, err := base64.StdEncoding.DecodeString(r.Header.Get("Sec-WebSocket-Key"))
	// Native clients omit Origin; browser clients must come from this HTTPS origin.
	if err != nil || len(key) != 16 || r.Header.Get("Sec-WebSocket-Version") != "13" || !hasHeader(r.Header.Get("Connection"), "upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || !sameOrigin(r) {
		http.Error(w, "invalid websocket handshake", 400)
		return nil, errors.New("invalid handshake")
	}
	h, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("hijack unavailable")
	}
	c, b, err := h.Hijack()
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = fmt.Fprintf(b, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
	if err == nil {
		err = b.Flush()
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return &webSocket{conn: c, r: b.Reader}, nil
}
func (ws *webSocket) writeFrame(op byte, data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if len(data) > maxMessage {
		return errors.New("oversized output")
	}
	_ = ws.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	h := []byte{0x80 | op}
	if len(data) < 126 {
		h = append(h, byte(len(data)))
	} else if len(data) <= 65535 {
		h = append(h, 126, byte(len(data)>>8), byte(len(data)))
	} else {
		h = append(h, 127, 0, 0, 0, 0, byte(len(data)>>24), byte(len(data)>>16), byte(len(data)>>8), byte(len(data)))
	}
	if err := writeAll(ws.conn, h); err != nil {
		return err
	}
	return writeAll(ws.conn, data)
}
func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func (ws *webSocket) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ws.writeFrame(1, b)
}
func (ws *webSocket) read() ([]byte, error) {
	var message []byte
	fragmented := false
	for {
		_ = ws.conn.SetReadDeadline(time.Now().Add(45 * time.Second))
		h := make([]byte, 2)
		if _, err := io.ReadFull(ws.r, h); err != nil {
			return nil, err
		}
		fin := h[0]&0x80 != 0
		op := h[0] & 15
		if h[0]&0x70 != 0 || h[1]&0x80 == 0 {
			return nil, errors.New("RSV bits or unmasked client frame")
		}
		n := uint64(h[1] & 127)
		if n == 126 {
			var b [2]byte
			if _, err := io.ReadFull(ws.r, b[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(b[:]))
			if n < 126 {
				return nil, errors.New("noncanonical length")
			}
		} else if n == 127 {
			var b [8]byte
			if _, err := io.ReadFull(ws.r, b[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(b[:])
			if n <= 65535 {
				return nil, errors.New("noncanonical length")
			}
		}
		if n > maxMessage || (op < 8 && uint64(len(message))+n > maxMessage) {
			return nil, errors.New("message too large")
		}
		if op >= 8 && (!fin || n > 125) {
			return nil, errors.New("invalid control frame")
		}
		var mask [4]byte
		if _, err := io.ReadFull(ws.r, mask[:]); err != nil {
			return nil, err
		}
		data := make([]byte, int(n))
		if _, err := io.ReadFull(ws.r, data); err != nil {
			return nil, err
		}
		for i := range data {
			data[i] ^= mask[i%4]
		}
		switch op {
		case 8:
			if len(data) == 1 {
				return nil, errors.New("invalid close")
			}
			if len(data) >= 2 {
				code := binary.BigEndian.Uint16(data[:2])
				if code < 1000 || code >= 5000 || code == 1004 || code == 1005 || code == 1006 || code == 1015 || (code >= 1016 && code < 3000) || !utf8.Valid(data[2:]) {
					return nil, errors.New("invalid close code/reason")
				}
			}
			_ = ws.writeFrame(8, data)
			return nil, io.EOF
		case 9:
			if err := ws.writeFrame(10, data); err != nil {
				return nil, err
			}
			continue
		case 10:
			continue
		case 1:
			if fragmented {
				return nil, errors.New("overlapping fragmented message")
			}
			fragmented = true
		case 0:
			if !fragmented {
				return nil, errors.New("unexpected continuation")
			}
		default:
			return nil, errors.New("only text messages supported")
		}
		message = append(message, data...)
		if fin {
			if !utf8.Valid(message) {
				return nil, errors.New("invalid UTF-8")
			}
			return message, nil
		}
	}
}
