#!/bin/bash

#function command_infra() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## Command (SSH) to Infra "
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP


	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		# InfraPREFIX=avengers
		InfraID=${POSTFIX}
	fi

    LAUNCHCMD="sudo scope stop"
    
    echo "Launching Weavescope for the other nodes..."
    curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID -H 'Content-Type: application/json' -d @- <<EOF
    {
    "command"        : "[${LAUNCHCMD}]"
    }
EOF

#}

#command_infra