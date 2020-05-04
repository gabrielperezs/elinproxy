
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

all: test build

test:
	$(GOTEST) -run=^Test ./...

escape:
	$(GOTEST) -gcflags "-m -m"  -run="^Test" ./...

build:
	$(GOBUILD) -o elinproxy
	$(GOBUILD) -o elinproxycli ./client/

