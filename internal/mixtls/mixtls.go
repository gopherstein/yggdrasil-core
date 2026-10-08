// Package mixtls serves TLS and plain HTTP on one port (#175, #213), so a
// client that hasn't learned TLS yet still reaches a server that has.
package mixtls

import (
	"bufio"
	"crypto/tls"
	"net"
	"sync"
	"time"
)

// listener accepts TLS and plain HTTP connections on one port, by the
// first byte each sends: 0x16 starts a TLS handshake.
type listener struct {
	net.Listener
	config *tls.Config
	conns  chan net.Conn
	errs   chan error
	done   chan struct{}
	once   sync.Once
}

// sniffWait is how long a new connection has to send its first byte.
const sniffWait = 10 * time.Second

// NewListener serves ln's connections over TLS with config when they start
// a TLS handshake, and as they are otherwise.
func NewListener(ln net.Listener, config *tls.Config) net.Listener {
	m := &listener{Listener: ln, config: config, conns: make(chan net.Conn), errs: make(chan error, 1), done: make(chan struct{})}
	go m.loop()
	return m
}

func (m *listener) loop() {
	for {
		c, err := m.Listener.Accept()
		if err != nil {
			select {
			case m.errs <- err:
			case <-m.done:
			}
			return
		}
		go m.sniff(c)
	}
}

func (m *listener) sniff(c net.Conn) {
	_ = c.SetReadDeadline(time.Now().Add(sniffWait))
	br := bufio.NewReader(c)
	first, err := br.Peek(1)
	_ = c.SetReadDeadline(time.Time{})
	if err != nil {
		_ = c.Close()
		return
	}
	var out net.Conn = &peekedConn{Conn: c, r: br}
	if first[0] == 0x16 {
		out = tls.Server(out, m.config)
	}
	select {
	case m.conns <- out:
	case <-m.done:
		_ = c.Close()
	}
}

func (m *listener) Accept() (net.Conn, error) {
	select {
	case c := <-m.conns:
		return c, nil
	case err := <-m.errs:
		return nil, err
	case <-m.done:
		return nil, net.ErrClosed
	}
}

func (m *listener) Close() error {
	m.once.Do(func() { close(m.done) })
	return m.Listener.Close()
}

// peekedConn reads the bytes sniffing peeked before the rest.
type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }
