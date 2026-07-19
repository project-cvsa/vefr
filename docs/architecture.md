# Architecture

`vefr` is a single-process forward proxy. It deliberately keeps the runtime dependency graph small and uses the Go standard library for HTTP, TCP, configuration parsing, logging, and graceful shutdown.

```text
client
  |
  v
HTTP server -- authentication and target policy
  |
  +--> ordinary HTTP request --> http.Transport
  |
  +--> CONNECT tunnel --------> TCP relay
                                  |
                                  v
                         net.Dialer with IPv6 LocalAddr
                                  |
                                  v
                             destination
```

## Source address selection

The source pool accepts explicit IPv6 addresses and IPv6 prefixes. An explicit address is returned as-is. A prefix is sampled lazily by randomizing only its host bits. The selected address is passed to `net.Dialer` as the local TCP address.

The process does not add addresses or routes. The host, provider, and firewall must already permit the source addresses.

## Request lifecycle

Each ordinary HTTP request uses a new upstream connection so source rotation is observable per request. A `CONNECT` request keeps one selected source address for the lifetime of its bidirectional tunnel.

Target validation happens before dialing. Ports are checked against the configured allow-list, and DNS results are checked against the private/local destination policy by default.

## Trust boundaries

- The client-to-proxy connection is not encrypted by the proxy itself. Use a private listener, a local tunnel, or a trusted network.
- Basic credentials are only suitable over a trusted transport.
- The proxy is not a general-purpose firewall. Host-level egress policy remains necessary.
- `block_private: false` is a development escape hatch, not a production setting.
