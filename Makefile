GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean

DOCKERCMD=docker

ifndef TAG
	TAG=nodns
endif

DOCKERBUILD=$(DOCKERCMD) build -t izanagi1995/mikroproxy:$(TAG) .
DOCKERPUSH=$(DOCKERCMD) push izanagi1995/mikroproxy:$(TAG)

BINARY=mikroproxy

all: build docker docker-push
build:
	go build -a -tags netgo -ldflags '-w' -o $(BINARY) main.go
clean:
	$(GOCLEAN)
	rm -rf $(BINARY)
docker:
	$(DOCKERBUILD)
docker-push:
	$(DOCKERPUSH)
