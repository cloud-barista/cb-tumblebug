#!/bin/bash

#function create_securityGroup() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 4. SecurityGroup: Create"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

    resp=$(
			$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm securitygroup create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
				"{
					\"nsId\":  \"${NSID}\",
					\"securityGroup\": {
						\"name\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
						\"connectionName\": \"${CONN_CONFIG[$INDEX,$REGION]}\",
						\"vNetId\": \"${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\",
						\"description\": \"test description\",
						\"firewallRules\": [
								{
									\"FromPort\": \"1\",
									\"ToPort\": \"65535\",
									\"IPProtocol\": \"tcp\",
									\"Direction\": \"inbound\"
								},
								{
									\"FromPort\": \"1\",
									\"ToPort\": \"65535\",
									\"IPProtocol\": \"udp\",
									\"Direction\": \"inbound\"
								},
								{
									\"FromPort\": \"-1\",
									\"ToPort\": \"-1\",
									\"IPProtocol\": \"icmp\",
									\"Direction\": \"inbound\"
								}
							]
					}
				}"		
    ); echo ${resp} | jq ''
    echo ""
#}

#create_securityGroup