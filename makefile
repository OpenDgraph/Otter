#BINARY NAME
BIN=Otter
INSTALL_DIR=/usr/local/bin
# BUILD DIR
BD=build
# CURRENT DIR
CDR=cmd/proxy
CONFIG_PATH=manifest/config.yaml

PLATFORMS = \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64

all: build

build:
	go build -o $(BD)/$(BIN) $(CDR)/main.go

install: build
	sudo mv $(BD)/$(BIN) $(INSTALL_DIR)/$(BIN)

clean:
	rm -rf $(BD)

release:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BD)
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d/ -f1); \
		ARCH=$$(echo $$platform | cut -d/ -f2); \
		OUT=$(BD)/$(BIN)-$$OS-$$ARCH; \
		echo "-> $$OS/$$ARCH"; \
		GOOS=$$OS GOARCH=$$ARCH go build -o $$OUT $(CDR)/main.go || exit 1; \
	done

run-local: build
	CONFIG_FILE=$(CONFIG_PATH) ./$(BD)/$(BIN)

rund:
	docker compose -f examples/cluster/docker-compose.yml up --build

stopd:
	docker compose -f examples/cluster/docker-compose.yml down

build-docker:
	docker build -t otter-local -f examples/cluster/Dockerfile .

# --- Testing ---------------------------------------------------------------
#
# test        - alias for `test-unit`, the default fast local suite.
# test-unit   - unit and internal integration tests; no Docker required.
# test-e2e    - Docker-backed E2E suite; requires `make e2e-up` first.
# e2e         - one-shot: boot stack, wait, run E2E, tear down.
# e2e-up      - start the Docker stack in the background.
# e2e-down    - stop and clean the Docker stack.
# e2e-wait    - block until Otter answers on /health and /query.
# e2e-seed    - seed sample data into the live cluster.
#
# E2E tests are gated by the `e2e` build tag so they never run during a
# plain `go test ./...`.

.PHONY: test test-unit test-e2e e2e e2e-up e2e-down e2e-wait e2e-seed

test: test-unit

test-unit:
	go test ./cmd/... ./internal/...

test-e2e:
	go test -tags=e2e -count=1 ./e2e/...

e2e-up:
	docker compose -f examples/cluster/docker-compose.yml up -d --build

e2e-down:
	docker compose -f examples/cluster/docker-compose.yml down -v

e2e-wait:
	./scripts/wait-for-otter.sh

e2e-seed:
	DGRAPH_GRPC=$${DGRAPH_GRPC:-localhost:9081} go run ./e2e/setup

e2e: e2e-up e2e-wait
	@set -e; \
	  $(MAKE) e2e-seed || true; \
	  $(MAKE) test-e2e; status=$$?; \
	  $(MAKE) e2e-down; \
	  exit $$status

check-updates:
	@echo "Checking for available updates..."
	go list -u -m -json all | grep '"Path"\|"Version"\|"Update"'
 
upgrade-all:
	@echo "Upgrading all dependencies..."
	go get -u ./...
	go mod tidy
	@echo "All dependencies upgraded."
 
upgrade:
ifndef MODULE
	$(error You must provide a module name with MODULE=example.com/lib)
endif
	@echo "Upgrading $(MODULE)..."
	go get -u $(MODULE)
	go mod tidy
	@echo "$(MODULE) upgraded."

# Clean up unused dependencies
tidy:
	@echo "Tidying up unused dependencies..."
	go mod tidy
	@echo "go.mod and go.sum are clean."

# Display dependency graph
deps:
	@echo "Displaying dependency graph..."
	go mod graph

.PHONY: check-updates upgrade-all upgrade tidy deps
