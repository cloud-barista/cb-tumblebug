# syntax=docker/dockerfile:1.4
##############################################################
## Stage 1 - Go Build
##############################################################

FROM golang:1.21.6-bookworm AS builder

ENV GO111MODULE=on

WORKDIR /go/src/github.com/cloud-barista/cb-tumblebug

# Cache dependencies
COPY go.mod go.sum go.work go.work.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copying the source files to the container
COPY src ./src
COPY assets ./assets
COPY scripts ./scripts
COPY conf ./conf

# Building the Go application
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags '-w -s' -tags cb-tumblebug -v -o src/cb-tumblebug src/main.go

#############################################################
## Stage 2 - Application Setup
##############################################################

FROM ubuntu:22.04 AS prod

WORKDIR /app/src

# Installing necessary packages and cleaning up
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Copying necessary files from the builder stage to the production stage
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/assets/ /app/assets/
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/scripts/ /app/scripts/
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/conf/ /app/conf/
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/src/cb-tumblebug /app/src/

# Setting environment variables
ENV TB_ROOT_PATH=/app \
    TB_SPIDER_REST_URL=http://cb-spider:1024/spider \
    TB_DRAGONFLY_REST_URL=http://cb-dragonfly:9090/dragonfly \
    TB_SQLITE_URL=localhost:3306 \
    TB_SQLITE_DATABASE=cb_tumblebug \
    TB_SQLITE_USER=cb_tumblebug \
    TB_SQLITE_PASSWORD=cb_tumblebug \
    TB_ETCD_ENDPOINTS=http://etcd:2379 \
    TB_ETCD_AUTH_ENABLED=true \
    TB_ETCD_USERNAME=default \
    TB_ETCD_PASSWORD=default \
    TB_ALLOW_ORIGINS=* \
    TB_AUTH_ENABLED=true \
    TB_AUTH_MODE=basic \
    TB_AUTH_JWT_SIGNING_METHOD=RS256 \
    TB_AUTH_JWT_PUBLICKEY= \    
    TB_API_USERNAME=default \
    TB_API_PASSWORD=default \
    TB_AUTOCONTROL_DURATION_MS=10000 \
    TB_SELF_ENDPOINT=localhost:1323 \
    TB_DEFAULT_NAMESPACE=ns01 \
    TB_DEFAULT_CREDENTIALHOLDER=admin \
    TB_LOGFILE_PATH=/app/log/tumblebug.log \
    TB_LOGFILE_MAXSIZE=10 \
    TB_LOGFILE_MAXBACKUPS=3 \
    TB_LOGFILE_MAXAGE=30 \
    TB_LOGFILE_COMPRESS=false \
    TB_LOGLEVEL=debug \
    TB_LOGWRITER=both \
    TB_NODE_ENV=development

ENTRYPOINT [ "/app/src/cb-tumblebug" ]

EXPOSE 1323
