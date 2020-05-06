#!/bin/bash

source ../conf.env

echo "## 1. VPC: Create"
curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d \
	'{
		"name": "VPC-01",
		"connectionName": "'${CONN_CONFIG}'",
		"cspVNetName": "VPC-01",
		"cidrBlock": "192.168.0.0/16",
		"subnetInfoList": [ {
			"Name": "Subnet-01", 
			"IPv4_CIDR": "192.168.1.0/24"
		} ]
	}' | json_pp #|| return 1
