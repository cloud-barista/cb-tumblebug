#!/bin/bash

function full_test() {
	echo "####################################################################"
	echo "## Full Test Scripts for CB-tumblebug IID Working Version - 2020.04.22."
	echo "##   1. VPC: Create -> List -> Get"
	echo "##   2. SecurityGroup: Create -> List -> Get"
	echo "##   3. KeyPair: Create -> List -> Get"
	echo "##   4. VM: StartVM -> List -> Get -> ListStatus -> GetStatus -> Suspend -> Resume -> Reboot"
	echo "## ---------------------------------"
	echo "##   4. VM: Terminate(Delete)"
	echo "##   3. KeyPair: Delete"
	echo "##   2. SecurityGroup: Delete"
	echo "##   1. VPC: Delete"
	echo "####################################################################"

	echo "####################################################################"
	echo "## 1. VPC: Create -> List -> Get"
	echo "####################################################################"

#	echo "## 1. VPC: Delete"
#	curl -sX DELETE http://localhost:1024/spider/vpc/VPC-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1

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
		}' | json_pp || return 1

#	echo "## 1. VPC: List"
#	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d \
#		'{ 
#			"connectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "## 1. VPC: Get"
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet/VPC-01 | json_pp || return 1 #-H 'Content-Type: application/json' -d \
#		'{ 
#			"connectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "#-----------------------------"

	echo This is the begin of the block comment
	: << 'END'

	echo "####################################################################"
	echo "## 2. SecurityGroup: Create -> List -> Get"
	echo "####################################################################"

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
		    }' || return 1
#		    }' | json_pp || return 1
		
#		'{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "SG-01", "VPCName": "VPC-01", "SecurityRules": [ {"FromPort": "1", "ToPort" : "65535", "IPProtocol" : "tcp", "Direction" : "inbound"} ] } }' | json_pp || return 1

#	echo "## 2. SecurityGroup: List"
#	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d \
#		'{ 
#			"ConnectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "## 2. SecurityGroup: Get"
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup/SG-01 | json_pp || return 1 #-H 'Content-Type: application/json' -d \
#		'{ 
#			"ConnectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "#-----------------------------"

	echo "####################################################################"
	echo "## 3. KeyPair: Create -> List -> Get"
	echo "####################################################################"

	echo "## 3. KeyPair: Create"
	curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d \
		'{ 
			"connectionName": "'${CONN_CONFIG}'", 
			"name": "KEYPAIR-01" 
		}' || return 1

#	echo "## 3. KeyPair: Create"
#	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1

	echo "## 3. KeyPair: Get"
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey/KEYPAIR-01 | json_pp || return 1 #-H 'Content-Type: application/json' -d \
#		'{ 
#			"connectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "#-----------------------------"

	echo "####################################################################"
	echo "## 4. VM: StartVM -> List -> Get -> ListStatus -> GetStatus -> Suspend -> Resume -> Reboot"
	echo "####################################################################"

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
				"description": "description",
				"vm_access_passwd": "",
				"name": "VM-01"
			} ]
		}' | json_pp || return 1
#		'{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "VM-01", "ImageName": "'${IMAGE_NAME}'", "VPCName": "VPC-01", "SubnetName": "Subnet-01", "SecurityGroupNames": [ "SG-01" ], "VMSpecName": "'${SPEC_NAME}'", "KeyPairName": "KEYPAIR-01"} }' | json_pp || return 1

	echo "============== sleep 30 after start VM"
	sleep 30
	curl -sX GET http://localhost:1323/tumblebug/vm -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/vm/VM-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/vmstatus -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/vmstatus/VM-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/controlvm/VM-01?action=suspend -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 50 after suspend VM"
	sleep 50
	curl -sX GET http://localhost:1323/tumblebug/controlvm/VM-01?action=resume -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 30 after resume VM"
	sleep 30
	curl -sX GET http://localhost:1323/tumblebug/controlvm/VM-01?action=reboot -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 60 after reboot VM"
	sleep 60 
	echo "#-----------------------------"


	echo "####################################################################"
	echo "####################################################################"
	echo "####################################################################"

	echo "####################################################################"
	echo "## 4. VM: Terminate(Delete)"
	echo "####################################################################"
	curl -sX DELETE http://localhost:1323/tumblebug/vm/VM-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 60 after delete VM"
	sleep 60 

	echo "####################################################################"
	echo "## 3. KeyPair: Delete"
	echo "####################################################################"
	curl -sX DELETE http://localhost:1323/tumblebug/keypair/KEYPAIR-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "####################################################################"
	echo "## 2. SecurityGroup: Delete"
	echo "####################################################################"
	curl -sX DELETE http://localhost:1323/tumblebug/securitygroup/SG-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "####################################################################"
	echo "## 1. VPC: Delete"
	echo "####################################################################"
	curl -sX DELETE http://localhost:1323/tumblebug/vNet/VPC-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1

END
	echo This is the end of the block comment
}

full_test
