FROM golang:1.26 AS builder

WORKDIR /go/src/github.com/algolia/sup3rS3cretMes5age

ARG VERSION

# Copy the module files first so the download layer survives any source
# edit, then only the Go packages so web-asset changes (handled by the
# web-builder stage) don't invalidate the compile layer.
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/

# build process
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -a \
    -ldflags "-X main.version=${VERSION} -s -w -extldflags '-static'" \
    -o /tmp/sup3rS3cretMes5age \
    cmd/sup3rS3cretMes5age/main.go

# Web assets minification stage
FROM node:25.8.1-alpine AS web-builder
WORKDIR /app

# Exact pin: a floating pin re-resolves on every build, so a new minor
# release can silently change minification output between two builds of the
# same commit. Installed before COPY so this layer is cached — it invalidates
# only when the pin changes, not on every web-asset edit.
ARG NODE_MINIFY_VERSION=10.5.0
RUN npm install -g \
    @node-minify/cli@${NODE_MINIFY_VERSION} \
    @node-minify/terser@${NODE_MINIFY_VERSION} \
    @node-minify/lightningcss@${NODE_MINIFY_VERSION} \
    @node-minify/html-minifier@${NODE_MINIFY_VERSION} \
    @node-minify/jsonminify@${NODE_MINIFY_VERSION}

COPY web/ ./

# Regenerate the language manifest from the locales directory so the shipped
# manifest always matches the locale files in the image. The committed copy in
# web/static/ is only a fallback for non-Docker (local) runs.
RUN node -e "const fs=require('fs');const langs=fs.readdirSync('static/locales').filter(f=>f.endsWith('.json')).map(f=>f.slice(0,-5)).sort();fs.writeFileSync('static/locales-manifest.json',JSON.stringify({languages:langs},null,2)+'\n')"

# Minify JS and CSS files. set -eu aborts the build if any node-minify
# invocation fails — without it only the trailing mv was skipped and the
# loop continued, silently shipping the unminified file.
RUN set -eu && \
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
  org.opencontainers.image.licenses="MIT" \
  org.opencontainers.image.url="https://github.com/algolia/sup3rS3cretMes5age" \
  org.opencontainers.image.source="https://github.com/algolia/sup3rS3cretMes5age"


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
    && find ./static -type f -exec chmod 644 {} \; \
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
