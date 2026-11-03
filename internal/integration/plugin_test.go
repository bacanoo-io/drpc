// Copyright (C) 2026 Norca contributors
// See LICENSE for copying information.

package integration

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zeebo/assert"

	"storj.io/drpc"
	"storj.io/drpc/drpcplugin"
)

const testCookie = "DRPCPLUGIN_TEST_COOKIE"

// servePlugin serves impl like a plugin process would and returns the
// handshake line and a stop function.
func servePlugin(t *testing.T, srv DRPCServiceServer) (string, func()) {
	t.Helper()
	t.Setenv(testCookie, "yes")
	t.Setenv(drpcplugin.EnvProtocolVersions, "1,2")

	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- drpcplugin.Serve(drpcplugin.ServeConfig{
			MagicCookieKey:   testCookie,
			MagicCookieValue: "yes",
			ProtocolVersion:  2,
			Context:          ctx,
			Stdout:           out,
			Register: func(m drpc.Mux) error {
				return DRPCRegisterService(m, srv)
			},
		})
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.HasSuffix(out.String(), "\n") {
		if time.Now().After(deadline) {
			t.Fatal("no handshake line")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return strings.TrimSpace(out.String()), func() {
		cancel()
		assert.NoError(t, <-done)
	}
}

func TestPlugin_HandshakeAndCalls(t *testing.T) {
	block := make(chan struct{})
	srv := impl{
		Method1Fn: func(ctx context.Context, in *In) (*Out, error) {
			if in.In == 0 { // a long call, like an executor Wait
				select {
				case <-block:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return out(in.In), nil
		},
		Method3Fn: func(in *In, stream DRPCService_Method3Stream) error {
			for i := int64(0); i < in.In; i++ {
				if err := stream.Send(out(i)); err != nil {
					return err
				}
			}
			return nil
		},
	}
	line, stop := servePlugin(t, srv)
	defer stop()

	parts := strings.Split(line, "|")
	assert.Equal(t, len(parts), 5)
	assert.Equal(t, parts[0], "1")
	assert.Equal(t, parts[1], "2")
	assert.Equal(t, parts[4], drpcplugin.Protocol)

	conn := drpcplugin.Dial(parts[2], parts[3])
	defer func() { assert.NoError(t, conn.Close()) }()
	cli := NewDRPCServiceClient(conn)

	// a blocked call does not hold up the others
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		got, err := cli.Method1(context.Background(), in(0))
		assert.NoError(t, err)
		assert.Equal(t, got.Out, int64(0))
	}()

	stream, err := cli.Method3(context.Background(), in(3))
	assert.NoError(t, err)
	for i := int64(0); i < 3; i++ {
		got, err := stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, got.Out, i)
	}

	for i := int64(1); i <= 20; i++ {
		wg.Add(1)
		go func(i int64) {
			defer wg.Done()
			got, err := cli.Method1(context.Background(), in(i))
			assert.NoError(t, err)
			assert.Equal(t, got.Out, i)
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
	close(block)
	wg.Wait()
}

func TestPlugin_Refusals(t *testing.T) {
	cfg := drpcplugin.ServeConfig{
		MagicCookieKey:   testCookie,
		MagicCookieValue: "yes",
		ProtocolVersion:  3,
		Register:         func(drpc.Mux) error { return nil },
		Stdout:           &bytes.Buffer{},
	}

	t.Setenv(testCookie, "")
	assert.Equal(t, drpcplugin.Serve(cfg), drpcplugin.ErrNotPlugin)

	t.Setenv(testCookie, "yes")
	t.Setenv(drpcplugin.EnvProtocolVersions, "1,2")
	err := drpcplugin.Serve(cfg)
	assert.Error(t, err)
	assert.That(t, strings.Contains(err.Error(), "protocol versions"))

	t.Setenv(drpcplugin.EnvProtocolVersions, "")
	t.Setenv(drpcplugin.EnvClientCert, "cert")
	err = drpcplugin.Serve(cfg)
	assert.Error(t, err)
	assert.That(t, strings.Contains(err.Error(), "TLS"))
}

func TestPlugin_DeadPlugin(t *testing.T) {
	line, stop := servePlugin(t, impl{
		Method1Fn: func(ctx context.Context, in *In) (*Out, error) { return out(in.In), nil },
	})
	parts := strings.Split(line, "|")
	conn := drpcplugin.Dial(parts[2], parts[3])
	defer func() { _ = conn.Close() }()
	cli := NewDRPCServiceClient(conn)

	_, err := cli.Method1(context.Background(), in(1))
	assert.NoError(t, err)

	stop()
	if parts[2] == "unix" {
		_, statErr := os.Stat(parts[3])
		assert.That(t, os.IsNotExist(statErr))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Method1(ctx, in(1))
	assert.Error(t, err)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
