.PHONY: build clean test docker-build docker-run docker-clean all

# Go commands
GO=go
GOCLEAN=$(GO) clean
GOTEST=$(GO) test
GOBUILD=$(GO) build

# Binary names
BINARY_NAME=load-balancer

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/balancer

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Docker commands
docker-build:
	docker build -t load-balancer .

docker-run:
	docker-compose up --build

docker-clean:
	docker-compose down
	docker rmi load-balancer

# Run locally
run:
	cp config.example.yaml config.yaml
	./$(BINARY_NAME) --config config.yaml

# Convenience targets
rebuild: clean build
docker-rebuild: docker-clean docker-build docker-run