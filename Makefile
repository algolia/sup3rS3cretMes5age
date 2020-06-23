# For MacOS use darwin
TARGET_OS ?= linux

# When developing locally, change this to whatever fqdn you are using for 127.0.0.1
VIRTUAL_HOST ?= localhost


nginx/certs:
	@mkdir -p $@

nginx/certs/default.crt: nginx/certs
	@openssl req \
	-x509 \
	-newkey rsa:4096 \
	-days 365 \
	-keyout nginx/certs/default.key \
	-nodes \
	-subj "/C=US/ST=Oregon/L=Portland/O=Localhost LLC/OU=Org/CN=$(VIRTUAL_HOST)" \
	-out $@

test:
	go test ./... -v

build: 
	@docker-compose build

clean:
	@rm -f bin/*
	@docker-compose rm -fv

run-local: clean nginx/certs/default.crt
	@NGINX_CONF_PATH=$(PWD)/nginx \
	STATIC_FILES_PATH=$(PWD)/static \
	VIRTUAL_HOST=$(VIRTUAL_HOST) \
	CERT_NAME=default \
	docker-compose up --build -d

run: clean 
	@NGINX_CONF_PATH=$(PWD)/nginx \
	STATIC_FILES_PATH=$(PWD)/static \
	VIRTUAL_HOST=$(VIRTUAL_HOST) \
        LETSENCRYPT_HOST=$(VIRTUAL_HOST) \
        LETSENCRYPT_EMAIL=webmaster@$(VIRTUAL_HOST) \
        CERT_NAME=$(VIRTUAL_HOST) \
	docker-compose up --build -d

logs:
	@docker-compose logs -f

stop:
	@docker-compose stop

.PHONY: test build clean run-local run logs stop
