#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 2. SecurityGroup: Create"
echo "####################################################################"

INDEX=${1-"1"}

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d \
	'{
		"name": "SG-0'$INDEX'",
		"connectionName": "'${CONN_CONFIG[INDEX]}'",
		"vNetId": "VPC-0'$INDEX'",
		"description": "jhseo test description",
		    "firewallRules": [
			    {
				    "FromPort": "1",
				    "ToPort": "65535",
				    "IPProtocol": "tcp",
				    "Direction": "inbound"
			    }
		    ]
	    }' | json_pp #|| return 1
