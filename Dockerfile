FROM alpine:latest

EXPOSE 1234

ENV \
    VAULT_ADDR \
    VAULT_TOKEN

RUN \
apk add --no-cache ca-certificates ;\
mkdir -p /opt/supersecret/static

WORKDIR /opt/supersecret

COPY bin/sup3rs3cretMes5age /opt/supersecret
COPY static /opt/supersecret/static

CMD [ "./sup3rs3cretMes5age" ]
