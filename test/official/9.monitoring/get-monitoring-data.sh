#!/bin/bash

#function get_monitoring_data() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## Get monitoring data for MCIS (cpu/memory/disk/network)"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
	USERCMD=${4}

	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/monitoring/mcis/$MCISID/metric/$USERCMD | json_pp #|| return 1
#}

#get_monitoring_data