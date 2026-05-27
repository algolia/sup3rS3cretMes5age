# Contributing to sup3rS3cretMes5age

Thanks for your interest in contributing.

This guide follows contribution patterns commonly used across Algolia repositories, adapted for this Go + Vault project.

## Reporting an issue

Opening an issue is one of the best ways to contribute.

Before creating one:

- Search existing issues to avoid duplicates.
- Include your environment details (OS, Go version, Docker version).
- Provide clear reproduction steps.
- Include expected behavior and actual behavior.
- If possible, include a minimal reproducible example.

## Security issues

If you believe you found a security vulnerability, please do not post exploit details publicly first. Follow [SECURITY.md](SECURITY.md) and open a private security report through GitHub Security Advisories for this repository.

## Code contribution process

For any code contribution:

1. Open an issue first for non-trivial changes, behavior changes, or API-affecting updates.
2. Fork and clone the repository.
3. Create a dedicated branch (`fix/<issue-number>` or `feat/<short-description>`).
4. Keep changes focused and small.
5. Add or update tests.
6. Open a pull request against `master`.

Then:

- CI checks will run automatically.
- A maintainer will review your pull request.
- Once checks are green and review is approved, the PR can be merged.

## Commit conventions

Use Conventional Commits:

```text
type(scope): description
```

Common types:

- `fix`: bug fixes
- `feat`: new features
- `refactor`: code changes with no feature or bug fix
- `docs`: documentation only
- `chore`: tooling, CI, maintenance

Examples:

- `fix(vault): delete secret after successful read`
- `feat(handler): support custom ttl parsing`
- `docs(readme): clarify docker compose usage`

If your commit resolves an issue, add a closing keyword in the commit body or PR description, for example `Closes #123`.

## Development setup

Requirements:

- Go 1.26.1+
- Docker
- `curl`
- `jq`

Install dependencies:

```bash
go mod download
```

Build binary:

```bash
go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go
```

## Running locally

### Option 1: Docker Compose (recommended)

```bash
make run
```

App is exposed on `http://localhost:8082`.

### Option 2: Local binary + Vault dev server

Start Vault:

```bash
docker run -d --name vault-dev -p 8200:8200 \
  -e VAULT_DEV_ROOT_TOKEN_ID=supersecret \
  hashicorp/vault:latest
```

Run app:

```bash
VAULT_ADDR=http://localhost:8200 \
VAULT_TOKEN=supersecret \
SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=":8080" \
./sup3rs3cret
```

Cleanup:

```bash
docker stop vault-dev && docker rm vault-dev
```

## Testing and quality checks

Run these before opening a pull request:

```bash
gofmt -s -l .
go vet ./...
golangci-lint run --timeout 300s
make test
```

Expected results:

- `gofmt -s -l .` returns no output.
- `go vet`, `golangci-lint`, and `make test` complete without errors.

## Manual validation

After changes to request handling, Vault integration, or message lifecycle, validate end-to-end behavior:

```bash
TOKEN=$(curl -X POST -s -F 'msg=test secret message' http://localhost:8080/secret | jq -r .token)
curl -s "http://localhost:8080/secret?token=$TOKEN" | jq .
curl -s "http://localhost:8080/secret?token=$TOKEN" | jq .
```

The first read should return the secret, and the second read should fail because the message is self-destructing.

## Pull request checklist

- [ ] Change is scoped and focused.
- [ ] Tests added or updated where needed.
- [ ] `gofmt`, `go vet`, `golangci-lint`, and `make test` pass locally.
- [ ] Docs updated if behavior or configuration changed.
- [ ] PR description explains what changed and why.

## License

By contributing, you agree that your contributions are provided under the repository license (MIT).
