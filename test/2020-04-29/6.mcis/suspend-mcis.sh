#!/bin/bash

source ../conf.env

INDEX=${1-"1"}

curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/mcis/MCIS-0$INDEX?action=suspend -H 'Content-Type: application/json' -d \
    '{ 
        "ConnectionName": "'${CONN_CONFIG[INDEX]}'"
    }' | json_pp
