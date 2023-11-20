OS?=linux
ARCH?=amd64
GOPATH:=$(shell go env GOPATH)
OUT_DIR?=./_output
REGISTRY?=keppel.eu-de-1.cloud.sap/ccloud
TEMP_DIR:=$(shell mktemp -d)
VERSION?=latest

IMAGE_NAME:=keppel.eu-de-1.cloud.sap/ccloud/netappsd
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD | head -c 7)
IMAGE_TAG:=$(shell date -u +%Y%m%d%H%M%S)-$(BRANCH)-$(HASH)

GOFILES := $(wildcard *.go) $(wildcard pkg/*/*.go) $(wildcard pkg/*/*/*.go)

all: build

build: bin/netappsd

bin/netappsd: $(GOFILES)
	go build -o $@ *.go

bin/netappsd-linux: $(GOFILES)
	GOOS=linux GOARCH=amd64 go build -o $@ *.go

.Phony: clean docker

docker: Dockerfile bin/netappsd-linux
	docker build --platform linux/amd64 -t ${IMAGE_NAME}:${IMAGE_TAG} .
	docker push ${IMAGE_NAME}:${IMAGE_TAG}

# Build
# -----

.PHONY: build-netappsd
build-netappsd:
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(OUT_DIR)/$(ARCH)/netappsd ./cmd/netappsd/

.PHONY: netappsd-container
netappsd-container: build-netappsd
	cp deployments/netappsd/Dockerfile $(TEMP_DIR)
	cp $(OUT_DIR)/$(ARCH)/netappsd $(TEMP_DIR)/netappsd
	cd $(TEMP_DIR) && sed -i.bak "s|BASEIMAGE|scratch|g" Dockerfile
	docker build -t $(REGISTRY)/netappsd-$(ARCH):$(VERSION) $(TEMP_DIR)
	docker push $(REGISTRY)/netappsd-$(ARCH):$(VERSION)

clean:
	rm -f bin/*
