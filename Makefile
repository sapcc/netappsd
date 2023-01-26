IMAGE_NAME:=keppel.eu-de-1.cloud.sap/ccloud/netappsd
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD | head -c 7)
IMAGE_TAG:=$(BRANCH)-$(shell date -u +%Y%m%d%H%M%S)-$(HASH)

GOFILES := $(wildcard *.go) $(wildcard pkg/*/*.go)

all: build

build: bin/netappsd

bin/netappsd: $(GOFILES)
	go build -o $@ main.go

bin/netappsd-linux: $(GOFILES)
	GOOS=linux GOARCH=amd64 go build -o $@ main.go

.Phony: clean docker

docker: Dockerfile bin/netappsd-linux
	docker build --platform linux/amd64 -t ${IMAGE_NAME}:${IMAGE_TAG} .
	docker push ${IMAGE_NAME}:${IMAGE_TAG}

clean:
	rm -f bin/*
