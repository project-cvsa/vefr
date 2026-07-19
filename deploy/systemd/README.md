# systemd deployment

The unit assumes:

- the binary is installed at `/usr/local/bin/vefr`;
- the configuration is installed at `/etc/vefr/config.toml`;
- a locked-down `vefr` system user and group exist;
- the host's IPv6 AnyIP routes are configured before the service starts.

Create the service account and install the artifacts:

```sh
sudo useradd --system --home-dir /var/lib/vefr --create-home --shell /usr/sbin/nologin vefr
sudo install -d -m 0750 -o root -g vefr /etc/vefr
sudo install -m 0755 bin/vefr /usr/local/bin/vefr
sudo install -m 0640 -o root -g vefr config.toml /etc/vefr/config.toml
sudo install -m 0644 deploy/systemd/vefr.service /etc/systemd/system/vefr.service
sudo systemctl daemon-reload
sudo systemctl enable --now vefr
```

Useful commands:

```sh
systemctl status vefr
journalctl -u vefr -f
systemctl restart vefr
```
