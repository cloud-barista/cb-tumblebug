#!/bin/bash

source ../conf.env

echo "## 2. SecurityGroup: Create"
curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d \
	'{
		"name": "SG-01",
		"connectionName": "'${CONN_CONFIG}'",
		"vNetId": "VPC-01",
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
