#!/bin/bash

#function lookup_spec_list() {

SECONDS=0

TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## 7. spec: Lookup Spec List"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

resp=$(
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/lookupSpecs -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX, $REGION]}"
		}
EOF
)
echo ${resp} | jq ''
echo ""

printElapsed $@

#lookup_spec_list
