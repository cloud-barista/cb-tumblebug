#!/bin/bash

source ../conf.env

echo "## 3. KeyPair: Create"
curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d \
	'{ 
		"connectionName": "'${CONN_CONFIG}'", 
		"name": "KEYPAIR-01" 
	}' | json_pp #|| return 1
