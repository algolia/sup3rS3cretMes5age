# SPDX-License-Identifier: MIT
#
#!BuildTag: opensuse/algolia/sup3rS3cretMes5age:latest opensuse/algolia/sup3rS3cretMes5age:%PKG_VERSION% opensuse/algolia/sup3rS3cretMes5age:%PKG_VERSION%.%RELEASE%

FROM opensuse/tumbleweed AS builder

WORKDIR /tmp

RUN zypper --non-interactive in go

COPY sup3rS3cretMes5age-0.3.0.tar.xz ./
COPY vendor.tar.gz ./

RUN tar xf sup3rS3cretMes5age-0.3.0.tar.xz && \
  cd sup3rS3cretMes5age-0.3.0 && \
  tar xf xf ../vendor.tar.gz && \
  CGO_ENABLED=0 go build -buildmode=pie -mod=vendor -a -ldflags '-extldflags "-static"' -o sup3rS3cretMes5age .

FROM alpine:latest

# labelprefix=org.opensuse.example
LABEL org.opencontainers.image.title="sup3rS3cretMes5age"
LABEL org.opencontainers.image.description="A simple, secure self-destructing message service, using HashiCorp Vault product as a backend."
LABEL org.opencontainers.image.version="%PKG_VERSION%.%RELEASE%"
LABEL org.opensuse.reference="registry.opensuse.org/opensuse/algolia/sup3rS3cretMes5age:%PKG_VERSION%.%RELEASE%"
LABEL org.openbuildservice.disturl="%DISTURL%"
LABEL org.opencontainers.image.created="%BUILDTIME%"
# endlabelprefix

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
