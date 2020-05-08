#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 4. spec: Lookup Spec"
echo "####################################################################"

curl -sX GET http://localhost:1323/tumblebug/lookupSpec/${SPEC_NAME[INDEX]} -H 'Content-Type: application/json' -d \
	'{ 
		"connectionName": "'${CONN_CONFIG[INDEX]}'"
	}' | json_pp #|| return 1
