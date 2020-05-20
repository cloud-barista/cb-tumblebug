#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 2. SecurityGroup: Create"
echo "####################################################################"

CSP=${1}
POSTFIX=${2:-developer}
if [ "${CSP}" == "aws" ]; then
	echo "[Test for AWS]"
	INDEX=1
elif [ "${CSP}" == "azure" ]; then
	echo "[Test for Azure]"
	INDEX=2
elif [ "${CSP}" == "gcp" ]; then
	echo "[Test for GCP]"
	INDEX=3
elif [ "${CSP}" == "alibaba" ]; then
	echo "[Test for Alibaba]"
	INDEX=4
else
	echo "[No acceptable argument was provided (aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
	CSP="aws"
	INDEX=1
fi

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d \
	'{
		"name": "sg-'$CSP'-'$POSTFIX'",
		"connectionName": "'${CONN_CONFIG[INDEX]}'",
		"vNetId": "vpc-'$CSP'-'$POSTFIX'",
		"description": "jhseo test description",
		    "firewallRules": [
			    {
				    "FromPort": "1",
				    "ToPort": "65535",
				    "IPProtocol": "tcp",
				    "Direction": "inbound"
			    },
				{
				    "FromPort": "-1",
				    "ToPort": "-1",
				    "IPProtocol": "icmp",
				    "Direction": "inbound"
			    }
		    ]
	    }' | json_pp #|| return 1
