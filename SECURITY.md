# Security policy

## Deployment guidance

`vefr` is a network-facing component and can be abused as an open proxy. Deploy it behind network controls, configure credentials, keep destination restrictions enabled, and do not expose the health endpoint on an untrusted interface.

Never commit `config.toml`, production credentials, or provider-specific address allocations.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Contact the repository maintainers privately with:

- a concise description and impact;
- affected versions or commit IDs;
- reproduction steps or a minimal proof of concept;
- any suggested mitigation.

Allow maintainers reasonable time to investigate and release a fix before public disclosure.
