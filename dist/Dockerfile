# SPDX-License-Identifier: MIT
#
#!BuildTag: opensuse/algolia/sup3rs3cretmes5age:latest opensuse/algolia/sup3rs3cretmes5age:0.3.0

FROM opensuse/tumbleweed AS builder

WORKDIR /tmp
RUN zypper addrepo https://download.opensuse.org/repositories/openSUSE:Factory/standard/openSUSE:Factory.repo && \
  zypper refresh && \
  zypper --non-interactive in go

COPY sup3rS3cretMes5age-0.3.0.tar.xz ./
COPY vendor.tar.gz ./

RUN tar xf sup3rS3cretMes5age-0.3.0.tar.xz && \
  cd sup3rS3cretMes5age-0.3.0 && \
  tar xf xf ../vendor.tar.gz && \
  CGO_ENABLED=0 go build -buildmode=pie -mod=vendor -a -ldflags '-extldflags "-static"' -o sup3rS3cretMes5age .

FROM opensuse/busybox

# labelprefix=org.opensuse.example
LABEL org.opencontainers.image.title="sup3rs3cretmes5age"
LABEL org.opencontainers.image.description="A simple, secure self-destructing message service, using HashiCorp Vault product as a backend."
LABEL org.opencontainers.image.version="0.3.0"
LABEL org.opensuse.reference="registry.opensuse.org/opensuse/algolia/sup3rs3cretmes5age:0.3.0"
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

RUN mkdir -p /opt/supersecret/static

WORKDIR /opt/supersecret

COPY --from=builder /tmp/sup3rS3cretMes5age-0.3.0/sup3rS3cretMes5age .
COPY --from=builder /tmp/sup3rS3cretMes5age-0.3.0/static /opt/supersecret/static

CMD [ "./sup3rS3cretMes5age" ]
