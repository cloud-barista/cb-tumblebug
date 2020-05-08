#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 4. spec: Fetch"
echo "####################################################################"

curl -sX GET http://localhost:1024/spider/vmspec -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG[INDEX]}'" }' | json_pp

