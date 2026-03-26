# sup3rS3cretMes5age Development Instructions

Always reference these instructions first and fall back to search or bash commands only when information is missing or inconsistent.

## Working Effectively

### Bootstrap and Dependencies
- Install Go 1.26.1+: `go version` must show `go1.26.1` or later.
- Install Docker: required for Vault development server and docker-compose workflows.
- Install Node.js/npm: required for JavaScript linting and Docker web-asset minification stage.
- Install CLI tools for validation:
  ```bash
  # Ubuntu/Debian
  sudo apt-get update && sudo apt-get install -y curl jq

  # Check installations
  go version
  docker --version
  node --version
  npm --version
  curl --version
  jq --version
  ```

### Download Dependencies and Build
- Download Go modules: `go mod download` (1-2 minutes). NEVER CANCEL. Set timeout to 180+ seconds.
- Build local binary: `go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go`.
- Install Go linter:
  ```bash
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
    sh -s -- -b "$(go env GOPATH)/bin" v2.7.2
  ```
- Install JavaScript linter (repo uses flat ESLint config; pinned to match CI):
  ```bash
  npm install --no-save eslint@9.39.2
  ```

### Testing and Validation
- Run unit tests: `make test` (2-3 minutes). NEVER CANCEL. Set timeout to 300+ seconds.
- Run Go linting:
  ```bash
  export PATH="$PATH:$(go env GOPATH)/bin"
  golangci-lint run --timeout 300s
  ```
  Takes ~300 seconds. NEVER CANCEL. Set timeout to 600+ seconds.
- Run JavaScript linting:
  ```bash
  npx eslint --config eslint.config.mjs
  ```
- Check formatting: `gofmt -s -l .` (must return no output).
- Run static analysis: `go vet ./...`.

## Running the Application

### Local Binary + Vault Dev
1. Start Vault dev server:
   ```bash
   docker run -d --name vault-dev -p 8200:8200 \
     -e VAULT_DEV_ROOT_TOKEN_ID=supersecret \
     hashicorp/vault:latest
   ```
2. Wait 3-5 seconds and verify:
   ```bash
   curl -s http://localhost:8200/v1/sys/health
   ```
3. Build and run app (from `web/` so the static/ directory — pages,
   locales, assets — resolves; the server fails to start otherwise):
   ```bash
   go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go
   cd web
   VAULT_ADDR=http://localhost:8200 \
   VAULT_TOKEN=supersecret \
   SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=":8080" \
   ../sup3rs3cret
   ```
4. Cleanup:
   ```bash
   docker stop vault-dev && docker rm vault-dev
   ```

