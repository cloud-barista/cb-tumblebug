#!/bin/bash

#function fetch_specs() {

SECONDS=0

TestSetFile=${4:-../testSet.env}

FILE=$TestSetFile
if [ ! -f "$FILE" ]; then
	echo "$FILE does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## 7. spec: Fetch"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

if [ "$CSP" == '' ]; then #|| [ "$CSP" == "all" ]
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/fetchSpecs | jq '' #|| return 1

else
	source ../common-functions.sh
	getCloudIndex $CSP


	resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/fetchSpecs -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
			}
EOF
	); echo ${resp} | jq ''
	echo ""

fi

#}

source ../common-functions.sh
printElapsed $@
#fetch_specs

