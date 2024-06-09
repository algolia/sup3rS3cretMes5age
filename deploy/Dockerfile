FROM golang:1.21 AS builder

WORKDIR /go/src/github.com/algolia/sup3rS3cretMes5age

ARG VERSION

COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -a -ldflags "-X main.version=${VERSION} -s -w -extldflags '-static'" -o /tmp/sup3rS3cretMes5age cmd/sup3rS3cretMes5age/main.go


FROM alpine:latest

WORKDIR /opt/supersecret

COPY --from=builder /tmp/sup3rS3cretMes5age .
COPY web/static/ /opt/supersecret/static/

ENV \
    VAULT_ADDR \
    VAULT_TOKEN \
    SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS \
    SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS \
    SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED \
    SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN \
    SUPERSECRETMESSAGE_TLS_CERT_FILEPATH \
    SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH \
    SUPERSECRETMESSAGE_VAULT_PREFIX

RUN apk add --no-cache ca-certificates


CMD ["./sup3rS3cretMes5age" ]
