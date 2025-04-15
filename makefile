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
