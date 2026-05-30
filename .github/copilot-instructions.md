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
- Install JavaScript linter (repo uses flat ESLint config):
  ```bash
  npm install eslint
  ```

### Testing and Validation
- Run unit tests: `make test` (2-3 minutes). NEVER CANCEL. Set timeout to 300+ seconds.
- Run Go linting:
  ```bash
  export PATH="$PATH:$(go env GOPATH)/bin"
  golangci-lint run --timeout 300s
  ```
  Takes 30-45 seconds. NEVER CANCEL. Set timeout to 600+ seconds.
- Run JavaScript linting:
  ```bash
  npx eslint --config eslint.config.js
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
3. Build and run app:
   ```bash
   go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go
   VAULT_ADDR=http://localhost:8200 \
   VAULT_TOKEN=supersecret \
   SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=":8080" \
   ./sup3rs3cret
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
Equivalent direct command:
```bash
docker compose -f deploy/docker-compose.yml up --build -d
```

## Branch-Specific Behavior (ai-multi-lang)

### Frontend and i18n
- Frontend uses vanilla JavaScript modules:
  - `web/static/utils.js`
  - `web/static/index.js`
  - `web/static/getmsg.js`
- Supported languages: `en`, `fr`, `es`, `de`, `it`.
- Translation files are in `web/static/locales/*.json` and loaded dynamically.
- Language selection sources:
  1. URL parameter `?lang=xx`
  2. Browser `Accept-Language`
  3. Fallback to English (`en`)
- Language switching is asynchronous and uses request IDs to avoid race conditions.

### Static Asset Serving and Caching
- Static files are served with cache tiers:
  - `/static/fonts/*`: long immutable cache
  - `/static/icons/*`: long cache
  - `/static/locales/*`: medium cache
  - other `/static/*`: short cache
- HTML pages set `Content-Language` and `Vary: Accept-Language`.
- `getmsg` with token uses `Cache-Control: no-store, private`.
- API/health responses include `Vary: Accept-Encoding`; gzip middleware is enabled globally.

### Security and API Notes
- Rate limiting remains enabled via Echo middleware (10 req/s, burst 20).
- File upload validation includes path traversal checks and 50MB max file size.
- Token validation accepts `hvs.` and `hvb.` formats with strict regex validation.

### Docker Build Pipeline
- `deploy/Dockerfile` is multi-stage:
  - Go builder stage
  - Node web-builder stage that minifies JS/HTML/CSS/locale JSON
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
- `npx eslint --config eslint.config.js`
- `make test`

## Configuration Environment Variables
- `VAULT_ADDR`: Vault server address (example: `http://localhost:8200`)
- `VAULT_TOKEN`: Vault authentication token (example: `supersecret` for dev)
- `SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS`: HTTP bind address (example: `:8080`)
- `SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS`: HTTPS bind address (example: `:443`)
- `SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED`: HTTP -> HTTPS redirect (`true`/`false`)
- `SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN`: domain for Let's Encrypt auto TLS
- `SUPERSECRETMESSAGE_TLS_CERT_FILEPATH`: manual TLS cert path
- `SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH`: manual TLS key path
- `SUPERSECRETMESSAGE_VAULT_PREFIX`: Vault secret prefix (default `cubbyhole/`)

## Repository Structure (Current)
```
.
├── cmd/sup3rS3cretMes5age/main.go
├── internal/
│   ├── config.go
│   ├── handlers.go
│   ├── server.go
│   ├── vault.go
│   └── *_test.go
├── web/static/
│   ├── index.html
│   ├── getmsg.html
│   ├── application.css
│   ├── utils.js
│   ├── index.js
│   ├── getmsg.js
│   ├── locales/
│   │   ├── de.json
│   │   ├── en.json
│   │   ├── es.json
│   │   ├── fr.json
│   │   └── it.json
│   ├── clipboard-2.0.11.min.js
│   ├── fonts/
│   └── icons/
├── deploy/
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── charts/supersecretmessage/
├── eslint.config.js
├── Makefile
├── README.md
└── go.mod
```

## CI Pipeline (CircleCI)
- `lint` job (Go formatter + golangci-lint, Go image `cimg/go:1.26`)
- `jslint` job (Node image `cimg/node:25.8`, runs ESLint)
- `test` job (`make test`, requires `lint`)

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

- `go: ... tls: failed to verify certificate` during containerized build:
  - Use local Go build: `go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go`
- `jq: command not found`:
  - Install with `sudo apt-get install jq` (Linux) or `brew install jq` (macOS).
- Vault connection refused:
  - `docker ps | grep vault`
  - `curl -s http://localhost:8200/v1/sys/health`
  - `docker restart vault-dev`
- Port 8082 in use:
  - `sudo lsof -i :8082`
  - then `make stop`
- If tests emit verbose Vault logs:
  - This is expected for integration-style Vault tests; do not cancel test runs.
