##############################################################
## Stage 1 - Go Build
##############################################################

FROM golang:alpine AS builder

RUN apk update && apk add --no-cache bash

#RUN apk add gcc

ADD . /go/src/github.com/cloud-barista/cb-tumblebug

WORKDIR /go/src/github.com/cloud-barista/cb-tumblebug

WORKDIR src

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-w -extldflags "-static"' -tags cb-tumblebug -o cb-tumblebug -v

#############################################################
## Stage 2 - Application Setup
##############################################################

FROM ubuntu:latest

# use bash
RUN rm /bin/sh && ln -s /bin/bash /bin/sh

WORKDIR /app

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/conf/* /app/conf/

COPY --from=builder /go/src/github.com/cloud-barista/cb-tumblebug/src/cb-tumblebug /app/src/

#RUN /bin/bash -c "source /app/conf/setup.env"
ENV CBSTORE_ROOT /app
ENV CBLOG_ROOT /app
ENV SPIDER_URL http://cb-spider:1024

ENTRYPOINT [ "/app/src/cb-tumblebug" ]

EXPOSE 1323
