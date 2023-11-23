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

GOFILES := $(shell find . -name '*.go')
# $(wildcard internal/*/*.go) $(wildcard internal/*/*/*.go)

all: build

# build: bin/netappsd

bin/netappsd: $(GOFILES)
	go build -o $@ *.go

bin/netappsd-linux: $(GOFILES)
	GOOS=linux GOARCH=amd64 go build -o $@ *.go

docker: Dockerfile bin/netappsd-linux
	docker build --platform linux/amd64 -t ${IMAGE_NAME}:${IMAGE_TAG} .
	docker push ${IMAGE_NAME}:${IMAGE_TAG}

# Build
# -----
build-netappsd: $(OUT_DIR)/$(ARCH)/netappsd

$(OUT_DIR)/$(ARCH)/netappsd: $(GOFILES)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(OUT_DIR)/$(ARCH)/netappsd ./cmd/netappsd/
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(OUT_DIR)/$(ARCH)/netappsd-worker ./cmd/netappsd-worker/

build-container: build-netappsd
	cp deployments/Dockerfile $(TEMP_DIR)
	cp $(OUT_DIR)/$(ARCH)/netappsd $(TEMP_DIR)/netappsd
	cp $(OUT_DIR)/$(ARCH)/netappsd-worker $(TEMP_DIR)/netappsd-worker
	cd $(TEMP_DIR) && sed -i.bak "s|BASEIMAGE|scratch|g" Dockerfile
	docker build --platform $(OS)/$(ARCH) -t $(REGISTRY)/netappsd-$(ARCH):$(VERSION) $(TEMP_DIR)
	docker push $(REGISTRY)/netappsd-$(ARCH):$(VERSION)

# Deploy
# ------
# .Phony: deploy-k8s
deployments/netappsd.yaml: deployments/templates/netappsd.yaml
	gomplate < deployments/templates/netappsd.yaml > deployments/netappsd.yaml

deploy-k8s: deployments/netappsd.yaml
	gomplate < deployments/kubernetes/netappsd.yaml | kubectl apply -f -

.Phony: delete-k8s
delete-k8s:
	kubectl delete -f deployments/kubernetes
