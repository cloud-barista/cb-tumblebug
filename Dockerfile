##############################################################
## Stage 1 - Go Build
##############################################################

# Using a specific version of golang based on bookworm for building the application
# Debian "bookworm" is the current stable release (checked on 2024-07-15)
# Debian 12.6 was released on June 29th, 2024.
FROM golang:1.21.6-bookworm AS builder

# Installing necessary packages
# sqlite3 and libsqlite3-dev for SQLite support
# build-essential for common build requirements
RUN apt-get update && apt-get install -y sqlite3 libsqlite3-dev build-essential

# Copying only necessary files for the build
WORKDIR /go/src/github.com/cloud-barista/cb-tumblebug
COPY go.mod go.sum go.work go.work.sum ./
RUN go mod download
COPY src ./src
COPY assets ./assets
COPY scripts ./scripts
COPY conf ./conf

# Building the Go application with specific flags
RUN go build -ldflags '-w -extldflags "-static"' -tags cb-tumblebug -v -o src/cb-tumblebug src/main.go

#############################################################
## Stage 2 - Application Setup
##############################################################

# Using the latest Ubuntu image for the production stage
FROM ubuntu:latest AS prod

# Setting the working directory for the application
WORKDIR /app/src

# Copying necessary files from the builder stage to the production stage
# Assets, scripts, and configuration files are copied excluding credentials.conf
# which should be specified in .dockerignore
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/assets/ /app/assets/
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/scripts/ /app/scripts/
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/conf/ /app/conf/
COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/src/cb-tumblebug /app/src/

# Setting various environment variables required by the application
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

# Setting the entrypoint for the application
ENTRYPOINT [ "/app/src/cb-tumblebug" ]

# Exposing the port that the application will run on
EXPOSE 1323
