FROM alpine:3.18
LABEL source_repository="https://github.com/sapcc/netappsd"

ENV OS=linux
ENV ARCH=amd64
ENV GOGC=100
WORKDIR /app

COPY _output/${ARCH}/netappsd netappsd
COPY _output/${ARCH}/netappsd-worker netappsd-worker
ENTRYPOINT [ "netappsd" ]