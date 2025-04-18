# stage 1
# --------
FROM golang:1.21-alpine3.18 as builder

ENV DOCKER_BUILDKIT=1
ENV GOGC=100

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY cmd/ cmd/
COPY internal/ internal/
RUN go build -v -o /netappsd ./cmd

# stage 2
# --------

FROM alpine:3.18
LABEL source_repository="https://github.com/sapcc/netappsd.git"
COPY --from=builder netappsd /
ENTRYPOINT [ "/netappsd" ]
