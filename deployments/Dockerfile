FROM alpine:3.18
LABEL source_repository="https://github.com/sapcc/netappsd"

RUN apk add --update curl && rm -rf /var/cache/apk/*

ENV OS=linux
ENV ARCH=amd64
ENV GOGC=100
WORKDIR /app

COPY netappsd netappsd
COPY netappsd-worker netappsd-worker
ENTRYPOINT [ "netappsd" ]
