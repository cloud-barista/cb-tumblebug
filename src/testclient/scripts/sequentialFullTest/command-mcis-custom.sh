#!/bin/bash

#function command_mcis_custom() {

TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## Command (SSH) to MCIS "
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

USERCMD=${5}

MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

VAR1=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${USERCMD}"
	} 
EOF
)
echo "${VAR1}" | jq ''

#}

#command_mcis_custom
