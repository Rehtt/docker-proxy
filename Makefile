.PHONY: all build test vet clean run docker-build docker-run compose-up compose-down compose-logs

BINARY      ?= bin/docker-proxy
MAIN        ?= ./cmd/docker-proxy
IMAGE       ?= docker-proxy:latest
COMPOSE     ?= docker compose

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) $(MAIN)

test:
	go test -count=1 ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY) -listen :8080 -config config.yaml

docker-build:
	docker build -t $(IMAGE) .

docker-run: docker-build
	docker run --rm -p 8080:8080 -v $$(pwd)/config.yaml:/etc/docker-proxy/config.yaml:ro $(IMAGE)

compose-up:
	@test -f config.yaml || (echo "缺少 config.yaml，请先: cp config.example.yaml config.yaml" && exit 1)
	$(COMPOSE) up -d --build

compose-down:
	$(COMPOSE) down

compose-logs:
	$(COMPOSE) logs -f
