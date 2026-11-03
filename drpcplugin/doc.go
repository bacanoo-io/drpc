// Copyright (C) 2026 Norca contributors
// See LICENSE for copying information.

// Package drpcplugin serves and dials plugins over drpc.
//
// A plugin is a subprocess started by a host. The host passes a magic
// cookie and the protocol versions it speaks in the environment; the plugin
// listens on a local socket and writes one handshake line to its standard
// output:
//
//	CORE-PROTOCOL-VERSION|APP-PROTOCOL-VERSION|NETWORK|ADDRESS|drpc
//
// This is the handshake that hosts built on github.com/hashicorp/go-plugin
// read, with "drpc" as the protocol name, so such a host can start and
// supervise a drpc plugin (its process, logs and reattach) and dial the
// address itself with Dial instead of using its own client protocol.
//
// The package does not depend on go-plugin. It reads the same environment
// variables a go-plugin host sets:
//
//	PLUGIN_PROTOCOL_VERSIONS   app protocol versions the host accepts
//	PLUGIN_UNIX_SOCKET_DIR     directory of the socket (Unix)
//	PLUGIN_UNIX_SOCKET_GROUP   group that may use the socket (Unix)
//	PLUGIN_MIN_PORT, PLUGIN_MAX_PORT   port range on localhost (Windows)
//
// Automatic mutual TLS (PLUGIN_CLIENT_CERT) is not supported: Serve fails
// when a host asks for it, and the host falls back to its own protocol.
package drpcplugin

// Protocol is the protocol name in the handshake line.
const Protocol = "drpc"

// CoreProtocolVersion is the version of the handshake itself.
const CoreProtocolVersion = 1

// Environment variables read by Serve.
const (
	EnvProtocolVersions = "PLUGIN_PROTOCOL_VERSIONS"
	EnvUnixSocketDir    = "PLUGIN_UNIX_SOCKET_DIR"
	EnvUnixSocketGroup  = "PLUGIN_UNIX_SOCKET_GROUP"
	EnvMinPort          = "PLUGIN_MIN_PORT"
	EnvMaxPort          = "PLUGIN_MAX_PORT"
	EnvClientCert       = "PLUGIN_CLIENT_CERT"
)