### Docker Compose (Recommended Local Stack)
```bash
make run      # Vault + app (HTTP on :8082)
make logs
make stop
make clean
```
Equivalent direct command (note: prefer `make build` for builds — it passes
VERSION/BUILD_DATE/VCS_REF, which a bare compose build leaves empty in the
image's OCI labels):
```bash
docker compose -f deploy/docker-compose.yml up --build -d
```

## Branch-Specific Behavior (ai-multi-lang)

### Frontend and i18n
- Frontend uses vanilla JavaScript modules:
  - `web/static/utils.js`
  - `web/static/index.js`
  - `web/static/getmsg.js`
- Supported languages are derived from `web/static/locales/*.json` (the single
  source of truth). Adding a language: drop `<code>.json` in that directory and
  regenerate `web/static/locales-manifest.json` — server, client validation and
  the language selector all follow automatically (a Go test enforces manifest/
  directory consistency).
- Translation files are in `web/static/locales/*.json` and loaded dynamically.
- Language selection sources:
  1. URL parameter `?lang=xx`
  2. Browser `Accept-Language`
  3. Fallback to English (`en`)
- Language switching is asynchronous and uses request IDs to avoid race conditions.

### Static Asset Serving and Caching
- Static files are served with cache tiers:
  - `/static/fonts/*`: 7-day cache (no `immutable`; filenames are unversioned)
  - `/static/icons/*`: 24-hour cache
  - `/static/locales/*`: short cache (same 5-minute TTL as HTML pages, so fresh
    HTML never pairs with stale locale JSON after a deploy)
  - other `/static/*`: short cache
- HTML pages set `Content-Language` and `Vary: Accept-Language`.
- `getmsg` with token uses `Cache-Control: no-store, private`.
- The gzip middleware is enabled globally (skipping `Range` requests) and is
  what adds `Vary: Accept-Encoding`; handlers must not set it manually.

### Security and API Notes
- Rate limiting remains enabled via Echo middleware (10 req/s, burst 20).
- File upload validation includes path traversal checks and 50MB max file size.
- Token validation accepts `hvs.` and `hvb.` formats with strict regex validation.

### Docker Build Pipeline
- `deploy/Dockerfile` is multi-stage:
  - Go builder stage (module files copied before sources for layer caching)
  - Node web-builder stage that regenerates `locales-manifest.json` and
    minifies JS/HTML/CSS/locale JSON (pinned via `NODE_MINIFY_VERSION`,
    fails the build on minify errors)
  - Final Alpine runtime image with non-root user
- Image labels include OCI metadata (`version`, `created`, `revision`).

## Validation

### Manual End-to-End Scenarios
1. Basic message flow:
   ```bash
   TOKEN=$(curl -X POST -s -F 'msg=test secret message' http://localhost:8080/secret | jq -r .token)
   curl -s "http://localhost:8080/secret?token=$TOKEN" | jq .
   curl -s "http://localhost:8080/secret?token=$TOKEN" | jq .
   ```
2. CLI-style flow:
   ```bash
   echo "test CLI message" | curl -sF 'msg=<-' http://localhost:8080/secret | \
     jq -r .token | awk '{print "http://localhost:8080/getmsg?token="$1}'
   ```
3. Health check:
   ```bash
   curl -s http://localhost:8080/health
   ```
4. Language behavior quick checks:
   ```bash
   curl -sI 'http://localhost:8080/msg?lang=fr' | grep -i 'Content-Language\|Vary\|Cache-Control'
   curl -sI 'http://localhost:8080/getmsg?token=dummy' | grep -i 'Cache-Control\|Content-Language'
   ```

### Pre-commit Validation

Run all of the following before committing:
- `gofmt -s -l .`
- `go vet ./...`
- `export PATH="$PATH:$(go env GOPATH)/bin" && golangci-lint run --timeout 300s`
- `npx eslint --config eslint.config.mjs`
- `make test`

## PR Code Review Instructions

When performing code reviews on pull requests in this repository, apply the checklists below in addition to the general validation steps. This service's core guarantee is **one-time secret retrieval** — a review must flag anything that weakens it, at any layer (crypto, HTTP semantics, caching, logging).

### Security review checklist

- **One-time token semantics**: Vault one-time tokens use exactly 2 uses (1 create, 1 read) and messages are deleted on retrieval. Flag anything that adds a second read path, extends token uses, or makes retrieval a *get* instead of a *get-and-delete*.
- **Token validation**: retrieval tokens are validated against a strict format regex (`^hv[sb]\.…`) *before* any Vault lookup — the token is concatenated into a Vault path, so the regex is the path-injection guard. Flag regex relaxations, lookups before validation, or tokens accepted from request bodies/headers without validation.
- **Cache-Control on one-time responses**: retrieval responses (`/secret?token=…`, `/getmsg?token=…`) must carry `no-store` (plus `private` where set). A cacheable one-time response lets a second reader get a cached copy after the first read consumed the Vault token.
- **Access logs must not contain one-time tokens**: request logging must redact token-bearing query parameters (any param whose name contains `token` — `token`, `filetoken`). A token in the access log is a second copy of the secret, readable before the first retrieval.
- **Content-Security-Policy**: `style-src` must not include `'unsafe-inline'`. Pages carry no `<style>` elements or `style` attributes — initial hidden state lives in the stylesheet (`.hidden` utility), and scripts reveal elements by assigning `element.style` (a JS property, never restricted by inline-style CSP rules). Flag new inline styles or CSP relaxations.
- **Input validation bounds**: message ≤ 1 MB, TTL parsed duration within 1 min–7 days, file ≤ 50 MB with path-traversal characters (`..`, `/`, `\`) rejected in filenames. Flag removed bounds or validation applied after use.
- **Error hygiene**: client-facing errors must not embed raw Vault errors or echo untrusted input (e.g. token format errors carry a constant message). Flag `err` passed straight into `echo.NewHTTPError(5xx, err)`.
- **Rate limiting and body limits**: 10 RPS/burst 20 per client IP and 50 M body limit must stay in place. Flag skipper additions for authenticated-looking paths (there is no auth — every path is unauthenticated).

### RFC compliance checklist

- **RFC 9110 — Range requests**: gzip must skip requests carrying a `Range` header. A gzipped 206 would pair a gzip body with identity `Content-Range`/`Content-Length`, corrupting any range-capable client's download. Flag gzip-skipper removals and handlers that mutate bodies on 206 responses.
- **RFC 9110 — Accept-Language**: q-values are case-insensitive (`en;Q=0` is valid), restricted to 0–1, out-of-range/malformed weights default to 1.0, region subtags normalize (`fr-CA` → `fr`). Flag parsers that treat `Q=` as invalid or accept q > 1.
- **RFC 9110 — caching semantics**: `Vary` must list every header the response varies on — `Accept-Language` is set by handlers because `Content-Language` depends on it; `Vary: Accept-Encoding` belongs to the gzip middleware. Cache tiers use named constants (HTML/JS/CSS/locales 5 min, icons 24 h, fonts 7 days `must-revalidate`, no `immutable` — filenames are not content-hashed). Flag hardcoded `Vary` values in generic handlers and `immutable` on unhashed filenames.
- **RFC 9110 — Content-Language negotiation**: an unsupported `?lang` value must fall through to the `Accept-Language` header order (not shadow it); the language state (`<html lang>`, selector, `Content-Language`) must always reflect what is actually rendered.

### How other agents should use these reviews

Other agents (Claude Code, or any coding agent with GitHub access) triage Copilot reviews on this repository as follows:

1. **Fetch the review**: `gh api repos/algolia/sup3rS3cretMes5age/pulls/<PR>/reviews` (list) then `/reviews/<review-id>/comments` (inline findings), or the GitHub MCP equivalent. Agent reviews also surface as `pullrequestreview` events.
2. **Verify against code, never the description**: check each finding's `path`/`line` against the actual file content before agreeing — findings routinely cite the right concept with the wrong location, or a condition that no longer exists after later commits.
3. **Triage protocol**: classify each finding as *valid* (implement), *wontfix* (document the rationale in the thread — e.g. the `Object.entries().find()` lookup in `lookupTranslation` is intentional: it avoids computed member access, which scanners flag as a generic-object-injection sink), or *superseded* (already fixed by a later commit). Quantify any performance claim before accepting it (e.g. a 25-key translations object with ~20–40 lookups per interaction is not perf-relevant).
4. **Security findings are never closed silently**: a wontfix on a security or RFC checklist item requires the documented reasoning above to stay in the thread, so the next reviewer (human or agent) sees why.
5. **Regression pins**: every accepted finding on a security/RFC checklist item should land with a test (`internal/*_test.go`) that fails if the behavior regresses — e.g. `TestServerSecurityHeaders` asserts the CSP carries no `'unsafe-inline'`.
6. **Re-request review** after fixes land so the next Copilot pass verifies the thread resolutions.

## Repository Structure (Current)
```
.
├── cmd/sup3rS3cretMes5age/
│   └── main.go                        # Application entry point
├── internal/                          # Core application logic
│   ├── config.go                      # Configuration handling
│   ├── config_test.go                 # Configuration unit tests
│   ├── handlers.go                    # HTTP request handlers (secrets, validation)
│   ├── handlers_test.go               # Handler unit tests
│   ├── server.go                      # Web server setup (middleware, routes, security headers)
│   ├── server_test.go                 # Server unit tests (headers, cache tiers, gzip)
│   ├── vault.go                       # Vault integration (one-time tokens, renewal)
│   └── vault_test.go                  # Vault unit tests
├── web/static/                        # Frontend assets (HTML, CSS, JS)
│   ├── index.html                     # Main page (message creation)
│   ├── getmsg.html                    # Message retrieval page
│   ├── index.js, getmsg.js, utils.js  # Frontend logic (classic scripts; utils.js defines $ helpers)
│   ├── locales/                        # Localization files (e.g., en.json, fr.json)
│   ├── application.css                # Styling (includes .hidden utility for CSP-safe initial state)
│   ├── clipboard-2.0.11.min.js        # Vendored copy functionality (lint-ignored)
│   ├── montserrat.css                 # Font definitions
│   ├── robots.txt                     # Search engine rules
│   ├── fonts/                         # Self-hosted Montserrat font files
│   └── icons/                         # Favicon, app icons and PWA manifest
├── deploy/                            # Docker and deployment configs
│   ├── Dockerfile                     # Multi-stage container build (minify stage, layer caching)
│   ├── docker-compose.yml             # Local development stack (Vault + App)
│   └── charts/supersecretmessage/     # Helm chart for Kubernetes deployment
├── .circleci/config.yml               # CI pipeline (lint, jslint, test)
├── .dockerignore                      # Build-context exclusions (VCS, CI, docs, AI-agent config)
├── eslint.config.mjs                  # ESLint configuration
├── Makefile                           # Build automation
├── README.md                          # Project overview
├── go.mod                             # Go module definition
└── go.sum                             # Go module checksums
```


#### Package.json Equivalent (go.mod)
Direct requirements (verify against `go.mod` for exact versions — they move):
```go
module github.com/algolia/sup3rS3cretMes5age

go 1.26.1

require (
    github.com/hashicorp/vault v1.21.4
    github.com/hashicorp/vault/api v1.23.0
    github.com/labstack/echo/v4 v4.15.4
    github.com/stretchr/testify v1.12.1
    golang.org/x/crypto v0.56.0
)
```

### CLI Functions (from CLI.md)
Add to your shell profile for convenient CLI usage:

```bash
# Basic function for Bash/Zsh
o() { 
    local url="http://localhost:8080"
    local response
    
    if [ $# -eq 0 ]; then
        response=$(curl -sF 'msg=<-' "$url/secret")
    else
        response=$(cat "$@" | curl -sF 'msg=<-' "$url/secret")
    fi
    
    if [ $? -eq 0 ]; then
        echo "$response" | jq -r .token | awk -v url="$url" '{print url"/getmsg?token="$1}'
    else
        echo "Error: Failed to create secure message" >&2
        return 1
    fi
}
```

### Troubleshooting

**"go: ... tls: failed to verify certificate"**
- This may occur in Docker builds in some CI environments
- Solution: Use local Go builds instead: `go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go`

**"jq: command not found"**
```bash
# Ubuntu/Debian
sudo apt-get install jq

# macOS  
brew install jq
```

**"vault connection refused"**
- Ensure Vault dev server is running: `docker ps | grep vault`
- Check Vault health: `curl http://localhost:8200/v1/sys/health`
- Restart if needed: `docker restart vault-dev`

**Test failures with Vault errors**
- Tests create their own Vault instances
- Verbose logging is normal (200+ lines per test)
- NEVER CANCEL tests - they clean up automatically

**Port 8082 already in use**
```bash
# Find what's using the port
sudo lsof -i :8082
# or
sudo netstat -tulpn | grep 8082

# Stop docker-compose if running
make stop
```

**Build fails with "cannot find package"**
```bash
# Clean Go module cache and re-download
go clean -modcache
go mod download
```

### Makefile Targets Reference
```bash
make test          # Run all unit tests (takes 2-3 min)
make image         # Build multi-platform Docker image with attestations
make build         # Build Docker image via docker-compose
make run           # Start docker-compose stack (Vault + App on :8082)
make run-local     # Clean and start docker-compose
make logs          # Tail docker-compose logs
make stop          # Stop docker-compose services
make clean         # Remove docker-compose containers
```

### CircleCI Pipeline
The project uses CircleCI with three jobs:
1. **lint**: Format checking (gofmt), golangci-lint
2. **jslint**: JavaScript linting via pinned ESLint (see `eslint.config.mjs`)
3. **test**: Unit tests via `make test`

Pipeline runs on Go 1.26 docker image (`cimg/go:1.26`) for Go jobs and Node 25 (`cimg/node:25.8`) for jslint.

### Helm Deployment
- Helm chart path: `deploy/charts/supersecretmessage/`.
- Includes: Deployment, Service, Ingress, HPA, ServiceAccount
- Configurable: Vault connection, TLS settings, resource limits
- See [deploy/charts/README.md](../deploy/charts/README.md) for details
- Basic install command:
  ```bash
  helm install supersecret ./deploy/charts/supersecretmessage \
    --set config.vault.address=http://vault.default.svc.cluster.local:8200 \
    --set config.vault.token_secret.name=vault-token
  ```
- Typical updates:
  - `helm upgrade --install supersecret ./deploy/charts/supersecretmessage ...`
  - Adjust ingress, resources, and Vault settings in `values.yaml` or via `--set`.

## Troubleshooting

**"go: ... tls: failed to verify certificate"**
- This may occur in Docker builds in some CI environments
- Solution: Use local Go builds instead: `go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go`

**"jq: command not found"**
```bash
# Ubuntu/Debian
sudo apt-get install jq

# macOS  
brew install jq
```

**"vault connection refused"**
- Ensure Vault dev server is running: `docker ps | grep vault`
- Check Vault health: `curl http://localhost:8200/v1/sys/health`
- Restart if needed: `docker restart vault-dev`

**Test failures with Vault errors**
- Tests create their own Vault instances
- Verbose logging is normal (200+ lines per test)
- NEVER CANCEL tests - they clean up automatically

**Port 8082 already in use**
```bash
# Find what's using the port
sudo lsof -i :8082
# or
sudo netstat -tulpn | grep 8082

# Stop docker-compose if running
make stop
```

**Build fails with "cannot find package"**
```bash
# Clean Go module cache and re-download
go clean -modcache
go mod download
```

### Makefile Targets Reference
```bash
make test          # Run all unit tests (takes 2-3 min)
make image         # Build multi-platform Docker image with attestations
make build         # Build Docker image via docker-compose
make run           # Start docker-compose stack (Vault + App on :8082)
make run-local     # Clean and start docker-compose
make logs          # Tail docker-compose logs
make stop          # Stop docker-compose services
make clean         # Remove docker-compose containers
```

## CI Pipeline (CircleCI)
- `lint` job (Go formatter + golangci-lint, Go image `cimg/go:1.26`)
- `jslint` job (Node image `cimg/node:25.8`, runs ESLint 9.39.2 — pinned, cached)
- `test` job (`make test`, requires `lint`)
- `deploy-check` job (helm lint 3.16.4, hadolint 2.12.0, docker compose)
