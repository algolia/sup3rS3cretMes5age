# sup3rS3cretMes5age Development Instructions

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap and Dependencies
- Install Go 1.25.1+: `go version` must show go1.25.1 or later
- Install Docker: Required for Vault development server
- Install CLI tools for testing:
  ```bash
  # Ubuntu/Debian
  sudo apt-get update && sudo apt-get install -y curl jq
  
  # Check installations
  go version    # Must be 1.25.1+
  docker --version
  curl --version
  jq --version
  ```

### Download Dependencies and Build
- Download Go modules: `go mod download` -- takes 1-2 minutes. NEVER CANCEL. Set timeout to 180+ seconds.
- Build binary: `go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go` -- takes <1 second after dependencies downloaded.
- Install linter: `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.7.2` -- takes 30-60 seconds. Current system has v2.7.2.

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

### Docker Build and Deployment
The project includes comprehensive Docker support:

#### Local Development with Docker Compose
```bash
# Start full stack (Vault + App on port 8082)
make run
# or
docker compose -f deploy/docker-compose.yml up --build -d

# View logs
make logs

# Stop services
make stop

# Clean up
make clean
```

The default `docker-compose.yml` runs the app on port 8082 (HTTP) with Vault using token `supersecret`.

#### Production Docker Image
```bash
# Build multi-platform image with attestations
make image
# Builds for linux/amd64 and linux/arm64 with SBOM and provenance

# Alternative: Build local image only
docker compose -f deploy/docker-compose.yml build
```

**Note**: In some CI/containerized environments, Docker builds may encounter certificate verification issues with Go proxy. If this occurs, use local Go builds instead.

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

### Key Application Features
- **Self-Destructing Messages**: Messages are automatically deleted after first read
- **Vault Backend**: Uses HashiCorp Vault's cubbyhole for secure temporary storage
- **TTL Support**: Configurable time-to-live (default 48h, max 168h/7 days)
- **File Upload**: Support for file uploads with base64 encoding (max 50MB)
- **One-Time Tokens**: Vault tokens with exactly 2 uses (1 to create, 1 to read)
- **Rate Limiting**: 10 requests per second to prevent abuse
- **TLS Support**: Auto TLS via Let's Encrypt or manual certificate configuration
- **No External Dependencies**: All JavaScript/fonts self-hosted for privacy

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
├── cmd/sup3rS3cretMes5age/
│   └── main.go                     # Application entry point (23 lines)
├── internal/                       # Core application logic
│   ├── config.go                   # Configuration handling (77 lines)
│   ├── handlers.go                 # HTTP request handlers (88 lines)
│   ├── handlers_test.go            # Handler unit tests (87 lines)
│   ├── server.go                   # Web server setup (94 lines)
│   ├── vault.go                    # Vault integration (174 lines)
│   └── vault_test.go               # Vault unit tests (66 lines)
├── web/static/                     # Frontend assets (HTML, CSS, JS)
│   ├── index.html                  # Main page (5KB)
│   ├── getmsg.html                 # Message retrieval page (7.8KB)
│   ├── application.css             # Styling (2.3KB)
│   ├── clipboard-2.0.11.min.js     # Copy functionality (9KB)
│   ├── montserrat.css              # Font definitions
│   ├── robots.txt                  # Search engine rules
│   ├── fonts/                      # Self-hosted Montserrat font files
│   └── icons/                      # Favicon and app icons
├── deploy/                         # Docker and deployment configs
│   ├── Dockerfile                  # Multi-stage container build
│   ├── docker-compose.yml          # Local development stack (Vault + App)
│   └── charts/supersecretmessage/  # Helm c(lint + test pipeline)
.codacy.yml        # Code quality config
.dockerignore      # Docker ignore patterns
.git/              # Git repository data
.github/           # GitHub configuration (copilot-instructions.md)
.gitignore         # Git ignore patterns
CLI.md             # Command-line usage guide (313 lines, Bash/Zsh/Fish examples)
CODEOWNERS         # GitHub code owners
LICENSE            # MIT license
Makefile           # Build targets (test, image, build, run, logs, stop, clean)
Makefile.buildx    # Advanced buildx targets (multi-platform, AWS ECR)
README.md          # Main documentation (176 lines)
cmd/               # Application entry points
deploy/            # Deployment configurations (Docker, Helm)
go.mod             # Go module file (go 1.25.1)
go.sum             # Go dependency checksums
internal/          # Internal packages (609 lines total)
web/               # Web assets (static HTML, CSS, JS, fonts, icons)
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

go 1.25.1

require (
    github.com/hashicorp/vault v1.21.0
    github.com/hashicorp/vault/api v1.22.0
    github.com/labstack/echo/v4 v4.13.4
    github.com/stretchr/testify v1.11.1
    golang.org/x/crypto v0.45.0
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
The project uses CircleCI with two jobs:
1. **lint**: Format checking (gofmt), golangci-lint v2.6.0
2. **test**: Unit tests via `make test`

Pipeline runs on Go 1.25 docker image (`cimg/go:1.25`).

### Helm Deployment
Helm chart located in `deploy/charts/supersecretmessage/`:
- Chart version: 0.1.0
- App version: 0.2.5
- Includes: Deployment, Service, Ingress, HPA, ServiceAccount
- Configurable: Vault connection, TLS settings, resource limits
- See [deploy/charts/README.md](deploy/charts/README.md) for details
