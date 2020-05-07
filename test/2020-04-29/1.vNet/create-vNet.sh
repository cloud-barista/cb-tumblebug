#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 1. VPC: Create"
echo "####################################################################"

INDEX=${1-"1"}

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d \
	'{
		"name": "VPC-0'$INDEX'",
		"connectionName": "'${CONN_CONFIG[INDEX]}'",
		"cidrBlock": "192.168.0.0/16",
		"subnetReqInfoList": [ {
			"Name": "Subnet-0'$INDEX'", 
			"IPv4_CIDR": "192.168.1.0/24"
		} ]
	}' | json_pp #|| return 1
