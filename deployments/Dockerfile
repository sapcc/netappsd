FROM alpine:3.18
LABEL source_repository="https://github.com/sapcc/netappsd"

ENV GOGC=100
RUN apk add --update curl yq && rm -rf /var/cache/apk/*

COPY netappsd /netappsd
ENTRYPOINT [ "/netappsd" ]
