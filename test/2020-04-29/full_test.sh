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
#		'{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "vNet-01", "IPv4_CIDR": "192.168.0.0/16", "SubnetInfoList": [ { "Name": "Subnet-01", "IPv4_CIDR": "192.168.1.0/24"} ] } }' | json_pp || return 1

	echo This is the begin of the block comment
	: << 'END'

	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d '{ "connectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/vNet/VPC-01 -H 'Content-Type: application/json' -d '{ "connectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "#-----------------------------"

	echo "####################################################################"
	echo "## 2. SecurityGroup: Create -> List -> Get"
	echo "####################################################################"
	curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/securitygroup -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "SG-01", "VPCName": "VPC-01", "SecurityRules": [ {"FromPort": "1", "ToPort" : "65535", "IPProtocol" : "tcp", "Direction" : "inbound"} ] } }' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/securitygroup -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/securitygroup/SG-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "#-----------------------------"

	echo "####################################################################"
	echo "## 3. KeyPair: Create -> List -> Get"
	echo "####################################################################"
	curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/keypair -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "KEYPAIR-01" } }' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/keypair -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/keypair/KEYPAIR-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' | json_pp || return 1
	echo "#-----------------------------"

	echo "####################################################################"
	echo "## 4. VM: StartVM -> List -> Get -> ListStatus -> GetStatus -> Suspend -> Resume -> Reboot"
	echo "####################################################################"
	curl -sX POST http://localhost:1323/tumblebug/vm -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'", "ReqInfo": { "Name": "VM-01", "ImageName": "'${IMAGE_NAME}'", "VPCName": "VPC-01", "SubnetName": "Subnet-01", "SecurityGroupNames": [ "SG-01" ], "VMSpecName": "'${SPEC_NAME}'", "KeyPairName": "KEYPAIR-01"} }' | json_pp || return 1
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
