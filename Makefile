ifeq ($(KUBECTL_CONTEXT),)
$(error KUBECTL_CONTEXT is not set)
endif

OS?=linux
ARCH?=amd64
PROFILE?=master,worker

GOFILES:=$(shell find . -name '*.go' -not -path "./vendor/*")
GOPATH:=$(shell go env GOPATH)
OUT_DIR?=_output
TEMP_DIR:=$(shell mktemp -d)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD | head -c 7)

IMAGE_NAME:=keppel.eu-de-1.cloud.sap/ccloud/netappsd-$(ARCH)
IMAGE_TAG?=latest

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
	skaffold delete --profile=$(PROFILE) --namespace=netapp-exporters --kube-context $(KUBECTL_CONTEXT)

# export debug=0
# export enable_master=1
# export enable_worker=1
#
# .Phony: debug debug-master debug-worker
#
# debug: export debug=1
# debug: 
# 	$(call generate_manifests,$(OUT_DIR)/manifests.yaml)
# 	skaffold debug --profile=debug --namespace=netapp-exporters --kube-context $(KUBECTL_CONTEXT)

# debug-master: export enable_worker=0
# debug-master: debug
#
# debug-worker: export enable_master=0
# debug-worker: debug
#
# # Deploy
# # ------
# # TODO: use skaffold to deploy to k8s
#
# .Phony: manifests.yaml deploy delete-k8s
#
#
# deploy: $(OUT_DIR)/manifests.yaml build-container
# 	kubectl apply -f $(OUT_DIR)/manifests.yaml
#
# delete-k8s: $(OUT_DIR)/manifests.yaml
# 	kubectl delete -f $(OUT_DIR)/manifests.yaml
