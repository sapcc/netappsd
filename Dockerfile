FROM alpine:3.5
LABEL source_repository="https://github.com/sapcc/netappsd"

COPY bin/netappsd-linux /usr/local/bin/netappsd

ENTRYPOINT [ "netappsd" ]
