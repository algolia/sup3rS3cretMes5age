# For MacOS use darwin
TARGET_OS ?= linux

# When developing locally, change this to whatever fqdn you are using for 127.0.0.1
DOMAIN ?= localhost


test:
	go test ./... -v

build: 
	@docker-compose build

clean:
	@docker-compose rm -fv

run-local: clean
        @DOMAIN=$(DOMAIN) \
        docker-compose up --build -d

run: 
	@DOMAIN=$(DOMAIN) \
        docker-compose up --build -d

logs:
	@docker-compose logs -f

stop:
	@docker-compose stop

.PHONY: test build clean run-local run logs stop
