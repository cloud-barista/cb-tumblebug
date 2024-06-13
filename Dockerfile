##############################################################
## Stage 1 - Go Build
##############################################################

# Using a specific version of golang based on alpine for building the application
FROM golang:1.21.6-alpine AS builder

# Installing necessary packages
# sqlite-libs and sqlite-dev for SQLite support
# build-base for common build requirements
RUN apk add --no-cache sqlite-libs sqlite-dev build-base

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
FROM ubuntu:latest as prod

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
ENV CBTUMBLEBUG_ROOT=/app \
    CBSTORE_ROOT=/app \
    CBLOG_ROOT=/app \
    SPIDER_CALL_METHOD=REST \
    DRAGONFLY_CALL_METHOD=REST \
    SPIDER_REST_URL=http://cb-spider:1024/spider \
    DRAGONFLY_REST_URL=http://cb-dragonfly:9090/dragonfly \
    DB_URL=localhost:3306 \
    DB_DATABASE=cb_tumblebug \
    DB_USER=cb_tumblebug \
    DB_PASSWORD=cb_tumblebug \
    ALLOW_ORIGINS=* \
    AUTH_ENABLED=true \
    AUTH_MODE=basic \
    AUTH_JWT_SIGNING_METHOD=RS256 \
    AUTH_JWT_PUBLICKEY= \    
    API_USERNAME=default \
    API_PASSWORD=default \
    AUTOCONTROL_DURATION_MS=10000 \
    SELF_ENDPOINT=localhost:1323 \
    API_DOC_PATH=/app/src/api/rest/docs/swagger.json \
    DEFAULT_NAMESPACE=ns01 \
    DEFAULT_CREDENTIALHOLDER=admin \
    LOGFILE_PATH=$CBTUMBLEBUG_ROOT/log/tumblebug.log \
    LOGFILE_MAXSIZE=10 \
    LOGFILE_MAXBACKUPS=3 \
    LOGFILE_MAXAGE=30 \
    LOGFILE_COMPRESS=false \
    LOGLEVEL=debug \
    LOGWRITER=both \
    NODE_ENV=development

# Setting the entrypoint for the application
ENTRYPOINT [ "/app/src/cb-tumblebug" ]

# Exposing the port that the application will run on
EXPOSE 1323
