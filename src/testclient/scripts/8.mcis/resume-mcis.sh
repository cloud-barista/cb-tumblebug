#!/bin/bash

#function resume_mcis() {

TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## 8. VM: Resume from suspended MCIS"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

ControlCmd=resume
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?action=${ControlCmd} | jq ''

#}

#resume_mcis
