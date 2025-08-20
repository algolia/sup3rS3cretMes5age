# GitHub Copilot Instructions for sup3rS3cretMes5age

## Project Overview

**sup3rS3cretMes5age** is a secure, self-destructing message service built in Go that provides a simple way to share sensitive information that automatically expires. The application uses HashiCorp Vault as its backend for secure secret storage and management.

### Key Features
- **Self-destructing messages**: Messages automatically delete after being read or after expiration
- **Secure storage**: Uses HashiCorp Vault for backend secret management
- **Web interface**: Modern vanilla JavaScript frontend with minimal dependencies
- **Container-ready**: Docker and Kubernetes deployment support
- **Production-ready**: HTTPS/TLS support, monitoring, and observability features

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Web Frontend  │────│  Go Backend API  │────│  HashiCorp Vault │
│  (Vanilla JS)   │    │   (Echo/Gin)     │    │   (Secret Store) │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Core Components
- **Frontend**: Located in `/web/static/` - vanilla JavaScript with ClipboardJS for copy functionality
- **Backend**: Go application using Echo framework for HTTP handling
- **Storage**: HashiCorp Vault for secure secret persistence
- **Deployment**: Docker containers, Kubernetes/Helm charts, AWS deployment options

## Repository Structure

```
├── cmd/                    # Application entry points
│   └── sup3rS3cretMes5age/ # Main application binary
├── internal/               # Private application code
│   ├── config.go          # Configuration management
│   ├── handlers.go        # HTTP request handlers
│   ├── server.go          # HTTP server setup
│   └── vault.go           # Vault client integration
├── web/                   # Frontend assets
│   └── static/            # Static web files (HTML, CSS, JS)
├── deploy/                # Deployment configurations
│   ├── Dockerfile         # Container build instructions
│   ├── docker-compose.yml # Local development setup
│   └── charts/            # Helm charts for Kubernetes
├── AWS_DEPLOYMENT.md      # Comprehensive AWS deployment guide
├── README.md              # Project documentation
└── Makefile              # Build and development commands
```

## Development Guidelines

### Go Code Standards
- **Framework**: Uses Echo v4 for HTTP routing and middleware
- **Testing**: Uses testify for assertions and mocking
- **Dependencies**: Minimal external dependencies, prefer standard library
- **Error handling**: Always handle errors explicitly, return meaningful error messages
- **Logging**: Use structured logging with appropriate log levels

### Security Considerations
- **No plaintext secrets**: All sensitive data must go through Vault
- **Input validation**: Validate all user inputs for XSS, injection attacks
- **HTTPS enforcement**: Production deployments should enforce HTTPS
- **Token management**: Use secure random tokens for message access
- **Rate limiting**: Implement rate limiting for API endpoints

### API Design Patterns
- **RESTful endpoints**: Follow REST conventions for API design
- **JSON responses**: All API responses should be JSON formatted
- **Error responses**: Consistent error response format with proper HTTP status codes
- **Validation**: Input validation on all endpoints
- **CORS**: Proper CORS configuration for cross-origin requests

### Frontend Guidelines
- **Vanilla JavaScript**: No frameworks, keep it lightweight
- **Progressive enhancement**: Ensure basic functionality without JavaScript
- **Accessibility**: Follow WCAG guidelines for accessibility
- **Security**: Sanitize all user inputs, prevent XSS
- **Performance**: Minimize bundle size, optimize for mobile

## Key Files and Their Purpose

### Backend (`/internal/`)
- **`config.go`**: Application configuration, environment variable handling
- **`handlers.go`**: HTTP request handlers for message creation, retrieval, deletion
- **`server.go`**: HTTP server setup, middleware configuration, routing
- **`vault.go`**: HashiCorp Vault client integration, secret operations

### Frontend (`/web/static/`)
- **HTML files**: User interface templates
- **CSS files**: Styling with mobile-first responsive design
- **JavaScript files**: Client-side functionality, API communication

### Deployment (`/deploy/`)
- **`Dockerfile`**: Multi-stage Docker build for optimized container images
- **`docker-compose.yml`**: Local development environment with Vault
- **`charts/`**: Helm charts for Kubernetes deployment

## Common Development Tasks

### Local Development
```bash
# Start local development environment
make run-local

# Run tests
make test

# Build Docker image
make image

# View logs
make logs
```

### Environment Variables
- `VAULT_ADDR`: Vault server address
- `VAULT_TOKEN`: Vault authentication token
- `DOMAIN`: Application domain name
- `HTTPS_ENABLED`: Enable HTTPS mode
- `PORT`: Application port (default: 8080)

### Testing Patterns
- Unit tests for all handler functions
- Integration tests for Vault interactions
- End-to-end tests for critical user flows
- Security tests for input validation

## Deployment Options

### 1. Docker Compose (Development)
Quick local setup with Vault container included.

### 2. Kubernetes/Helm (Production)
Production-ready deployment with proper scaling, monitoring, and security.

### 3. AWS Deployment (Cloud)
Multiple AWS deployment options documented in `AWS_DEPLOYMENT.md`:
- ECS with Fargate (recommended)
- EKS (Kubernetes)
- EC2 with Docker

## Security Best Practices

### Code Security
- Never log sensitive information
- Use context for request cancellation and timeouts
- Implement proper input sanitization
- Use HTTPS in production environments
- Rotate secrets regularly

### Vault Integration
- Use short-lived tokens when possible
- Implement proper secret versioning
- Use Vault policies for access control
- Monitor Vault audit logs

### Deployment Security
- Use non-root containers
- Implement network policies
- Use secrets management (AWS Secrets Manager, Kubernetes secrets)
- Enable audit logging

## Performance Considerations

- **Caching**: Implement appropriate caching strategies
- **Database connections**: Use connection pooling for Vault connections
- **Static assets**: Serve static files efficiently
- **Monitoring**: Implement health checks and metrics
- **Scaling**: Design for horizontal scaling

## Monitoring and Observability

- **Health checks**: Implement `/health` endpoint
- **Metrics**: Expose Prometheus metrics
- **Logging**: Structured logging with correlation IDs
- **Tracing**: Distributed tracing for request flows
- **Alerts**: Set up monitoring alerts for critical metrics

## Contributing Guidelines

When making changes:
1. Follow Go best practices and gofmt formatting
2. Add tests for new functionality
3. Update documentation for API changes
4. Ensure security implications are considered
5. Test deployment configurations
6. Update relevant documentation (README, deployment guides)

## Common Patterns

### Error Handling
```go
if err != nil {
    return c.JSON(http.StatusInternalServerError, map[string]string{
        "error": "Internal server error",
    })
}
```

### Vault Operations
```go
secret, err := client.Logical().Read("secret/data/messages/" + messageID)
if err != nil {
    // Handle error
}
```

### JSON Responses
```go
return c.JSON(http.StatusOK, map[string]interface{}{
    "message": "Success",
    "data": responseData,
})
```

This project prioritizes security, simplicity, and reliable deployment across multiple platforms while maintaining a minimal attack surface and excellent user experience.