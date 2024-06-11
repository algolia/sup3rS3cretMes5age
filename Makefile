# For MacOS use darwin
TARGET_OS ?= linux

# When developing locally, change this to whatever fqdn you are using for 127.0.0.1
DOMAIN ?= localhost

COMPOSE_OPTS := -f deploy/docker-compose.yml
DOCKER_OPS := -f deploy/Dockerfile

TAG=$(shell git describe --tags --abbrev=0)
VERSION=$(shell echo "$(TAG)" | sed -e 's/^v//')
COMMIT=$(shell git rev-parse --short HEAD)

test:
	go test ./... -v

image:
	docker build \
		--build-arg VERSION=${VERSION} \
		-t algolia/supersecretmessage:${VERSION} \
		-t algolia/supersecretmessage:${COMMIT} \
		-t algolia/supersecretmessage:latest \
		$(DOCKER_OPS) .

build: 
	@docker compose $(COMPOSE_OPTS) build

clean:
	@docker compose $(COMPOSE_OPTS) rm -fv

run-local: clean
        @DOMAIN=$(DOMAIN) \
	docker compose $(COMPOSE_OPTS) up --build -d

run: 
	@DOMAIN=$(DOMAIN) \
        docker compose $(COMPOSE_OPTS) up --build -d

logs:
	@docker compose $(COMPOSE_OPTS) logs -f

stop:
	@docker compose $(COMPOSE_OPTS) stop

.PHONY: test image build clean run-local run logs stop
