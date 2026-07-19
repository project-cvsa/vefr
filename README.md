# Vefr

![vefr](.github/banner.png)

**Vefr** helps you convert IPv6 addresses assigned to your host into a single HTTP proxy.

## Install

The one-line installer supports Linux `x86_64` and `arm64`. It downloads the
matching GitHub Release archive, verifies its SHA-256 checksum, installs the
binary, preserves an existing configuration, and asks the binary to register
its own systemd unit.

```sh
curl -fsSL https://raw.githubusercontent.com/project-cvsa/vefr/main/scripts/install.sh | bash
```

On a new host, the command installs the example configuration but leaves the
service stopped until it is configured. Edit the configuration, then validate
and start the service:

```sh
sudoedit /etc/vefr/config.toml
sudo /usr/local/bin/vefr check --config /etc/vefr/config.toml
sudo systemctl enable --now vefr
```

Set proxy credentials and configure `source_ips` or `source_cidrs` with IPv6
addresses routed to the host. Verify one of those addresses before installing:

```sh
curl --interface 2001:db8:1234::10 https://example.com/
```

Replace the documentation address with a real one. The example configuration
is in [config.example.toml](config.example.toml).

Pin a release or install an existing configuration during the same operation:

```sh
curl -fsSL https://raw.githubusercontent.com/project-cvsa/vefr/main/scripts/install.sh \
  | bash -s -- --version v1.2.3 --config ./config.toml
```

The installer does not configure IPv6 routes, provider announcements,
firewalls, or reverse-path filtering. See [docs/linux-anyip.md](docs/linux-anyip.md)
if the source-address check fails.

## CLI

After installation, the same binary can be run directly or managed by
systemd:

```sh
vefr version
vefr check --config /etc/vefr/config.toml
vefr run --config /etc/vefr/config.toml

sudo vefr systemd install --config /etc/vefr/config.toml

sudo systemctl status vefr
sudo systemctl restart vefr
```

Running `vefr` without a subcommand starts the proxy. The legacy forms
`vefr -config ...` and `vefr -check -config ...` remain supported. The
`systemd install` command requires root and only installs/enables the unit; it
does not start the service unless `systemctl enable --now` is run separately.

## Authentication

`username` and `password` are proxy-client credentials, not the Linux
`vefr` service account. Authentication is enabled by default to prevent an
accidental open proxy.

For a proxy bound only to localhost or an otherwise trusted, isolated network,
authentication can be disabled explicitly:

```toml
auth_enabled = false
```

Do not disable it on a shared or internet-facing listener. The systemd service
account remains enabled either way so the process does not run as root.

## Development

Requires Go 1.22+.

```sh
make check
make build
make race
```

See [docs/linux-anyip.md](docs/linux-anyip.md) for IPv6 setup and
[docs/operations.md](docs/operations.md) for operations.

## License

[MIT](LICENSE)
