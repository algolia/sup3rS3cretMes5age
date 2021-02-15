FROM golang:latest AS builder

WORKDIR /go/src/github.com/algolia/sup3rS3cretMes5age
ADD . .

RUN go get -v 
RUN CGO_ENABLED=0 GOOS=linux go build  -o sup3rS3cretMes5age .


FROM alpine:latest

ENV \
    VAULT_ADDR \
    VAULT_TOKEN \
    SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS \
    SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS \
    SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED \
    SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN \
    SUPERSECRETMESSAGE_TLS_CERT_FILEPATH \
    SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH

RUN mkdir -p /opt/supersecret/static

WORKDIR /opt/supersecret
COPY --from=builder /go/src/github.com/algolia/sup3rS3cretMes5age/sup3rS3cretMes5age .
COPY static /opt/supersecret/static

CMD [ "./sup3rS3cretMes5age" ]
