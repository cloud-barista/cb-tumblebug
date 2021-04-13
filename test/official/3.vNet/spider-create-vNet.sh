#!/bin/bash

#function create_vNet() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 3. vNet: Create"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

        resp=$(
            curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/vpc -H 'Content-Type: application/json' -d @- <<EOF
            {
                "ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
                "ReqInfo": {
                    "Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
                    "IPv4_CIDR": "192.168.0.0/16",
                    "SubnetInfoList": [ {
                        "Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
                        "IPv4_CIDR": "192.168.1.0/24"

                    } ]

                }
            }
EOF
    ); echo ${resp} | jq
    echo ""
#}

#create_vNet
