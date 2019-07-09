all: build

build: bin/netappsd

bin/netappsd: cmd/main.go
	go build -o $@ cmd/main.go 

.Phony: clean

clean:
	rm -f bin/*
