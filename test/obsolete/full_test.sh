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
#	curl -H "${AUTH}" -sX DELETE http://localhost:1024/spider/vpc/vpc-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1

	echo "## 1. VPC: Create"
	curl -H "${AUTH}" -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d \
		'{
			"name": "vpc-01",
			"connectionName": "'${CONN_CONFIG}'",
			"cspVNetName": "vpc-01",
			"cidrBlock": "192.168.0.0/16",
			"subnetInfoList": [ {
				"Name": "subnet-01", 
				"IPv4_CIDR": "192.168.1.0/24"
			} ]
		}' | json_pp || return 1

#	echo "## 1. VPC: List"
#	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d \
#		'{ 
#			"connectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "## 1. VPC: Get"
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet/vpc-01 | json_pp || return 1 #-H 'Content-Type: application/json' -d \
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
	curl -H "${AUTH}" -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d \
		'{
			"name": "sg-01",
			"connectionName": "'${CONN_CONFIG}'",
			"vNetId": "vpc-01",
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
		
#		'{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "sg-01", "VPCName": "vpc-01", "SecurityRules": [ {"FromPort": "1", "ToPort" : "65535", "IPProtocol" : "tcp", "Direction" : "inbound"} ] } }' | json_pp || return 1

#	echo "## 2. SecurityGroup: List"
#	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d \
#		'{ 
#			"ConnectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "## 2. SecurityGroup: Get"
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/securityGroup/sg-01 | json_pp || return 1 #-H 'Content-Type: application/json' -d \
#		'{ 
#			"ConnectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "#-----------------------------"

	echo "####################################################################"
	echo "## 3. KeyPair: Create -> List -> Get"
	echo "####################################################################"

	echo "## 3. KeyPair: Create"
	curl -H "${AUTH}" -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d \
		'{ 
			"connectionName": "'${CONN_CONFIG}'", 
			"name": "keypair-01" 
		}' || return 1

#	echo "## 3. KeyPair: Create"
#	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1

	echo "## 3. KeyPair: Get"
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/sshKey/keypair-01 | json_pp || return 1 #-H 'Content-Type: application/json' -d \
#		'{ 
#			"connectionName": "'${CONN_CONFIG}'"
#		}' | json_pp || return 1

	echo "#-----------------------------"

	echo "####################################################################"
	echo "## 4. VM: StartVM -> List -> Get -> ListStatus -> GetStatus -> Suspend -> Resume -> Reboot"
	echo "####################################################################"

	echo "## 4. VM: Create MCIS"
	curl -H "${AUTH}" -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d \
		'{
			"name": "mcis-01",
			"vm_num": "1",
			"description": "Tumblebug demo",
			"vm_req": [ {
				"imageId": "image-01",
				"vmUserAccount": "cb-user",
				"connectionName": "aws-us-east-1-config",
				"sshKeyId": "keypair-01",
				"specId": "spec-01",
				"securityGroupIds": [
					"sg-01"
				],
				"vNetId": "vpc-01",
				"description": "description",
				"vmUserPassword": "",
				"name": "vm-01"
			} ]
		}' | json_pp || return 1
#		'{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "vm-01", "ImageName": "'${IMAGE_NAME}'", "VPCName": "vpc-01", "SubnetName": "subnet-01", "SecurityGroupNames": [ "sg-01" ], "VMSpecName": "'${SPEC_NAME}'", "KeyPairName": "keypair-01"} }' | json_pp || return 1

	echo "============== sleep 30 after start VM"
	sleep 30
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/vm -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/vm/vm-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/vmstatus -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/vmstatus/vm-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/controlvm/vm-01?action=suspend -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 50 after suspend VM"
	sleep 50
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/controlvm/vm-01?action=resume -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 30 after resume VM"
	sleep 30
	curl -H "${AUTH}" -sX GET http://localhost:1323/tumblebug/controlvm/vm-01?action=reboot -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 60 after reboot VM"
	sleep 60 
	echo "#-----------------------------"


	echo "####################################################################"
	echo "####################################################################"
	echo "####################################################################"

	echo "####################################################################"
	echo "## 4. VM: Terminate(Delete)"
	echo "####################################################################"
	curl -H "${AUTH}" -sX DELETE http://localhost:1323/tumblebug/vm/vm-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "============== sleep 60 after delete VM"
	sleep 60 

	echo "####################################################################"
	echo "## 3. KeyPair: Delete"
	echo "####################################################################"
	curl -H "${AUTH}" -sX DELETE http://localhost:1323/tumblebug/keypair/keypair-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "####################################################################"
	echo "## 2. SecurityGroup: Delete"
	echo "####################################################################"
	curl -H "${AUTH}" -sX DELETE http://localhost:1323/tumblebug/securitygroup/sg-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "####################################################################"
	echo "## 1. VPC: Delete"
	echo "####################################################################"
	curl -H "${AUTH}" -sX DELETE http://localhost:1323/tumblebug/vNet/vpc-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1

END
	echo This is the end of the block comment
}

full_test
