BIN=Otter #BINARY NAME 
INSTALL_DIR=/usr/local/bin
BD=build # BUILD DIR
CDR=cmd/proxy # CURRENT DIR

BDCMD=GOARCH=amd64 go build -o $(BD)/$(BIN)

all: build

build:
	go build -o $(BD)/$(BIN) $(CDR)/$(CDR)/main.go

install: build
	sudo mv $(BD)/$(BIN) $(INSTALL_DIR)/$(BIN)

clean:
	rm -f $(BD)/$(BIN)

release:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BD)
	GOOS=linux $(BDCMD)-linux-amd64 $(CDR)/main.go
	GOOS=linux $(BDCMD)-linux-arm64 $(CDR)/main.go
	GOOS=darwin $(BDCMD)-darwin-amd64 $(CDR)/main.go
	GOOS=darwin $(BDCMD)-darwin-arm64 $(CDR)/main.go
