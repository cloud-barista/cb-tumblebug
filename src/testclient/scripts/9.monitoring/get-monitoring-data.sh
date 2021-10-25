#!/bin/bash

#function get_monitoring_data() {


	TestSetFile=${5:-../testSet.env}
    
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## Get monitoring data for MCIS (cpu/memory/disk/network)"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
	if [ "${INDEX}" == "0" ]; then
		# MCISPREFIX=avengers
		MCISID=${POSTFIX}
	fi

	USERCMD=${4}

	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/monitoring/mcis/$MCISID/metric/$USERCMD | jq '' #|| return 1
#}

#get_monitoring_data