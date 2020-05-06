#!/bin/bash

source ../conf.env

curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/mcis/MCIS-01?action=suspend -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' |json_pp
