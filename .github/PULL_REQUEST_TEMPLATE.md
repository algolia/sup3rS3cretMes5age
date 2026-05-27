<!--
Thanks for submitting a pull request.
Please provide enough detail so maintainers can review efficiently.
-->

## Summary

<!--
Explain the motivation for this change and what problem it solves.
Link related issues/discussions when relevant.
-->

Closes #

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor
- [ ] Documentation
- [ ] Chore/maintenance
- [ ] Breaking change

## Validation

List the exact commands you ran and their results.

```bash
gofmt -s -l .
go vet ./...
golangci-lint run --timeout 300s
make test
```

Results:

- [ ] `gofmt -s -l .` returns no output
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run --timeout 300s` passes
- [ ] `make test` passes

## Manual checks

If your change affects handlers, Vault integration, or message lifecycle, describe your end-to-end verification (create secret -> first read succeeds -> second read fails).

## Checklist

- [ ] Change is scoped and focused
- [ ] Tests added/updated where needed
- [ ] Docs updated if behavior/configuration changed
- [ ] Commit messages follow Conventional Commits
