#!/bin/bash

#function create_vNet() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 3. vNet: Create"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	CIDRNum=$(($INDEX+1))
	CIDRDiff=$(($CIDRNum*$REGION))
	CIDRDiff=$(($CIDRDiff%254))

    resp=$(
			$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm network create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
			"{
				\"nsId\":  \"${NSID}\",
				\"vNet\": {
					\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
					\"connectionName\": \"${CONN_CONFIG[$INDEX,$REGION]}\",
					\"cidrBlock\": \"192.168.0.0/16\",
					\"subnetInfoList\": [ {
						\"Name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
						\"IPv4_CIDR\": \"192.168.${CIDRDiff}.0/24\"
					} ]
				}
			}"			
    ); echo ${resp} | jq ''
    echo ""
#}

#create_vNet