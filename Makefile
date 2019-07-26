IMAGE_NAME=hub.global.cloud.sap/monsoon/netappsd
IMAGE_TAG=v0.1.1

all: build

build: bin/netappsd

bin/netappsd: cmd/*.go
	go build -o $@ cmd/main.go 

bin/netappsd-linux: cmd/*.go
	GOOS=linux go build -o $@ cmd/main.go

.Phony: clean docker

docker: Dockerfile bin/netappsd-linux
	docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .
	docker push ${IMAGE_NAME}:${IMAGE_TAG}

clean:
	rm -f bin/*

