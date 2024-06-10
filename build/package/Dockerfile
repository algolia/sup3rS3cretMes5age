# SPDX-License-Identifier: MIT
#
#!BuildTag: opensuse/algolia/sup3rs3cretmes5age:latest opensuse/algolia/sup3rs3cretmes5age:%PKG_VERSION% opensuse/algolia/sup3rs3cretmes5age:%PKG_VERSION%-%RELEASE%

FROM opensuse/tumbleweed AS builder

RUN zypper refresh && \
  zypper --non-interactive in go ca-certificates ca-certificates-mozilla

WORKDIR /tmp
COPY sup3rS3cretMes5age-%PKG_VERSION%.tar.xz ./
COPY vendor.tar.gz ./

RUN tar xf sup3rS3cretMes5age-%PKG_VERSION%.tar.xz && \
  cd sup3rS3cretMes5age-%PKG_VERSION% && \
  tar xf ../vendor.tar.gz && \
  ls -l && \
  CGO_ENABLED=0 go build -mod=vendor -trimpath -a -ldflags "-X main.version=%PKG_VERSION% -s -w -extldflags '-static'" -o sup3rS3cretMes5age cmd/sup3rS3cretMes5age/main.go


FROM opensuse/tumbleweed

# Define labels according to https://en.opensuse.org/Building_derived_containers
# labelprefix=org.opensuse.example
LABEL org.opencontainers.image.title="sup3rs3cretmes5age"
LABEL org.opencontainers.image.description="A simple, secure self-destructing message service, using HashiCorp Vault product as a backend."
LABEL org.opencontainers.image.version="%PKG_VERSION%-%RELEASE%"
LABEL org.opencontainers.image.url="https://build.opensuse.org/package/show/home:Algolia:OSS/sup3rS3cretMes5age"
LABEL org.opensuse.reference="registry.opensuse.org/opensuse/algolia/sup3rs3cretmes5age:%PKG_VERSION%-%RELEASE%"
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

WORKDIR /opt/supersecret

COPY --from=builder /etc/ssl/ /etc/ssl/
COPY --from=builder /tmp/sup3rS3cretMes5age-%PKG_VERSION%/sup3rS3cretMes5age .
COPY --from=builder /tmp/sup3rS3cretMes5age-%PKG_VERSION%/web/static /opt/supersecret/static

CMD [ "./sup3rS3cretMes5age" ]
