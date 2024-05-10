FROM golang:1.22 AS builder

WORKDIR /tmp

COPY sup3rS3cretMes5age-0.3.0.tar.xz ./
COPY vendor.tar.gz ./

RUN tar xf sup3rS3cretMes5age-0.3.0.tar.xz && \
  cd sup3rS3cretMes5age-0.3.0 && \
  tar xf xf ../vendor.tar.gz && \
  CGO_ENABLED=0 go build -buildmode=pie -mod=vendor -a -ldflags '-extldflags "-static"' -o sup3rS3cretMes5age .

FROM alpine:latest

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

RUN \
    apk add --no-cache ca-certificates ;\
    mkdir -p /opt/supersecret/static

WORKDIR /opt/supersecret

COPY --from=builder /tmp/sup3rS3cretMes5age-0.3.0/sup3rS3cretMes5age .
COPY --from=builder /tmp/sup3rS3cretMes5age-0.3.0/static /opt/supersecret/static

CMD [ "./sup3rS3cretMes5age" ]
