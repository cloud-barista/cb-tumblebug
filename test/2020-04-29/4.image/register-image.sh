#!/bin/bash

source ../conf.env

INDEX=${1-"1"}

echo "####################################################################"
echo "## 4. image: Register"
echo "####################################################################"

curl -sX POST http://localhost:1323/tumblebug/ns/$NS_ID/resources/image?action=registerWithInfo -H 'Content-Type: application/json' -d \
	'{ 
		"connectionName": "'${CONN_CONFIG[INDEX]}'", 
		"name": "IMAGE-0'$INDEX'",
        "cspImageId": "'${IMAGE_NAME[INDEX]}'",
        "cspImageName": "",
        "creationDate": "",
        "description": "",
        "guestOS": "",
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
	}' | json_pp #|| return 1
