#!/bin/bash

#function command_mci() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## Command (SSH) to MCI "
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP


	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		# MCIPREFIX=avengers
		MCIID=${POSTFIX}
	fi

    LAUNCHCMD="sudo scope stop"
    
    echo "Launching Weavescope for the other nodes..."
    curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID -H 'Content-Type: application/json' -d @- <<EOF
    {
    "command"        : "[${LAUNCHCMD}]"
    }
EOF

#}

#command_mci