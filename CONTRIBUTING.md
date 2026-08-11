# Contributing to NexaFlow

Thank you for helping build NexaFlow as a durable open-source business platform.

## Before opening a change

- Discuss large features or architecture changes in an issue first.
- Keep pull requests focused on one roadmap capability.
- Do not bypass Handler → Service → Repository boundaries.
- Include tests for business behavior and regressions.
- Update documentation and the changelog when public behavior changes.

## Development workflow

1. Fork the repository and create a focused branch.
2. Follow `docs/development.md` to install and validate the toolchain.
3. Run `scripts/verify.ps1` before submitting.
4. Explain design trade-offs, tenant/security impact, and migration risk in the
   pull request.

## Commit style

Use concise imperative subjects. Conventional Commit prefixes such as `feat:`,
`fix:`, `docs:`, and `refactor:` are encouraged.

## License

By contributing, you agree that your contributions are licensed under Apache
License 2.0.
