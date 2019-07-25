FROM alpine:3.5

COPY bin/netappsd-linux /usr/local/bin/netappsd

ENTRYPOINT [ "netappsd" ]
