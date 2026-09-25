# Contributing

## Developer Certificate of Origin

All contributions must be signed off under the [Developer Certificate of Origin](https://developercertificate.org/) (DCO). Use the `-s` flag when committing:

```bash
git commit -s -m "your commit message"
```

## Code owners

See [CODEOWNERS](CODEOWNERS) for the list of maintainers who review pull requests.

## Development workflow

1. Fork the repository and create a feature branch.
2. Follow the build and test instructions in [README.md](README.md).
3. Run `make check` before pushing — this runs `fmt`, `lint`, `license-check`, and `go vet`.
4. Open a pull request against `main`.

## License headers

Every `.go` file must begin with:

```go
// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT
```

The `make check` target runs `license-check`, which verifies the copyright line is present in every `.go` file.

## Security issues

Please report security vulnerabilities via [GitHub private security reporting](https://github.com/linuxfoundation/lfx-v2-invite-service/security/advisories/new) rather than opening a public issue. See [SECURITY.md](SECURITY.md) for details.
