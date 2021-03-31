#!/bin/bash

#function unregister_cloud() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	source ../credentials.conf
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 1. Remove All Cloud Connction Config(s)"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	OPTION=${4:-none}

	RESTSERVER=localhost


	if [ "${OPTION}" == "leave" ]; then
		echo "[Leave Cloud Credential and Cloud Driver for other Regions]"
		exit
	fi
	
	# for Cloud Connection Config Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/connectionconfig/${CONN_CONFIG[$INDEX,$REGION]}

	# for Cloud Region Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/region/${RegionName[$INDEX,$REGION]}

	# for Cloud Credential Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/credential/${CredentialName[INDEX]}
	
	# for Cloud Driver Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/driver/${DriverName[INDEX]}
#}

#unregister_cloud
