# sup3rS3cretMes5age Development Instructions

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap and Dependencies
- Install Go 1.24+: `go version` must show go1.24 or later
- Install Docker: Required for Vault development server
- Install CLI tools for testing:
  ```bash
  # Ubuntu/Debian
  sudo apt-get update && sudo apt-get install -y curl jq
  
  # Check installations
  go version    # Must be 1.24+
  docker --version
  curl --version
  jq --version
  ```

### Download Dependencies and Build
- Download Go modules: `go mod download` -- takes 1-2 minutes. NEVER CANCEL. Set timeout to 180+ seconds.
- Build binary: `go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go` -- takes <1 second after dependencies downloaded.
- Install linter: `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.64.8` -- takes 30-60 seconds.

### Testing and Validation
- Run tests: `make test` -- takes 2-3 minutes. NEVER CANCEL. Set timeout to 300+ seconds.
- Run linting: `export PATH=$PATH:$(go env GOPATH)/bin && golangci-lint run --timeout 300s` -- takes 30-45 seconds. NEVER CANCEL. Set timeout to 600+ seconds.
- Check formatting: `gofmt -s -l .` -- should return no output if properly formatted
- Run static analysis: `go vet ./...` -- takes <5 seconds

### Running the Application
**ALWAYS run the bootstrapping steps first before starting the application.**

#### Start Development Vault Server
```bash
docker run -d --name vault-dev -p 8200:8200 -e VAULT_DEV_ROOT_TOKEN_ID=supersecret hashicorp/vault:latest
```
Wait 3-5 seconds for Vault to start, then verify: `curl -s http://localhost:8200/v1/sys/health`

#### Start the Application
```bash
VAULT_ADDR=http://localhost:8200 VAULT_TOKEN=supersecret SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=":8080" ./sup3rs3cret
```

The application will start on port 8080. Access at http://localhost:8080

#### Cleanup Development Environment
```bash
docker stop vault-dev && docker rm vault-dev
```

### Docker Build Issues
**IMPORTANT**: Docker builds currently fail in CI/containerized environments due to certificate verification issues with Go proxy:
```
go: cloud.google.com/go@v0.112.1: Get "https://proxy.golang.org/...": tls: failed to verify certificate: x509: certificate signed by unknown authority
```

Do NOT attempt Docker builds (`make build`, `make image`, `docker compose up --build`) in sandboxed environments. These commands will fail after 15-30 seconds. Use local Go builds instead.

If you need to test Docker functionality, run individual commands:
- `make build` -- WILL FAIL in CI. Takes 15-30 seconds to fail.
- `make image` -- WILL FAIL in CI. Takes 15-30 seconds to fail.

## Validation

### Manual Testing Scenarios
ALWAYS run through these complete end-to-end scenarios after making changes:

#### Test 1: Basic Message Flow
```bash
# Create secret message
TOKEN=$(curl -X POST -s -F 'msg=test secret message' http://localhost:8080/secret | jq -r .token)

# Retrieve message (should work once)
curl -s "http://localhost:8080/secret?token=$TOKEN" | jq .

# Try to retrieve again (should fail - message self-destructs)
curl -s "http://localhost:8080/secret?token=$TOKEN" | jq .
```

#### Test 2: CLI Integration
```bash
# Test CLI workflow
echo "test CLI message" | curl -sF 'msg=<-' http://localhost:8080/secret | jq -r .token | awk '{print "http://localhost:8080/getmsg?token="$1}'
```

#### Test 3: Health Check
```bash
curl -s http://localhost:8080/health  # Should return "OK"
```

### Pre-commit Validation
Always run these commands before committing:
- `gofmt -s -l .` -- Should return no output
- `go vet ./...` -- Should complete without errors  
- `export PATH=$PATH:$(go env GOPATH)/bin && golangci-lint run --timeout 300s` -- Should complete without errors. NEVER CANCEL. Set timeout to 600+ seconds.
- `make test` -- Should pass all tests. NEVER CANCEL. Set timeout to 300+ seconds.

## Common Tasks

### Configuration Environment Variables
- `VAULT_ADDR`: Vault server address (e.g., `http://localhost:8200`)
- `VAULT_TOKEN`: Vault authentication token (e.g., `supersecret` for dev)
- `SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS`: HTTP port (e.g., `:8080`)
- `SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS`: HTTPS port (e.g., `:443`)
- `SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED`: Enable HTTP->HTTPS redirect (`true`/`false`)
- `SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN`: Domain for Let's Encrypt auto-TLS
- `SUPERSECRETMESSAGE_TLS_CERT_FILEPATH`: Manual TLS certificate path
- `SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH`: Manual TLS certificate key path
- `SUPERSECRETMESSAGE_VAULT_PREFIX`: Vault path prefix (default: `cubbyhole/`)

### Repository Structure
```
.
├── cmd/sup3rS3cretMes5age/main.go  # Application entry point
├── internal/                       # Core application logic
│   ├── config.go                   # Configuration handling
│   ├── handlers.go                 # HTTP request handlers  
│   ├── server.go                   # Web server setup
│   └── vault.go                    # Vault integration
├── web/static/                     # Frontend assets (HTML, CSS, JS)
├── deploy/                         # Docker and deployment configs
│   ├── Dockerfile                  # Container build (fails in CI)
│   ├── docker-compose.yml          # Local development stack
│   └── charts/                     # Helm charts for Kubernetes
├── Makefile                        # Build automation
├── go.mod                          # Go module definition
└── README.md                       # Project documentation
```

### Frequently Used Commands Output

#### Repository Root Files
```bash
$ ls -la
.circleci/         # CircleCI configuration  
.codacy.yml        # Code quality config
.dockerignore      # Docker ignore patterns
.git/              # Git repository data
.gitignore         # Git ignore patterns
CLI.md             # Command-line usage guide
CODEOWNERS         # GitHub code owners
LICENSE            # MIT license
Makefile           # Build targets
README.md          # Main documentation
cmd/               # Application entry points
deploy/            # Deployment configurations
go.mod             # Go module file
go.sum             # Go checksum file
internal/          # Internal packages
web/               # Web assets
```

#### Package.json Equivalent (go.mod)
```go
module github.com/algolia/sup3rS3cretMes5age

go 1.24

require (
    github.com/hashicorp/vault v1.16.3
    github.com/hashicorp/vault/api v1.14.0
    github.com/labstack/echo/v4 v4.13.4
    github.com/stretchr/testify v1.10.0
    golang.org/x/crypto v0.40.0
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
- This occurs in Docker builds in CI environments
- Use local Go builds instead: `go build cmd/sup3rS3cretMes5age/main.go`

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