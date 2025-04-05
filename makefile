#BINARY NAME
BIN=Otter
INSTALL_DIR=/usr/local/bin
# BUILD DIR
BD=build
# CURRENT DIR
CDR=cmd/proxy


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
