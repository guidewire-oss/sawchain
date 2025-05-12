# Contributing Guidelines

Thank you for considering contributing to this project! We welcome contributions of all kinds, including code, documentation, bug reports, and feature requests.

## Getting Started

1. Check open issues or discussions to avoid duplication.
1. Open a new issue if you plan to:
   - Report a bug
   - Request a feature
   - Discuss a design change
1. Fork the repository and create a branch for your contribution.

## Development Setup

To get started with local development:

```bash
make init    # Install dependencies
make test    # Run tests
make debug   # Debug tests (requires Delve and VSCode)
```

## Test, Lint, Format

- Ensure all tests pass with `make test`.
- Keep tests clear, focused, and consistent with existing examples.
- Lint and format code with `go vet ./...` and `go fmt ./...`.

## Code Style

- Use idiomatic Go.
- Keep functions short and focused.
- Follow existing patterns for structure, naming, and behavior.
- Avoid over-abstraction unless it improves clarity or testability.

## Commit Messages

Use clear, descriptive commit messages. For example:

```txt
fix: handle nil pointer in FetchSingle
feat: add support for YAML templating in resource creation
docs: clarify usage of CheckFunc
```

Following [Conventional Commits](https://www.conventionalcommits.org/) is recommended.

## Pull Requests

- Reference any related issues in your PR description.
- Include relevant tests and/or documentation updates.
- Ensure your PR is scoped to a single fix or feature.
- When updating public function documentation, regenerate [`api-reference.md`](./docs/api-reference.md) using `make docs`.

## Code of Conduct

This project follows the [Contributor Covenant](https://www.contributor-covenant.org/) Code of Conduct. Please be respectful, inclusive, and constructive.

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License, as described in [`LICENSE`](./LICENSE).
