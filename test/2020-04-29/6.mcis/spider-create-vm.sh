#!/bin/bash

source ../conf.env

INDEX=${1-"1"}

curl -sX POST http://localhost:1024/spider/vm -H 'Content-Type: application/json' -d \
	'{ 
		"ConnectionName": "'${CONN_CONFIG[INDEX]}'", 
		"ReqInfo": { 
			"Name": "VM-0'$INDEX'", 
			"ImageName": "'${IMAGE_NAME[INDEX]}'", 
			"VPCName": "VPC-0'$INDEX'", 
			"SubnetName": "Subnet-0'$INDEX'", 
			"SecurityGroupNames": [ "SG-0'$INDEX'" ], 
			"VMSpecName": "'${SPEC_NAME[INDEX]}'", 
			"KeyPairName": "KEYPAIR-0'$INDEX'"
		} 
	}' | json_pp
