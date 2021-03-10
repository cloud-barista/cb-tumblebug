##############################################################
## Stage 1 - Go Build
##############################################################

FROM golang:1.14.5-alpine3.12 AS builder

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

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/assets/* /app/assets/

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/conf/* /app/conf/

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/src/cb-tumblebug /app/src/

#RUN /bin/bash -c "source /app/conf/setup.env"
ENV CBSTORE_ROOT /app
ENV CBLOG_ROOT /app
ENV CBTUMBLEBUG_ROOT /app
ENV SPIDER_CALL_METHOD REST
ENV SPIDER_REST_URL http://cb-spider:1024/spider
ENV DRAGONFLY_REST_URL http://cb-dragonfly:9090/dragonfly

ENV DB_URL localhost:3306
ENV DB_DATABASE cb_tumblebug
ENV DB_USER cb_tumblebug
ENV DB_PASSWORD cb_tumblebug

ENV API_USERNAME default
ENV API_PASSWORD default

ENTRYPOINT [ "/app/src/cb-tumblebug" ]

EXPOSE 1323
EXPOSE 50252
