OS?=linux
ARCH?=amd64

GOPATH:=$(shell go env GOPATH)
GOFILES:=$(shell find . -name '*.go' -not -path "./vendor/*")
OUT_DIR?=./_output
TEMP_DIR:=$(shell mktemp -d)

REGISTRY?=keppel.eu-de-1.cloud.sap/ccloud
IMAGE_NAME:=$(REGISTRY)/netappsd-$(ARCH)
VERSION?=latest

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD | head -c 7)
IMAGE_TAG:=$(shell date -u +%Y%m%d%H%M%S)-$(BRANCH)-$(HASH)


all: build-netappsd


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
	docker build --platform $(OS)/$(ARCH) -t $(IMAGE_NAME):$(VERSION) $(TEMP_DIR)
	docker push $(IMAGE_NAME):$(VERSION)

# Deploy
# ------
$(OUT_DIR)/manifests.yaml: deployments/templates/netappsd.yaml
	gomplate < deployments/templates/netappsd.yaml > $@
	
$(OUT_DIR)/manifests-debug.yaml: deployments/templates/netappsd.yaml
	debug=true gomplate < deployments/templates/netappsd.yaml > $@

.Phony: debug
debug: $(OUT_DIR)/manifests-debug.yaml
	skaffold debug --profile=debug

# TODO: use skaffold to deploy to k8s
.Phony: deploy
deploy-k8s: $(OUT_DIR)/manifests.yaml build-container
	kubectl apply -f $(OUT_DIR)/manifests.yaml

.Phony: delete-k8s
delete-k8s: $(OUT_DIR)/manifests.yaml
	kubectl delete -f $(OUT_DIR)/manifests.yaml
