# vefr

`vefr` is a Linux-oriented IPv6 HTTP forward proxy written in Go. It uses source addresses from an explicit IPv6 list or an IPv6 CIDR, which makes it suitable for hosts configured with AnyIP-style routing.

The project is intentionally small at runtime and opinionated at the repository level: it includes reproducible local checks, CI, a minimal container image, a systemd unit, deployment examples, and operational documentation.

## Features

- HTTP forward proxying.
- HTTPS tunneling through `CONNECT`.
- Explicit IPv6 source addresses and lazy IPv6 address generation from CIDRs.
- Random and round-robin source selection.
- Basic Proxy Authentication.
- Destination port allow-listing; ports 80 and 443 are used by default.
- Private, loopback, link-local, multicast, unspecified, and cloud metadata destinations blocked by default.
- `/healthz` endpoint with basic runtime counters.
- Graceful shutdown and configurable connection/request timeouts.
- Docker, Docker Compose, and systemd deployment assets.

## Requirements

- Go 1.22 or newer for local development.
- Linux is recommended for production because source-address behavior depends on host IPv6 routing and AnyIP configuration.
- The configured source addresses must be usable by the host. `vefr` does not configure routes, addresses, firewall rules, or reverse-path filtering.

## Quick start

```sh
cp config.example.toml config.toml
# Edit config.toml and replace the documentation IPv6 addresses.
go run ./cmd/proxy -config config.toml
```

Run the repository checks and build the binary:

```sh
make check
make build
./bin/vefr -config config.toml
```

Test the proxy:

```sh
curl -x http://proxy-user:change-me@127.0.0.1:8080 https://example.com/ -I
curl http://127.0.0.1:8080/healthz
```

The health endpoint is intentionally unauthenticated so it can be used by service managers and container health checks. Keep the listener on a trusted interface or protect it with network policy.

## Configuration

`config.example.toml` is the canonical example. The important fields are:

| Field | Description |
| --- | --- |
| `listen` | Proxy listen address. Defaults to `127.0.0.1:8080`. |
| `username`, `password` | Required proxy credentials. Requests are rejected when credentials are not configured. |
| `source_ips` | Explicit IPv6 source addresses. |
| `source_cidrs` | IPv6 prefixes from which a source address is generated for a new upstream connection. |
| `rotation` | `random` or `round_robin`. |
| `allow_ports` | Destination port allow-list. Defaults to `[80, 443]`. |
| `block_private` | Defaults to `true`; set to `false` only in an isolated development environment. |
| `timeouts` | Durations for connect, header read, idle, and request operations. |

The `2001:db8::/32` addresses in the example are documentation-only addresses and cannot be used for real traffic.

## AnyIP and Linux networking

Before starting the proxy, configure the IPv6 prefix and routing on the host. A typical setup may involve a route such as:

```sh
ip -6 route add local 2001:db8:1234::/64 dev lo
```

The exact route, interface, firewall, provider announcement, and reverse-path filtering configuration is environment-specific. Verify the host can originate traffic with a selected source address before debugging the proxy:

```sh
curl --interface 2001:db8:1234::10 https://example.com/
```

See [docs/linux-anyip.md](docs/linux-anyip.md) for the operational checklist.

## Container deployment

Build the image:

```sh
docker build -t vefr:local .
```

For AnyIP source binding, host networking is normally required. Edit `deploy/docker/config.toml`, then run:

```sh
docker compose -f deploy/docker/compose.yaml up -d
```

The Compose example uses a read-only filesystem, drops Linux capabilities, and mounts only the configuration file. See [docs/operations.md](docs/operations.md).

## systemd deployment

The repository contains a hardened unit at `deploy/systemd/vefr.service`. Install the binary and configuration under `/usr/local/bin` and `/etc/vefr`, create a dedicated `vefr` user, review the unit, then enable it:

```sh
sudo install -m 0755 bin/vefr /usr/local/bin/vefr
sudo install -d -m 0750 -o root -g vefr /etc/vefr
sudo install -m 0640 -o root -g vefr config.toml /etc/vefr/config.toml
sudo install -m 0644 deploy/systemd/vefr.service /etc/systemd/system/vefr.service
sudo systemctl daemon-reload
sudo systemctl enable --now vefr
```

## Development workflow

Common commands are defined in the [Makefile](Makefile):

```sh
make fmt          # format Go sources
make fmt-check    # fail if formatting is needed
make test         # unit tests
make race         # race detector
make vet          # go vet
make check        # formatting, vet, and tests
make build        # bin/vefr
make docker-build # local container image
```

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a change. CI runs formatting, tests, the race detector, vet, and a build on Linux. Go and repository source files use hard tabs with a four-column display width; YAML keeps two-space indentation because tabs are invalid there.

## Security model

This service can become an open proxy if deployed carelessly. Keep authentication enabled, bind to a private interface, restrict destination ports, and apply host-level firewall policy. Do not disable private-address blocking on a shared or internet-facing deployment.

For vulnerability reports, see [SECURITY.md](SECURITY.md).

## Project layout

```text
cmd/proxy/             service entry point
internal/config/       TOML configuration and validation
internal/ippool/       IPv6 source selection
internal/proxy/        HTTP and CONNECT proxy implementation
deploy/docker/         Compose deployment example
deploy/systemd/        systemd unit and installation notes
docs/                  architecture and operational documentation
```

## License

This project is licensed under the [MIT License](LICENSE).
