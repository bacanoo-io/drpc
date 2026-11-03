// Copyright (C) 2026 Norca contributors
// See LICENSE for copying information.

//go:build windows

package drpcplugin

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// listen listens on the first free port of PLUGIN_MIN_PORT..PLUGIN_MAX_PORT
// (10000..25000 by default) on localhost.
func listen() (net.Listener, error) {
	minPort, maxPort := envPort(EnvMinPort, 10000), envPort(EnvMaxPort, 25000)
	if minPort > maxPort {
		return nil, fmt.Errorf("%s (%d) is above %s (%d)", EnvMinPort, minPort, EnvMaxPort, maxPort)
	}
	for port := minPort; port <= maxPort; port++ {
		lis, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			return lis, nil
		}
	}
	return nil, fmt.Errorf("no free port between %d and %d", minPort, maxPort)
}

func envPort(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}
