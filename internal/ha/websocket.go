package ha

// A minimal RFC 6455 client: text messages, ping/pong and close. It is all the
// Home Assistant API needs, and keeps the binary free of extra dependencies.

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	opContinuation = 0x0
	opText         = 0x1
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA

	maxMessage   = 8 << 20
	writeTimeout = 10 * time.Second
)

type wsConn struct {
	c  net.Conn
	br *bufio.Reader
	mu sync.Mutex // serialises writes
}

func dialWS(rawURL string) (*wsConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if u.Port() == "" {
		port := "80"
		if u.Scheme == "wss" {
			port = "443"
		}
		host = net.JoinHostPort(u.Hostname(), port)
	}
	d := &net.Dialer{Timeout: 10 * time.Second}
	var c net.Conn
	if u.Scheme == "wss" {
		c, err = tls.DialWithDialer(d, "tcp", host, &tls.Config{ServerName: u.Hostname()})
	} else {
		c, err = d.Dial("tcp", host)
	}
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		c.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(nonce)
	c.SetDeadline(time.Now().Add(15 * time.Second))
	_, err = fmt.Fprintf(c, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\n"+
		"Connection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		u.RequestURI(), u.Host, key)
	if err != nil {
		c.Close()
		return nil, err
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		c.Close()
		return nil, err
	}
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if resp.StatusCode != http.StatusSwitchingProtocols ||
		resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) {
		c.Close()
		return nil, fmt.Errorf("websocket handshake refused: %s", resp.Status)
	}
	c.SetDeadline(time.Time{})
	return &wsConn{c: c, br: br}, nil
}

func (w *wsConn) Close() error { return w.c.Close() }

func (w *wsConn) writeFrame(op byte, payload []byte) error {
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	buf := []byte{0x80 | op}
	switch n := len(payload); {
	case n < 126:
		buf = append(buf, 0x80|byte(n))
	case n <= 0xFFFF:
		buf = append(buf, 0x80|126)
		buf = binary.BigEndian.AppendUint16(buf, uint16(n))
	default:
		buf = append(buf, 0x80|127)
		buf = binary.BigEndian.AppendUint64(buf, uint64(n))
	}
	buf = append(buf, mask[:]...)
	for i, b := range payload {
		buf = append(buf, b^mask[i%4])
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.c.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := w.c.Write(buf)
	return err
}

func (w *wsConn) readFrame() (fin bool, op byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(w.br, h[:]); err != nil {
		return
	}
	fin, op = h[0]&0x80 != 0, h[0]&0x0F
	masked := h[1]&0x80 != 0
	n := uint64(h[1] & 0x7F)
	switch n {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(w.br, b[:]); err != nil {
			return
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(w.br, b[:]); err != nil {
			return
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > maxMessage {
		err = errors.New("websocket frame too large")
		return
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(w.br, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(w.br, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return
}

// ReadMessage returns the next data message, answering pings on the way. It
// fails if nothing at all arrives within idle.
func (w *wsConn) ReadMessage(idle time.Duration) ([]byte, error) {
	var msg []byte
	for {
		w.c.SetReadDeadline(time.Now().Add(idle))
		fin, op, p, err := w.readFrame()
		if err != nil {
			return nil, err
		}
		switch op {
		case opPing:
			if err := w.writeFrame(opPong, p); err != nil {
				return nil, err
			}
		case opPong:
		case opClose:
			return nil, io.EOF
		default:
			if len(msg)+len(p) > maxMessage {
				return nil, errors.New("websocket message too large")
			}
			msg = append(msg, p...)
			if fin {
				return msg, nil
			}
		}
	}
}

func (w *wsConn) WriteText(p []byte) error { return w.writeFrame(opText, p) }
