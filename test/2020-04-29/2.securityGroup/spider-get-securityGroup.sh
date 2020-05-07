#!/bin/bash

source ../conf.env

INDEX=${1}

curl -sX GET http://localhost:1024/spider/securitygroup/SG-0$INDEX -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG[INDEX]}'"}' | json_pp
