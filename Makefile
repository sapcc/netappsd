# ifeq ($(KUBECTL_CONTEXT),)
# $(error KUBECTL_CONTEXT is not set)
# endif

OS?=linux
ARCH?=amd64
PROFILE?=master,worker
KUBECTL_CONTEXT?=qa-de-1

GOFILES:=$(shell find . -name '*.go' -not -path "./vendor/*")
GOPATH:=$(shell go env GOPATH)
OUT_DIR?=_output
TEMP_DIR:=$(shell mktemp -d)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD | head -c 7)

IMAGE_NAME:=keppel.eu-de-1.cloud.sap/ccloud/netappsd
IMAGE_TAG?=$(shell git rev-parse --short HEAD)

all: build manifests

# Build
# -----
.PHONY: build image
build: $(OUT_DIR)/$(ARCH)/netappsd
 
image: $(OUT_DIR)/amd64/netappsd
	cp deployments/Dockerfile $(TEMP_DIR)
	cp $(OUT_DIR)/$(ARCH)/netappsd $(TEMP_DIR)/netappsd
	cd $(TEMP_DIR) && sed -i.bak "s|BASEIMAGE|scratch|g" Dockerfile
	docker build --platform $(OS)/$(ARCH) -t $(IMAGE_NAME):$(IMAGE_TAG) $(TEMP_DIR)
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

$(OUT_DIR)/$(ARCH)/netappsd: $(GOFILES)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $@ ./cmd

# manifests
# ---------
define generate_manifests
	gomplate < deployments/k8s/$(1) > $(OUT_DIR)/$(1)
endef

.PHONY: manifests
manifests: netappsd.yaml master.yaml worker.yaml

netappsd.yaml:
	$(call generate_manifests,$@)

master.yaml:
	$(call generate_manifests,$@)

worker.yaml:
	$(call generate_manifests,$@)

# Dev
# ------

.PHONY: dev
dev: export production=0
dev: build manifests
	skaffold dev --profile=$(PROFILE) --namespace=netapp-exporters --kube-context $(KUBECTL_CONTEXT)

.PHONY: debug
debug: export production=0
debug: build manifests
	skaffold debug --profile=$(PROFILE) --namespace=netapp-exporters --kube-context $(KUBECTL_CONTEXT)

.PHONY: clear
clear:
	kubectl delete -f $(OUT_DIR)/netappsd.yaml --ignore-not-found
	kubectl delete -f $(OUT_DIR)/master.yaml --ignore-not-found
	kubectl delete -f $(OUT_DIR)/worker.yaml --ignore-not-found
