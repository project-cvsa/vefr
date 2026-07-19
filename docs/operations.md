# Operations guide

## Pre-flight checklist

1. Confirm the host has a working IPv6 default route.
2. Configure AnyIP/local routes for the selected prefix.
3. Verify one explicit source address with `curl --interface`.
4. Replace all example credentials and documentation addresses.
5. Keep the listener on localhost or a private management network.
6. Restrict destination ports and keep `block_private` enabled.
7. Run `make check` and `make build` before deployment.

## Health and logs

`GET /healthz` returns a small JSON document containing service status, total requests, active requests, and configured source-pool size. The endpoint does not prove that an upstream destination is reachable; use a separate synthetic probe for end-to-end monitoring.

The service writes structured text logs to standard output. Under systemd, inspect them with `journalctl -u vefr`. In containers, collect stdout/stderr through the container runtime.

## Failure modes

### All upstream connections fail

Check the selected source address, local route, provider announcement, firewall egress rules, and reverse-path filtering. Test the same source with `curl --interface` outside the proxy.

### Requests are rejected with 403

Check the destination port and whether DNS resolves to a private, loopback, link-local, multicast, unspecified, or metadata address. Do not disable the protection unless the environment is isolated and the destination is known to be safe.

### The service refuses to start

Validate the TOML file, credentials, IPv6 addresses, CIDRs, ports, rotation mode, and timeout strings. Run the binary directly with the same `-config` path to see the startup error.

## Upgrades

Build and verify a new binary, replace it atomically, then restart the service. Keep the previous binary available for rollback. For container deployments, tag images immutably in production rather than relying only on `latest`.

## Backup and secrets

The TOML configuration contains proxy credentials and must be treated as a secret. Use mode `0640` with a dedicated group for systemd deployments, avoid committing `config.toml`, and rotate credentials after accidental exposure.
