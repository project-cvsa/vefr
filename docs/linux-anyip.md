# Linux AnyIP checklist

`vefr` assumes that the host can originate a TCP connection with every address it may select. It does not configure the kernel or the network provider.

## Verify the prefix

Use the prefix and routing model required by your provider. A local route is commonly represented by:

```sh
sudo ip -6 route add local 2001:db8:1234::/64 dev lo
ip -6 route show table local
```

Do not copy the documentation prefix above into production. Confirm the prefix is actually announced or routed to the host.

## Verify source binding

```sh
curl --interface 2001:db8:1234::10 https://example.com/
ip -6 route get 2606:4700:4700::1111 from 2001:db8:1234::10
```

If this fails, fix the host networking first. A proxy configuration change cannot repair a missing route, provider filter, or reverse-path check.

## Firewall and filtering

Review host egress rules, cloud security groups, nftables/iptables policy, and reverse-path filtering. Ensure the source range is allowed for outbound traffic and that return traffic can reach the host.

## Service boundaries

Run the proxy as the dedicated `vefr` user. The systemd unit restricts capabilities and filesystem access, but it intentionally does not apply an egress deny policy because that policy is deployment-specific.
