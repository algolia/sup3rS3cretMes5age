# sup3rS3cretMes5age

[![Go Version](https://img.shields.io/github/go-mod/go-version/algolia/sup3rS3cretMes5age.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CircleCI](https://img.shields.io/circleci/build/github/algolia/sup3rS3cretMes5age/master)](https://circleci.com/gh/algolia/sup3rS3cretMes5age)
[![Go Report Card](https://goreportcard.com/badge/github.com/algolia/sup3rS3cretMes5age)](https://goreportcard.com/report/github.com/algolia/sup3rS3cretMes5age)  

[![Awesome F/OSS](https://awsmfoss.com/content/images/2024/02/awsm-foss-badge.600x128.rounded.png)](https://awsmfoss.com/sup3rs3cretmes5age)

A simple, secure, **self-destructing message service** that uses HashiCorp Vault as a backend for temporary secret storage. Share sensitive information with confidence knowing it will be automatically deleted after being read once.

![self-destruct](https://media.giphy.com/media/LBlyAAFJ71eMw/giphy.gif)

> 🔐 **Security First**: Messages are stored in Vault's cubbyhole backend with one-time tokens and automatic expiration.

Read more about the reasoning behind this project in the [relevant blog post](https://blog.algolia.com/secure-tool-for-one-time-self-destructing-messages/).

## ✨ Features

- **🔥 Self-Destructing Messages**: Messages are automatically deleted after first read
- **⏰ Configurable TTL**: Set custom expiration times (default 48h, max 7 days)
- **📎 File Upload Support**: Share files up to 50MB with base64 encoding
- **🔐 Vault-Backed Security**: Uses HashiCorp Vault's cubbyhole for tamper-proof storage
- **🎫 One-Time Tokens**: Vault tokens with exactly 2 uses (create + retrieve)
- **🚦 Rate Limiting**: Built-in protection (10 requests/second)
- **🌍 Multi-Language Support**: Interface available in 5 languages (EN, FR, DE, ES, IT)
  - Automatic language detection from browser preferences
  - URL-based language selection (`?lang=fr`)
  - Dynamic switching without page reload
- **🔒 TLS/HTTPS Support**: 
  - Automatic TLS via [Let's Encrypt](https://letsencrypt.org/)
  - Manual certificate configuration
  - HTTP to HTTPS redirection
- **🌐 No External Dependencies**: All assets self-hosted for privacy
- **📦 Lightweight**: Only 8.9KB JavaScript (no jQuery)
- **🐳 Docker Ready**: Multi-platform images (amd64, arm64) with SBOM
- **☸️ Kubernetes Support**: Helm chart included
- **🖥️ CLI Integration**: Shell functions for Bash, Zsh, and Fish

## 📋 Table of Contents

- [Features](#-features)
- [Frontend Dependencies](#frontend-dependencies)
- [Quick Start](#-quick-start)
- [Deployment](#deployment)
- [Configuration](#configuration-options)
- [Multi-Language Support](#-multi-language-support)
- [Command Line Usage](#command-line-usage)
- [Helm Chart](#helm)
- [API Reference](#-api-reference)
- [Development](#-development)
- [Contributing](#contributing)
- [License](#license)

## Frontend Dependencies

The web interface is built with modern **vanilla JavaScript** (ES6 modules) and has minimal external dependencies:

| Dependency | Size | Purpose |
|------------|------|----------|
| ClipboardJS v2.0.11 | 8.9KB | Copy to clipboard functionality |
| Montserrat Font | 46KB | Self-hosted typography |
| Custom CSS | 3.3KB | Application styling (minified) |
| Translation files | ~1KB each | i18n support (loaded on-demand) |

✅ **No external CDNs or tracking** - All dependencies are self-hosted for privacy and security.

📦 **Total JavaScript bundle size**: 8.9KB (previously 98KB with jQuery)

🌍 **Internationalization**: 5 languages supported (English, French, German, Spanish, Italian)
- Translations loaded asynchronously on-demand
- Browser language auto-detection
- Seamless language switching without page reload

## 🚀 Quick Start

Get up and running in less than 2 minutes:

```bash
# Clone the repository
git clone https://github.com/algolia/sup3rS3cretMes5age.git
cd sup3rS3cretMes5age

# Start with Docker Compose (recommended)
make run

# Access the application
open http://localhost:8082
```

The service will start with:
- **Application**: http://localhost:8082
- **Vault dev server**: In-memory storage with token `supersecret`

### Alternative: Local Build

```bash
# Start Vault dev server
docker run -d --name vault-dev -p 8200:8200 \
  -e VAULT_DEV_ROOT_TOKEN_ID=supersecret \
  hashicorp/vault:latest

# Build and run the application
go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go
VAULT_ADDR=http://localhost:8200 \
VAULT_TOKEN=supersecret \
SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=":8080" \
./sup3rs3cret
```

## Deployment

### Local Development

#### Using Make (Recommended)

```bash
make run         # Start services (Vault + App)
make logs        # View logs
make stop        # Stop services
make clean       # Remove containers
```

#### Using Docker Compose Directly

```bash
docker compose -f deploy/docker-compose.yml up --build -d
```

By default, the application runs on **port 8082** in HTTP mode: [http://localhost:8082](http://localhost:8082)

💡 You can modify `deploy/docker-compose.yml` to enable HTTPS, HTTP redirection, or change ports. See [Configuration options](#configuration-options).

### Production Deployment

The image is available at:
- **Docker Hub**: `algolia/supersecretmessage:latest`
- **Platforms**: linux/amd64, linux/arm64

#### Docker Image

Build multi-platform images with SBOM and provenance attestations:

```bash
# Build for multiple architectures
make image
# Builds: linux/amd64, linux/arm64 with SBOM and provenance
```

#### AWS Deployment

For detailed step-by-step instructions on deploying to AWS, see our comprehensive [AWS Deployment Guide](AWS_DEPLOYMENT.md). The guide covers:

- **ECS with Fargate** (recommended) - Serverless containers with Application Load Balancer
- **EKS (Kubernetes)** - Using the provided Helm chart on Amazon EKS  
- **EC2 with Docker** - Simple deployment using Docker Compose

```bash
# Build for multiple architectures
make image
# Builds: linux/amd64, linux/arm64 with SBOM and provenance
```

#### Deployment Platforms

Deploy using your preferred orchestration tool:

| Platform | Documentation |
|----------|---------------|
| Kubernetes | See [Helm Chart](#helm) below |
| Docker Swarm | Use the provided `docker-compose.yml` |
| AWS ECS | Use the Docker image with ECS task definition |

**Important**: Deploy alongside a production Vault server. Configure via environment variables:
- `VAULT_ADDR`: Your Vault server URL
- `VAULT_TOKEN`: Vault authentication token

See [configuration examples](#configuration-examples) below.

### 🔒 Security Notice

> ⚠️ **Critical**: Always run this service behind SSL/TLS in production. Secrets sent over HTTP are vulnerable to interception!

#### TLS Termination Options

**Option 1: Inside the Container** (Recommended for simplicity)
- Configure via environment variables
- Automatic Let's Encrypt certificates
- See [Configuration examples - TLS](#tls)

**Option 2: External Load Balancer/Reverse Proxy**
- Simpler certificate management
- Offload TLS processing
- **Ensure secure network** between proxy and container
- Examples: AWS ALB, Nginx, Traefik, Cloudflare

#### Security Best Practices

- ✅ Use HTTPS/TLS in production
- ✅ Use a production Vault server (not dev mode)
- ✅ Rotate Vault tokens regularly
- ✅ Enable rate limiting (built-in: 10 req/s)
- ✅ Monitor Vault audit logs
- ✅ Use strong Vault policies
- ✅ Keep dependencies updated

## Helm

Deploy to Kubernetes using the included Helm chart:

```bash
helm install supersecret ./deploy/charts/supersecretmessage \
  --set config.vault.address=http://vault.default.svc.cluster.local:8200 \
  --set config.vault.token_secret.name=vault-token
```

**Chart Details**:
- Chart Version: 0.1.0
- App Version: 0.2.5
- Includes: Deployment, Service, Ingress, HPA, ServiceAccount

For full documentation, see the [Helm Chart README](deploy/charts/README.md)

## 📡 API Reference

### Create Secret Message

**Endpoint**: `POST /secret`

**Content-Type**: `multipart/form-data`

**Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `msg` | string | Yes | The secret message content |
| `ttl` | string | No | Time-to-live (default: 48h, max: 168h) |
| `file` | file | No | File to upload (max 50MB) |

**Response**:
```json
{
  "token": "s.abc123def456",
  "filetoken": "s.xyz789uvw012",  // If file uploaded
  "filename": "secret.pdf"        // If file uploaded
}
```

**Example**:
```bash
# Text message
curl -X POST -F 'msg=This is a secret' http://localhost:8082/secret

# With custom TTL
curl -X POST -F 'msg=Short-lived secret' -F 'ttl=1h' http://localhost:8082/secret

# With file
curl -X POST -F 'msg=Check this file' -F 'file=@secret.pdf' http://localhost:8082/secret
```

### Retrieve Secret Message

**Endpoint**: `GET /secret?token=<token>`

**Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `token` | string | Yes | The token from POST response |

**Response**:
```json
{
  "msg": "This is a secret"
}
```

**Example**:
```bash
curl "http://localhost:8082/secret?token=s.abc123def456"
```

⚠️ **Note**: After retrieval, the message and token are permanently deleted. Second attempts will fail.

### Health Check

**Endpoint**: `GET /health`

**Response**: `OK` (HTTP 200)

## Command Line Usage

For convenient command line integration and automation, see our comprehensive [CLI Guide](CLI.md) which includes shell functions for Bash, Zsh, Fish, and WSL.

Quick example:
```bash
# Add to your ~/.bashrc or ~/.zshrc
o() { cat "$@" | curl -sF 'msg=<-' https://your-domain.com/secret | jq -r .token | awk '{print "https://your-domain.com/getmsg?token="$1}'; }

# Usage
echo "secret message" | o
o secret-file.txt
```

## Configuration options

* `VAULT_ADDR`: address of the Vault server used for storing the temporary secrets.
* `VAULT_TOKEN`: Vault token used to authenticate to the Vault server.
* `SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS`: HTTP binding address (e.g. `:80`).
* `SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS`: HTTPS binding address (e.g. `:443`).
* `SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED`: whether to enable HTTPS redirection or not (e.g. `true`).
* `SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN`: domain to use for "Auto" TLS, i.e. automatic generation of certificate with Let's Encrypt. See [Configuration examples - TLS - Auto TLS](#auto-tls).
* `SUPERSECRETMESSAGE_TLS_CERT_FILEPATH`: certificate filepath to use for "manual" TLS.
* `SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH`: certificate key filepath to use for "manual" TLS.
* `SUPERSECRETMESSAGE_VAULT_PREFIX`: vault prefix for secrets (default `cubbyhole/`)

## Configuration examples

Here is an example of a functionnal docker-compose.yml file
```yaml
version: '3.2'

services:
  vault:
    image: vault:latest
    container_name: vault
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: root
    cap_add:
      - IPC_LOCK
    expose:
      - 8200

  supersecret:
    build: ./
    image: algolia/supersecretmessage:latest
    container_name: supersecret
    environment:
      VAULT_ADDR: http://vault:8200
      VAULT_TOKEN: root
      SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS: ":80"
      SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS: ":443"
      SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED: "true"
      SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN: secrets.example.com
    ports:
      - "80:80"
      - "443:443"
    depends_on:
      - vault
```

### Configuration types

#### Plain HTTP

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=:80
```

#### TLS

##### Auto TLS

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS=:443
SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN=secrets.example.com
```

##### Auto TLS with HTTP > HTTPS redirection

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=:80
SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS=:443
SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED=true
SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN=secrets.example.com
```

##### Manual TLS

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS=:443
SUPERSECRETMESSAGE_TLS_CERT_FILEPATH=/mnt/ssl/cert_secrets.example.com.pem
SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH=/mnt/ssl/key_secrets.example.com.pem
```

## 🌍 Multi-Language Support

The application supports 5 languages with automatic detection and seamless switching:

### Supported Languages

| Language | Code | Translation Coverage |
|----------|------|---------------------|
| 🇬🇧 English | `en` | Complete (23 keys) |
| 🇫🇷 French | `fr` | Complete (23 keys) |
| 🇩🇪 German | `de` | Complete (23 keys) |
| 🇪🇸 Spanish | `es` | Complete (23 keys) |
| 🇮🇹 Italian | `it` | Complete (23 keys) |

### Usage

**Automatic Detection**: The application automatically detects the user's preferred language from:
1. URL parameter: `https://example.com/?lang=fr`
2. Browser language settings
3. Defaults to English if no match

**Manual Selection**: Users can switch languages using the selector in the top-right corner.

**Features**:
- ✅ No page reload required
- ✅ Language preference persisted in URL
- ✅ Dynamic updates of all UI elements
- ✅ Translates meta tags for SEO
- ✅ Updates HTML `lang` attribute for accessibility
- ✅ Translations loaded asynchronously (only active language)

### Technical Implementation

- **ES6 Modules**: Modern JavaScript with proper import/export
- **CSP-Compliant**: All event handlers use `addEventListener()`
- **i18n System**: Centralized in `utils.js` with `data-i18n` attributes
- **Translation Files**: JSON format in `/static/locales/`
- **Size Impact**: ~1KB per language file (loaded on-demand)

## 📸 Screenshots

### Message Creation Interface
![supersecretmsg](https://github.com/user-attachments/assets/95fa8704-118b-4a42-b4a0-4f59b82ce1d1)

*Clean, intuitive interface for creating self-destructing messages with optional file uploads and custom TTL.*

### Message Retrieval Interface
![supersecretmsg](https://github.com/user-attachments/assets/74a6ff23-b459-4ead-8c6d-13bdf15a3a65)

*Simple, secure interface for viewing self-destructing messages that are permanently deleted upon retrieval.*

## 🛠️ Development

### Prerequisites

- Go 1.25.1 or later
- Docker (for Vault dev server)
- Make (optional, for convenience)

### Setup

```bash
# Clone the repository
git clone https://github.com/algolia/sup3rS3cretMes5age.git
cd sup3rS3cretMes5age

# Download dependencies
go mod download

# Build the binary
go build -o sup3rs3cret cmd/sup3rS3cretMes5age/main.go
```

### Running Tests

```bash
# Run all tests
make test

# Or directly with go
go test ./... -v
```

### Code Quality

```bash
# Format code
gofmt -s -w .

# Lint
golangci-lint run --timeout 300s

# Static analysis
go vet ./...
```

### Project Structure

```
.
├── cmd/sup3rS3cretMes5age/    # Application entry point
│   └── main.go               # (67 lines)
├── internal/                  # Core business logic
│   ├── config.go             # Configuration handling (83 lines)
│   ├── handlers.go           # HTTP request handlers (201 lines)
│   ├── server.go             # Web server setup (370 lines)
│   └── vault.go              # Vault integration (192 lines)
├── web/static/               # Frontend assets
│   ├── index.html           # Message creation page
│   ├── getmsg.html          # Message retrieval page
│   ├── index.js             # Main page logic (ES6 modules)
│   ├── getmsg.js            # Retrieval page logic (ES6 modules)
│   ├── utils.js             # i18n utilities & helpers (130 lines)
│   ├── application.css      # Styling
│   ├── clipboard-2.0.11.min.js
│   └── locales/             # Translation files
│       ├── en.json          # English (23 keys)
│       ├── fr.json          # French (23 keys)
│       ├── de.json          # German (23 keys)
│       ├── es.json          # Spanish (23 keys)
│       └── it.json          # Italian (23 keys)
├── deploy/                   # Deployment configs
│   ├── Dockerfile           # Multi-stage build with security hardening
│   ├── docker-compose.yml   # Local dev stack with resource limits
│   └── charts/              # Helm chart for Kubernetes
└── Makefile                 # Build automation & minification
```

**Total Code**: 1,043 lines of Go across 4 core files (excluding tests)

## Contributing

Contributions are welcome! 🎉

### How to Contribute

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Guidelines

- Write tests for new features
- Follow existing code style
- Update documentation as needed
- Ensure all tests pass (`make test`)
- Run linters (`golangci-lint run`)

All pull requests will be reviewed by the Algolia team.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

This project is built on the shoulders of giants:

- **[HashiCorp Vault](https://www.vaultproject.io/)** - Secure secret storage backend
- **[Echo](https://echo.labstack.com/)** - High performance Go web framework
- **[Let's Encrypt](https://letsencrypt.org/)** - Free SSL/TLS certificates
- **[ClipboardJS](https://clipboardjs.com/)** - Modern clipboard functionality
