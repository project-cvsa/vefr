# Contributing

## Before opening a change

Run the same checks used by CI:

```sh
make check
make race
make build
```

If Docker is available, also run `make docker-build`. Keep generated binaries and local configuration out of commits.

## Change guidelines

- Keep runtime dependencies minimal and document any new dependency.
- Add or update tests for behavior changes.
- Keep security-sensitive defaults restrictive.
- Update `README.md` and the relevant document under `docs/` when operational behavior changes.
- Do not include real credentials, provider prefixes, or host-specific routes in examples.

## Commit and review expectations

Describe the user-visible behavior, security impact, deployment impact, and test commands in the pull request. Changes to proxy policy, source selection, authentication, or deployment hardening need explicit review.
