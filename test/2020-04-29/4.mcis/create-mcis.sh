#!/bin/bash

source ../conf.env

echo "## 4. VM: Create MCIS"
curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d \
	'{
		"name": "MCIS-01",
		"vm_num": "1",
		"description": "Tumblebug demo",
		"vm_req": [ {
			"image_id": "IMAGE-01",
			"vm_access_id": "cb-user",
			"config_name": "aws-us-east-1-config",
			"ssh_key_id": "KEYPAIR-01",
			"spec_id": "SPEC-01",
			"security_group_ids": [
				"SG-01"
			],
			"vnet_id": "VPC-01",
			"subnet_id": "Subnet-01",
			"description": "description",
			"vm_access_passwd": "",
			"name": "VM-01"
		} ]
	}' | json_pp || return 1

