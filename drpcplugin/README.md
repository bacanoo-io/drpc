# package drpcplugin

`import "storj.io/drpc/drpcplugin"`

Package drpcplugin serves and dials plugins over drpc.

A plugin is a subprocess started by a host. The host passes a magic cookie and
the protocol versions it speaks in the environment; the plugin listens on a
local socket and writes one handshake line to its standard output:

    CORE-PROTOCOL-VERSION|APP-PROTOCOL-VERSION|NETWORK|ADDRESS|drpc

This is the handshake that hosts built on `github.com/hashicorp/go-plugin` read,
with `drpc` as the protocol name, so such a host can start and supervise a drpc
plugin (its process, logs and reattach) and dial the address itself with `Dial`
instead of using its own client protocol. The package does not depend on
go-plugin.

## Usage

Plugin side:

```go
err := drpcplugin.Serve(drpcplugin.ServeConfig{
	MagicCookieKey:   "MY_PLUGIN_COOKIE",
	MagicCookieValue: "...",
	ProtocolVersion:  1,
	Register: func(m drpc.Mux) error {
		return pb.DRPCRegisterLogger(m, impl)
	},
})
```

Host side, after reading the handshake line (or a stored reattach address):

```go
conn := drpcplugin.Dial(network, address)
defer conn.Close()
client := pb.NewDRPCLoggerClient(conn)
```

`Conn` keeps a small pool of drpc connections, because one drpc connection
carries one RPC at a time: a long call (for example a wait for a process to
exit) and open streams do not block other calls.

## Environment

| Variable | Meaning |
| --- | --- |
| `PLUGIN_PROTOCOL_VERSIONS` | app protocol versions the host accepts (comma separated) |
| `PLUGIN_UNIX_SOCKET_DIR` | directory of the socket (Unix) |
| `PLUGIN_UNIX_SOCKET_GROUP` | group that may use the socket (Unix) |
| `PLUGIN_MIN_PORT`, `PLUGIN_MAX_PORT` | port range on localhost (Windows) |

Automatic mutual TLS (`PLUGIN_CLIENT_CERT`) is not supported: `Serve` fails
when a host asks for it.
