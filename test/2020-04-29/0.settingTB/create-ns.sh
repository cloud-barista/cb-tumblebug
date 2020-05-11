#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Namespace: Create"
echo "####################################################################"

INDEX=${1}

curl -sX POST http://localhost:1323/tumblebug/ns -H 'Content-Type: application/json' -d \
	'{
		"name": "'$NS_ID'",
		"description": "NameSpace for General Testing"
	}' | json_pp #|| return 1
