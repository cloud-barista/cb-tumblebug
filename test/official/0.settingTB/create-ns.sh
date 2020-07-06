#!/bin/bash

source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## 0. Namespace: Create"
echo "####################################################################"

INDEX=${1}

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns -H 'Content-Type: application/json' -d \
	'{
		"name": "'$NS_ID'",
		"description": "NameSpace for General Testing"
	}' | json_pp #|| return 1
