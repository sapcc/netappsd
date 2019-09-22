IMAGE_NAME:=hub.global.cloud.sap/monsoon/netappsd
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD | head -c 7)

IMAGE_TAG := ${BRANCH}-${HASH}

all: build docker

build: bin/netappsd

bin/netappsd: cmd/*.go *.go
	go build -o $@ cmd/main.go 

bin/netappsd-linux: cmd/*.go
	GOOS=linux go build -o $@ cmd/main.go

.Phony: clean docker

docker: Dockerfile bin/netappsd-linux
	docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .
	docker push ${IMAGE_NAME}:${IMAGE_TAG}

clean:
	rm -f bin/*
