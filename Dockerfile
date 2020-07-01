FROM golang:latest AS builder

WORKDIR /go/src/github.com/algolia/sup3rS3cretMes5age
ADD . .

RUN go get -v 
RUN CGO_ENABLED=0 GOOS=linux go build  -o sup3rS3cretMes5age .

RUN go run /usr/local/go/src/crypto/tls/generate_cert.go --host localhost

FROM alpine:latest

EXPOSE 1234

ENV \
    VAULT_ADDR \
    VAULT_TOKEN

RUN \
apk add --no-cache ca-certificates ;\
mkdir -p /opt/supersecret/static

WORKDIR /opt/supersecret
COPY --from=builder /go/src/github.com/algolia/sup3rS3cretMes5age/*.pem ./
COPY --from=builder /go/src/github.com/algolia/sup3rS3cretMes5age/sup3rS3cretMes5age .
COPY static /opt/supersecret/static

CMD [ "./sup3rS3cretMes5age" ]
