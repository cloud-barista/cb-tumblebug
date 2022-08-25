##############################################################
## Stage 1 - Go Build
##############################################################

FROM golang:1.19-alpine AS builder

#RUN apk update && apk add --no-cache bash

#RUN apk add gcc

RUN apk add --no-cache sqlite-libs sqlite-dev

RUN apk add --no-cache build-base

ADD . /go/src/github.com/cloud-barista/cb-tumblebug

WORKDIR /go/src/github.com/cloud-barista/cb-tumblebug

WORKDIR src

RUN go build -mod=mod -ldflags '-w -extldflags "-static"' -tags cb-tumblebug -o cb-tumblebug -v

#############################################################
## Stage 2 - Application Setup
##############################################################

FROM ubuntu:latest as prod

# use bash
RUN rm /bin/sh && ln -s /bin/bash /bin/sh

WORKDIR /app/src

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/assets/ /app/assets/

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/scripts/ /app/scripts/

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/conf/ /app/conf/

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/src/cb-tumblebug /app/src/

#RUN /bin/bash -c "source /app/conf/setup.env"
ENV CBTUMBLEBUG_ROOT /app
ENV CBSTORE_ROOT /app
ENV CBLOG_ROOT /app
ENV SPIDER_CALL_METHOD REST
ENV DRAGONFLY_CALL_METHOD REST
ENV SPIDER_REST_URL http://cb-spider:1024/spider
ENV DRAGONFLY_REST_URL http://cb-dragonfly:9090/dragonfly

ENV DB_URL localhost:3306
ENV DB_DATABASE cb_tumblebug
ENV DB_USER cb_tumblebug
ENV DB_PASSWORD cb_tumblebug

ENV API_USERNAME default
ENV API_PASSWORD default

# Set period for auto control goroutine invocation
ENV AUTOCONTROL_DURATION_MS 10000

# Set SELF_ENDPOINT, if you want to access Swagger API dashboard from outside. (Ex: export SELF_ENDPOINT=xxx.xxx.xxx.xxx:1323)
ENV SELF_ENDPOINT localhost:1323


# Environment variables that you don't need to touch

# Ignore a protocol buffer namespace conflict 
ENV GOLANG_PROTOBUF_REGISTRATION_CONFLICT ignore

# Swagger UI API document file path 
ENV API_DOC_PATH /app/src/api/rest/docs/swagger.json

ENTRYPOINT [ "/app/src/cb-tumblebug" ]

EXPOSE 1323
EXPOSE 50252
