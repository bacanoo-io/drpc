// Copyright (C) 2026 Norca contributors
// See LICENSE for copying information.

//go:build !windows

package drpcplugin

import (
	"net"
	"os"
	"os/user"
	"strconv"
)

// listen listens on a new Unix socket in PLUGIN_UNIX_SOCKET_DIR (or the
// temporary directory); with PLUGIN_UNIX_SOCKET_GROUP, members of that
// group may use it too.
func listen() (net.Listener, error) {
	f, err := os.CreateTemp(os.Getenv(EnvUnixSocketDir), "plugin")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	_ = f.Close()
	if err := os.Remove(path); err != nil {
		return nil, err
	}

	lis, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	// Close removes the socket file
	lis.(*net.UnixListener).SetUnlinkOnClose(true)

	if group := os.Getenv(EnvUnixSocketGroup); group != "" {
		gid, err := lookupGroup(group)
		if err == nil {
			err = os.Chown(path, -1, gid)
		}
		if err == nil {
			err = os.Chmod(path, 0o660)
		}
		if err != nil {
			_ = lis.Close()
			return nil, err
		}
	}
	return lis, nil
}

func lookupGroup(group string) (int, error) {
	if gid, err := strconv.Atoi(group); err == nil {
		return gid, nil
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(g.Gid)
}
