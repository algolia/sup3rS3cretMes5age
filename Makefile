GOLANG_VERSION := 1.9
PROJECT_OWNER := algolia
PROJECT_PATH := src/github.com/$(PROJECT_OWNER)/sup3rs3cretMes5age
TARGET_OS ?= linux

$(GOPATH)/bin/govendor:
	@go get -u github.com/kardianos/govendor

.PHONY: vendor
vendor: $(GOPATH)/bin/govendor
	@govendor sync

bin/sup3rs3cretMes5age: vendor
	@CGO_ENABLED=0 GOOS=$(TARGET_OS) GOARCH=amd64 go build -o $@

nginx/certs:
	@mkdir -p $@

nginx/certs/default.crt: nginx/certs
	@openssl req \
	-x509 \
	-newkey rsa:4096 \
	-days 365 \
	-keyout nginx/certs/default.key \
	-nodes \
	-subj "/C=US/ST=Oregon/L=Portland/O=Localhost LLC/OU=Org/CN=localhost" \
	-out $@

.PHONY: build
build:
	@docker run \
	--rm \
	-v $(PWD):/go/$(PROJECT_PATH) \
	-w /go/$(PROJECT_PATH) \
	golang:$(GOLANG_VERSION) \
	make bin/sup3rs3cretMes5age

.PHONY: clean
clean:
	@rm -f bin/*
	@docker-compose rm -fv

run-local: clean build nginx/certs/default.crt
	@NGINX_CONF_PATH=$(PWD)/nginx \
	STATIC_FILES_PATH=$(PWD)/static \
	docker-compose up --build -d

.PHONY: run
run: clean build
	@NGINX_CONF_PATH=$(PWD)/nginx \
	STATIC_FILES_PATH=$(PWD)/static \
	docker-compose up --build -d

.PHONY: stop
stop:
	@docker-compose stop
