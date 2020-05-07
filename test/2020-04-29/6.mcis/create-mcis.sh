#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 4. VM: Create MCIS"
echo "####################################################################"

INDEX=${1}

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d \
	'{
		"name": "MCIS-0'$INDEX'",
		"vm_num": "1",
		"description": "Tumblebug demo",
		"vm_req": [ {
			"image_id": "IMAGE-0'$INDEX'",
			"vm_access_id": "cb-user",
			"config_name": "'${CONN_CONFIG[INDEX]}'",
			"ssh_key_id": "KEYPAIR-0'$INDEX'",
			"spec_id": "SPEC-0'$INDEX'",
			"security_group_ids": [
				"SG-0'$INDEX'"
			],
			"vnet_id": "VPC-0'$INDEX'",
			"subnet_id": "Subnet-0'$INDEX'",
			"description": "description",
			"vm_access_passwd": "",
			"name": "VM-0'$INDEX'"
		} ]
	}' | json_pp || return 1

