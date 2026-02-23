# Binary name
BINARY_NAME=genkit-server

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run

# Build flags
LDFLAGS=-ldflags="-s -w"

.PHONY: all build build-ui clean test run help

all: build

build-ui:
	cd ../genkit-flow-ui && pnpm build --outDir ../genkit-flow/internal/ui/dist

build: build-ui
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

run:
	$(GORUN) ./cmd/server $(FLAGS)

clean:
	$(GOCLEAN)
	rm -rf bin/

test:
	$(GOTEST) -v ./...

help:
	@echo "Usage: make [target] [FLAGS=\"...\"]"
	@echo ""
	@echo "Targets:"
	@echo "  build   Build the compressed binary"
	@echo "  run     Run the server directly (pass flags via FLAGS=\"-provider=...\")"
	@echo "  clean   Remove build artifacts"
	@echo "  test    Run tests"
	@echo "  help    Show this help message"
