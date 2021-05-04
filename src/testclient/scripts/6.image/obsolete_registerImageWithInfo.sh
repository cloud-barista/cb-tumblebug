#!/bin/bash

#function registerImageWithInfo() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 6. image: Register"
    echo "####################################################################"

    CSP=${1}
    REGION=${2:-1}
    POSTFIX=${3:-developer}
    
	source ../common-functions.sh
	getCloudIndex $CSP

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/image?action=registerWithInfo -H 'Content-Type: application/json' -d @- <<EOF
        { 
            "connectionName": "${CONN_CONFIG[$INDEX,$REGION]}", 
            "name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
            "cspImageId": "${IMAGE_NAME[$INDEX,$REGION]}",
            "cspImageName": "",
            "creationDate": "",
            "description": "Canonical, Ubuntu, 18.04 LTS, amd64 bionic",
            "guestOS": "Ubuntu",
            "status": "",
            "keyValueList": [
                {
                    "Key": "",
                    "Value": ""
                },
                {
                    "Key": "",
                    "Value": ""
                }
            ]
        }
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#registerImageWithInfo