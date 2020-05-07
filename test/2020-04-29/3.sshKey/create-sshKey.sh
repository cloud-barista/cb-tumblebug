#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 3. sshKey: Create"
echo "####################################################################"

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d \
	'{ 
		"connectionName": "'${CONN_CONFIG[INDEX]}'", 
		"name": "KEYPAIR-0'$INDEX'" 
	}' | json_pp #|| return 1
