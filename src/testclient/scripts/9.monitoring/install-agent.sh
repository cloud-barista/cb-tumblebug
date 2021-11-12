#!/bin/bash

#function install_agent() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## Install monitoring agent to MCIS "
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

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/monitoring/install/mcis/$MCISID -H 'Content-Type: application/json' -d \
		'{
			"command": "echo -n [CMD] Works! [Hostname: ; hostname ; echo -n ]"
		}' | jq '' #|| return 1
#}

#install_agent