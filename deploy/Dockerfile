FROM golang:1.25 AS builder

WORKDIR /go/src/github.com/algolia/sup3rS3cretMes5age

ARG VERSION
ARG BUILD_DATE
ARG VCS_REF

# Add security-related labels
LABEL org.opencontainers.image.title="sup3rS3cretMes5age" \
      org.opencontainers.image.description="Secure self-destructing message service" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.vendor="Algolia" \
      org.opencontainers.image.licenses="MIT"

COPY . .

# build process
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -a \
    -ldflags "-X main.version=${VERSION} -s -w -extldflags '-static'" \
    -o /tmp/sup3rS3cretMes5age \
    cmd/sup3rS3cretMes5age/main.go

# Web assets minification stage
FROM node:latest AS web-builder
WORKDIR /app
COPY web/ ./
# Minify JS and CSS files
RUN npm install -g @node-minify/cli \
    @node-minify/terser \
    @node-minify/lightningcss \
    @node-minify/html-minifier \
    @node-minify/jsonminify && \
    cd static && \
    for fi in utils.js index.js getmsg.js; \
    do \
      node-minify --compressor terser --input "$fi" --output "min.$fi" && mv "min.$fi" "$fi"; \
    done && \
    for fi in *.html; \
    do \
      node-minify --compressor html-minifier --input "$fi" --output "min.$fi" && mv "min.$fi" "$fi"; \
    done && \
    node-minify --compressor lightningcss --input application.css --output min.application.css && mv min.application.css application.css && \
    ls -l && \
    cd locales && \
    for fi in *.json; \
    do \
      node-minify --compressor jsonminify --input "$fi" --output "min.$fi" && mv "min.$fi" "$fi"; \
    done && \
    ls -l

# Multi-stage build with security hardening
FROM alpine:latest

# Install only necessary certificates and packages
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user with restricted permissions
RUN addgroup -S -g 1001 supersecret \
    && adduser -S -u 1001 -G supersecret supersecret

# Set up working directory with restricted permissions
WORKDIR /opt/supersecret

# Copy binary and static assets
COPY --from=builder --chown=supersecret:supersecret /tmp/sup3rS3cretMes5age ./sup3rS3cretMes5age
COPY --from=web-builder --chown=supersecret:supersecret /app/static ./static

# Set proper file permissions
RUN chmod 755 ./sup3rS3cretMes5age \
    && chmod 644 ./static/* \
    && find ./static -type d -exec chmod 755 {} \;

# Define environment variables
ENV \
    VAULT_ADDR="" \
    VAULT_TOKEN="" \
    SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=":8082" \
    SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS="" \
    SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED="false" \
    SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN="" \
    SUPERSECRETMESSAGE_TLS_CERT_FILEPATH="" \
    SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH="" \
    SUPERSECRETMESSAGE_VAULT_PREFIX="cubbyhole/" \
    GODEBUG=x509ignoreCN=0 \
    GOGC=200 \
    GOMAXPROCS=1

USER supersecret

# Expose only necessary ports
EXPOSE 8082

ENTRYPOINT ["/opt/supersecret/sup3rS3cretMes5age"]
CMD []
