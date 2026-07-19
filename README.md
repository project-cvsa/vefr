# Vefr

## Installation

Run this command on a Linux host with `curl`, `tar`, `sudo`, and `systemd`:

```sh
curl -fsSL https://raw.githubusercontent.com/project-cvsa/vefr/main/scripts/install.sh | bash
```

To install a specific release or use a different configuration path:

```sh
curl -fsSL https://raw.githubusercontent.com/project-cvsa/vefr/main/scripts/install.sh \
  | bash -s -- --version v0.1.0 --config /etc/vefr/config.toml
```

## Configure

Open the configuration file:

```sh
sudoedit /etc/vefr/config.toml
```

Set proxy credentials and at least one usable IPv6 source address or CIDR. The
addresses in `config.example.toml` are documentation addresses; replace them
with addresses routed to this host.

Validate the configuration:

```sh
sudo /usr/local/bin/vefr check --config /etc/vefr/config.toml
```

Before starting Vefr, verify that the host can send traffic from one of the
configured addresses:

```sh
curl --interface 2001:db8:1234::10 https://example.com/
```

Replace the example address with a real address. If this check fails, configure
the IPv6 route, provider announcement, firewall, and reverse-path filtering
first. See [docs/linux-anyip.md](docs/linux-anyip.md).

## Start and stop the service

Start the service at boot and start it now:

```sh
sudo systemctl enable --now vefr
```

Check its status and logs:

```sh
sudo systemctl status vefr
sudo journalctl -u vefr -f
```

Restart or stop it:

```sh
sudo systemctl restart vefr
sudo systemctl stop vefr
```

The service listens on `127.0.0.1:8080` by default. Test it with:

```sh
curl -x http://proxy-user:change-me@127.0.0.1:8080 https://example.com/ -I
curl http://127.0.0.1:8080/healthz
```

## Command line

Run Vefr directly:

```sh
vefr run --config /etc/vefr/config.toml
vefr check --config /etc/vefr/config.toml
vefr version
vefr --help
```

Install or update the systemd unit manually:

```sh
sudo vefr systemd install --config /etc/vefr/config.toml
```

The legacy forms `vefr -config PATH` and `vefr -check -config PATH` are still
accepted.

## Configuration safety

Keep authentication enabled for any listener reachable by other machines. Do
not set `block_private = false` on a shared or internet-facing host. Treat the
configuration file as a secret because it contains proxy credentials.

## Development

Install Go 1.22 or newer, then run:

```sh
make check
make build
make race
```

See [docs/operations.md](docs/operations.md) for operational checks and
backup guidance.

## License

[MIT](LICENSE)
