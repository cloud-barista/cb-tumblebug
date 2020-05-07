#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 4. spec: Register"
echo "####################################################################"

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/spec -H 'Content-Type: application/json' -d \
	'{ 
		"connectionName": "'${CONN_CONFIG[INDEX]}'", 
		"name": "SPEC-0'$INDEX'",
        "cspSpecName": "'${SPEC_NAME[INDEX]}'"
	}' | json_pp #|| return 1
