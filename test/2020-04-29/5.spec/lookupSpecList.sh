#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 4. spec: Lookup Spec List"
echo "####################################################################"

curl -sX GET http://localhost:1323/tumblebug/lookupSpec -H 'Content-Type: application/json' -d \
	'{ 
		"connectionName": "'${CONN_CONFIG[INDEX]}'"
	}' | json_pp #|| return 1
