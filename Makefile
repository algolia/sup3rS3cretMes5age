GOLANG_VERSION := 1.9

deps:
	@go get -u github.com/hashicorp/vault
	@go get -u github.com/labstack/echo
	@go get -u github.com/dgrijalva/jwt-go

bin/sup3rs3cretMes5age: deps
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@

build:
	@docker run \
	--rm \
	-v $(PWD):/usr/src/supersecret \
	-w /usr/src/supersecret \
	golang:$(GOLANG_VERSION) \
	make bin/sup3rs3cretMes5age

clean:
	@rm -f bin/*
	@docker-compose rm -fv

run: clean build
	@docker-compose up --build -d

stop:
	@docker-compose stop

.PHONY: deps build clean run stop
