// Copyright (C) 2026 Norca contributors
// See LICENSE for copying information.

package drpcplugin

import (
	"context"
	"net"
	"sync"
	"time"

	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcpool"
)

// Conn is a client connection to a plugin. A drpc connection carries one
// RPC at a time, so Conn keeps a small pool of them: concurrent calls and
// open streams each use their own, and idle ones are reused. It is safe
// for concurrent use.
type Conn struct {
	pool *drpcpool.Pool[string, *drpcconn.Conn]
	conn drpc.Conn

	closeOnce sync.Once
	closed    chan struct{}
}

var _ drpc.Conn = (*Conn)(nil)

// Dial returns a connection to a plugin listening on network and address
// (from its handshake line or a reattach configuration). It connects
// lazily: an unreachable plugin shows up as an error of the first call.
func Dial(network, address string) *Conn {
	pool := drpcpool.New[string, *drpcconn.Conn](drpcpool.Options{
		Expiration:  5 * time.Minute,
		KeyCapacity: 32,
	})
	dial := func(ctx context.Context, _ string) (*drpcconn.Conn, error) {
		var d net.Dialer
		raw, err := d.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return drpcconn.New(raw), nil
	}
	return &Conn{
		pool:   pool,
		conn:   pool.Get(context.Background(), network+"|"+address, dial),
		closed: make(chan struct{}),
	}
}

// Invoke issues a unary RPC.
func (c *Conn) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) error {
	return c.conn.Invoke(ctx, rpc, enc, in, out)
}

// NewStream starts a stream.
func (c *Conn) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (drpc.Stream, error) {
	return c.conn.NewStream(ctx, rpc, enc)
}

// Close closes the idle connections; calls still running keep theirs.
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		err = c.pool.Close()
	})
	return err
}

// Closed is closed when Close was called.
func (c *Conn) Closed() <-chan struct{} { return c.closed }

// Unblocked is always ready: every call can get a connection.
func (c *Conn) Unblocked() <-chan struct{} { return closedCh }

var closedCh = func() chan struct{} { ch := make(chan struct{}); close(ch); return ch }()
