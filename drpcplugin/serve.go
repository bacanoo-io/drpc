// Copyright (C) 2026 Norca contributors
// See LICENSE for copying information.

package drpcplugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"storj.io/drpc"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
)

// ErrNotPlugin is returned by Serve when the magic cookie is missing: the
// binary was started directly, not by a host.
var ErrNotPlugin = errors.New("drpcplugin: not started as a plugin " +
	"(the magic cookie is missing); this binary is meant to be executed by its host")

// ServeConfig configures Serve.
type ServeConfig struct {
	// MagicCookieKey and MagicCookieValue must be set in the environment
	// by the host.
	MagicCookieKey   string
	MagicCookieValue string

	// ProtocolVersion is the app protocol version of the plugin.
	ProtocolVersion int

	// Register registers the services of the plugin.
	Register func(drpc.Mux) error

	// Context ends serving when it is done; nil serves until the process
	// is killed.
	Context context.Context

	// Stdout receives the handshake line; nil means os.Stdout.
	Stdout io.Writer

	// Listener replaces the socket chosen from the environment (tests).
	Listener net.Listener
}

// Serve checks the environment set by the host, listens, writes the
// handshake line and serves the registered services until the context is
// done. Interrupt signals are ignored: the host stops the plugin.
func Serve(cfg ServeConfig) error {
	if cfg.MagicCookieKey == "" || os.Getenv(cfg.MagicCookieKey) != cfg.MagicCookieValue {
		return ErrNotPlugin
	}
	if os.Getenv(EnvClientCert) != "" {
		return errors.New("drpcplugin: automatic mutual TLS is not supported")
	}
	if err := checkVersion(os.Getenv(EnvProtocolVersions), cfg.ProtocolVersion); err != nil {
		return err
	}
	if cfg.Register == nil {
		return errors.New("drpcplugin: no Register function")
	}

	mux := drpcmux.New()
	if err := cfg.Register(mux); err != nil {
		return fmt.Errorf("drpcplugin: registering services: %w", err)
	}

	lis := cfg.Listener
	if lis == nil {
		var err error
		if lis, err = listen(); err != nil {
			return fmt.Errorf("drpcplugin: listening: %w", err)
		}
	}
	defer func() { _ = lis.Close() }()

	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// The host gets the interrupts of a terminal too; it stops the plugin.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		for range sigCh {
		}
	}()

	out := cfg.Stdout
	if out == nil {
		out = os.Stdout
	}
	if _, err := fmt.Fprintf(out, "%s\n", Handshake(cfg.ProtocolVersion, lis.Addr())); err != nil {
		return fmt.Errorf("drpcplugin: writing the handshake: %w", err)
	}

	err := drpcserver.New(mux).Serve(ctx, lis)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// Handshake returns the handshake line (without the newline) for a plugin
// listening on addr.
func Handshake(version int, addr net.Addr) string {
	return fmt.Sprintf("%d|%d|%s|%s|%s", CoreProtocolVersion, version, addr.Network(), addr.String(), Protocol)
}

// checkVersion accepts version when the host lists it, or when the host
// lists no versions.
func checkVersion(list string, version int) error {
	if strings.TrimSpace(list) == "" {
		return nil
	}
	for _, s := range strings.Split(list, ",") {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v == version {
			return nil
		}
	}
	return fmt.Errorf("drpcplugin: the host accepts protocol versions %s, the plugin speaks %d", list, version)
}
